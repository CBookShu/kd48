# Data Source Routing (dsroute)

**Scope:** Multi-pool MySQL/Redis connection routing via LPM (Longest Prefix Match)

## Overview

Central routing system for MySQL and Redis connection pools. Replaces direct pool access with routing key resolution.

## Key Types

| Type | Purpose |
|------|---------|
| `Router` | Main entry point, holds pools and routes |
| `RouteRule` | Prefix→Pool mapping with metadata |
| `ResolvePoolName()` | LPM algorithm implementation |

## Routing Key Convention

Format: `<service>:<domain>`

```go
"user:session"      // User service session data
"user:data"         // User service business data
"lobby:config-data" // Lobby config from MySQL
"lobby:config-notify" // Lobby config notifications via Redis
```

## Usage

```go
// ❌ FORBIDDEN: Direct pool access
db := mysqlPools["default"]

// ✅ CORRECT: Via Router
db, poolName, err := router.ResolveDB(ctx, "user:session")
if err != nil {
    return fmt.Errorf("resolve db: %w", err)
}
```

## Route Resolution (LPM)

1. Match routing key against all route prefixes
2. Select longest matching prefix
3. Return associated pool name
4. Fallback to "default" pool if no match

## Atomic Updates

```go
// Router.UpdateRoutes() - thread-safe route table swap
router.UpdateRoutes(mysqlRoutes, redisRoutes)
```

Used by gateway bootstrap when Etcd config changes.

## Configuration

Routes stored in Etcd:
- Key: `kd48/routing/mysql_routes`
- Key: `kd48/routing/redis_routes`

Initialize via `gateway/cmd/seed-gateway-meta`.

## Testing

```bash
go test ./pkg/dsroute/... -v
```

Includes LPM matching, route validation, integration tests.
