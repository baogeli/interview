// Demo: 基于 etcd 的服务注册与发现。
//
// 前置条件：本机需要一个可用的 etcd。没有的话用 docker 一键起一个：
//
//	docker run -d --name etcd-demo -p 2379:2379 \
//	  -e ALLOW_NONE_AUTHENTICATION=yes \
//	  -e ETCD_ADVERTISE_CLIENT_URLS=http://0.0.0.0:2379 \
//	  bitnami/etcd:3.5
//
// 运行：
//
//	go run ./8-etcd
//	go run ./8-etcd -endpoints=127.0.0.1:2379
//
// 演示脚本会依次展示四种场景下消费端实例列表的变化：
//
//	① 实例陆续上线      -> Watch 收到 PUT 事件
//	② 实例优雅下线      -> 主动 Revoke 租约，秒级摘除
//	③ 实例进程崩溃      -> 停止续约，租约 TTL 到期后被 etcd 自动摘除
//	④ 消费端读取列表    -> 全程走本地缓存，零 etcd 请求
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"boa/8-etcd/registry"
)

const (
	serviceName = "user-service"

	// 租约 TTL。演示里故意设小，好让「进程崩溃后被自动摘除」在几秒内看到效果。
	// 生产环境一般 10~30s：太小会因网络抖动误摘活着的实例，太大则故障感知变慢。
	leaseTTL = 5
)

func main() {
	var endpoints string
	flag.StringVar(&endpoints, "endpoints", "127.0.0.1:2379", "etcd 地址，多个用逗号分隔")
	flag.Parse()

	log.SetFlags(log.Ltime)

	cli, err := registry.NewClient(strings.Split(endpoints, ","), 3*time.Second)
	if err != nil {
		log.Printf("连接 etcd 失败: %v", err)
		log.Printf("请先启动 etcd，例如：")
		log.Printf("  docker run -d --name etcd-demo -p 2379:2379 \\")
		log.Printf("    -e ALLOW_NONE_AUTHENTICATION=yes \\")
		log.Printf("    -e ETCD_ADVERTISE_CLIENT_URLS=http://0.0.0.0:2379 bitnami/etcd:3.5")
		os.Exit(1)
	}
	defer cli.Close()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// ---------- 消费端：启动服务发现 ----------
	discovery := registry.NewDiscovery(cli, serviceName)
	discovery.AddListener(func(instances []registry.ServiceInstance) {
		printInstances(instances)
	})
	if err = discovery.Start(ctx); err != nil {
		log.Fatalf("启动服务发现失败: %v", err)
	}
	defer discovery.Close()

	// ---------- 提供端：三个实例陆续上线 ----------
	section("场景①：三个实例陆续上线")
	providers := make([]*registry.Register, 0, 3)
	for i := 1; i <= 3; i++ {
		inst := registry.ServiceInstance{
			Name:     serviceName,
			ID:       fmt.Sprintf("%s-%d", serviceName, i),
			Addr:     fmt.Sprintf("127.0.0.1:%d", 9000+i),
			Weight:   i * 10,
			Metadata: map[string]string{"zone": "sh", "version": "v1.0.0"},
		}

		r := registry.NewRegister(cli, leaseTTL)
		if err = r.Register(ctx, inst); err != nil {
			log.Fatalf("注册 %s 失败: %v", inst.ID, err)
		}
		providers = append(providers, r)

		if !sleep(ctx, 2*time.Second) {
			cleanup(providers)
			return
		}
	}

	// ---------- 场景②：优雅下线 ----------
	section("场景②：user-service-2 优雅下线（主动 Revoke 租约，消费端秒级感知）")
	if err = providers[1].Deregister(ctx); err != nil {
		log.Printf("下线失败: %v", err)
	}
	providers[1].Close()
	if !sleep(ctx, 3*time.Second) {
		cleanup(providers)
		return
	}

	// ---------- 场景③：进程崩溃 ----------
	section(fmt.Sprintf(
		"场景③：user-service-3 模拟进程崩溃（不 Revoke，直接停止续约，约 %ds 后租约到期被自动摘除）",
		leaseTTL))
	providers[2].Close() // 只停心跳，不撤销租约 —— 等价于 kill -9
	if !sleep(ctx, time.Duration(leaseTTL+3)*time.Second) {
		cleanup(providers)
		return
	}

	// ---------- 场景④：消费端读缓存 ----------
	section("场景④：消费端读取实例列表（走本地缓存，不发起任何 etcd 请求）")
	for _, inst := range discovery.GetInstances() {
		log.Printf("  可用实例 %s weight=%d meta=%v", inst, inst.Weight, inst.Metadata)
	}

	section("演示结束，清理剩余实例")
	cleanup(providers)
}

func cleanup(providers []*registry.Register) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	for _, r := range providers {
		_ = r.Deregister(ctx)
		r.Close()
	}
}

// sleep 可被 ctx 取消的等待，返回 false 表示收到了退出信号。
func sleep(ctx context.Context, d time.Duration) bool {
	select {
	case <-ctx.Done():
		return false
	case <-time.After(d):
		return true
	}
}

func section(title string) {
	log.Printf("\n=========== %s ===========", title)
}

func printInstances(instances []registry.ServiceInstance) {
	if len(instances) == 0 {
		log.Printf(">>> 当前实例列表: (空)")
		return
	}
	names := make([]string, 0, len(instances))
	for _, inst := range instances {
		names = append(names, inst.String())
	}
	log.Printf(">>> 当前实例列表(%d): %s", len(instances), strings.Join(names, ", "))
}
