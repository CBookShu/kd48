package dsroute

import (
	"database/sql"
	"fmt"

	"github.com/redis/go-redis/v9"
)

func ValidateConfig(cfg *DataSourcesConfig) error {
	if cfg == nil {
		return &ValidationError{Message: "config is nil"}
	}

	for i, rule := range cfg.MySQLRoutes {
		if _, exists := cfg.MySQLPools[rule.Pool]; !exists {
			return &ValidationError{
				Message: fmt.Sprintf("mysql_routes[%d]: pool %q not found in mysql_pools", i, rule.Pool),
			}
		}
	}

	for i, rule := range cfg.RedisRoutes {
		if _, exists := cfg.RedisPools[rule.Pool]; !exists {
			return &ValidationError{
				Message: fmt.Sprintf("redis_routes[%d]: pool %q not found in redis_pools", i, rule.Pool),
			}
		}
	}

	if err := ValidateRoutes(cfg.MySQLRoutes); err != nil {
		return fmt.Errorf("mysql_routes: %w", err)
	}

	if err := ValidateRoutes(cfg.RedisRoutes); err != nil {
		return fmt.Errorf("redis_routes: %w", err)
	}

	return nil
}

func NewRouterFromConfig(
	cfg *DataSourcesConfig,
	mysqlPools map[string]*sql.DB,
	redisPools map[string]redis.UniversalClient,
	serviceName string,
) (*Router, error) {
	if err := ValidateConfig(cfg); err != nil {
		return nil, fmt.Errorf("config validation: %w", err)
	}

	return NewRouter(mysqlPools, redisPools, cfg.MySQLRoutes, cfg.RedisRoutes, serviceName)
}
