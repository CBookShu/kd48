package dsroute

import (
	_ "database/sql"
	_ "github.com/redis/go-redis/v9"
)

/*
Package dsroute provides routing_key based datasource routing for MySQL and Redis.

# Overview

This package implements Longest Prefix Match (LPM) routing from abstract routing keys
to named connection pools. It is designed to be reused across services (Gateway, User, Lobby, etc.)

# Routing Key

A routing_key is a logical string representing a data domain, e.g.:
  - "session" - for session data
  - "lobby:config" - for lobby configuration
  - "analytics:rollup" - for analytics data

The routing_key is decoupled from physical keys (e.g., Redis key "user:session:{token}").

# Longest Prefix Match (LPM)

Routes are matched by longest prefix:
  - Non-empty prefix: matches if HasPrefix(routing_key, prefix), effective_len = len(prefix)
  - Empty prefix (""): always matches, effective_len = 0 (fallback)

The rule with the largest effective_len wins. If no rule matches, an error is returned.

# Configuration

Example YAML:

	data_sources:
	  mysql_pools:
	    default: { dsn: "...", max_open: 20, max_idle: 5 }
	    lobby:   { dsn: "...", max_open: 20, max_idle: 5 }
	  redis_pools:
	    default:   { addr: "redis-main:6379" }
	    session:   { addr: "redis-session:6379" }
	  mysql_routes:
	    - { prefix: "lobby:", pool: "lobby" }
	    - { prefix: "", pool: "default" }
	  redis_routes:
	    - { prefix: "session", pool: "session" }
	    - { prefix: "", pool: "default" }

# Usage

	router, err := dsroute.NewRouter(mysqlPools, redisPools, mysqlRoutes, redisRoutes)
	if err != nil {
	    // validation failed
	}

	db, poolName, err := router.ResolveDB(ctx, "lobby:config")
	client, poolName, err := router.ResolveRedis(ctx, "session")

# Design Specification

See: docs/superpowers/specs/2026-04-17-datasource-routing-and-pools.md

Key constraints:
  - No hot-reload: routes and pools are loaded at startup only
  - Shared LPM: MySQL and Redis use the same routing algorithm
  - Startup validation: duplicate prefixes or missing pools cause errors
*/
