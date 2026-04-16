# 多数据源路由与连接池（MySQL / Redis）

> **状态**：已落盘（讨论结论 **Y**：使用 **抽象路由键 `routing_key`**，与真实 Redis key / SQL 表名 **解耦**）。  
> **关联**：根目录 [`spec.md`](../../../spec.md) §2；各服务 `config.yaml`、[`pkg/`](../../../pkg/) 公共能力演进。  
> **日期**：2026-04-17  

---

## 1. 目标

1. **MySQL、Redis 在发起查询/命令之前**，须先根据 **`routing_key`** 解析到 **某一命名连接池**（各自维护 `database/sql` 与 go-redis 的 **进程内池**）。  
2. **不同数据域** 可落到 **不同 DSN / Redis 端点**（多库、读写分离预留、按域隔离等）。  
3. **`routing_key` 与规则 `prefix` 的匹配**：采用 **最长前缀匹配（LPM）**；规则可重叠时 **更长 `prefix` 优先**；另支持 **字面 `prefix: "*"`** 作为 **全集兜底**（见 **§2.4**），与 `prefix: ""` **二选一**。  
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
- 每条规则：`prefix`（普通非空前缀、**`""`**、或 **字面 `"*"`**，见 **§2.4**）+ `pool`（命名池 id）。  
- **含义（普通前缀）**：若 `routing_key` 以某规则的 `prefix` 为前缀，则该规则 **候选**；在 **所有候选中取用于 LPM 比较的长度最大** 的一条，其 `pool` 即为解析结果（**§3**）。

### 2.4 全集兜底：`prefix: "*"`（推荐书写）

- **动机**：运维上「默认库」用 **`*`** 比 **空字符串 `""`** 更直观；迁移时只改 **路由表里某条 `prefix` → `pool` 的映射**（或改 **`mysql_pools` / `redis_pools` 中某池的 DSN**）即可 **无代码变更** 切换目标库（在 **`routing_key` 集合不变** 的前提下）。  
- **语义**：`prefix` **恰好等于** 一个字符 **`*`**（ASCII `0x2A`）时，**不** 按字面 `HasPrefix(routing_key, "*")` 理解（否则几乎永不命中）。实现须在匹配前 **归一化**：将该规则视为 **「恒为候选」**，且参与 LPM 时 **`effective_len = 0`**（与 **`prefix: ""`** **同级**：更长具体前缀始终优先）。  
- **与 `prefix: ""` 互斥**：同一路由表（`mysql_routes` / `redis_routes` 各自）**禁止** 同时出现 **`""` 与 `"*"`** 两条兜底（启动失败）；**须二选一**，避免重复语义。  
- **禁止（v1）**：`lobby*`、`*:foo`、`**` 等 **glob** 或 **通配符嵌入**——除 **整格恰好为 `*`** 与 **`""`** 外，`prefix` 必须按 **字面前缀** 做 `HasPrefix`；若需后缀/中段匹配，**另开规格**，不混入本篇。

---

## 3. 最长前缀匹配（LPM）

**算法（规范级）**（普通前缀与 Go `strings.HasPrefix(routing_key, prefix)` 一致；**`"*"` / `""`** 见 **§2.4**）：

1. **归一化规则**：将 **`prefix == "*"`** 的规则在候选收集时视为 **恒匹配**，其 **LPM 比较长度** `effective_len = 0`。  
2. 收集候选：  
   - 对每条 **`prefix` 为普通字符串**（非 `""`、非 `"*"`）的规则：若 `HasPrefix(routing_key, prefix)` 则为候选，**`effective_len = len(prefix)`**。  
   - 对 **`prefix == ""`**：候选，`effective_len = 0`。  
   - 对 **`prefix == "*"`**：候选，`effective_len = 0`。  
3. 若无候选：**§5（策略 A）**：`Resolve*` **返回错误**。  
4. 若有候选：取 **`effective_len` 最大** 的一条。若 **最大 `effective_len`** 下仍有多条候选且 **`pool` 不一致**（实质多为 **相同 `prefix` 重复声明** 或 **`""` 与 `"*"` 并存**），**启动校验失败**（配置非法，拒绝进程启动或拒绝加载该段配置）。

**示例**

| `routing_key` | 规则（prefix → pool） | 结果池 |
|---------------|------------------------|--------|
| `lobby:config:notify` | `lobby:`→`redis_a`，`lobby:config:`→`redis_b` | `redis_b` |
| `lobby:other` | 同上 | `redis_a` |
| `session` | `sess`→…（无）、`session`→`redis_session` | `redis_session` |
| `misc:foo` | `lobby:`→A，`*`→`default` | `default`（`*` 的 `effective_len=0`，无更长前缀命中） |

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
- **禁止**：在不知道 `routing_key` 归属的情况下，直接持有全局单例 DB/Redis 执行写路径。M0 单池迁移：在路由表中 **显式** 配置 **`prefix: "*"` → `default` 池**（或 **`prefix: ""`**，**二选一**，见 §2.4），或仅为各业务线配置非空前缀规则，使 **任意** 会用到的 `routing_key` 在 LPM 下 **恒有候选**，避免触发 §5 错误。

---

## 5. 未命中策略（已定案：**策略 A**）

- **无任一规则与 `routing_key` 形成候选**（即：无 **§3** 所述任一候选；**不** 把未配置的 `*` 当作隐式存在）→ **`ResolveDB` / `ResolveRedis` 必须返回错误**；**禁止** 回落到未在路由表中声明的「隐式默认池」。  
- **与全集兜底的关系**：配置了 **`{ prefix: "*", pool: "default" }`** 或 **`{ prefix: "", pool: "default" }`**（**二者不得同表并存**，§2.4）时，**任意** `routing_key` 至少有一条候选；LPM 在存在 **更长具体前缀** 时仍优先具体规则。**未配置**兜底时，未命中任何前缀的 `routing_key` **无候选** → 报错。

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

  mysql_routes:   # LPM；策略 A：无候选即 Resolve 报错
    - { prefix: "lobby:", pool: "lobby" }
    - { prefix: "*", pool: "default" }   # 推荐：显式「默认库」；与 prefix:"" 二选一

  redis_routes:
    - { prefix: "lobby:config:", pool: "lobby_cfg" }
    - { prefix: "session", pool: "session" }
    - { prefix: "*", pool: "default" }
```

**校验（启动时）**：

- 每条规则的 `pool` 必须在对应 `*_pools` 中存在。  
- **禁止重复 `prefix`**：同一路由表内 **相同 `prefix` 值** 不得出现多条且指向 **不同** `pool`（配置非法，启动失败）。  
- **`""` 与 `"*"` 互斥**：同一路由表 **不得** 同时含 `prefix: ""` 与 `prefix: "*"`（启动失败）。  
- **可选**：若服务文档声明「任意 `routing_key` 必须可解析」，则启动时 **应** 校验路由表含 **`prefix: "*"`** 或 **`prefix: ""`**（二选一）或已枚举全部业务前缀。

---

## 7. 迁移与切换（建议）

- **只读切换新库**（`routing_key` 不变）：数据同步完成后，把 **命中该域的那条规则** 的 **`pool`** 从旧池名改为新池名（新池指向新 DSN），**重启或热加载路由** 即可；**无需**改业务代码。  
- **只改 DSN、不改规则**：在 **`mysql_pools` / `redis_pools`** 内把 **同一 `pool_name`** 的 endpoint 换成新库——同样可实现切库；与「改规则指向新 `pool_name`」二选一，由运维偏好决定（前者 **池名稳定**，后者 **可保留旧池做回滚副本**）。  
- **双写 / 灰度**：仅靠改 `prefix → pool` **不够**；须应用层或同步任务 **双写**、按用户/分片切读等，**超出**本文范围。  
- **`*` 作为默认池**：适合「少数前缀走专库，**其余全部** 进主库」；迁移专库时仍优先用 **更长 `prefix` 规则** 精确切走流量。

---

## 8. 可观测性

- **日志**：解析成功后，建议在 **debug** 级打 `routing_key`、`matched_prefix`、`pool_name`（**禁止** 默认 info 打印 DSN/密码）。  
- **指标**（可选）：按 `pool_name` 分桶的连接等待、错误率，便于区分「哪条池」饱和。

---

## 9. 非目标（本文档不解决）

- **跨池事务**：不支持单事务跨 DSN；需业务层 Saga / 单写多读等另行设计。  
- **动态热增池**：首版允许 **仅重启加载**；热加载可作为后续演进。  
- **gRPC 连接池**（etcd resolver）：与本文 **无关**。

---

## 10. 自检清单

- [ ] 服务文档是否写明 **策略 A（无候选即错）** 及是否配置 **`prefix: "*"` 或 `prefix: ""`**（二选一）作为显式兜底？  
- [ ] `routing_key` 的赋值点（网关 metadata / RPC 层 / 定时任务）是否可追溯？  
- [ ] LPM 并列、`""` 与 `"*"` 互斥是否在 **启动时** 被拒绝？
