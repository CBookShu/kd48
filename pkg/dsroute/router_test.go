package dsroute

import (
	"context"
	"database/sql"
	"testing"

	"github.com/redis/go-redis/v9"
)

func TestNewRouter_MySQLPoolNotFound(t *testing.T) {
	mysqlPools := map[string]*sql.DB{
		"primary": nil,
	}
	redisPools := map[string]redis.UniversalClient{
		"cache": nil,
	}
	mysqlRoutes := []RouteRule{
		{Prefix: "user:", Pool: "primary"},
		{Prefix: "order:", Pool: "unknown"},
	}
	redisRoutes := []RouteRule{
		{Prefix: "session:", Pool: "cache"},
	}

	_, err := NewRouter(mysqlPools, redisPools, mysqlRoutes, redisRoutes, "test")
	if err == nil {
		t.Error("expected error for unknown pool, got nil")
	}
}

func TestNewRouter_RedisPoolNotFound(t *testing.T) {
	mysqlPools := map[string]*sql.DB{
		"primary": nil,
	}
	redisPools := map[string]redis.UniversalClient{
		"cache": nil,
	}
	mysqlRoutes := []RouteRule{
		{Prefix: "user:", Pool: "primary"},
	}
	redisRoutes := []RouteRule{
		{Prefix: "session:", Pool: "unknown"},
	}

	_, err := NewRouter(mysqlPools, redisPools, mysqlRoutes, redisRoutes, "test")
	if err == nil {
		t.Error("expected error for unknown pool, got nil")
	}
}

func TestNewRouter_DuplicateMySQLPrefix(t *testing.T) {
	mysqlPools := map[string]*sql.DB{
		"primary":   nil,
		"secondary": nil,
	}
	redisPools := map[string]redis.UniversalClient{}
	mysqlRoutes := []RouteRule{
		{Prefix: "user:", Pool: "primary"},
		{Prefix: "user:", Pool: "secondary"},
	}
	redisRoutes := []RouteRule{}

	_, err := NewRouter(mysqlPools, redisPools, mysqlRoutes, redisRoutes, "test")
	if err == nil {
		t.Error("expected error for duplicate prefix, got nil")
	}
}

func TestNewRouter_DuplicateRedisPrefix(t *testing.T) {
	mysqlPools := map[string]*sql.DB{}
	redisPools := map[string]redis.UniversalClient{
		"cache": nil,
		"temp":  nil,
	}
	mysqlRoutes := []RouteRule{}
	redisRoutes := []RouteRule{
		{Prefix: "session:", Pool: "cache"},
		{Prefix: "session:", Pool: "temp"},
	}

	_, err := NewRouter(mysqlPools, redisPools, mysqlRoutes, redisRoutes, "test")
	if err == nil {
		t.Error("expected error for duplicate prefix, got nil")
	}
}

func TestNewRouter_Success(t *testing.T) {
	mysqlPools := map[string]*sql.DB{
		"primary": nil,
	}
	redisPools := map[string]redis.UniversalClient{
		"cache": nil,
	}
	mysqlRoutes := []RouteRule{
		{Prefix: "user:", Pool: "primary"},
		{Prefix: "", Pool: "primary"},
	}
	redisRoutes := []RouteRule{
		{Prefix: "session:", Pool: "cache"},
	}

	router, err := NewRouter(mysqlPools, redisPools, mysqlRoutes, redisRoutes, "test")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if router == nil {
		t.Error("expected router, got nil")
	}
}

func TestRouter_ResolveDB_Success(t *testing.T) {
	mysqlPools := map[string]*sql.DB{
		"primary": nil,
	}
	redisPools := map[string]redis.UniversalClient{}
	mysqlRoutes := []RouteRule{
		{Prefix: "user:", Pool: "primary"},
		{Prefix: "", Pool: "primary"},
	}
	redisRoutes := []RouteRule{}

	router, err := NewRouter(mysqlPools, redisPools, mysqlRoutes, redisRoutes, "test")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	db, poolName, err := router.ResolveDB(context.Background(), "user:123")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if db != nil {
		t.Error("expected nil db (placeholder), got non-nil")
	}
	if poolName != "primary" {
		t.Errorf("expected poolName 'primary', got %q", poolName)
	}
}

func TestRouter_ResolveDB_NoMatch(t *testing.T) {
	mysqlPools := map[string]*sql.DB{
		"primary": nil,
	}
	redisPools := map[string]redis.UniversalClient{}
	mysqlRoutes := []RouteRule{
		{Prefix: "order:", Pool: "primary"},
	}
	redisRoutes := []RouteRule{}

	router, err := NewRouter(mysqlPools, redisPools, mysqlRoutes, redisRoutes, "test")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	_, _, err = router.ResolveDB(context.Background(), "user:123")
	if err == nil {
		t.Error("expected error for no match, got nil")
	}
}

func TestRouter_ResolveRedis_Success(t *testing.T) {
	mysqlPools := map[string]*sql.DB{}
	redisPools := map[string]redis.UniversalClient{
		"cache": nil,
	}
	mysqlRoutes := []RouteRule{}
	redisRoutes := []RouteRule{
		{Prefix: "session:", Pool: "cache"},
		{Prefix: "", Pool: "cache"},
	}

	router, err := NewRouter(mysqlPools, redisPools, mysqlRoutes, redisRoutes, "test")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	client, poolName, err := router.ResolveRedis(context.Background(), "session:abc")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if client != nil {
		t.Error("expected nil client (placeholder), got non-nil")
	}
	if poolName != "cache" {
		t.Errorf("expected poolName 'cache', got %q", poolName)
	}
}

func TestRouter_ResolveRedis_NoMatch(t *testing.T) {
	mysqlPools := map[string]*sql.DB{}
	redisPools := map[string]redis.UniversalClient{
		"cache": nil,
	}
	mysqlRoutes := []RouteRule{}
	redisRoutes := []RouteRule{
		{Prefix: "temp:", Pool: "cache"},
	}

	router, err := NewRouter(mysqlPools, redisPools, mysqlRoutes, redisRoutes, "test")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	_, _, err = router.ResolveRedis(context.Background(), "session:abc")
	if err == nil {
		t.Error("expected error for no match, got nil")
	}
}

func TestRouter_ResolveDB_LongestPrefix(t *testing.T) {
	mysqlPools := map[string]*sql.DB{
		"primary": nil,
		"shard1":  nil,
	}
	redisPools := map[string]redis.UniversalClient{}
	mysqlRoutes := []RouteRule{
		{Prefix: "user:", Pool: "primary"},
		{Prefix: "user:vip:", Pool: "shard1"},
	}
	redisRoutes := []RouteRule{}

	router, err := NewRouter(mysqlPools, redisPools, mysqlRoutes, redisRoutes, "test")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	_, poolName, err := router.ResolveDB(context.Background(), "user:vip:456")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if poolName != "shard1" {
		t.Errorf("expected poolName 'shard1' (longest match), got %q", poolName)
	}
}
