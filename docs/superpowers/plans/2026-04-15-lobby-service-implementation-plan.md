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

- [ ] **Step 2**：表包含设计规格中的概念字段：`config_id`（或字符串主键）、`revision`、`csv_text`、`json_payload`、`updated_at`；索引便于按 `config_id`+最大 `revision` 查询。

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

- [ ] **Step 2**：定义首版 **示例** `LobbyGameConfig`（或生成物占位）与 MySQL `json_payload` 对齐；`json.Unmarshal` 失败时 **不替换** 旧快照并打错误日志（行为写进测试或注释）。

- [ ] **Step 3**：`Bootstrap(ctx)`：启动时查询当前 `config_id`（可从静态配置或环境读取）对应 **最大 `revision`** 一行，填充 `atomic.Value`。

- [ ] **Step 4**：`go test ./...` 通过。

- [ ] **Step 5**：Commit。

---

### Task 5: Redis 变更通知（非轮询 MySQL）

**Files:** `internal/config/watcher.go` 或与 loader 同包

- [ ] **Step 1（TDD）**：用 **miniredis** 或 fake：模拟 `PUBLISH` 后，watcher 触发 **一次** `LoadByConfigID`（可注入 `Loader` 接口并记录调用次数）；**禁止**在测试中依赖「定时轮询 MySQL」作为主触发。

- [ ] **Step 2**：实现 **Redis `SUBSCRIBE`**（或 **Stream + 消费者组** 若选型更偏可靠）；消息体为 **JSON**：`{"config_id":"...","revision":42}`（字段名在实现中固定并文档化）。

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

- [ ] 独立 CLI 或子模块：**CSV 校验 → JSON → 写 MySQL → Redis `PUBLISH`**；顺序 **先 MySQL 再 Redis**（与设计一致）。  
- [ ] **从同一 schema 生成 Go**（`make gen-config`），CI 校验生成物已提交。  
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
