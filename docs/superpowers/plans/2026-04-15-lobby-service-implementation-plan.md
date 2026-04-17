# Lobby 服务实现计划

> **For agentic workers:** 按 [`AGENTS.md`](../../../AGENTS.md) 执行：**TDD**（先失败测试再实现）、**verification-before-completion**（声称完成前须跑通约定命令）。多 Task 建议按 **subagent-driven-development** 一 Task 一会话或子代理。  
> **设计依据**：[Lobby 服务设计](../specs/2026-04-15-lobby-service-design.md)。

**Goal:** 落地 Lobby：**无状态 gRPC 服务**（Etcd 注册 `kd48/lobby-service`）、**MySQL 存 CSV+JSON 配置权威**、**Redis 推送变更**（Lobby **不** 以轮询 MySQL 作变更主路径）、**启动 bootstrap 读 MySQL**；经 **Gateway `GatewayIngress`** 暴露至少一条可调用 RPC（M0 打通链路）；**打表工具与 CSV→Go 代码生成** 可在后续 Task 或独立仓库实现，本计划先预留 **强类型反序列化** 的接口与占位生成物。

**Scope 边界**：本计划 **不** 实现完整任务/签到/排行业务逻辑；**不** 锁死各活动 Redis/MySQL 模型。首版以 **服务骨架 + 配置加载管线 + Ingress 路由打通** 为主。

## 批准记录（人类门闩）

- **状态**：待批准  
- **批准范围**：（手填，例如「全文」或「Task 1～4」）  
- **批准人 / 日期**：（手填）  
- **TDD**：强制（若豁免须写理由）  
- **Subagent**：按任务拆分 / 本步单会话豁免（若豁免须写理由）  

---

**Architecture:**

- **进程**：`services/lobby`，显式 DI（`sql.DB`、`redis.UniversalClient`、配置、OTel），`grpc.NewServer` 注册 **`lobby.v1.LobbyService`**（首版最小 RPC）与 **`gateway.v1.GatewayIngress`**（按 `IngressRequest.route` 用 **protojson** 分发给 Lobby RPC，与 `services/user/cmd/user/ingress.go` 同模式）。  
- **配置**：MySQL 表（名在迁移中最终确定）存 `config_name`、`revision`（或单调版本）、`csv_text`、`json_payload`（`LONGTEXT`/JSON）、`updated_at` 等；Lobby 内 **`atomic.Value` 或 `sync.RWMutex`** 持有只读快照。  
- **变更路径**：外部打表工具 **事务写 MySQL → `PUBLISH`（或 `XADD`）Redis**；Lobby **订阅** → 收到 `config_name`+`revision` → **单条 SELECT 拉 JSON** → 校验 → `json.Unmarshal` 到 **生成或手写的 struct**（首版可用 **`LobbyConfigEnvelope` + `Data` 为 `[]LobbySheetRow`** 或等价，与 [策划 CSV 规格](../specs/2026-04-16-lobby-config-csv-and-tooling-spec.md) **§3** `json_payload` 中 **`data` 数组** 对齐）。  
- **网关**：Etcd meta 增加 `kd48/lobby-service` 的 `ServiceType` + 至少一条 `WsRouteSpec`（`IngressRoute` 指向 `/lobby.v1.LobbyService/…`）；`seed-gateway-meta` 或等价种子更新。  
- **`go.work`**：追加 `./services/lobby`。

**Tech Stack:** 与 User 服务对齐：Go 1.26、`golang-migrate`、`sqlc`（若本计划引入查询则加 `sqlc.yaml`；否则手写极小 DAL + 单测）、`go-redis/v9`、OTel、`pkg/registry`、`pkg/conf` 模式。

---

## 配置存储与通知（指引用规格）

**CSV（`sheet_v1`）、`json_payload`、空值默认、内置类型、Map 语法、打表工具流水线** 的 **单一信源** 为专项规格：
→ **[Lobby 策划 CSV 与打表工具规格](../specs/2026-04-16-lobby-config-csv-and-tooling-spec.md)**（含 **§4 配置示例与转换后的 JSON 示例**）。

下文 **§C 起** 保留 MySQL 表 **`lobby_config_revision`**、**§C.1** `config_name` 命名、**Redis（§D）**、**Lobby `config.yaml`（§E）** 及 **Task 映射（§F）**。Lobby / 打表实现解析 CSV、`json_payload` 时须与专项规格一致。

---

### C. MySQL 表结构（建议名与列，迁移时可微调但须同步本文档）

**表名（建议）**：`lobby_config_revision`（**一行 = 某 `config_name` 的某一版 revision**）。

**刻意不包含（已定案）**

- **`env` / 环境列**：**不同环境用不同数据库实例**，不在表内区分。  
- **`status` / 草稿发布列**：**不做**「配置状态机」；是否可上线由 **发布流程 + 写库时机** 控制，**Lobby 不负责**在库内切 draft/published。

| 列名 | 类型 | 作用（具体） |
|------|------|----------------|
| `id` | `BIGINT` PK AUTO_INCREMENT | 行代理键。 |
| `config_name` | `VARCHAR(64)` NOT NULL | **稳定配置名**；与 [策划 CSV 规格](../specs/2026-04-16-lobby-config-csv-and-tooling-spec.md) **§5.1** 文件名规范与推导一致，便于人读与检索。 |
| `revision` | `BIGINT` NOT NULL | **该 `config_name` 内**版本号（打表工具保证；推荐 `unix_millis`）。 |
| **`scope`** | `VARCHAR(64)` NOT NULL | **业务域**，用于筛选（如 `checkin`、`reward`、`rank`、`task`）；与 `config_name` 建议保持语义对齐，打表工具可做校验。 |
| **`title`** | `VARCHAR(256)` NULL | **列表/搜索用短标题**（人读），可与 CSV 内某展示名一致或独立维护。 |
| **`tags`** | `JSON` NULL | **标签 JSON 数组**（如 `["s3","pvp"]`），供 `JSON_CONTAINS` 或应用层筛选；无则 `NULL`。 |
| **`start_time`** | `DATETIME(3)` NULL | **生效起始**（口语 **startTime**）；`NULL` = 不限制。 |
| **`end_time`** | `DATETIME(3)` NULL | **生效结束**（口语 **endTime**）；语义钉死为 **`[start_time, end_time)` 左闭右开**（`end_time` 该时刻 **已失效**）；任一为 `NULL` 则该侧不限制；实现须写单测。 |
| `csv_text` | `MEDIUMTEXT` NOT NULL | 策划 CSV 原文（审计）。 |
| `json_payload` | `JSON` NOT NULL | 打表生成的 JSON（Lobby 主读）。 |
| `created_at` | `DATETIME(3)` NOT NULL DEFAULT CURRENT_TIMESTAMP(3) | 插入时间。 |

**约束与索引**

- `UNIQUE KEY uk_config_revision (`config_name`, `revision`)`  
- `KEY idx_config_latest (`config_name`, `revision` DESC)` — 取某配置最新版。  
- **`KEY idx_scope_config_rev (`scope`, `config_name`, `revision` DESC)`** — 按业务域 **枚举配置**、再取最新 revision。  
- **`KEY idx_scope_time (`scope`, `start_time`, `end_time`)`** — 按域 + **时间窗** 筛配置行。  
- （可选）在 `title` 上建 **FULLTEXT** — 仅当确实需要中文分词/全文再引入，M0 可只用 `LIKE` + `title` 非空约束。

**查询约定（Lobby bootstrap / 通知后拉取）**

- **按 name 取最新**：`WHERE config_name = ? ORDER BY revision DESC LIMIT 1`。  
- **按通知精确拉**：`WHERE config_name = ? AND revision = ?`。  
- **按域 + 当前时刻取候选集**（若 Lobby 需要）：`WHERE scope = ? AND (start_time IS NULL OR start_time <= NOW()) AND (end_time IS NULL OR NOW() < end_time)`（与上表 **左闭右开** 一致）再按业务规则取 `revision` 最大者等——**实现阶段写死一种语义**，避免「最大 revision」与「时间窗」混用产生歧义。

---

### C.1 `config_name` 命名规范（UpperCamelCase；**不含时间**）

**目的**：`config_name` **长期稳定**，不随档期、赛季、活动起止而改名（否则引用它的脚本、Lobby 配置键、以及未来通知/查询键都得跟着改）。**何时生效** 只由列 **`start_time` / `end_time`**（以及 **`revision`** 滚数据版本）表达，**禁止**把日期、赛季、周次等 **编码进 `config_name`**。

**推荐形态（与打表工具保持一致）**

```text
{ConfigName}
```

| 段 | 规则 | 示例 |
|----|------|------|
| `{ConfigName}` | UpperCamelCase，建议正则 `^[A-Z][A-Za-z0-9]*$`；由打表工具按文件名规范推导（见策划 CSV 规格 **§5.1**）。 | `CheckinDaily` |

**完整示例**

| `config_name` | 建议 `scope` | `title` 示例 | 说明 |
|-------------|--------------|--------------|------|
| `CheckinDaily` | `checkin` | 每日签到参数 | 生效窗用 **`start_time`/`end_time`**；换赛季仍用同一 `config_name` 亦可，靠 revision + 列。 |
| `RewardDoubleCard` | `reward` | 双倍卡活动 | **勿** 写成 `RewardDoubleCard202604` 这类带日期 name；档期只写在 **`start_time`/`end_time`** 与 `title`。 |

**校验（打表工具推荐）**

- **`config_name` 不得匹配日期/赛季类后缀**（可实现为：禁止 `20\d{2}` 等简单模式，或人工 code review + CI 名单）。  
- 可选：约定 `config_name` 前缀与 `scope` 对齐（如 `CheckinDaily` ↔ `checkin`），工具仅做弱校验或仅记录日志。

---

### D. Redis：通知频道与消息体（仅事件，不含正文）

**频道名（M0 固定）**

- `kd48:lobby:config:notify`  
- 多租户或极大量配置时再拆为 `kd48:lobby:config:notify:{config_name}`；M0 单频道足够，消息内带 `config_name`。

**消息体：单行 JSON UTF-8（一行一个完整 JSON 对象）**

| 键 | 类型 | 必填 | 说明 |
|----|------|------|------|
| `kind` | string | 是 | **M0 固定 `lobby_config_published`** |
| `config_name` | string | 是 | 与 MySQL 一致 |
| `revision` | number | 是 | 与刚写入 MySQL 的行一致 |
| `sha256` | string | 否 | `json_payload` 或整行 canonical 的校验（供对账，Lobby 可选用） |

**示例**

```json
{"kind":"lobby_config_published","config_name":"Global","revision":3}
```

**顺序（再次强调）**

1. `BEGIN` → `INSERT` MySQL → `COMMIT` 成功。  
2. 再执行 `PUBLISH kd48:lobby:config:notify '<json>'`。  
3. Lobby **禁止**用定时任务轮询 MySQL 比较变更；**允许**订阅重连后 **单次** `ORDER BY revision DESC LIMIT 1` 对账。

---

### E. Lobby `config.yaml`（运行时）

在 `services/lobby/config.yaml`（示例）增加 **仅与配置管线相关** 项，与现有 `ServerConf` / MySQL / Redis 并列，例如：

```yaml
lobby_config:
  config_name: "Global"
  redis_notify_channel: "kd48:lobby:config:notify"
```

（字段名实现时可映射到 `pkg/conf` 结构体；若与现有 YAML 风格冲突，以 `services/user/config.yaml` 为命名参考微调，但 **语义** 不变。）

---

### F. Task 与格式的对应关系（补充）

| Task | 须落实的本节条目 |
|------|------------------|
| Task 2 | §C 表 DDL；[策划 CSV 规格](../specs/2026-04-16-lobby-config-csv-and-tooling-spec.md) **§3** `json_payload` 与索引 |
| Task 4 | 策划 CSV 规格 **§3** envelope + `data`；§C 查询；§E `config_name` |
| Task 5 | §D 频道与消息 JSON；重连对账 |
| Task 7 | 策划 CSV 规格 **§2** CSV 解析与校验、**§5** 工具；写 §C；发 §D |

---

## 文件结构（创建 / 修改）

| 路径 | 职责 |
|------|------|
| `api/proto/lobby/v1/lobby.proto` | Lobby 业务契约（首版最小 RPC，如 `Ping`） |
| `gen_proto.sh` | 纳入 `lobby/v1/lobby.proto` |
| `services/lobby/go.mod` | Lobby 独立 module |
| `services/lobby/cmd/lobby/main.go` | 监听、gRPC 注册、Etcd 注册、生命周期 |
| `services/lobby/cmd/lobby/ingress.go` | `GatewayIngress` 分发 |
| `services/lobby/internal/config/…` | 配置快照、bootstrap、Redis 监听与 MySQL 拉取 |
| `services/lobby/migrations/…` | 配置表 DDL |
| `services/lobby/config.yaml`（示例） | 与 user 对齐的最小配置结构 |
| `go.work` | `use ./services/lobby` |
| `gateway/cmd/seed-gateway-meta/main.go`（或等价） | 注册 Lobby 服务类型与示例路由 |

---

### Task 1: `lobby.v1` Proto 与代码生成

**Files:** `api/proto/lobby/v1/lobby.proto`、`gen_proto.sh`、生成物 `*.pb.go`、`*_grpc.pb.go`

- [ ] **Step 1（TDD 前置）**：无需业务测试；完成后 `go build ./...`（`api/proto` 模块）。

- [ ] **Step 2**：新增 `lobby.proto`（`package lobby.v1`，`go_package` 与 user 风格一致），至少：

  - `rpc Ping(PingRequest) returns (PingReply);`  
  - 消息体含可选 `string detail` 等便于 Ingress 联调。

- [ ] **Step 3**：更新 `gen_proto.sh` 增加 `lobby/v1/lobby.proto`，执行 `bash gen_proto.sh`。

- [ ] **Step 4**：`cd api/proto && go build ./...` 通过。

- [ ] **Step 5**：Commit（建议信息：`feat(proto): add lobby.v1 minimal API`）。

---

### Task 2: MySQL 迁移（配置权威表）

**Files:** `services/lobby/migrations/000001_lobby_config.up.sql`、`.down.sql`（编号以仓库惯例为准）

- [ ] **Step 1（TDD）**：无迁移前可写 **集成测试跳过**（`testing.Short()`）或仅文档；迁移落地后补 **repository 单测**（Task 4）用 **sqlmock** 或嵌入式 DB。

- [ ] **Step 2**：按上文 **§C + §C.1** 建表：含 **`scope`、`title`、`tags`、`start_time`、`end_time`**；**不含** `env`、`status`；索引含 `idx_scope_config_rev`、`idx_scope_time`；`UNIQUE(config_name,revision)` 保留。

- [ ] **Step 3**：本地 `migrate up` 验证（命令与 `spec.md` / README 一致）。

- [ ] **Step 4**：Commit。

---

### Task 3: `services/lobby` 骨架与 gRPC 注册

**Files:** `services/lobby/go.mod`、`cmd/lobby/main.go`、LobbyService 实现文件（如 `cmd/lobby/server.go`）

- [ ] **Step 1（TDD）**：`lobby_test.go` 或 `server_test.go`：对 **LobbyService.Ping** 用 **bufconn** / 内存 gRPC 调一次，断言返回码与字段（先写红）。

- [ ] **Step 2**：实现 `Ping`（可返回固定 `pong` + 可选从配置快照读 `revision` 以证明 DI 接通）。

- [ ] **Step 3**：`main` 中加载 `config.yaml`、MySQL、Redis、OTel（照抄 user 的骨架并删减无关部分）、`grpc.NewServer` + `otelgrpc`、监听、`registry.Register`。

- [ ] **Step 4**：`go test ./...`（`services/lobby`）与 `go build` 本 module 通过。

- [ ] **Step 5**：`go.work` 加入 `./services/lobby`；根目录 `go work sync`（若项目使用）。

- [ ] **Step 6**：Commit。

---

### Task 4: 配置加载（Bootstrap + 强类型 JSON）

**Files:** `services/lobby/internal/config/store.go`、`loader.go` 等

- [ ] **Step 1（TDD）**：对 **LoadFromRow(JSON bytes) → struct** 写单测；对 **SELECT 最新 revision** 用 sqlmock。

- [ ] **Step 2**：按 [策划 CSV 规格](../specs/2026-04-16-lobby-config-csv-and-tooling-spec.md) **§3** 实现 `LobbyConfigEnvelope` + **`Data` 为对象数组**（如 `[]LobbySheetRow`，字段与 CSV 第 2 行一致）；`json.Unmarshal` 失败时 **不替换** 旧快照并打错误日志（行为写进测试或注释）。

- [ ] **Step 3**：`Bootstrap(ctx)`：启动时查询当前 `config_name`（可从静态配置或环境读取）对应 **最大 `revision`** 一行，填充 `atomic.Value`。

- [ ] **Step 4**：`go test ./...` 通过。

- [ ] **Step 5**：Commit。

---

### Task 5: Redis 变更通知（非轮询 MySQL）

**Files:** `internal/config/watcher.go` 或与 loader 同包

- [ ] **Step 1（TDD）**：用 **miniredis** 或 fake：模拟 `PUBLISH` 后，watcher 触发 **一次** `LoadByConfigID`（可注入 `Loader` 接口并记录调用次数）；**禁止**在测试中依赖「定时轮询 MySQL」作为主触发。

- [ ] **Step 2**：实现 **Redis `SUBSCRIBE`** 至 **§D** 频道；解析 **§D** 消息 JSON（校验 `kind == lobby_config_published`）；非法消息打日志并丢弃。

- [ ] **Step 3**：收到通知后 **仅从 MySQL 拉取** 对应 `revision`（或拉最大 revision 并比对）；成功后 **原子替换** 快照。

- [ ] **Step 4**：**重连**：订阅断开时退避重连；重连成功后 **可选** 执行一次 **全量 reload**（对账，非高频轮询）。

- [ ] **Step 5**：`go test ./...`；Commit。

---

### Task 6: `GatewayIngress` 与网关元数据种子

**Files:** `services/lobby/cmd/lobby/ingress.go`、`ingress_test.go`、`gateway/cmd/seed-gateway-meta/main.go`

- [ ] **Step 1（TDD）**：`ingress_test.go`：`/lobby.v1.LobbyService/Ping` 路径下 protojson 往返（先红后绿）。

- [ ] **Step 2**：`RegisterGatewayIngressServer`，`switch route` 分发至 `LobbyService.Ping`。

- [ ] **Step 3**：`main.go` 注册 Ingress + LobbyService。

- [ ] **Step 4**：更新 **seed-gateway-meta**：新增 `kd48/lobby-service` 类型（`UseGatewayIngress: true`）、一条 WS 路由指向 `Ping` 的 `IngressRoute`。

- [ ] **Step 5**：`go test ./...`（至少 `services/lobby`、`gateway` 受影响部分）；`go build` gateway + lobby。

- [ ] **Step 6**：Commit。

---

### Task 7（可选 / 后续）：打表工具与 Go 结构代码生成

- [ ] 独立 CLI 或子模块：按 [策划 CSV 规格](../specs/2026-04-16-lobby-config-csv-and-tooling-spec.md) **§2** 解析三行头 CSV → **§3** 生成 `json_payload` → 按 **§C** 写 MySQL → 按 **§D** `PUBLISH`；顺序 **先 MySQL 再 Redis**（与设计一致）。  
- [ ] **从三行头生成 Go**（`make gen-config`）：第 2 行 → 字段名与 **`json` tag**；第 3 行 → Go 类型（含 `[]T`、`map[...]...` 等与 JSON 形状映射）；CI 校验生成物已提交。  
- [ ] 本 Task **可与 Lobby 运行时开发并行**，不阻塞 Task 1～6 的 M0 打通。

---

## 验证命令（默认）

在仓库根（`go.work`）：

```bash
go test ./...
```

受影响模块单独：

```bash
cd services/lobby && go test ./...
cd gateway && go test ./...
cd api/proto && go build ./...
```

---

## 与设计的可追溯映射

| 设计规格 § | 本计划 Task |
|------------|-------------|
| §3 拓扑、仅经 Gateway | Task 6 |
| §4.2 MySQL 权威 | Task 2、4 |
| §4.3 Redis 仅通知 | Task 5 |
| §4.4 bootstrap + 事件拉取 | Task 4、5 |
| §4.5 强类型 JSON | Task 4、7 |
| §5 观测与 Etcd | Task 3 |

---

## 备注

- 人类批准本计划后，实现会话须在文首 **批准记录** 中更新状态与范围。  
- 若网关侧 **AtomicRouter** 对多 target 已有完整支持，Task 6 仅需种子数据；若缺项，在 Task 6 中 **显式列出差额文件** 并修补（不静默跳过）。
