# Gateway WebSocket Module

**Scope:** WebSocket connection lifecycle, protocol conversion, heartbeat management

## Key Components

| File | Purpose |
|------|---------|
| `connection_manager.go` | Connection registry, heartbeat monitoring, user session mapping |
| `handler.go` | WS message routing, auth verification, request/response wrapping |
| `heartbeat.go` | Heartbeat timeout detection and cleanup |
| `atomic_router.go` | Thread-safe route table updates from Etcd |
| `wrapper.go` | gRPC stream wrapping for bidirectional communication |

## Architecture

```
Client WS ←→ ConnectionManager ←→ Handler ←→ gRPC → Backend Services
                ↓
          HeartbeatManager (30s interval)
                ↓
          User Session Mapping (for kick functionality)
```

## Critical Patterns

**Connection Registration:**
- Each connection assigned unique `clientID`
- Maps: `clientID→conn` and `userID→clientID` for session invalidation
- All map access protected by `sync.RWMutex`

**Heartbeat Protocol:**
- Server sends heartbeat every 30s
- Client must respond within `ServerTimeout` (90s default)
- Missing 3 heartbeats = connection closed

**Session Invalidation (Kick):**
```go
// 1. Delete Redis session key
// 2. Lookup userID → clientID mapping
// 3. Force close connection via ConnectionManager
```

## Thread Safety

- `ConnectionManager.connMu` guards all connection maps
- `AtomicRouter` uses `atomic.Value` for route table
- Handler executes per-connection goroutine

## Testing

```bash
go test ./gateway/internal/ws/... -v
```

Key tests: concurrent connections, heartbeat timeout, session invalidation
