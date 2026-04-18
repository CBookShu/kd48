package dsroute

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"sync/atomic"

	"github.com/redis/go-redis/v9"
)

type Router struct {
	mysqlPools  map[string]*sql.DB
	redisPools  map[string]redis.UniversalClient
	mysqlRoutes atomic.Value // []RouteRule
	redisRoutes atomic.Value // []RouteRule
}

func NewRouter(
	mysqlPools map[string]*sql.DB,
	redisPools map[string]redis.UniversalClient,
	mysqlRoutes []RouteRule,
	redisRoutes []RouteRule,
) (*Router, error) {
	for i, rule := range mysqlRoutes {
		if _, exists := mysqlPools[rule.Pool]; !exists {
			return nil, fmt.Errorf("mysql route[%d]: pool %q not found in mysqlPools", i, rule.Pool)
		}
	}

	for i, rule := range redisRoutes {
		if _, exists := redisPools[rule.Pool]; !exists {
			return nil, fmt.Errorf("redis route[%d]: pool %q not found in redisPools", i, rule.Pool)
		}
	}

	if err := ValidateRoutes(mysqlRoutes); err != nil {
		return nil, fmt.Errorf("mysql routes validation: %w", err)
	}

	if err := ValidateRoutes(redisRoutes); err != nil {
		return nil, fmt.Errorf("redis routes validation: %w", err)
	}

	r := &Router{
		mysqlPools: mysqlPools,
		redisPools: redisPools,
	}

	// 原子存储初始路由
	r.mysqlRoutes.Store(mysqlRoutes)
	r.redisRoutes.Store(redisRoutes)

	return r, nil
}

// UpdateRoutes 原子更新路由表
func (r *Router) UpdateRoutes(mysqlRoutes, redisRoutes []RouteRule) {
	r.mysqlRoutes.Store(mysqlRoutes)
	r.redisRoutes.Store(redisRoutes)
	slog.Info("router routes updated",
		"mysql_routes", len(mysqlRoutes),
		"redis_routes", len(redisRoutes))
}

func (r *Router) ResolveDB(ctx context.Context, routingKey string) (*sql.DB, string, error) {
	routes := r.mysqlRoutes.Load().([]RouteRule)
	poolName, _, err := ResolvePoolName(routes, routingKey)
	if err != nil {
		return nil, "", err
	}
	db := r.mysqlPools[poolName]
	return db, poolName, nil
}

func (r *Router) ResolveRedis(ctx context.Context, routingKey string) (redis.UniversalClient, string, error) {
	routes := r.redisRoutes.Load().([]RouteRule)
	poolName, _, err := ResolvePoolName(routes, routingKey)
	if err != nil {
		return nil, "", err
	}
	client := r.redisPools[poolName]
	return client, poolName, nil
}
