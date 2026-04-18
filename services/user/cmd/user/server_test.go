package main

import (
	"context"
	"database/sql"
	"testing"

	"github.com/CBookShu/kd48/pkg/dsroute"
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
