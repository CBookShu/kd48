# lobby 服务数据源访问规范化重构

**日期**: 2026-04-23
**状态**: 待批准

## 背景

项目规范（AGENTS.md）要求所有 MySQL/Redis 连接必须通过 `dsroute.Router` 解析，禁止直接访问连接池 map。

当前 `user` 服务已正确遵循规范，但 `lobby` 服务存在以下违规：

| 文件 | 行号 | 违规代码 |
|------|------|----------|
| `services/lobby/cmd/lobby/main.go` | 126 | `mysqlPools["default"]` |
| `services/lobby/cmd/lobby/main.go` | 136 | `redisPools["default"]` |
| `services/lobby/cmd/lobby/server.go` | 45 | `s.mysqlPools[name]` |
| `services/lobby/cmd/lobby/server.go` | 54 | `s.redisPools[name]` |

## 目标

将 lobby 服务的数据源访问从不规范的直接访问改为通过 `dsroute.Router` 解析，与 user 服务保持一致的架构风格。

## 架构变更

### 改造前

```
main.go
  ├─ mysqlPools["default"] ──→ ConfigLoader
  ├─ redisPools["default"] ──→ ConfigWatcher
  └─ mysqlPools, redisPools ──→ lobbyService
```

### 改造后

```
main.go
  └─ dsroute.Router ──┬─→ ConfigLoader (routingKey: "lobby:config-data")
                      ├─→ ConfigWatcher (routingKey: "lobby:config-notify")
                      └─→ lobbyService
```

## 组件改动

### 1. `services/lobby/cmd/lobby/main.go`

- 初始化连接池后，创建 `dsroute.Router`
- 从 etcd 加载路由配置
- 将 Router 注入到 ConfigLoader、ConfigWatcher、lobbyService
- 启动路由配置监听（支持热更新）

参考 `services/user/cmd/user/main.go` 的实现模式。

### 2. `services/lobby/cmd/lobby/server.go`

- 移除 `mysqlPools` 和 `redisPools` 字段
- 改为持有 `*dsroute.Router`
- 删除 `getMySQLDB` 和 `getRedis` 方法
- 更新 `NewLobbyService` 构造函数签名

```go
type lobbyService struct {
    lobbyv1.UnimplementedLobbyServiceServer
    router *dsroute.Router
}

func NewLobbyService(router *dsroute.Router) *lobbyService {
    return &lobbyService{router: router}
}
```

### 3. `services/lobby/internal/config/loader.go`

- 改为持有 `*dsroute.Router` 和 `routingKey string`
- `LoadOne` 方法内部通过 `router.ResolveDB(ctx, routingKey)` 获取连接

```go
type ConfigLoader struct {
    router     *dsroute.Router
    routingKey string
    store      *ConfigStore
}

func NewConfigLoader(router *dsroute.Router, routingKey string, store *ConfigStore) *ConfigLoader {
    return &ConfigLoader{router: router, routingKey: routingKey, store: store}
}

func (l *ConfigLoader) LoadOne(ctx context.Context, name string) error {
    db, _, err := l.router.ResolveDB(ctx, l.routingKey)
    if err != nil {
        return fmt.Errorf("resolve db: %w", err)
    }
    // ... 后续逻辑不变
}
```

### 4. `services/lobby/internal/config/watcher.go`

- 改为持有 `*dsroute.Router` 和 `routingKey string`
- `Start` 方法内部通过 `router.ResolveRedis(ctx, routingKey)` 获取连接

```go
type ConfigWatcher struct {
    router     *dsroute.Router
    routingKey string
    loader     *ConfigLoader
    channel    string
}

func NewConfigWatcher(router *dsroute.Router, routingKey string, loader *ConfigLoader, channel string) *ConfigWatcher {
    return &ConfigWatcher{router: router, routingKey: routingKey, loader: loader, channel: channel}
}
```

## Routing Key 定义

| 组件 | Routing Key | 用途 |
|------|-------------|------|
| ConfigLoader | `lobby:config-data` | MySQL 配置数据读取 |
| ConfigWatcher | `lobby:config-notify` | Redis Pub/Sub 配置变更通知 |

需要在 etcd 路由配置中添加这两条规则，指向 `default` 连接池。

## 测试影响

需要更新的测试文件：

- `services/lobby/cmd/lobby/server_test.go`
- `services/lobby/internal/config/loader_test.go`
- `services/lobby/internal/config/watcher_test.go`

测试策略：创建测试用 `dsroute.Router`，配置指向测试数据库和 Redis。

## 风险

- **低风险**：改动范围明确，参考 user 服务已有实现
- **向后兼容**：路由配置需在部署前更新

## 验收标准

1. `grep -r "mysqlPools\[" services/lobby/` 无匹配
2. `grep -r "redisPools\[" services/lobby/` 无匹配
3. `go test ./services/lobby/...` 全部通过
4. `go build ./...` 成功
