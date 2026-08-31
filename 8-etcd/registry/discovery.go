package registry

import (
	"context"
	"fmt"
	"log"
	"sort"
	"sync"
	"time"

	"go.etcd.io/etcd/api/v3/mvccpb"
	clientv3 "go.etcd.io/etcd/client/v3"
)

// ChangeFunc 是实例列表变更回调，参数为变更后的全量实例快照。
type ChangeFunc func(instances []ServiceInstance)

// Discovery 负责发现某个服务的全部实例，并在本地维护一份实时缓存。
//
// 核心机制（面试高频）：
//  1. 全量 + 增量：先 Get(prefix) 拿到全量快照，再 Watch 增量事件。
//  2. Revision 衔接：Get 的响应头里带 Revision，Watch 必须从 Revision+1 开始。
//     如果直接 Watch(当前时刻)，Get 和 Watch 之间那个时间窗内发生的变更就丢了，
//     会出现「实例已经下线但消费端列表里还挂着」的脏数据。这是最经典的考点。
//  3. 本地缓存：读请求走内存，不打 etcd。etcd 是 CP 系统且承载能力有限，
//     绝不能每次调用 RPC 前都去 Get 一次。
//     副作用是 etcd 全挂时消费端仍能用上一份缓存降级运行（可用性兜底）。
//  4. Watch 重建：Watch 通道可能因为网络断开或 revision 被 compact 而关闭，
//     此时要重新做一次全量同步，拿到新的 revision 再续上 Watch。
type Discovery struct {
	cli    *clientv3.Client
	name   string
	prefix string

	mu        sync.RWMutex
	instances map[string]ServiceInstance // etcd key -> 实例

	listenerMu sync.Mutex
	listeners  []ChangeFunc

	ctx       context.Context
	cancel    context.CancelFunc
	wg        sync.WaitGroup
	closeOnce sync.Once
}

// NewDiscovery 创建针对 serviceName 的服务发现器。
func NewDiscovery(cli *clientv3.Client, serviceName string) *Discovery {
	ctx, cancel := context.WithCancel(context.Background())
	return &Discovery{
		cli:       cli,
		name:      serviceName,
		prefix:    ServicePrefix(serviceName),
		instances: make(map[string]ServiceInstance),
		ctx:       ctx,
		cancel:    cancel,
	}
}

// Start 先做一次全量同步，然后启动后台 Watch 协程。
// 返回后即可通过 GetInstances 读到当前实例列表。
func (d *Discovery) Start(ctx context.Context) error {
	rev, err := d.fullSync(ctx)
	if err != nil {
		return err
	}

	d.wg.Add(1)
	go d.watchLoop(rev + 1)
	return nil
}

// fullSync 拉取全量实例快照，返回本次快照对应的 etcd revision。
func (d *Discovery) fullSync(ctx context.Context) (int64, error) {
	resp, err := d.cli.Get(ctx, d.prefix, clientv3.WithPrefix())
	if err != nil {
		return 0, fmt.Errorf("discovery: get prefix %s: %w", d.prefix, err)
	}

	snapshot := make(map[string]ServiceInstance, len(resp.Kvs))
	for _, kv := range resp.Kvs {
		inst, err := UnmarshalInstance(kv.Value)
		if err != nil {
			// 单个脏数据不应该拖垮整个服务列表，跳过并告警即可
			log.Printf("[discovery] 跳过非法实例数据 key=%s: %v", kv.Key, err)
			continue
		}
		snapshot[string(kv.Key)] = inst
	}

	d.mu.Lock()
	d.instances = snapshot
	d.mu.Unlock()

	log.Printf("[discovery] 全量同步 service=%s 实例数=%d revision=%d",
		d.name, len(snapshot), resp.Header.Revision)
	d.notify()
	return resp.Header.Revision, nil
}

// watchLoop 从指定 revision 开始监听前缀下的增量变更，断开后自动重建。
func (d *Discovery) watchLoop(rev int64) {
	defer d.wg.Done()

	for {
		if d.ctx.Err() != nil {
			return
		}

		// WithPrefix：监听整个服务前缀下的所有实例
		// WithRev：从指定 revision 续上，保证不丢事件
		// WithPrevKV：DELETE 事件里 Kv.Value 是空的，靠 PrevKv 才能拿到下线实例的详情
		wch := d.cli.Watch(d.ctx, d.prefix,
			clientv3.WithPrefix(),
			clientv3.WithRev(rev),
			clientv3.WithPrevKV(),
		)

		for resp := range wch {
			if err := resp.Err(); err != nil {
				// 典型场景：etcdserver: mvcc: required revision has been compacted
				// 说明要续的 revision 已经被压缩掉了，只能重新全量同步
				log.Printf("[discovery] watch 异常: %v，将重建", err)
				break
			}
			if len(resp.Events) > 0 {
				d.applyEvents(resp.Events)
				d.notify()
			}
			rev = resp.Header.Revision + 1
		}

		if d.ctx.Err() != nil {
			return
		}

		// Watch 通道被关闭，重新全量同步后再续上监听
		log.Printf("[discovery] watch 通道关闭 service=%s，重建中", d.name)
		newRev, err := d.resyncWithRetry()
		if err != nil {
			return // ctx 已取消
		}
		rev = newRev + 1
	}
}

// resyncWithRetry 带退避地重试全量同步，直到成功或 Discovery 被关闭。
func (d *Discovery) resyncWithRetry() (int64, error) {
	backoff := time.Second
	const maxBackoff = 30 * time.Second

	for {
		select {
		case <-d.ctx.Done():
			return 0, d.ctx.Err()
		case <-time.After(backoff):
		}

		ctx, cancel := context.WithTimeout(d.ctx, 3*time.Second)
		rev, err := d.fullSync(ctx)
		cancel()
		if err == nil {
			return rev, nil
		}

		log.Printf("[discovery] 重建失败: %v, %v 后重试", err, backoff)
		if backoff *= 2; backoff > maxBackoff {
			backoff = maxBackoff
		}
	}
}

// applyEvents 把 Watch 事件合并进本地缓存。
func (d *Discovery) applyEvents(events []*clientv3.Event) {
	d.mu.Lock()
	defer d.mu.Unlock()

	for _, ev := range events {
		key := string(ev.Kv.Key)
		switch ev.Type {
		case mvccpb.PUT:
			// PUT 同时覆盖「新实例上线」和「已有实例更新元数据」两种情况
			inst, err := UnmarshalInstance(ev.Kv.Value)
			if err != nil {
				log.Printf("[discovery] 跳过非法实例数据 key=%s: %v", key, err)
				continue
			}
			action := "上线"
			if _, exists := d.instances[key]; exists {
				action = "更新"
			}
			d.instances[key] = inst
			log.Printf("[discovery] 实例%s %s", action, inst)

		case mvccpb.DELETE:
			// 主动 Revoke 下线、或租约 TTL 到期被 etcd 自动清理，都会走到这里
			old := ServiceInstance{ID: key}
			if ev.PrevKv != nil {
				if inst, err := UnmarshalInstance(ev.PrevKv.Value); err == nil {
					old = inst
				}
			}
			delete(d.instances, key)
			log.Printf("[discovery] 实例下线 %s", old)
		}
	}
}

// GetInstances 返回当前实例快照（按实例 ID 排序，保证结果稳定）。
// 读的是本地缓存，不产生任何 etcd 请求。
func (d *Discovery) GetInstances() []ServiceInstance {
	d.mu.RLock()
	list := make([]ServiceInstance, 0, len(d.instances))
	for _, inst := range d.instances {
		list = append(list, inst)
	}
	d.mu.RUnlock()

	sort.Slice(list, func(i, j int) bool { return list[i].ID < list[j].ID })
	return list
}

// AddListener 注册实例列表变更回调。
// 回调在 Watch 协程中同步执行，实现里不要做阻塞操作，否则会拖慢事件处理。
func (d *Discovery) AddListener(fn ChangeFunc) {
	d.listenerMu.Lock()
	d.listeners = append(d.listeners, fn)
	d.listenerMu.Unlock()
}

func (d *Discovery) notify() {
	d.listenerMu.Lock()
	listeners := make([]ChangeFunc, len(d.listeners))
	copy(listeners, d.listeners)
	d.listenerMu.Unlock()

	if len(listeners) == 0 {
		return
	}
	snapshot := d.GetInstances()
	for _, fn := range listeners {
		fn(snapshot)
	}
}

// Close 停止 Watch 协程。
func (d *Discovery) Close() {
	d.closeOnce.Do(func() {
		d.cancel()
		d.wg.Wait()
	})
}
