package dsroute

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/redis/go-redis/v9"
)

type Router struct {
	mysqlPools  map[string]*sql.DB
	redisPools  map[string]redis.UniversalClient
	mysqlRoutes []RouteRule
	redisRoutes []RouteRule
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

	return &Router{
		mysqlPools:  mysqlPools,
		redisPools:  redisPools,
		mysqlRoutes: mysqlRoutes,
		redisRoutes: redisRoutes,
	}, nil
}

func (r *Router) ResolveDB(ctx context.Context, routingKey string) (*sql.DB, string, error) {
	poolName, _, err := ResolvePoolName(r.mysqlRoutes, routingKey)
	if err != nil {
		return nil, "", err
	}
	db := r.mysqlPools[poolName]
	return db, poolName, nil
}

func (r *Router) ResolveRedis(ctx context.Context, routingKey string) (redis.UniversalClient, string, error) {
	poolName, _, err := ResolvePoolName(r.redisRoutes, routingKey)
	if err != nil {
		return nil, "", err
	}
	client := r.redisPools[poolName]
	return client, poolName, nil
}
