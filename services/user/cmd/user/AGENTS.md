# User Service

**Scope:** Authentication, session management, user data

## Entry Point

`main.go` initializes:
1. Config loader
2. Tracing
3. Router with pools
4. gRPC server
5. Etcd registration

## Routing Keys

```go
"user:session" // Redis - session tokens
"user:data"    // MySQL - user profiles
```

## Key Files

| File | Purpose |
|------|---------|
| `server.go` | gRPC handlers: Register, Login, GetUser |
| `ingress.go` | Gateway ingress adapter |
| `main.go` | Service initialization |

## Session Management

**Token Format:** 32-byte crypto/rand, hex encoded

**Redis Key:** `user:session:{token}`

**Value:** `userId:username`

**TTL:** 7 days (configurable)

### Login Flow

```
1. Verify credentials (bcrypt password)
2. Generate secure random token
3. Store in Redis with TTL
4. Return token to client
5. Gateway stores in WS session
```

### Logout/Kick Flow

```
1. Delete Redis key
2. Notify gateway (Redis pub/sub)
3. Gateway closes WS connection
```

## Database

**Table:** `users`

```sql
id INT PRIMARY KEY AUTO_INCREMENT
username VARCHAR(255) UNIQUE
password_hash VARCHAR(255)  -- bcrypt
nickname VARCHAR(255)
created_at TIMESTAMP
updated_at TIMESTAMP
```

Migrations: `services/user/migrations/`

SQLC queries: `internal/data/query/`
Generated code: `internal/data/sqlc/`

## Running

```bash
go run ./services/user/cmd/user
# Port: 9000 (gRPC)
```

## Testing

```bash
go test ./services/user/cmd/user/... -v
```

Key tests: ingress handling, auth flow, session expiration
