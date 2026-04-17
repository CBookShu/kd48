//go:build integration
// +build integration

package dsroute

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	clientv3 "go.etcd.io/etcd/client/v3"
)

func TestRouteLoader_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	cli, err := clientv3.New(clientv3.Config{
		Endpoints:   []string{"localhost:2379"},
		DialTimeout: 5 * time.Second,
	})
	if err != nil {
		t.Fatalf("failed to connect etcd: %v", err)
	}
	defer cli.Close()

	loader := NewRouteLoader(cli, "test/routing")
	ctx := context.Background()

	// 清理测试数据（测试前）
	cli.Delete(ctx, "test/routing/", clientv3.WithPrefix())

	// 设置测试数据
	mysqlRoutes := []RouteRule{{Prefix: "user_", Pool: "user_pool"}}
	redisRoutes := []RouteRule{{Prefix: "session_", Pool: "session_pool"}}

	mysqlData, _ := json.Marshal(mysqlRoutes)
	redisData, _ := json.Marshal(redisRoutes)

	_, err = cli.Put(ctx, "test/routing/mysql_routes", string(mysqlData))
	if err != nil {
		t.Fatalf("failed to put mysql routes: %v", err)
	}
	_, err = cli.Put(ctx, "test/routing/redis_routes", string(redisData))
	if err != nil {
		t.Fatalf("failed to put redis routes: %v", err)
	}

	// 测试 Get
	cfg, err := loader.Get(ctx)
	if err != nil {
		t.Fatalf("failed to get routes: %v", err)
	}

	if len(cfg.MySQLRoutes) != 1 || cfg.MySQLRoutes[0].Prefix != "user_" {
		t.Errorf("unexpected mysql routes: %+v", cfg.MySQLRoutes)
	}

	if len(cfg.RedisRoutes) != 1 || cfg.RedisRoutes[0].Prefix != "session_" {
		t.Errorf("unexpected redis routes: %+v", cfg.RedisRoutes)
	}

	// 清理（测试后）
	cli.Delete(ctx, "test/routing/", clientv3.WithPrefix())
}
