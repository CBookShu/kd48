# Lobby 服务设计（活动域 + 策划配置管线）

> **状态**：已获「可以落盘」许可，brainstorming 收口落盘。  
> **关联**：根目录 [`spec.md`](../../../spec.md) §3～§6；[网关与后端连接设计](./2026-04-13-gateway-backend-connection-design.md)（Ingress、Etcd、客户端入口）；[路线图](./2026-04-13-kd48-roadmap.md)。  
> **日期**：2026-04-15  

---

## 1. 目标与非目标

### 1.1 目标

1. **Lobby 为活动域业务承接方**：任务、签到、排行赛等 **在 `api/proto` 的 Lobby 契约中定义 RPC**；进程 **无本地会话状态**，支持 **水平扩展**（多副本等价）。
2. **运行期能力**：Lobby 提供 **MySQL、Redis 客户端能力** 与统一可观测性（与 M0 基座一致方向）；**具体活动** 的表结构、Redis key、幂等策略由 **该活动实现自行决定**，本设计 **不** 统一活动内容的存储形态。
3. **策划配置管线**：产品使用 **有规则的 CSV**（字段名、类型、中文注释、校验规则等）维护配置；**打表工具** 校验后生成 **JSON**，与 **Go 结构体代码生成** 同源绑定；Lobby 运行时使用 **反序列化后的强类型结构**，不以 `map[string]any` 为主路径。
4. **配置权威与变更通知**：**MySQL** 持久化 **CSV 原文** 与 **生成后的 JSON**；**Redis** 仅用于 **变更推送**（如 Pub/Sub 或 Stream）；Lobby **不因轮询 MySQL 比较版本** 作为变更发现主路径。
5. **访问路径**：**客户端不直连 Lobby**；玩家流量经 **Gateway** 转 **Lobby gRPC**（与现有「唯一外入口」一致）。

### 1.2 非目标

- 不在本设计锁死 **某一类活动** 的 DDL、Redis 前缀或领域事件模型。
- 不把 **网关动态 IDL**、**多数据源 registry** 等全局演进项作为 Lobby 交付前提（见 `spec.md` §9）。
- **打表工具** 的具体实现语言、仓库位置：实现阶段再定；本设计只约束 **输入/输出与顺序语义**。

---

## 2. Lobby 与 Gateway 的职责边界

| 能力 | 建议落点 | 说明 |
|------|----------|------|
| 路由（WS/HTTP → 服务 / RPC） | Gateway | 外协议与多服务转发。 |
| 会话鉴权（Token、强踢、未认证期策略） | Gateway | 与 `spec.md` §3.3 一致；Lobby 信任 **网关注入的 gRPC Metadata**（如 `user_id`）。 |
| 授权（能否参加某活动、黑名单等） | Lobby | 活动域规则，依赖配置与存储。 |
| 限流 | **双层** | Gateway：连接/握手、粗 QPS、防探测。Lobby：与活动相关的细粒度配额（常配合 Redis）。 |
| 染色 / 灰度路由 | Gateway（+ Etcd/配置） | 决定流量进入哪类实例池；Lobby 不重复实现入站路由策略。 |

---

## 3. 拓扑与信任模型

```
客户端 ──WS/HTTP──► Gateway ──gRPC──► Lobby ──MySQL/Redis──► （各活动自选存储形态）
```

- **Lobby** 仅暴露 **gRPC**；**默认调用方** 为 Gateway。若未来存在 **服务间直连 Lobby**，须单独约定 **mTLS / 内网 ACL / Metadata**，本设计不禁止但 **当前未列为必须**。
- **身份**：Lobby **不** 重复实现面向玩家的 Redis Session 全链路校验；以 Gateway 已校验为前提，Lobby 侧可做 **必填身份字段校验** 与业务授权。

---

## 4. 策划配置：CSV → JSON → MySQL + Redis 通知

### 4.1 CSV 与表结构

- CSV 为 **有规则表**：含 **字段类型、字段名、中文注释** 等元信息约定（可由表头行、独立 schema 文件或工具约定表达，实现阶段细化）。
- 打表工具对 CSV 做 **类型与业务校验**，通过后生成 **JSON**。
- **可执行格式（M0，三行头 CSV，文档内可称 `sheet_v1`）**：CSV **三行头**（中文说明 / 变量名 / 类型）+ **第 4 行起数据行**；类型含 **`int32,int64,string,time`**（`time` 为 RFC3339 → JSON string）、**`T[]`（`|` 分隔，`string[]` 元素须 `''`/`""`）**、**`int32 = string` 等 map**；**`json_payload` 不含 `config_format_version`**；载荷根对象见实现计划 **§B**；MySQL 表、Redis 通知等同上引用。

### 4.2 MySQL 持久化（权威）

除 **`config_id`、`revision`、正文（`csv_text` / `json_payload`）、`created_at`** 外，增加 **`scope`、`title`、`tags`、`start_time` / `end_time`（生效窗，语义 startTime/endTime）**。**不** 用表内 `env`；**不** 用 `status`。**`config_id` 命名** 见 [实现计划 §C.1](../plans/2026-04-15-lobby-service-implementation-plan.md)（**不把时间写进 id**；生效靠 **`start_time`/`end_time` + `revision`**）。

| 概念 | 含义 |
|------|------|
| CSV 原文 | 产品侧产出的源文本，便于审计与 diff。 |
| JSON 载荷 | 校验通过后写入，供 Lobby `json.Unmarshal` 至 **生成的 Go 类型**。 |
| 版本 / revision | 单调递增或等价机制，用于并发更新与对账。 |
| **scope / title / tags / start_time / end_time** | **筛选与运营列表**；**生效时间只以这两列为准**，`config_id` 保持稳定。 |

### 4.3 Redis：仅用于「有变更」通知

- **禁止** 以「Lobby 周期性轮询 MySQL 比较版本」作为 **变更发现的主路径**。
- 推荐模式：**MySQL 为正文权威**；**Redis Pub/Sub** 或 **Redis Stream** 承载 **轻量通知**（例如 `config_id` + `version` / `revision`，正文不放或仅放 hash 摘要）。
- **打表工具写库顺序（必须）**：**先** 在事务内 **提交 MySQL**，**再** 向 Redis **发布通知**。若顺序颠倒，Lobby 可能在通知到达后读到未提交或旧数据。

### 4.4 Lobby 加载行为

1. **启动（bootstrap）**：每个副本 **至少一次** 从 MySQL 读取当前生效配置并构建内存只读快照（冷启动基线；**不属于**「轮询比较变更」）。  
2. **运行期**：订阅 Redis 通知 → 收到后按 **`config_id` + `version`**（或等价）**从 MySQL 拉取对应 JSON** → 校验 → `Unmarshal` 到生成结构 → **原子替换** 内存快照。  
3. **丢消息与重连**：实现阶段在「仅事件驱动」与「重连后对账一次 MySQL」之间选型；**不** 将高频轮询 MySQL 作为默认策略。若采用 **Stream + 消费者组**，可强化 **至少一次** 投递语义。

### 4.5 Go 代码生成与 JSON 契约

- **Go struct** 由打表/同 schema 流水线生成（CI 或 `make gen-config`），带 **`json` tag**，与 MySQL 中 JSON **字段名一致**。  
- **表结构或字段类型变更** 通常伴随 **重新生成 Go 与 Lobby 发版**；CSV/JSON 热更新主要解决 **数据行内容** 变更。若需跨版本兼容，须在活动或配置层显式引入 **版本分支或扩展字段** 策略。

---

## 5. 错误、观测与发现

- **gRPC Status + 业务错误码**：与 `spec.md` §4.3 方向一致；具体 Error Details 在 Lobby proto 或公共包中演进。  
- **OpenTelemetry**：Lobby 作为下游服务接入 trace；与 Gateway Root Span 衔接。  
- **服务发现**：Lobby 向 Etcd 注册（如 `kd48/lobby-service`），与 User 服务同一套路；Gateway 或 meta 中登记路由目标（与网关专题对齐）。

---

## 6. 数据结构与接口草图（评审用）

> 下列为 **M0 拟议形状**：用于判断 **配置 JSON 是否合理**、**gRPC 是否好接网关**、**Lobby 内部边界是否清晰**；实现时可微调命名，但 **语义** 不宜无协商漂移。细则仍以 [Lobby 实现计划](../plans/2026-04-15-lobby-service-implementation-plan.md) 为准。

### 6.1 配置 JSON（`json_payload` 载荷）

**根对象（存入 MySQL `json_payload`）**

| 字段 | JSON 类型 | 说明 |
|------|-----------|------|
| `config_id` | string | 逻辑配置 id，与 DB 行一致（与 `revision` 一并便于自检） |
| `revision` | number | 与 DB 行一致 |
| `data` | **array of object** | 每个元素对应 CSV 一条数据行；对象 **键 = CSV 第 2 行变量名** |

**不包含**：`config_format_version`。CSV→JSON **文法**不随条目标注；演进依赖 **工具/Lobby 发版**（与 §4.5 一致）。

**`data[]` 中一条记录（与根目录 `exp.csv` 列对齐的示例形状）**

| 键 | JSON 类型 | 来源列 / 类型行 |
|----|------------|-----------------|
| `note` | string | `string` |
| `amount` | number | `int32` |
| `tags` | array of string | `string[]` |
| `extra_map` | object（键为 string） | `int32 = string`；Go 侧可映射为 `map[int32]string` |

### 6.2 Go 侧拟议类型（示意，非已生成代码）

```go
// 信封：Lobby 进程内只读快照的顶层反序列化目标。
type LobbyConfigEnvelope struct {
	ConfigID string           `json:"config_id"`
	Revision int64            `json:"revision"`
	Data     []LobbySheetRow  `json:"data"` // 或 json.RawMessage + 二次解析，见实现计划
}

// 与当前示例 CSV 列一一对应；Task 7 可由打表工具改为生成此 struct。
type LobbySheetRow struct {
	Note      string            `json:"note"`
	Amount    int32             `json:"amount"`
	Tags      []string          `json:"tags"`
	ExtraMap  map[string]string `json:"extra_map"` // 或 map[int32]string + 自定义 UnmarshalJSON
}
```

**易用性说明**：业务 RPC 只读 **`LobbyConfigEnvelope` 指针 / 拷贝快照** 即可，**不** 再碰 `[]byte`；新增列 = 改 CSV + 再生 struct + 发版（与 §4.5 一致）。

### 6.3 对外 gRPC（`lobby.v1`，经网关 Ingress）

与 User 相同模式：**网关只调稳定 `gateway.v1.GatewayIngress/Call`**，`route` 指向 Lobby 的 **全方法名**；载荷为 **protojson** 与请求/响应 message 对应。

**M0 拟议服务（节选）**

```protobuf
service LobbyService {
  rpc Ping(PingRequest) returns (PingReply);
  // 后续：任务 / 签到 / 排行等在同一 service 上增量添加 RPC
}
message PingRequest { optional string client_hint = 1; }
message PingReply {
  string pong = 1;
  int64 config_revision = 2; // 可选：带回当前已加载 revision，便于客户端判断
}
```

**拟议 Ingress 路由表（WS `method` → 与网关 meta 对齐）**

| `IngressRequest.route` | 含义 |
|------------------------|------|
| `/lobby.v1.LobbyService/Ping` | 健康 / 联调 / 带回配置版本探测 |

**Metadata（网关 → Lobby，拟议）**

| Key | 说明 |
|-----|------|
| `x-user-id`（或仓库统一常量名） | 玩家 id；Lobby **授权**依赖它 |

（具体 key 名实现时与网关 `WrapIngress` 填入的 metadata 对齐，写进 proto 注释或 `pkg/` 常量。）

### 6.4 Lobby 内部接口边界（拟议，非对外 gRPC）

用于拆分 **配置加载** 与 **业务 RPC**，便于单测与替换实现：

```go
// 从 MySQL 拉取并解析为信封；由 bootstrap / Redis 通知触发。
type ConfigLoader interface {
	LoadLatest(ctx context.Context, configID string) (*LobbyConfigEnvelope, error)
	LoadRevision(ctx context.Context, configID string, revision int64) (*LobbyConfigEnvelope, error)
}

// 订阅 Redis；收到消息后调用 Loader 再原子替换快照。
type ConfigNotifier interface {
	Run(ctx context.Context, onReload func(ctx context.Context) error) error
}

// 业务层只读；gRPC handler 注入。
type ConfigSnapshot interface {
	Current() *LobbyConfigEnvelope // 或 (T, ok)，无配置时行为在实现计划 Task 4 约定
}
```

**易用性**：业务 handler **只依赖 `ConfigSnapshot`**，不依赖 Redis/MySQL 细节；**配置变更** 与 **活动逻辑** 解耦。

---

## 7. 风险与待实现阶段确认项

| 项 | 说明 |
|----|------|
| Pub/Sub 不持久 | 纯 Pub/Sub 在订阅断开期可能丢消息；须 **bootstrap + 重连策略** 或 **Stream/消费者组**。 |
| 多副本一致性 | 各副本独立订阅；拉取 MySQL 后本地快照应 **只读**；写仍经业务逻辑写回各自存储。 |
| 配置与代码版本错配 | JSON 新增字段但 Lobby 未发版时 `Unmarshal` 行为须约定（忽略未知字段 vs 报错）。 |

---

## 8. 文档自检（落盘时）

- 已消除与「Excel 为唯一载体」「Lobby 轮询 MySQL 发现变更」的冲突表述。  
- 「活动内容不定型」与「配置 CSV/JSON 管线」并存：**前者指活动运行时数据模型**，**后者指策划表驱动配置**。  

---

## 9. 后续步骤（流程门闩）

实现前须按 [`AGENTS.md`](../../../AGENTS.md) 另立 **`docs/superpowers/plans/YYYY-MM-DD-<feature>.md`** 并获 **计划批准**；开发遵循 **TDD** 与 **verification-before-completion**。
