package registry

import (
	"context"
	"fmt"
	"time"

	clientv3 "go.etcd.io/etcd/client/v3"
	"go.uber.org/zap"
)

// NewClient 创建 etcd 客户端并做一次主动探活。
//
// 注意：clientv3.New 是「惰性连接」的，即使 endpoints 完全写错也会返回 nil error，
// 真正的连接错误要等到第一次 RPC 才暴露。
// 所以这里主动 Status 一次做健康检查，让配置错误在启动阶段就失败，而不是拖到线上第一次调用。
func NewClient(endpoints []string, dialTimeout time.Duration) (*clientv3.Client, error) {
	if len(endpoints) == 0 {
		return nil, fmt.Errorf("registry: empty endpoints")
	}

	cli, err := clientv3.New(clientv3.Config{
		Endpoints:   endpoints,
		DialTimeout: dialTimeout,
		Logger:      zap.NewNop(), // 屏蔽客户端自身的重连日志噪音，只保留我们自己的日志
	})
	if err != nil {
		return nil, fmt.Errorf("registry: new etcd client: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), dialTimeout)
	defer cancel()
	if _, err = cli.Status(ctx, endpoints[0]); err != nil {
		_ = cli.Close()
		return nil, fmt.Errorf("registry: etcd %s 不可达: %w", endpoints[0], err)
	}
	return cli, nil
}
