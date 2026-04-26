# Lobby Service

**Scope:** Room management, check-in system, configuration hot-reload

## Entry Point

`main.go` initializes:
1. Config loader (YAML + env)
2. Tracing (OpenTelemetry)
3. Router (MySQL + Redis pools)
4. Config loader (MySQL-based hot-reload)
5. gRPC server with ingress handlers
6. Etcd registration

## Routing Keys

```go
routingKeyConfigData   = "lobby:config-data"   // MySQL
routingKeyConfigNotify = "lobby:config-notify" // Redis pub/sub
```

## Key Services

| Service | File | Purpose |
|---------|------|---------|
| Checkin | `checkin_server.go` | Check-in logic, rewards |
| Item | `item_server.go` | Virtual items, inventory |
| Ingress | `ingress.go` | Gateway protocol adapter |

## Config Hot-Reload

```go
// 1. Config loaded from MySQL via ConfigLoader
// 2. Redis pub/sub notifies of changes
// 3. ConfigStore atomically swaps config
// 4. No service restart required
```

See `internal/config/` for implementation.

## Database Schema

Tables:
- `lobby_config_revision` - Config versions with JSON payload
- `lobby_checkin` - Check-in records
- `lobby_item` - Item definitions

Migrations: `services/lobby/migrations/`

## Running

```bash
go run ./services/lobby/cmd/lobby
# Port: 9001 (gRPC)
```

## Testing

```bash
go test ./services/lobby/cmd/lobby/... -v
```
