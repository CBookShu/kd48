package dsroute

import (
	"testing"
)

func TestValidateConfig_NilConfig(t *testing.T) {
	err := ValidateConfig(nil)
	if err == nil {
		t.Fatal("expected error for nil config")
	}
}

func TestValidateConfig_EmptyConfig(t *testing.T) {
	cfg := &DataSourcesConfig{}
	err := ValidateConfig(cfg)
	if err != nil {
		t.Fatalf("empty config should be valid: %v", err)
	}
}

func TestValidateConfig_MySQLRouteMissingPool(t *testing.T) {
	cfg := &DataSourcesConfig{
		MySQLPools: map[string]MySQLPoolSpec{
			"default": {DSN: "dsn"},
		},
		MySQLRoutes: []RouteRule{
			{Prefix: "", Pool: "missing"},
		},
	}
	err := ValidateConfig(cfg)
	if err == nil {
		t.Fatal("expected error for missing mysql pool")
	}
}

func TestValidateConfig_RedisRouteMissingPool(t *testing.T) {
	cfg := &DataSourcesConfig{
		RedisPools: map[string]RedisPoolSpec{
			"default": {Addr: "localhost:6379"},
		},
		RedisRoutes: []RouteRule{
			{Prefix: "", Pool: "missing"},
		},
	}
	err := ValidateConfig(cfg)
	if err == nil {
		t.Fatal("expected error for missing redis pool")
	}
}

func TestValidateConfig_DuplicatePrefix(t *testing.T) {
	cfg := &DataSourcesConfig{
		MySQLPools: map[string]MySQLPoolSpec{
			"default": {DSN: "dsn"},
			"lobby":   {DSN: "dsn2"},
		},
		MySQLRoutes: []RouteRule{
			{Prefix: "lobby:", Pool: "lobby"},
			{Prefix: "lobby:", Pool: "default"},
		},
	}
	err := ValidateConfig(cfg)
	if err == nil {
		t.Fatal("expected error for duplicate prefix")
	}
}

func TestValidateConfig_ValidConfig(t *testing.T) {
	cfg := &DataSourcesConfig{
		MySQLPools: map[string]MySQLPoolSpec{
			"default": {DSN: "user:pass@tcp(localhost:3306)/db", MaxOpen: 20, MaxIdle: 5},
			"lobby":   {DSN: "user:pass@tcp(localhost:3307)/db", MaxOpen: 20, MaxIdle: 5},
		},
		RedisPools: map[string]RedisPoolSpec{
			"default": {Addr: "localhost:6379"},
			"session": {Addr: "localhost:6380"},
		},
		MySQLRoutes: []RouteRule{
			{Prefix: "lobby:", Pool: "lobby"},
			{Prefix: "", Pool: "default"},
		},
		RedisRoutes: []RouteRule{
			{Prefix: "session", Pool: "session"},
			{Prefix: "", Pool: "default"},
		},
	}
	err := ValidateConfig(cfg)
	if err != nil {
		t.Fatalf("valid config should pass: %v", err)
	}
}
