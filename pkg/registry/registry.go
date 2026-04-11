package registry

import (
	"context"
	"fmt"
	"time"

	"github.com/CBookShu/kd48/pkg/conf"
	clientv3 "go.etcd.io/etcd/client/v3"
)

// NewClient 创建并验证 Etcd 连接
func NewClient(c conf.EtcdConf) (*clientv3.Client, error) {
	cli, err := clientv3.New(clientv3.Config{
		Endpoints:   c.Endpoints,
		DialTimeout: 3 * time.Second,
	})
	if err != nil {
		return nil, fmt.Errorf("etcd client create failed: %w", err)
	}

	// 验证连接
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	_, err = cli.Get(ctx, "health")
	if err != nil {
		_ = cli.Close()
		return nil, fmt.Errorf("etcd health check failed: %w", err)
	}

	return cli, nil
}
