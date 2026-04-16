# 数据源路由（`routing_key` + LPM）实现计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development（推荐）或 superpowers:executing-plans，按 Task 逐步实现。步骤使用 `- [ ]` 勾选。  
> **设计依据**：[多数据源路由与连接池规格](../specs/2026-04-17-datasource-routing-and-pools.md)（**§3 LPM**、**§5 策略 A**、**§11 实现约束**）。

**Goal:** 在 **`pkg/`** 内实现 **可复用** 的 **`routing_key` → 命名池** 解析（**MySQL / Redis 共用 LPM 算法**），启动时 **校验 + 建池**，运行期 **只读快照、不热更**；首版以 **User 服务** 可迁移接入为验收目标（保持行为等价或显式配置新 YAML）。

**Architecture:**  
- **纯库**：`resolvePoolName(routing_key) → (pool_name, matched_prefix, err)` 与 **§3** 一致；`mysql_routes` / `redis_routes` **各一份规则切片**，**同一套** 扫描代码（避免复制）。  
- **Router**：持 `map[string]*sql.DB` 与 `map[string]redis.UniversalClient`（或项目统一 Redis 类型）+ 两份规则；**`ResolveDB` / `ResolveRedis`** 先 LPM 再查表。  
- **配置**：用 **Go struct** 表达规格中的 `data_sources` 形状；**`pkg/conf` 可扩展嵌套字段**，或由 Router 的 **`NewRouterFromConfig(...)`** 接收已 `Unmarshal` 的结构，**避免** 在 `pkg/dsroute` 内依赖 viper（边界清晰）。

**Tech Stack:** Go 1.26、`database/sql`、`github.com/redis/go-redis/v9`、`github.com/go-sql-driver/mysql`、表测用 **`testing`** / 必要时 **`miniredis`**；与仓库 **AGENTS.md**（TDD、验证门闩）一致。

---

## 文件结构（计划创建 / 修改）

| 路径 | 职责 |
|------|------|
| `pkg/dsroute/`（包名实现时可定为 `dsroute` 或 `datasource`，须与 `import "database/sql"` 区分） | LPM、`Router`、`NewRouter` 校验、对外 `ResolveDB` / `ResolveRedis` |
| `pkg/dsroute/lpm.go` | `resolvePoolName` + 类型 `RouteRule{Prefix, Pool}` |
| `pkg/dsroute/lpm_test.go` | LPM、策略 A、`prefix: ""`、重复 `prefix` 冲突 |
| `pkg/dsroute/router.go` | 组装池 map、关闭顺序文档化 |
| `pkg/conf/conf.go`（及示例 YAML） | 可选嵌套 `data_sources`；或与 User 局部 struct 二选一（见 Task 5） |
| `services/user/cmd/user/main.go` | 用 `Router` 替换裸 `sql.Open` + 单 `redis.NewClient`（或双轨兼容） |
| `services/user/internal/...` | 数据访问处传入 **`routing_key`** 常量（如 `session`），调用 `Resolve*` |

---

### Task 1: LPM 核心（TDD）

**Files:**  
- Create: `pkg/dsroute/lpm.go`  
- Create: `pkg/dsroute/lpm_test.go`  
- Modify: `pkg/go.mod`（若新增测试依赖如 `sqlmock` / `miniredis` 则 `go mod tidy`；**`dsroute` 为现有 `github.com/CBookShu/kd48/pkg` module 子目录**，不必改 `go.work`）

- [ ] **Step 1（TDD）**：在 `lpm_test.go` 写 **`resolvePoolName`**（或包内小写 `resolve`）用例：  
  - 更长前缀优先（与规格 §3 示例一致）；  
  - 仅 `prefix: ""` + 若干非空前缀；  
  - **无候选** → `err != nil`；  
  - **同 `prefix` 两条不同 `pool`** → 启动校验语义（可在 `ValidateRoutes` 测）。  
- [ ] **Step 2**：运行 `go test ./pkg/dsroute/...`，预期 **红**。  
- [ ] **Step 3**：实现 `lpm.go`（`effective_len`、`HasPrefix`、`""` 恒候选）。  
- [ ] **Step 4**：`go test` 绿。  
- [ ] **Step 5**：Commit（建议：`feat(dsroute): LPM resolvePoolName`）。

---

### Task 2: `Router` 与启动校验

**Files:**  
- Create: `pkg/dsroute/router.go`  
- Create: `pkg/dsroute/router_test.go`（可用 `miniredis` + `sql.Open` + `sqlmock` 或内存 SQLite **仅当**不引入过重依赖；**优先** fake interface 测 `Resolve*` 调用 `resolvePoolName` 次数与返回池名）

- [ ] **Step 1（TDD）**：`NewRouter`：规则里 **`pool` 不在 pools map** → error；**重复 `prefix` 同表**且不同 `pool` → error。  
- [ ] **Step 2**：实现 **`Router`**：`ResolveDB(ctx, routingKey)`、`ResolveRedis(ctx, routingKey)`，返回 **客户端 + `pool_name` + `matched_prefix`（可选 debug）**。  
- [ ] **Step 3**：`go test ./pkg/dsroute/...`。  
- [ ] **Step 4**：Commit。

---

### Task 3: 从配置 DTO 建池（无业务进程）

**Files:**  
- Create: `pkg/dsroute/config.go`（`DataSourcesConfig`、`MysqlPoolSpec`、`RedisPoolSpec`、`RouteRule` 等，字段名与 [规格 §6](../specs/2026-04-17-datasource-routing-and-pools.md) 对齐）  
- Create: `pkg/dsroute/build.go`：`OpenPools(cfg) (*Router, error)` 或 `NewRouterFromDataSources(...)`

- [ ] **Step 1**：实现 **`sql.Open` + `SetMaxOpenConns` 等**（规格 §2.2；数值从配置读，缺省与现有 `user` 对齐或显式默认）。  
- [ ] **Step 2**：实现 **Redis** `NewClient` 循环建池。  
- [ ] **Step 3（TDD）**：错误 DSN / 错误 addr 时 `NewRouter...` 返回 wrapped error（可只测校验路径，Ping 集成测放 Task 6）。  
- [ ] **Step 4**：`go test` + Commit。

---

### Task 4: `go.work` / `pkg` module 依赖

**Files:**  
- Modify: `pkg/go.mod`（增加 `go-redis`、`mysql` driver **若** `dsroute` 直接 `Open`；或把 `Open` 留在 `main` 注入 `map` 进 `Router` 以保持 `dsroute` 仅依赖 `sql.DB` 接口——**更干净**，见下「可选简化」）

**可选简化（推荐）**：`pkg/dsroute` **只接受** `map[string]*sql.DB` 与 `map[string]redis.UniversalClient`，**不负责** `sql.Open`；`services/user/cmd/user` 负责 Open 后注入。则 Task 3 收缩为 **校验 + 组装 Router 构造函数**，连接创建留在 Task 5。计划执行时 **二选一** 并在首个 PR 描述中写清。

- [ ] **Step 1**：敲定 **Open 在 pkg 还是在 main**（与审阅者一致后不再混用）。  
- [ ] **Step 2**：`go work sync` / `go mod tidy`。  
- [ ] **Step 3**：Commit。

---

### Task 5: User 服务接入（验收）

**Files:**  
- Modify: `services/user/cmd/user/main.go`  
- Modify: `services/user/...`（`NewUserService`、session 相关调用点）  
- Modify: `pkg/conf/conf.go` + 示例 `config.yaml`（仓库内若无可提交 `services/user/config.example.yaml`）

- [ ] **Step 1**：YAML 增加 **`data_sources`**（含 `mysql_routes` / `redis_routes` 与 **`prefix: ""`** 指向 `default` 池，与规格 §6 一致）；**或** 保留扁平 `mysql`/`redis` 并由代码 **合成** 单规则 `"" → default`（仅 M0 迁移桥，须在 PR 说明 **后续删除桥**）。  
- [ ] **Step 2**：`main` 构建 `Router`，将 **`routing_key` 常量**（如 **`session`**）用于 Redis session 访问；MySQL 同理若需。  
- [ ] **Step 3（TDD）**：若有纯函数可测，补单测；否则 **集成**：`go test ./services/user/...` + 本地起服务 **Ping / 登录路径**（与 AGENTS.md 验证命令对齐）。  
- [ ] **Step 4**：Commit。

---

### Task 6: 文档与规格对齐

**Files:**  
- Modify: [规格](../specs/2026-04-17-datasource-routing-and-pools.md) 仅当发现 **§11 与实现不一致** 时小修（须另 PR 或同 PR 末尾单独 commit）。  
- Modify: `docs/README.md` 增加本计划一行（可选）。

- [ ] **Step 1**：在 `pkg/dsroute/doc.go` 或 README 片段说明 **`routing_key` 由调用方传入**、与规格链接。  
- [ ] **Step 2**：Commit。

---

## 验证命令（默认）

```bash
cd /Users/cbookshu/dev/temp/kd48/pkg && go test ./...
# Router 接入后：
cd /Users/cbookshu/dev/temp/kd48/services/user && go test ./...
```

根目录若有一键脚本，以 **AGENTS.md / 仓库 README** 为准。

---

## 批准记录（人类门闩）

- **状态**：待批准  
- **批准范围**：（手填，例如「Task 1～3 仅库」或「全文」）  
- **批准人 / 日期**：（手填）

---

## 与 Lobby / Gateway 的关系

- **Lobby / Gateway**：本计划 **不强制** 同 PR 接入；后续 PR 复用 **`pkg/dsroute`** 即可。  
- **依赖顺序**：先 **Task 1～2** 合并，再 User 或他服务接入，避免半成品进 `main`。
