# 多数据源路由与连接池（MySQL / Redis）

> **状态**：已落盘（讨论结论 **Y**：使用 **抽象路由键 `routing_key`**，与真实 Redis key / SQL 表名 **解耦**）。  
> **关联**：根目录 [`spec.md`](../../../spec.md) §2；各服务 `config.yaml`、[`pkg/`](../../../pkg/) 公共能力演进。  
> **日期**：2026-04-17  

---

## 1. 目标

1. **MySQL、Redis 在发起查询/命令之前**，须先根据 **`routing_key`** 解析到 **某一命名连接池**（各自维护 `database/sql` 与 go-redis 的 **进程内池**）。  
2. **不同数据域** 可落到 **不同 DSN / Redis 端点**（多库、读写分离预留、按域隔离等）。  
3. **`routing_key` 与规则 `prefix` 的匹配**：采用 **最长前缀匹配（LPM）**；规则可重叠时 **更长 `prefix` 优先**。  
4. 本文档只约束 **语义与配置形状**；具体包名、函数签名以实现阶段 `pkg/` 为准，但 **不得违背** 下文接口语义。

---

## 2. 核心概念

### 2.1 `routing_key`（抽象路由键，选型 **Y**）

- **定义**：由 **业务或基础设施层** 传入的 **逻辑字符串**，表示「本次访问所属数据域 / 管线」，例如 `session`、`lobby:config`、`analytics:rollup`。  
- **刻意不是**：Redis 里最终的 `user:session:{token}` 等 **物理 key**（物理 key 仍由各模块自行拼接）。  
- **规范化**（解析前）：`strings.TrimSpace`；**大小写敏感**（规格默认；若服务统一小写须在服务内文档写明）。

### 2.2 命名池（`pool_name`）

- **MySQL**：每个命名池对应 **一个 `*sql.DB`**（即 **一个 DSN + 一套池参数**：`SetMaxOpenConns` / `SetMaxIdleConns` / `ConnMaxLifetime` 等）。  
- **Redis**：每个命名池对应 **一个 go-redis 客户端实例**（`redis.Client` 或 `UniversalClient`，即 **一个地址/Cluster 配置 + 一套池语义**）。  
- **进程级单例**：每个 `pool_name` **启动时初始化一次**（或懒加载一次后缓存）；**禁止** 每次查询新建 TCP 连接而不走池。

### 2.3 路由规则（前缀 → 池）

- 配置为 **有序列表**（展示顺序 **仅用于人类阅读**；匹配语义 **以 LPM 为准**，见 §3）。  
- 每条规则：`prefix`（非空字符串）+ `pool`（命名池 id）。  
- **含义**：若 `routing_key` 以某规则的 `prefix` 为前缀，则该规则 **候选**；在 **所有候选中取 `prefix` 最长** 的一条，其 `pool` 即为解析结果。

---

## 3. 最长前缀匹配（LPM）

**算法（规范级）**（与 Go `strings.HasPrefix(routing_key, prefix)` 一致）：

1. 收集所有满足 **`HasPrefix(routing_key, rule.prefix)`** 的规则。  
   - **注意**：`prefix == ""` 时 **恒为候选**（因 `HasPrefix(s, "") == true`），其长度为 0；**只要存在更长 `prefix` 的候选，LPM 必不选空串规则**，故可将 **`prefix: ""`** 用作 **兜底池**（策略 B）。  
2. 若无候选：**见 §5 未命中策略**。  
3. 若有候选：取 **`len(prefix)` 最大** 的一条；若仍并列（同长度多条），**启动校验失败**（配置非法，拒绝进程启动或拒绝加载该段配置）。

**示例**

| `routing_key` | 规则（prefix → pool） | 结果池 |
|---------------|------------------------|--------|
| `lobby:config:notify` | `lobby:`→`redis_a`，`lobby:config:`→`redis_b` | `redis_b` |
| `lobby:other` | 同上 | `redis_a` |
| `session` | `sess`→…（无）、`session`→`redis_session` | `redis_session` |

---

## 4. 接口语义（实现须等价）

以下为 **行为契约**；语言为 Go 时可为方法或函数，名称可调整。

### 4.1 Redis

- **`ResolveRedis(ctx, routing_key string) (client, pool_name string, err error)`**  
  - **前置**：`routing_key` 已规范化。  
  - **成功**：返回 **该命名池已建立的** Redis 客户端（连接池由客户端持有）、**实际选用的 `pool_name`**（便于日志与指标）。  
  - **失败**：未命中、配置缺失、池未初始化等 → `err != nil`，调用方 **不得** 静默降级到随机池。

### 4.2 MySQL

- **`ResolveDB(ctx, routing_key string) (db *sql.DB, pool_name string, err error)`**  
  - 语义与 Redis 对称：返回 **命名池对应的 `*sql.DB`**。  
  - **说明**：SQL 文本、表名仍由业务编写；**`routing_key` 只决定连哪条 DSN**，不解析 SQL。

### 4.3 与调用链的关系

- **网关 / User / Lobby** 等：在「某条业务管线」入口选定 **`routing_key`**（或从 RPC/metadata 映射），之后 **同一次请求内** 对该域的访问应 **一致地** 使用解析结果（可缓存在 `context` 或请求局部变量中，属实现细节）。  
- **禁止**：在不知道 `routing_key` 归属的情况下，直接持有全局单例 DB/Redis 执行写路径（M0 单池代码路径可保留为 **`default` 池 + 单条空 prefix 或显式 `routing_key="default"`**，见 §5）。

---

## 5. 未命中与默认池

**须在配置中二选一写死（服务级）**：

- **策略 A（严格）**：无候选规则 → `Resolve*` **返回错误**（推荐用于多租户已上线的服务）。  
- **策略 B（兼容 M0）**：声明 **`default_pool`**（MySQL / Redis 各一份或共用命名表）；无候选 → 使用 default；**须在日志中打 debug**（可选）以便发现漏配。

**推荐**：新服务用 **A**；从单池迁移时短期用 **B** 并监控 `routing_key` 分布。

---

## 6. 配置示例（形状级）

以下为 **说明性 YAML**；字段名实现可微调，但 **语义** 须一致。

```yaml
data_sources:
  mysql_pools:
    default: { dsn: "user:pass@tcp(mysql-main:3306)/kd48?parseTime=true", max_open: 20, max_idle: 5 }
    lobby:   { dsn: "user:pass@tcp(mysql-lobby:3306)/kd48_lobby?parseTime=true", max_open: 20, max_idle: 5 }
  redis_pools:
    default:   { addr: "redis-main:6379" }
    session:   { addr: "redis-session:6379" }
    lobby_cfg: { addr: "redis-lobby:6379" }

  mysql_routes:   # LPM：更长 prefix 优先
    - { prefix: "lobby:", pool: "lobby" }
    - { prefix: "", pool: "default" }   # 仅当采用策略 B；prefix 空表示兜底（长度 0，LPM 时最后考虑）

  redis_routes:
    - { prefix: "lobby:config:", pool: "lobby_cfg" }
    - { prefix: "session", pool: "session" }
    - { prefix: "", pool: "default" }
```

**校验（启动时）**：

- 每条规则的 `pool` 必须在对应 `*_pools` 中存在。  
- 若采用策略 B：**必须** 存在可命中 `routing_key=""` 的兜底或显式 `default_pool` 字段（实现二选一，须在服务 README 写清）。

---

## 7. 可观测性

- **日志**：解析成功后，建议在 **debug** 级打 `routing_key`、`matched_prefix`、`pool_name`（**禁止** 默认 info 打印 DSN/密码）。  
- **指标**（可选）：按 `pool_name` 分桶的连接等待、错误率，便于区分「哪条池」饱和。

---

## 8. 非目标（本文档不解决）

- **跨池事务**：不支持单事务跨 DSN；需业务层 Saga / 单写多读等另行设计。  
- **动态热增池**：首版允许 **仅重启加载**；热加载可作为后续演进。  
- **gRPC 连接池**（etcd resolver）：与本文 **无关**。

---

## 9. 自检清单

- [ ] 服务文档是否写明 **策略 A 或 B** 及 default 行为？  
- [ ] `routing_key` 的赋值点（网关 metadata / RPC 层 / 定时任务）是否可追溯？  
- [ ] LPM 并列同长是否在 **启动时** 被拒绝？
