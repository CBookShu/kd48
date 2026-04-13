# Gateway Etcd 元数据（类型 + 路由）与 Watch 实现计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 网关按 [网关与后端连接设计](../specs/2026-04-13-gateway-backend-connection-design.md) **§11** 从 Etcd 加载 **`ServiceTypeSpec` / `GatewayRouteSpec`（protojson）**，Bootstrap 后 **Watch `kd48/meta/`** 热更新；**§11.4** 任一侧 Range **物理无 key** 则 **Error + fail-fast**；Handler 按路由 **`public` / `establishes_session`** 替代后缀白名单。

## 批准记录（人类门闩）

- **状态**：待批准（追溯补记：本计划在首次实现前 **未** 按 `AGENTS.md` 执行计划批准 / TDD / subagent，后续变更须补批准）
- **批准范围**：（手填）
- **批准人 / 日期**：（手填）
- **TDD**：强制（新改动须从红测开始，除非用户当轮明文豁免）
- **Subagent**：按 Task 拆分（除非用户当轮明文豁免）

**Architecture:** `gateway/internal/bootstrap` 负责 Range、校验、**gRPC Dial 池**、组装 `WrapIngress`；`gateway/internal/ws` 提供 **`AtomicRouter`**（`atomic.Pointer` 不可变快照）供读路径无锁查询；**Manager** 在独立 goroutine 中 **Watch + debounce 全量重建**；旧 `ClientConn` 在替换后 **延迟 Close**（draining 简化版）。配置项挂在 **`gateway.meta_*`**。

**Tech Stack:** Go 1.26、`go.etcd.io/etcd/client/v3`、`google.golang.org/protobuf/encoding/protojson`、现有 gRPC + etcd resolver。

**规格锚点:** §11.4～§11.8、§11.10（物理空 key）、§11.12（完整落实）；§11.7 **`ws_method` 冲突**：**`route_id` 字典序较大者获胜**。

---

## 文件结构

| 路径 | 职责 |
|------|------|
| `api/proto/gateway/v1/gateway_route.proto` | 新增 **`establishes_session`**（登录/注册成功后置会话态） |
| `pkg/conf/conf.go` | `GatewayConf` 增加 meta 前缀字段 |
| `config.yaml` | 示例前缀 |
| `gateway/internal/ws/atomic_router.go` | `RouteBinding`、`AtomicRouter` |
| `gateway/internal/ws/handler.go` | 使用 `AtomicRouter`；鉴权 / 会话与 proto 字段对齐 |
| `gateway/internal/bootstrap/build.go` | Range、解析、校验、Dial、构建快照 |
| `gateway/internal/bootstrap/watch.go` | Watch 循环、debounce、断连重 Bootstrap |
| `gateway/cmd/gateway/main.go` | Bootstrap → Watch；关闭时 cancel |
| `cmd/seed-gateway-meta/main.go` | 开发用：写入最小类型 + 路由 |

---

## Task 1: Proto `establishes_session` + 生成代码

**Files:**
- Modify: `api/proto/gateway/v1/gateway_route.proto`
- Run: `gen_proto.sh`

- [x] **Step 1:** 在 `GatewayRouteSpec` 增加 `bool establishes_session = 8;`（注释：成功响应后网关将连接标为已认证，如 Login/Register）
- [x] **Step 2:** 执行 `bash gen_proto.sh`

---

## Task 2: 配置

**Files:**
- Modify: `pkg/conf/conf.go`
- Modify: `config.yaml`

- [x] **Step 1:** `GatewayConf` 增加 `MetaServiceTypesPrefix`、`MetaGatewayRoutesPrefix`（string，默认值与设计文档键空间一致）
- [x] **Step 2:** `config.yaml` 填入 `kd48/meta/service-types/` 与 `kd48/meta/gateway-routes/`

---

## Task 3: `AtomicRouter` + Handler

**Files:**
- Create: `gateway/internal/ws/atomic_router.go`
- Modify: `gateway/internal/ws/handler.go`

- [x] **Step 1:** 实现 `RouteBinding`（`Handler`、`Public`、`EstablishSession`）与 `AtomicRouter`（`Get`、`Store`）
- [x] **Step 2:** `NewHandler(tracer, *AtomicRouter)`；读路径：`Get(method)` → 无则 NotFound；未登录且非 `Public` 则 Unauthenticated；成功且 `EstablishSession` 则 `meta.isAuthenticated = true`；错误时若 `EstablishSession` 则 break（与旧 Login/Register 行为一致）
- [x] **Step 3:** `go test ./gateway/internal/ws/...`

---

## Task 4: Bootstrap 构建快照

**Files:**
- Create: `gateway/internal/bootstrap/build.go`
- Create: `gateway/internal/bootstrap/validate.go`（可选：校验函数单测）

- [x] **Step 1:** `Range` 两前缀；**`len(Kvs)==0` 返回可识别错误**（main 打 Error 并 `os.Exit(1)`）
- [x] **Step 2:** protojson 解析；§8.3 / §4.1：无效单条 **Warn + 跳过**；`STATELESS_LB`、`schema_version==1`、`grpc_etcd_target` 必填；`use_gateway_ingress` 须为 true
- [x] **Step 3:** 为每个有效 `type_key` `grpc.Dial` + `NewGatewayIngressClient`；路由引用未知 `service_type` 则跳过
- [x] **Step 4:** 同一 `ws_method` 多条：保留 **`route_id` 字典序最大** 的一条
- [x] **Step 5:** 返回 `*ws.AtomicRoutes`（或等价不可变结构）、`[]*grpc.ClientConn`、`revision := max(两 Header.Revision)`
- [x] **Step 6:** 包级单测：校验函数（无 Etcd）

---

## Task 5: Watch + main 接线

**Files:**
- Create: `gateway/internal/bootstrap/manager.go`（或 `watch.go`）
- Modify: `gateway/cmd/gateway/main.go`

- [x] **Step 1:** `Manager`：`Bootstrap(ctx)`、`Run(ctx)`；`Watch(ctx, "kd48/meta/", WithPrefix(), WithRev(rev+1))`；**debounce**（如 200ms）后全量 `Bootstrap`；Watch 错误或 channel 关闭则 **sleep 后重新 Bootstrap 并重建 Watch**
- [x] **Step 2:** 替换快照后 **延迟**（如 30s）`Close` 旧连接
- [x] **Step 3:** `main`：注册 resolver、`NewAtomicRouter`、`bootstrap.NewManager`、首次 `Bootstrap`、启动 `Run` 于后台；`signal` 时 `cancel` Watch
- [x] **Step 4:** `go test ./gateway/...` 与 `go build -o gateway.bin ./gateway/cmd/gateway`

---

## Task 6: Seed 工具

**Files:**
- Create: `cmd/seed-gateway-meta/main.go`

- [x] **Step 1:** 使用与 `config.yaml` 相同的 etcd endpoints（flag 或嵌入默认 `localhost:2379`）
- [x] **Step 2:** Put `kd48/meta/service-types/user` 与两条 `gateway-routes`（Login、Register），JSON 与 protojson 字段名一致，`establishes_session: true`

---

## Task 7: 文档与计划勾选

**Files:**
- Modify: `docs/superpowers/specs/2026-04-13-gateway-backend-connection-design.md`（§13 一行）
- Modify: `docs/README.md`（若需指向本计划）

- [x] **Step 1:** 设计文档 §13 增加「Etcd meta + Watch 已按本计划落地」类记录（若实现完成）

---

## 验证

1. 启动 etcd；运行 `seed-gateway-meta`；运行 `gateway`；WS 调用 Login/Register 仍成功。
2. `etcdctl del` 一条路由后（在实现 Watch 后）预期行为：debounce 后路由表更新（可手工观察日志）。
3. 空 meta 前缀下启动网关：**进程退出**、日志含 Error。
