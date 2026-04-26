package dsroute

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"sync/atomic"
	"time"

	"github.com/CBookShu/kd48/pkg/metrics"
	"github.com/redis/go-redis/v9"
)

type Router struct {
	mysqlPools  map[string]*sql.DB
	redisPools  map[string]redis.UniversalClient
	mysqlRoutes atomic.Value // []RouteRule
	redisRoutes atomic.Value // []RouteRule
	serviceName string
}

func NewRouter(
	mysqlPools map[string]*sql.DB,
	redisPools map[string]redis.UniversalClient,
	mysqlRoutes []RouteRule,
	redisRoutes []RouteRule,
	serviceName string,
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
		mysqlPools:  mysqlPools,
		redisPools:  redisPools,
		serviceName: serviceName,
	}

	// 原子存储初始路由
	r.mysqlRoutes.Store(mysqlRoutes)
	r.redisRoutes.Store(redisRoutes)

	go r.collectMetrics()

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
	start := time.Now()
	defer func() {
		metrics.DBPoolWaitDurationSeconds.WithLabelValues(r.serviceName, "mysql").Observe(time.Since(start).Seconds())
	}()

	routes := r.mysqlRoutes.Load().([]RouteRule)
	poolName, _, err := ResolvePoolName(routes, routingKey)
	if err != nil {
		return nil, "", err
	}
	db := r.mysqlPools[poolName]
	return db, poolName, nil
}

func (r *Router) ResolveRedis(ctx context.Context, routingKey string) (redis.UniversalClient, string, error) {
	start := time.Now()
	defer func() {
		metrics.DBPoolWaitDurationSeconds.WithLabelValues(r.serviceName, "redis").Observe(time.Since(start).Seconds())
	}()

	routes := r.redisRoutes.Load().([]RouteRule)
	poolName, _, err := ResolvePoolName(routes, routingKey)
	if err != nil {
		return nil, "", err
	}
	client := r.redisPools[poolName]
	return client, poolName, nil
}

func (r *Router) collectMetrics() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		for poolName, db := range r.mysqlPools {
			stats := db.Stats()
			metrics.DBPoolConnectionsActive.WithLabelValues(r.serviceName, "mysql", poolName).Set(float64(stats.InUse))
			metrics.DBPoolConnectionsIdle.WithLabelValues(r.serviceName, "mysql", poolName).Set(float64(stats.Idle))
		}

		for poolName, client := range r.redisPools {
			if pooler, ok := client.(interface{ PoolStats() *redis.PoolStats }); ok {
				stats := pooler.PoolStats()
				metrics.DBPoolConnectionsActive.WithLabelValues(r.serviceName, "redis", poolName).Set(float64(stats.Hits))
				metrics.DBPoolConnectionsIdle.WithLabelValues(r.serviceName, "redis", poolName).Set(float64(stats.IdleConns))
			}
		}
	}
}
