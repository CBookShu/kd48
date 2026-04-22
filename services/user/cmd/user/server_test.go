package main

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/CBookShu/kd48/pkg/dsroute"
	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func TestUserService_getQueries_Success(t *testing.T) {
	mysqlPools := map[string]*sql.DB{
		"default": nil,
	}
	redisPools := map[string]redis.UniversalClient{
		"default": nil,
	}
	mysqlRoutes := []dsroute.RouteRule{{Prefix: "sys:user:", Pool: "default"}}
	redisRoutes := []dsroute.RouteRule{{Prefix: "", Pool: "default"}}

	router, err := dsroute.NewRouter(mysqlPools, redisPools, mysqlRoutes, redisRoutes)
	if err != nil {
		t.Fatalf("failed to create router: %v", err)
	}

	us := &userService{router: router}

	routingKey := "sys:user:alice"
	_, _, err = us.router.ResolveDB(context.Background(), routingKey)
	if err != nil {
		t.Errorf("expected route resolve success, got error: %v", err)
	}
}

func TestUserService_getQueries_RouteNotFound(t *testing.T) {
	mysqlPools := map[string]*sql.DB{
		"default": nil,
	}
	redisPools := map[string]redis.UniversalClient{
		"default": nil,
	}
	mysqlRoutes := []dsroute.RouteRule{{Prefix: "game:", Pool: "game_pool"}}
	redisRoutes := []dsroute.RouteRule{{Prefix: "", Pool: "default"}}

	router, err := dsroute.NewRouter(mysqlPools, redisPools, mysqlRoutes, redisRoutes)
	if err != nil {
		t.Logf("expected error: %v", err)
		return
	}

	us := &userService{router: router}

	routingKey := "sys:user:alice"
	_, _, err = us.router.ResolveDB(context.Background(), routingKey)
	if err == nil {
		t.Error("expected error for route not found, got nil")
	}
}

func TestIssueSessionAtomic_NewUser(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis run: %v", err)
	}
	defer mr.Close()

	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	router := createMockRouterWithRedis(t, rdb)
	svc := NewUserService(router, 1*time.Hour)

	token, hasOldToken, err := svc.issueSessionAtomic(context.Background(), 123, "alice")
	if err != nil {
		t.Fatalf("issueSessionAtomic: %v", err)
	}
	if token == "" {
		t.Error("token should not be empty")
	}
	if hasOldToken {
		t.Error("hasOldToken should be false for new user")
	}

	// 验证双向映射
	userKey := "user:123:session"
	storedToken, err := mr.Get(userKey)
	if err != nil {
		t.Fatalf("failed to get user key: %v", err)
	}
	if storedToken != token {
		t.Errorf("user:123:session = %q, want %q", storedToken, token)
	}

	sessionKey := "user:session:" + token
	sessionVal, err := mr.Get(sessionKey)
	if err != nil {
		t.Fatalf("failed to get session key: %v", err)
	}
	if sessionVal != "123:alice" {
		t.Errorf("user:session:%s = %q, want 123:alice", token, sessionVal)
	}
}

func TestIssueSessionAtomic_KickOldSession(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis run: %v", err)
	}
	defer mr.Close()

	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	router := createMockRouterWithRedis(t, rdb)
	svc := NewUserService(router, 1*time.Hour)

	// 第一次登录
	token1, hasOldToken1, err := svc.issueSessionAtomic(context.Background(), 123, "alice")
	if err != nil {
		t.Fatalf("first login: %v", err)
	}
	if hasOldToken1 {
		t.Error("hasOldToken should be false for first login")
	}

	// 验证旧 session 存在
	sessionKey1 := "user:session:" + token1
	sessionVal1, err := mr.Get(sessionKey1)
	if err != nil {
		t.Fatalf("failed to get first session: %v", err)
	}
	if sessionVal1 != "123:alice" {
		t.Fatalf("first session not stored correctly, got %q", sessionVal1)
	}

	// 第二次登录（顶号）
	token2, hasOldToken, err := svc.issueSessionAtomic(context.Background(), 123, "alice")
	if err != nil {
		t.Fatalf("second login: %v", err)
	}
	if !hasOldToken {
		t.Error("hasOldToken should be true when replacing old session")
	}

	// 验证新 token 存在
	userKey := "user:123:session"
	storedToken, err := mr.Get(userKey)
	if err != nil {
		t.Fatalf("failed to get user key: %v", err)
	}
	if storedToken != token2 {
		t.Errorf("user:123:session = %q, want %q", storedToken, token2)
	}

	// 验证旧 token 被删除
	sessionKey1After := "user:session:" + token1
	_, err = mr.Get(sessionKey1After)
	if err == nil {
		t.Errorf("old session key should be deleted")
	}

	// 验证新 session 存在
	sessionKey2 := "user:session:" + token2
	sessionVal2, err := mr.Get(sessionKey2)
	if err != nil {
		t.Fatalf("failed to get new session: %v", err)
	}
	if sessionVal2 != "123:alice" {
		t.Errorf("new session should exist with correct value, got %q", sessionVal2)
	}
}

func createMockRouterWithRedis(t *testing.T, rdb redis.UniversalClient) *dsroute.Router {
	t.Helper()
	mysqlPools := map[string]*sql.DB{}
	redisPools := map[string]redis.UniversalClient{"default": rdb}
	mysqlRoutes := []dsroute.RouteRule{}
	redisRoutes := []dsroute.RouteRule{{Prefix: "sys:session", Pool: "default"}}
	router, err := dsroute.NewRouter(mysqlPools, redisPools, mysqlRoutes, redisRoutes)
	if err != nil {
		t.Fatalf("failed to create router: %v", err)
	}
	return router
}
