package dsroute

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	clientv3 "go.etcd.io/etcd/client/v3"
)

// RoutingConfig 包含所有路由配置
type RoutingConfig struct {
	MySQLRoutes []RouteRule
	RedisRoutes []RouteRule
}

// RouteLoader 从 etcd 加载和监听路由配置
type RouteLoader struct {
	client *clientv3.Client
	prefix string
}

// NewRouteLoader 创建 RouteLoader
func NewRouteLoader(client *clientv3.Client, prefix string) *RouteLoader {
	if prefix == "" {
		prefix = "kd48/routing"
	}
	return &RouteLoader{
		client: client,
		prefix: prefix,
	}
}

// Get 从 etcd 获取当前路由配置
func (l *RouteLoader) Get(ctx context.Context) (*RoutingConfig, error) {
	if l.client == nil {
		return nil, fmt.Errorf("etcd client is nil")
	}

	mysqlKey := l.prefix + "/mysql_routes"
	redisKey := l.prefix + "/redis_routes"

	mysqlRoutes, err := l.getRoutes(ctx, mysqlKey)
	if err != nil {
		return nil, fmt.Errorf("get mysql routes: %w", err)
	}

	redisRoutes, err := l.getRoutes(ctx, redisKey)
	if err != nil {
		return nil, fmt.Errorf("get redis routes: %w", err)
	}

	return &RoutingConfig{
		MySQLRoutes: mysqlRoutes,
		RedisRoutes: redisRoutes,
	}, nil
}

func (l *RouteLoader) getRoutes(ctx context.Context, key string) ([]RouteRule, error) {
	resp, err := l.client.Get(ctx, key)
	if err != nil {
		return nil, err
	}

	if len(resp.Kvs) == 0 {
		return nil, fmt.Errorf("key %s not found", key)
	}

	var rules []RouteRule
	if err := json.Unmarshal(resp.Kvs[0].Value, &rules); err != nil {
		return nil, fmt.Errorf("unmarshal routes: %w", err)
	}

	return rules, nil
}

// Watch 监听配置变更
func (l *RouteLoader) Watch(ctx context.Context, onChange func(*RoutingConfig)) error {
	mysqlKey := l.prefix + "/mysql_routes"
	redisKey := l.prefix + "/redis_routes"

	watchChan := l.client.Watch(ctx, l.prefix, clientv3.WithPrefix())

	for resp := range watchChan {
		if resp.Err() != nil {
			slog.Error("watch error", "error", resp.Err())
			continue
		}

		// 检查是否是我们关心的 key
		needUpdate := false
		for _, ev := range resp.Events {
			key := string(ev.Kv.Key)
			if key == mysqlKey || key == redisKey {
				needUpdate = true
				break
			}
		}

		if !needUpdate {
			continue
		}

		// 重新获取配置
		config, err := l.Get(ctx)
		if err != nil {
			slog.Error("failed to get updated routes", "error", err)
			continue
		}

		onChange(config)
	}

	return ctx.Err()
}
