# dbpool 完善设计方案

## 批准记录（人类门闩）

- **状态**：已批准
- **批准范围**：全文
- **批准人 / 日期**：用户 / 2026-04-17
- **TDD**：强制
- **Subagent**：按任务拆分

---

## 1. 目标

完善数据库连接池管理，实现：
1. 扩展连接池配置项（生命周期、超时控制）
2. 路由配置运行时动态更新
3. 利用 etcd 存储和监听路由配置

---

## 2. 现状问题

| 问题 | 当前状态 | 影响 |
|------|----------|------|
| 缺少连接生命周期配置 | 仅 `MaxOpen`/`MaxIdle` | MySQL 8小时超时断开 |
| 缺少超时控制 | 无 `DialTimeout`/`ReadTimeout` | 连接hang住无感知 |
| 路由配置静态 | 启动时从文件加载 | 无法运行时调整 |
| 无连接健康检查 | MySQL 启动时未 `Ping` | 失败连接未被检测 |

---

## 3. 架构设计

### 3.1 整体架构

```
┌─────────────────────────────────────────────┐
│              services/user                  │
│  ┌─────────────┐      ┌─────────────────┐   │
│  │ etcd.Client │──────│   RouteLoader   │   │
│  │  (已有组件) │      │  - Get()        │   │
│  └─────────────┘      │  - Watch()      │   │
│                       └────────┬────────┘   │
│                                │            │
│                       ┌────────┴────────┐   │
│                       │     Router      │   │
│                       │  - 路由表(原子) │   │
│                       │  - ResolveDB()  │   │
│                       └─────────────────┘   │
└─────────────────────────────────────────────┘
                      │
                      ▼
              etcd (kd48/routing/*)
```

### 3.2 组件职责

| 组件 | 职责 | 依赖 |
|------|------|------|
| `RouteLoader` | 从 etcd 获取/监听路由配置 | `etcd.Client` |
| `Router` | 维护路由表，提供解析接口 | `RouteLoader` (初始化) |
| `dsroute.Config` | 连接池静态配置 | 无 |

---

## 4. Proto 定义

**文件**: `api/proto/dsroute/v1/routing.proto`

```protobuf
syntax = "proto3";
package dsroute.v1;

option go_package = "github.com/CBookShu/kd48/api/proto/dsroute/v1";

// 单条路由规则
message RouteRule {
  // 路由前缀，空字符串表示默认匹配
  string prefix = 1;
  // 目标连接池名称
  string pool   = 2;
}

// MySQL 路由配置
message MySQLRoutes {
  repeated RouteRule rules = 1;
}

// Redis 路由配置
message RedisRoutes {
  repeated RouteRule rules = 1;
}
```

**etcd 存储格式**:
- Key: `kd48/routing/mysql_routes`
- Value: `MySQLRoutes` JSON 序列化
- Key: `kd48/routing/redis_routes`
- Value: `RedisRoutes` JSON 序列化

---

## 5. 配置扩展

### 5.1 MySQL 连接池配置

```yaml
data_sources:
  mysql_pools:
    pool1:
      dsn: "root:root@tcp(localhost:3306)/kd48?parseTime=true&loc=Local"
      max_open: 25              # 最大连接数
      max_idle: 10              # 最大空闲连接
      conn_max_lifetime: 30m    # 连接最大生命周期
      conn_max_idle_time: 5m    # 空闲连接最大存活时间
```

### 5.2 Redis 连接池配置

```yaml
data_sources:
  redis_pools:
    pool1:
      addr: "localhost:6379"
      db: 0
      password: ""
      pool_size: 10             # 连接池大小
      min_idle_conns: 5         # 最小空闲连接数
```

---

## 6. 失败处理策略

| 场景 | 行为 | 日志级别 |
|------|------|----------|
| **启动时 etcd 连接失败** | 优雅退出（os.Exit(1)） | ERROR |
| **启动时 etcd 拉取空配置** | 优雅退出（os.Exit(1)） | ERROR |
| **运行时 Watch 断开** | 重连，保留最后一次配置继续服务 | WARN |
| **运行时 Watch 重连后拉取为空** | 保留旧配置，继续服务 | WARN |
| **运行时 Watch 收到非法 JSON** | 丢弃该次更新，保留旧配置 | ERROR |

---

## 7. 关键接口

### 7.1 RouteLoader

```go
type RouteLoader interface {
    // Get 从 etcd 获取当前路由配置
    Get(ctx context.Context) (*RoutingConfig, error)
    // Watch 监听配置变更，通过 callback 通知
    Watch(ctx context.Context, onChange func(*RoutingConfig)) error
}
```

### 7.2 Router 更新

```go
type Router struct {
    mysqlRoutes atomic.Value // []RouteRule
    redisRoutes atomic.Value // []RouteRule
    // ...
}

// UpdateRoutes 原子更新路由表（供 RouteLoader 回调调用）
func (r *Router) UpdateRoutes(mysql, redis []RouteRule)
```

---

## 8. 连接健康检查

启动时增加 MySQL `Ping` 验证：

```go
db, err := sql.Open("mysql", spec.DSN)
if err != nil {
    return fmt.Errorf("open db %q: %w", name, err)
}
if err := db.Ping(ctx); err != nil {
    return fmt.Errorf("ping db %q: %w", name, err)
}
```

---

## 9. 配置加载顺序

1. 解析本地 YAML 配置文件（连接池静态配置）
2. 创建 MySQL/Redis 连接池
3. **从 etcd 获取路由配置**
4. 创建 Router 并注入路由表
5. **启动 RouteLoader Watch 监听**
6. 进入服务状态

---

## 10. 边界情况

| 情况 | 处理 |
|------|------|
| etcd 中路由指向不存在的池 | 启动时报错退出；运行时保留旧配置 |
| 重复 prefix 冲突 | LPM 算法处理，同长度后覆盖前 |
| 路由表非常大 | 分页考虑（当前假设 <1000 条） |
| etcd 权限不足 | 启动时返回权限错误，退出 |

---

## 11. 测试要点

- [ ] RouteLoader Get 成功/失败场景
- [ ] RouteLoader Watch 断开重连
- [ ] Router 原子更新无竞争
- [ ] 启动时 etcd 失败优雅退出
- [ ] 运行时收到非法 JSON 不崩溃
- [ ] MySQL Ping 失败检测
- [ ] 连接池配置项正确传递

---

## 12. 后续扩展（非本期）

- 连接池指标暴露（Prometheus）
- 热更新连接池配置（非路由）
- 路由配置 Web UI 管理
