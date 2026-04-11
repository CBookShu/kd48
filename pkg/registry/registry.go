package registry

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/CBookShu/kd48/pkg/conf"
	clientv3 "go.etcd.io/etcd/client/v3"
	"go.uber.org/zap"
)

// NewClient 创建并验证 Etcd 连接
func NewClient(c conf.EtcdConf) (*clientv3.Client, error) {
	cli, err := clientv3.New(clientv3.Config{
		Endpoints:   c.Endpoints,
		DialTimeout: 3 * time.Second,
		Logger:      zap.NewNop(), // 压制 etcd client 内部烦人的 debug 日志
	})
	if err != nil {
		return nil, fmt.Errorf("etcd client create failed: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	_, err = cli.Get(ctx, "health")
	if err != nil {
		_ = cli.Close()
		return nil, fmt.Errorf("etcd health check failed: %w", err)
	}

	return cli, nil
}

// endpointData 必须严格遵循 etcd client resolver 内部的反序列化结构
type endpointData struct {
	Addr     string            `json:"Addr"`
	Metadata map[string]string `json:"Metadata"`
}

// RegisterService 将服务地址注册到 Etcd
// name: 服务名 (如 "kd48/user-service"，必须与 gRPC dial 的 target 一致)
// addr: 服务地址 (如 "localhost:9000")
func RegisterService(cli *clientv3.Client, name, addr string) error {
	resp, err := cli.Grant(context.Background(), 10)
	if err != nil {
		return fmt.Errorf("etcd grant failed: %w", err)
	}

	// key 必须是 target + "/" + addr
	key := fmt.Sprintf("%s/%s", name, addr)

	// value 必须是 JSON 格式，包含 Addr 和 Metadata
	val := endpointData{
		Addr:     addr,
		Metadata: make(map[string]string),
	}
	valBytes, err := json.Marshal(val)
	if err != nil {
		return fmt.Errorf("marshal endpoint data failed: %w", err)
	}

	_, err = cli.Put(context.Background(), key, string(valBytes), clientv3.WithLease(resp.ID))
	if err != nil {
		return fmt.Errorf("etcd put failed: %w", err)
	}

	ch, err := cli.KeepAlive(context.Background(), resp.ID)
	if err != nil {
		return fmt.Errorf("etcd keepalive failed: %w", err)
	}

	go func() {
		for range ch {
			// 续约心跳
		}
		fmt.Printf("Etcd registration lost for %s\n", key)
	}()

	return nil
}
