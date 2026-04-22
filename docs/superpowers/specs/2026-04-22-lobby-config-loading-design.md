# Lobby 配置加载设计

**日期**: 2026-04-22
**状态**: 已批准
**作者**: Claude

---

## 1. 概述

为 Lobby 服务实现配置加载与热更新机制，支持：
- 启动时从 MySQL 加载配置
- 运行时通过 Redis Pub/Sub 接收热更新通知
- 类型安全的配置访问（泛型 + 接口约束）
- 自动注册机制（init 函数）

---

## 2. 架构概览

```
┌─────────────────────────────────────────────────────────────────┐
│                        Lobby Service                            │
│                                                                 │
│  ┌─────────────┐    ┌─────────────────────────────────────────┐│
│  │   Loader    │───▶│              ConfigStore                 ││
│  │ (启动加载)   │    │         (全局单例)                       ││
│  └─────────────┘    │                                         ││
│                     │  stores: map[name]*TypedStore[T]         ││
│  ┌─────────────┐    │                                         ││
│  │   Watcher   │───▶│  TypedStore[CheckinDaily]               ││
│  │ (热更新)    │    │  TypedStore[CheckinReward]              ││
│  └─────────────┘    │  ...                                    ││
│                     └─────────────────────────────────────────┘│
│                                      ▲                          │
│                                      │ 自动注册                  │
│  ┌──────────────────────────────────────────────────────────┐   │
│  │ generated/checkin_daily/checkin.go                       │   │
│  │   init() { Store = Register[CheckinDaily](...) }         │   │
│  └──────────────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────────────┘
         │                                          ▲
         │                                          │
    ┌────┴────┐                              ┌──────┴──────┐
    │ MySQL   │                              │   Redis     │
    │lobby_   │                              │ kd48:lobby: │
    │config_  │                              │config:notify│
    │revision │                              └─────────────┘
    └─────────┘                                    ▲
                                                   │
                                          ┌────────┴────────┐
                                          │ Config-Loader   │
                                          │ (打表工具)       │
                                          └─────────────────┘
```

---

## 3. 核心接口

### 3.1 Config 基础接口

```go
// pkg/config/config.go
package config

// Config 基础接口，所有生成的配置必须实现
type Config interface {
    // ConfigName 配置名称
    ConfigName() string

    // ConfigData 配置数据（返回切片指针，用于类型推导）
    ConfigData() any
}
```

### 3.2 TypedStore 泛型存储

```go
// services/lobby/internal/config/store.go
package config

// TypedStore 类型安全的配置存储
type TypedStore[T any] struct {
    mu       sync.RWMutex
    snapshot *Snapshot[T]
}

// Snapshot 配置快照
type Snapshot[T any] struct {
    Revision int64
    Data     []T
    ParsedAt time.Time
}

// Get 返回快照（线程安全）
func (s *TypedStore[T]) Get() *Snapshot[T]

// Update 更新快照（内部使用）
func (s *TypedStore[T]) Update(revision int64, data []T)
```

### 3.3 ConfigStore 全局管理

```go
// services/lobby/internal/config/registry.go
package config

var (
    globalStore *ConfigStore
    once        sync.Once
)

// ConfigStore 管理所有配置
type ConfigStore struct {
    mu     sync.RWMutex
    stores map[string]any  // name → *TypedStore[T]
}

// GetStore 获取全局 Store（单例）
func GetStore() *ConfigStore

// GetTypedStore 获取类型安全的 Store（内部使用）
func (cs *ConfigStore) GetTypedStore(name string) any

// Register 注册配置（init 中调用）
func Register[T any](pkg base.Config) *TypedStore[T]
```

---

## 4. Config-Loader 生成的代码

### 4.1 代码模板

```go
// services/lobby/internal/config/generated/checkin_daily/checkin.go
package checkin_daily

import (
    "kd48/pkg/config"
    lobbyconfig "kd48/services/lobby/internal/config"
)

type CheckinDaily struct {
    Day         int   `json:"day"`
    RewardID    int64 `json:"reward_id"`
    RewardCount int   `json:"reward_count"`
}

// Package 实现 config.Config 接口
type Package struct {
    data []CheckinDaily
}

func (p *Package) ConfigName() string {
    return "checkin_daily"
}

func (p *Package) ConfigData() any {
    return &p.data
}

// Store 全局句柄（业务层直接使用）
var Store *lobbyconfig.TypedStore[CheckinDaily]

func init() {
    Store = lobbyconfig.Register[CheckinDaily](&Package{})
}
```

### 4.2 Config-Loader 工具修改点

修改 `tools/config-loader/internal/codegen/generator.go`：
1. 生成 `Package` 结构体，实现 `config.Config` 接口
2. 生成 `Store` 全局变量
3. 生成 `init()` 函数，调用 `Register[T]`
4. 导入 `kd48/services/lobby/internal/config` 路径

---

## 5. ConfigLoader（启动加载器）

```go
// services/lobby/internal/config/loader.go
package config

type ConfigLoader struct {
    db    *sql.DB
    store *ConfigStore
}

// LoadAll 加载所有已注册的配置
func (l *ConfigLoader) LoadAll(ctx context.Context) error {
    names := l.store.getRegisteredNames()
    for _, name := range names {
        if err := l.LoadOne(ctx, name); err != nil {
            slog.Warn("failed to load config", "name", name, "error", err)
            // 继续加载其他配置
        }
    }
    return nil
}

// LoadOne 加载单个配置
func (l *ConfigLoader) LoadOne(ctx context.Context, name string) error {
    // 1. 从 MySQL 读取最新版本
    query := `
        SELECT data, revision
        FROM lobby_config_revision
        WHERE config_name = ?
        ORDER BY revision DESC
        LIMIT 1
    `

    var data []byte
    var revision int64
    err := l.db.QueryRowContext(ctx, query, name).Scan(&data, &revision)
    if err != nil {
        return fmt.Errorf("query config %s: %w", name, err)
    }

    // 2. 获取对应的 TypedStore
    ts := l.store.GetTypedStore(name)
    if ts == nil {
        return fmt.Errorf("config %s not registered", name)
    }

    // 3. 解析 JSON 并更新 Store
    return l.parseAndUpdate(name, data, revision, ts)
}
```

---

## 6. ConfigWatcher（热更新订阅）

```go
// services/lobby/internal/config/watcher.go
package config

type ConfigWatcher struct {
    rdb     *redis.Client
    loader  *ConfigLoader
    channel string
}

const ConfigNotifyChannel = "kd48:lobby:config:notify"

// Start 启动订阅
func (w *ConfigWatcher) Start(ctx context.Context) {
    pubsub := w.rdb.Subscribe(ctx, w.channel)
    defer pubsub.Close()

    ch := pubsub.Channel()
    for {
        select {
        case <-ctx.Done():
            return
        case msg, ok := <-ch:
            if !ok {
                return
            }
            w.handleMessage(ctx, msg.Payload)
        }
    }
}

// handleMessage 处理热更新消息
func (w *ConfigWatcher) handleMessage(ctx context.Context, payload string) {
    var notify struct {
        ConfigName string `json:"config_name"`
        Revision   int64  `json:"revision"`
    }

    if err := json.Unmarshal([]byte(payload), &notify); err != nil {
        slog.Warn("invalid config notify message", "error", err)
        return
    }

    slog.Info("received config update",
        "config_name", notify.ConfigName,
        "revision", notify.Revision)

    if err := w.loader.LoadOne(ctx, notify.ConfigName); err != nil {
        slog.Error("failed to reload config",
            "config_name", notify.ConfigName,
            "error", err)
    }
}
```

---

## 7. 业务层使用

### 7.1 main.go

```go
package main

import (
    // 导入生成的配置包，触发 init() 自动注册
    _ "kd48/services/lobby/internal/config/generated/checkin_daily"
    _ "kd48/services/lobby/internal/config/generated/checkin_rewards"

    "kd48/services/lobby/internal/config"
)

func main() {
    // 初始化 Loader
    loader := config.NewConfigLoader(db, config.GetStore())

    // 启动时加载所有配置
    if err := loader.LoadAll(ctx); err != nil {
        log.Fatal("failed to load configs", "error", err)
    }

    // 启动 Watcher
    watcher := config.NewConfigWatcher(rdb, loader, config.ConfigNotifyChannel)
    go watcher.Start(ctx)

    // 启动服务...
}
```

### 7.2 CheckinService

```go
package checkin

import "kd48/services/lobby/internal/config/generated/checkin_daily"

func (s *CheckinService) GetDailyReward(day int) *checkin_daily.CheckinDaily {
    snap := checkin_daily.Store.Get()
    if snap == nil {
        return nil
    }
    for i, r := range snap.Data {
        if r.Day == day {
            return &snap.Data[i]
        }
    }
    return nil
}
```

---

## 8. 数据流

### 8.1 启动阶段

```
1. main.go import generated 包
2. 各 generated 包的 init() 执行
3. Register[T]() 注册到全局 ConfigStore
4. loader.LoadAll() 从 MySQL 加载所有配置
5. 解析 JSON，存入对应的 TypedStore[T]
```

### 8.2 运行时热更新

```
1. config-loader 打表完成
2. 发布到 kd48:lobby:config:notify
3. Watcher 收到消息
4. loader.LoadOne() 刷新指定配置
5. TypedStore.Update() 更新快照
6. 业务层下次 Get() 获取最新数据
```

---

## 9. 错误处理

| 场景 | 处理 |
|------|------|
| MySQL 连接失败 | 启动失败，记录错误 |
| 单个配置加载失败 | 警告日志，继续其他配置 |
| 配置未注册 | 警告日志，跳过 |
| Redis 订阅断开 | 自动重连（go-redis 内置） |
| 消息格式错误 | 警告日志，跳过 |

---

## 10. 文件结构

```
pkg/config/
└── config.go                    # Config 接口定义

services/lobby/
├── cmd/lobby/
│   └── main.go                  # 导入 generated 包，启动 Loader/Watcher
├── internal/
│   └── config/
│       ├── store.go             # TypedStore[T], Snapshot[T]
│       ├── store_test.go        # TypedStore 单元测试
│       ├── registry.go          # ConfigStore, Register[T]
│       ├── registry_test.go     # Registry 单元测试
│       ├── loader.go            # ConfigLoader
│       ├── loader_test.go       # Loader 单元测试
│       ├── watcher.go           # ConfigWatcher
│       ├── watcher_test.go      # Watcher 单元测试
│       └── generated/           # Config-Loader 生成
│           ├── checkin_daily/
│           │   └── checkin.go   # 实现 Config 接口 + init()
│           └── checkin_rewards/
│               └── rewards.go

tools/config-loader/
└── internal/codegen/
    └── generator.go             # 生成实现 Config 接口的代码
```

---

## 11. 与 Config-Loader 工具的协作

现有 `tools/config-loader` 已实现：
- CSV 解析 → JSON payload
- MySQL 写入 `lobby_config_revision` 表
- Redis 发布到 `kd48:lobby:config:notify`

**需要修改**：
- `codegen/generator.go`：生成 `Package` 结构体、`init()` 函数

**无需修改**：
- CSV 解析器
- MySQL 写入器
- Redis 发布器

---

## 12. 测试策略（TDD）

### 12.1 测试覆盖要求

每个组件必须有独立的单元测试，覆盖率目标 **≥80%**。

### 12.2 测试用例

#### TypedStore 测试 (`store_test.go`)

| 测试用例 | 描述 |
|----------|------|
| `TestTypedStore_Get_Nil` | 初始状态 Get 返回 nil |
| `TestTypedStore_UpdateAndGet` | Update 后 Get 返回正确数据 |
| `TestTypedStore_ConcurrentAccess` | 并发读写安全 |
| `TestTypedStore_UpdateReplaces` | 多次 Update 替换旧数据 |

#### ConfigStore/Registry 测试 (`registry_test.go`)

| 测试用例 | 描述 |
|----------|------|
| `TestRegister_CreatesTypedStore` | Register 创建 TypedStore |
| `TestRegister_SameNameIdempotent` | 相同名称重复注册幂等 |
| `TestGetStore_Singleton` | GetStore 返回单例 |
| `TestGetTypedStore_NotFound` | 未注册名称返回 nil |

#### ConfigLoader 测试 (`loader_test.go`)

| 测试用例 | 描述 |
|----------|------|
| `TestLoadOne_Success` | 成功加载单个配置 |
| `TestLoadOne_NotFound` | 配置不存在返回错误 |
| `TestLoadOne_NotRegistered` | 未注册配置返回错误 |
| `TestLoadOne_InvalidJSON` | JSON 解析失败返回错误 |
| `TestLoadAll_PartialFailure` | 部分失败继续加载其他 |
| `TestLoadAll_EmptyRegistry` | 空注册表无错误 |

#### ConfigWatcher 测试 (`watcher_test.go`)

| 测试用例 | 描述 |
|----------|------|
| `TestWatcher_ValidMessage` | 正确消息触发加载 |
| `TestWatcher_InvalidJSON` | 无效 JSON 记录警告 |
| `TestWatcher_MissingField` | 缺失字段记录警告 |
| `TestWatcher_ContextCancel` | Context 取消停止监听 |

#### Config 接口测试 (`pkg/config/config_test.go`)

| 测试用例 | 描述 |
|----------|------|
| `TestConfigInterface_Compliance` | 生成的代码实现接口 |

### 12.3 测试工具

- **Mock MySQL**: 使用 `go-sqlmock` 模拟数据库
- **Mock Redis**: 使用 `miniredis` 模拟 Redis
- **并发测试**: 使用 `sync.WaitGroup` 和 `goroutine`

### 12.4 测试优先原则

1. **先写测试，后写实现**
2. 每个公开函数必须有测试
3. 边界条件必须覆盖
4. 错误路径必须测试

---

## 13. 测试要点

- [ ] Config 接口实现验证
- [ ] TypedStore 并发读写安全
- [ ] init() 自动注册机制
- [ ] MySQL 加载配置
- [ ] Redis Pub/Sub 热更新
- [ ] 业务层类型安全访问
- [ ] 配置缺失时的降级处理
- [ ] 单元测试覆盖率 ≥80%
