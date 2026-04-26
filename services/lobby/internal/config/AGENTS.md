# Lobby Config Loader

**Scope:** Type-safe configuration loading from MySQL with hot-reload

## Overview

Loads lobby configurations (activities, items, etc.) from MySQL with:
- Reflection-based type registration
- Versioned config storage
- Redis pub/sub change notifications
- Atomic config updates

## Key Types

| Type | Purpose |
|------|---------|
| `ConfigLoader` | Loads config from MySQL via Router |
| `ConfigStore` | In-memory config cache with typed access |
| `TypedStore` | Generic wrapper for type-safe config retrieval |

## Usage

```go
// 1. Register config type
store.Register("activity_config", &ActivityConfig{})

// 2. Load from MySQL
loader.LoadOne(ctx, "activity_config")

// 3. Retrieve anywhere
activity := store.Get("activity_config").(*ActivityConfig)
```

## MySQL Schema

```sql
CREATE TABLE lobby_config_revision (
    id BIGINT PRIMARY KEY AUTO_INCREMENT,
    config_name VARCHAR(255) NOT NULL,
    data JSON NOT NULL,
    revision BIGINT NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    INDEX idx_config_name (config_name),
    INDEX idx_revision (config_name, revision DESC)
);
```

## Hot-Reload Flow

```
MySQL update → Redis pub → All lobby instances → 
ConfigLoader.LoadOne() → ConfigStore.Update() → Atomic swap
```

## Type Registration

Config types must be struct pointers with JSON tags:

```go
type ActivityConfig struct {
    Scope     string    `json:"scope"`
    Title     string    `json:"title"`
    Tags      []string  `json:"tags"`
    StartTime time.Time `json:"start_time"`
    EndTime   time.Time `json:"end_time"`
}
```

## Change Notifications

Redis channel: `config:change:{config_name}`

Payload: config name string
