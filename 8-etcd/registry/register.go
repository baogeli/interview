package registry

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	clientv3 "go.etcd.io/etcd/client/v3"
)

// Register 负责把一个服务实例注册到 etcd，并持续保活。
//
// 核心机制（面试高频）：
//  1. Lease 租约：注册的 key 绑定一个 TTL 租约，而不是写成永久 key。
//     进程被 kill -9 / 宿主机宕机时，没人续约，租约到期后 etcd 会自动删除 key，
//     实例自然从服务列表里消失，不会留下「僵尸节点」。
//  2. KeepAlive 自动续约：etcd 客户端后台按 TTL/3 的频率发心跳续约，
//     并把每次续约结果推到一个缓冲为 16 的 channel 里。
//     这个 channel 必须起协程消费——它是唯一能感知「租约已经死了」的通道：
//     一旦客户端放弃续约（租约过期 / 一个 TTL 内没收到任何响应），
//     它会 close 这个 channel。不消费就永远发现不了自己已被摘除。
//     （v3.5 的实现里响应是非阻塞投递的，队列满只会丢弃响应并打警告，
//     不会直接停掉续约；但旧版本客户端有过因此续约异常的历史问题，
//     无论如何都不该把这个 channel 晾着。）
//  3. 断连自愈：channel 被关闭代表客户端已经放弃这个租约，
//     必须重新 Grant + Put，而不是傻等，否则服务再也回不到注册中心。
//  4. 优雅下线：主动 Revoke 租约，key 立刻消失，
//     消费端秒级感知，避免 TTL 到期前的那段时间里流量还被打到已停的实例上。
type Register struct {
	cli *clientv3.Client
	ttl int64 // 租约 TTL，单位秒

	ctx    context.Context // 控制保活协程生命周期，不能复用调用方的短生命周期 ctx
	cancel context.CancelFunc

	mu       sync.RWMutex
	instance ServiceInstance
	leaseID  clientv3.LeaseID

	wg        sync.WaitGroup
	closeOnce sync.Once
}

// NewRegister 创建一个注册器。
// ttlSeconds 建议 >= 5s：太小会让心跳过于频繁并放大网络抖动导致的误摘除。
func NewRegister(cli *clientv3.Client, ttlSeconds int64) *Register {
	if ttlSeconds <= 0 {
		ttlSeconds = 10
	}
	ctx, cancel := context.WithCancel(context.Background())
	return &Register{
		cli:    cli,
		ttl:    ttlSeconds,
		ctx:    ctx,
		cancel: cancel,
	}
}

// Register 注册实例并启动后台保活协程。
func (r *Register) Register(ctx context.Context, inst ServiceInstance) error {
	if inst.Name == "" || inst.ID == "" || inst.Addr == "" {
		return fmt.Errorf("registry: instance name/id/addr must not be empty")
	}

	r.mu.Lock()
	r.instance = inst
	r.mu.Unlock()

	ch, err := r.grantAndPut(ctx)
	if err != nil {
		return err
	}

	r.wg.Add(1)
	go r.keepAliveLoop(ch)
	return nil
}

// grantAndPut 申请租约 -> 写入 key -> 开启续约，返回续约结果 channel。
func (r *Register) grantAndPut(ctx context.Context) (<-chan *clientv3.LeaseKeepAliveResponse, error) {
	r.mu.RLock()
	inst := r.instance
	r.mu.RUnlock()

	value, err := inst.Marshal()
	if err != nil {
		return nil, err
	}

	// 1) 申请一个 TTL 租约
	lease, err := r.cli.Grant(ctx, r.ttl)
	if err != nil {
		return nil, fmt.Errorf("registry: grant lease: %w", err)
	}

	// 2) 写入实例 key，并绑定租约。租约一失效，这个 key 就会被 etcd 自动清理
	if _, err = r.cli.Put(ctx, inst.Key(), value, clientv3.WithLease(lease.ID)); err != nil {
		// Put 失败要把刚申请的租约回收掉，否则会泄漏一个空租约
		_, _ = r.cli.Revoke(context.Background(), lease.ID)
		return nil, fmt.Errorf("registry: put key %s: %w", inst.Key(), err)
	}

	// 3) 开启自动续约。注意这里传的是 r.ctx（跟随 Register 生命周期），
	//    而不是调用方传进来的 ctx —— 后者往往几秒就超时了，一旦超时续约就停了。
	ch, err := r.cli.KeepAlive(r.ctx, lease.ID)
	if err != nil {
		_, _ = r.cli.Revoke(context.Background(), lease.ID)
		return nil, fmt.Errorf("registry: keepalive: %w", err)
	}

	r.mu.Lock()
	r.leaseID = lease.ID
	r.mu.Unlock()

	log.Printf("[register] 注册成功 key=%s lease=%x ttl=%ds", inst.Key(), lease.ID, r.ttl)
	return ch, nil
}

// keepAliveLoop 消费续约响应，并在租约失效时自愈重注册。
func (r *Register) keepAliveLoop(ch <-chan *clientv3.LeaseKeepAliveResponse) {
	defer r.wg.Done()

	for {
		select {
		case <-r.ctx.Done():
			return

		case _, ok := <-ch:
			if ok {
				// 正常续约。生产中通常只在这里更新一个「上次续约时间」指标，
				// 供监控判断注册中心链路是否健康。
				continue
			}

			// channel 被关闭：租约已过期，或一个 TTL 周期内没收到任何续约响应
			// （deadlineLoop 会把这类租约判死并关闭 channel）。进入重注册自愈。
			r.mu.RLock()
			key := r.instance.Key()
			r.mu.RUnlock()
			log.Printf("[register] 续约通道关闭 key=%s，开始重新注册", key)

			newCh, ok := r.retryRegister()
			if !ok {
				return // ctx 已取消，正常退出
			}
			ch = newCh
		}
	}
}

// retryRegister 带退避地重试注册，直到成功或 Register 被关闭。
func (r *Register) retryRegister() (<-chan *clientv3.LeaseKeepAliveResponse, bool) {
	backoff := time.Second
	const maxBackoff = 30 * time.Second

	for {
		select {
		case <-r.ctx.Done():
			return nil, false
		case <-time.After(backoff):
		}

		ctx, cancel := context.WithTimeout(r.ctx, 3*time.Second)
		ch, err := r.grantAndPut(ctx)
		cancel()
		if err == nil {
			return ch, true
		}

		log.Printf("[register] 重新注册失败: %v, %v 后重试", err, backoff)
		if backoff *= 2; backoff > maxBackoff {
			backoff = maxBackoff
		}
	}
}

// Deregister 优雅下线：撤销租约，绑定在该租约上的 key 会被立即删除。
//
// 为什么用 Revoke 而不是 Delete：
//   - Revoke 一次调用即可清掉该租约下的所有 key（一个实例可能注册了多个 key）；
//   - Delete 只删 key，租约还在，会残留；
//   - 而如果什么都不做直接退出进程，key 要等最长 TTL 秒才消失，
//     这段时间内消费端仍会把流量打到已经停掉的实例上。
func (r *Register) Deregister(ctx context.Context) error {
	r.mu.RLock()
	leaseID, key := r.leaseID, r.instance.Key()
	r.mu.RUnlock()

	if leaseID == clientv3.NoLease {
		return nil
	}
	if _, err := r.cli.Revoke(ctx, leaseID); err != nil {
		return fmt.Errorf("registry: revoke lease %x: %w", leaseID, err)
	}
	log.Printf("[register] 优雅下线 key=%s lease=%x", key, leaseID)
	return nil
}

// Close 停止保活协程。通常在 Deregister 之后调用。
func (r *Register) Close() {
	r.closeOnce.Do(func() {
		r.cancel()
		r.wg.Wait()
	})
}
