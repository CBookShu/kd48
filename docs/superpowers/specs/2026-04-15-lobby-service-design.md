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
- **可执行格式（M0）**：`##SCHEMA` / `##DATA` 分段、UTF-8、RFC 4180、`json_payload` 根字段、`lobby_config_revision` 表、Redis 频道与通知 JSON 等，已写入 [Lobby 实现计划 §配置与消息格式规范](../plans/2026-04-15-lobby-service-implementation-plan.md#配置与消息格式规范m0-可执行约定)；本设计规格不重複展开，以免双源漂移。

### 4.2 MySQL 持久化（权威）

建议至少包含以下 **概念字段**（具体表名与范式在实现/迁移中确定）：

| 概念 | 含义 |
|------|------|
| CSV 原文 | 产品侧产出的源文本，便于审计与 diff。 |
| JSON 载荷 | 校验通过后写入，供 Lobby `json.Unmarshal` 至 **生成的 Go 类型**。 |
| 版本 / revision | 单调递增或等价机制，用于并发更新与对账。 |
| `updated_at`、环境、配置 id 等 | 运维与多环境隔离。 |

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

## 6. 风险与待实现阶段确认项

| 项 | 说明 |
|----|------|
| Pub/Sub 不持久 | 纯 Pub/Sub 在订阅断开期可能丢消息；须 **bootstrap + 重连策略** 或 **Stream/消费者组**。 |
| 多副本一致性 | 各副本独立订阅；拉取 MySQL 后本地快照应 **只读**；写仍经业务逻辑写回各自存储。 |
| 配置与代码版本错配 | JSON 新增字段但 Lobby 未发版时 `Unmarshal` 行为须约定（忽略未知字段 vs 报错）。 |

---

## 7. 文档自检（落盘时）

- 已消除与「Excel 为唯一载体」「Lobby 轮询 MySQL 发现变更」的冲突表述。  
- 「活动内容不定型」与「配置 CSV/JSON 管线」并存：**前者指活动运行时数据模型**，**后者指策划表驱动配置**。  

---

## 8. 后续步骤（流程门闩）

实现前须按 [`AGENTS.md`](../../../AGENTS.md) 另立 **`docs/superpowers/plans/YYYY-MM-DD-<feature>.md`** 并获 **计划批准**；开发遵循 **TDD** 与 **verification-before-completion**。
