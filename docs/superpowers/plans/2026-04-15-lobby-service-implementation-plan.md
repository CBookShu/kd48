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
- **配置**：MySQL 表（名在迁移中最终确定）存 `config_id`、`revision`（或单调版本）、`csv_text`、`json_payload`（`LONGTEXT`/JSON）、`updated_at` 等；Lobby 内 **`atomic.Value` 或 `sync.RWMutex`** 持有只读快照。  
- **变更路径**：外部打表工具 **事务写 MySQL → `PUBLISH`（或 `XADD`）Redis**；Lobby **订阅** → 收到 `config_id`+`revision` → **单条 SELECT 拉 JSON** → 校验 → `json.Unmarshal` 到 **生成或手写的 struct**（首版可用 **一个** 示例 `LobbyGameConfig` 占位，字段与 JSON 对齐）。  
- **网关**：Etcd meta 增加 `kd48/lobby-service` 的 `ServiceType` + 至少一条 `WsRouteSpec`（`IngressRoute` 指向 `/lobby.v1.LobbyService/…`）；`seed-gateway-meta` 或等价种子更新。  
- **`go.work`**：追加 `./services/lobby`。

**Tech Stack:** 与 User 服务对齐：Go 1.26、`golang-migrate`、`sqlc`（若本计划引入查询则加 `sqlc.yaml`；否则手写极小 DAL + 单测）、`go-redis/v9`、OTel、`pkg/registry`、`pkg/conf` 模式。

---

## 配置与消息格式规范（M0 可执行约定）

本节把设计文档里「有规则 CSV / MySQL 双存 / Redis 通知 / Go 强类型」落实为 **实现与打表工具可照着做** 的固定格式；若日后要扩展，须 **递增 `config_format_version`** 或另起 `config_id`，避免静默破坏兼容。

### A. 策划 CSV（写入 MySQL 的 `csv_text` 原文）

**编码与分隔**

- 文件：**UTF-8**（**不推荐**带 UTF-8 BOM，避免部分工具列偏移）；换行 **LF**。  
- 分隔符：**逗号 `,`**；若单元格内含逗号、双引号或换行，按 **RFC 4180** 用双引号包裹字段。  
- 空行：在 `##SCHEMA` / `##DATA` 段之间 **允许最多一行空行**；解析器应 **trim** 后识别段标记。

**逻辑结构：单文件双段（固定标记行）**

第一段 **`##SCHEMA`**（机器可读元数据，供打表工具校验与生成 Go / JSON；**Lobby 运行时不必解析 CSV**，只存原文备审计）。

| 列序 | 列名（表头字面量） | 含义 | 取值约束 |
|------|-------------------|------|----------|
| 1 | `name` | 字段在 **DATA 段表头** 中使用的列名（建议与 JSON / Go 字段一致，小写蛇形或 camel 二选一应全仓统一，**M0 约定：小写蛇形 `snake_case`**） | `[a-z][a-z0-9_]*` |
| 2 | `go_type` | 生成 Go 字段类型 | 白名单：`string`、`int`、`int32`、`int64`、`bool`、`float64`（首版）；扩展须改打表工具白名单 |
| 3 | `json_name` | JSON 中的 **键名**（与 `encoding/json` 的 tag 一致） | 非空，建议与 `name` 相同或与 proto/json 约定对齐 |
| 4 | `required` | 是否必填 | `0` / `1` |
| 5 | `comment_zh` | 中文注释（给人看） | 任意 UTF-8；若含逗号须 RFC 4180 引号 |

`##SCHEMA` 段内 **第一行必须为上述表头**；后续每行描述一个字段。

第二段 **`##DATA`**（产品填数）。

- **第一行**：表头，列名集合 **必须是** `##SCHEMA` 中声明的 `name` 集合的 **子集或全集**（M0 要求：**与 SCHEMA 行数一致、顺序一致** 为推荐模式，便于打表工具 diff；若实现选择「子集」，须在打表工具中显式校验「未出现列的默认值规则」）。  
- **后续行**：每条数据记录一行；M0 若仅一条全局配置，可为 **单行数据**。

**最小示例（`csv_text` 存库内容可与此等价）**

```text
##SCHEMA
name,go_type,json_name,required,comment_zh
title,string,title,1,活动标题
max_level,int32,max_level,1,等级上限

##DATA
title,max_level
春季签到,60
```

**打表工具职责（与 Lobby 边界）**

- 校验：`go_type` / `required` / DATA 行列数与类型；失败则 **不写库**、不发 Redis。  
- 产出：`json_payload` 字节与（Task 7）**Go struct** 源码；**Lobby 只消费 `json_payload`**。

---

### B. MySQL：`json_payload` 与强类型 Go 的 JSON 形状

**根对象（必须字段）**

| JSON 键 | 类型 | 说明 |
|---------|------|------|
| `config_format_version` | string | 配置 DSL 版本，**M0 固定 `"1"`** |
| `config_id` | string | 与表字段 `config_id` 一致 |
| `revision` | number | 与表字段 `revision` 一致（整数） |
| `data` | object | **实际策划参数**；首版 `LobbyGameConfig` 仅映射 `data` 内字段，避免与元数据键冲突 |

**示例（`json_payload`）**

```json
{
  "config_format_version": "1",
  "config_id": "global",
  "revision": 3,
  "data": {
    "title": "春季签到",
    "max_level": 60
  }
}
```

**Go 侧（M0 占位，可手写后与 Task 7 生成物对齐）**

- 定义外层 `LobbyConfigEnvelope`（含 `ConfigFormatVersion`、`ConfigID`、`Revision`、`Data LobbyGameConfig`）。  
- `json.Unmarshal`：**M0 约定允许未知字段**（不在 struct 上的键忽略），以减少「仅多一个键」导致整包失败；若产品要求 **严格模式**，在 Task 7 打表工具中加校验，而非 Lobby 默认 `DisallowUnknownFields`。  
- **`revision` / `config_id` 不一致**：以 **MySQL 行** 为准覆盖 envelope 再入库（打表工具保证）；Lobby 加载时若发现 JSON 内与行不一致，打 **Warn** 日志并以 **行数据** 修正内存 envelope（实现写进 Task 4）。

---

### C. MySQL 表结构（建议名与列，迁移时可微调但须同步本文档）

**表名（建议）**：`lobby_config_revision`

| 列名 | 类型 | 说明 |
|------|------|------|
| `id` | `BIGINT` PK AUTO_INCREMENT | 代理键 |
| `config_id` | `VARCHAR(64)` NOT NULL | 逻辑配置 id，如 `global`、活动包 id |
| `revision` | `BIGINT` NOT NULL | 单调递增（**每个 `config_id` 内**单调，由打表工具保证） |
| `csv_text` | `MEDIUMTEXT` NOT NULL | 策划 CSV 原文 |
| `json_payload` | `JSON` NOT NULL | MySQL 8 JSON；若环境限制可改为 `LONGTEXT` + 应用层校验 |
| `created_at` | `DATETIME(3)` 或 `TIMESTAMP(3)` | 写入时间，默认 `CURRENT_TIMESTAMP(3)` |

**约束与索引**

- `UNIQUE KEY uk_config_revision (`config_id`, `revision`)`  
- `KEY idx_config_latest (`config_id`, `revision` DESC)` — 便于 `SELECT … ORDER BY revision DESC LIMIT 1`

**查询约定（Lobby bootstrap / 通知后拉取）**

- **当前生效**：`WHERE config_id = ? ORDER BY revision DESC LIMIT 1`。  
- **按通知精确拉**：`WHERE config_id = ? AND revision = ?`（通知必带 `revision`，避免读到中间态）。

---

### D. Redis：通知频道与消息体（仅事件，不含正文）

**频道名（M0 固定）**

- `kd48:lobby:config:notify`  
- 多租户或极大量配置时再拆为 `kd48:lobby:config:notify:{config_id}`；M0 单频道足够，消息内带 `config_id`。

**消息体：单行 JSON UTF-8（一行一个完整 JSON 对象）**

| 键 | 类型 | 必填 | 说明 |
|----|------|------|------|
| `kind` | string | 是 | **M0 固定 `lobby_config_published`** |
| `config_id` | string | 是 | 与 MySQL 一致 |
| `revision` | number | 是 | 与刚写入 MySQL 的行一致 |
| `sha256` | string | 否 | `json_payload` 或整行 canonical 的校验（供对账，Lobby 可选用） |

**示例**

```json
{"kind":"lobby_config_published","config_id":"global","revision":3}
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
  config_id: "global"
  redis_notify_channel: "kd48:lobby:config:notify"
```

（字段名实现时可映射到 `pkg/conf` 结构体；若与现有 YAML 风格冲突，以 `services/user/config.yaml` 为命名参考微调，但 **语义** 不变。）

---

### F. Task 与格式的对应关系（补充）

| Task | 须落实的本节条目 |
|------|------------------|
| Task 2 | §C 表 DDL；§B `json_payload` 与索引 |
| Task 4 | §B envelope + `data`；§C 查询；§E `config_id` |
| Task 5 | §D 频道与消息 JSON；重连对账 |
| Task 7 | §A CSV 解析与校验；写 §C；发 §D |

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

- [ ] **Step 2**：按上文 **§C「MySQL 表结构」** 建表（列名、类型、`UNIQUE(config_id,revision)`、`idx_config_latest`）；与设计文档概念一致。

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

- [ ] **Step 2**：按 **§B** 实现 `LobbyConfigEnvelope` + `LobbyGameConfig`（映射 `data` 子对象）；`json.Unmarshal` 失败时 **不替换** 旧快照并打错误日志（行为写进测试或注释）。

- [ ] **Step 3**：`Bootstrap(ctx)`：启动时查询当前 `config_id`（可从静态配置或环境读取）对应 **最大 `revision`** 一行，填充 `atomic.Value`。

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

- [ ] 独立 CLI 或子模块：解析 **§A** CSV → 生成 **§B** `json_payload` → 按 **§C** 写 MySQL → 按 **§D** `PUBLISH`；顺序 **先 MySQL 再 Redis**（与设计一致）。  
- [ ] **从同一 SCHEMA 段生成 Go**（`make gen-config`），`json` tag 与 **§A `json_name`** 一致；CI 校验生成物已提交。  
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
