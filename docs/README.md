# kd48 项目文档

本目录用于存放 **结构化文档**（路线图、专题规格、设计记录等），与仓库根目录 **`spec.md`**（当前 M0 技术基座 **草稿**）并行维护。

**Agent 流程（强制）**：根目录 [`AGENTS.md`](../AGENTS.md)（计划批准、TDD、subagent、验证门闩）；Cursor 规则见 [`.cursor/rules/superpowers-workflow.mdc`](../.cursor/rules/superpowers-workflow.mdc)。

**演进目标**：持续充实 `docs/` 后，由本体系 **逐步承接** 根目录 `spec.md` 的「总规格」角色；迁移期在 `spec.md` 顶部保留指向本文档的索引即可（详见路线图 **§1.5**）。

## 文档索引

| 文档 | 说明 |
|------|------|
| [superpowers/specs/2026-04-13-kd48-roadmap.md](./superpowers/specs/2026-04-13-kd48-roadmap.md) | 产品与技术面长期路线图（大厅/游戏内、无状态与有状态服务、阶段划分等） |
| [superpowers/specs/2026-04-13-gateway-backend-connection-design.md](./superpowers/specs/2026-04-13-gateway-backend-connection-design.md) | 网关与后端连接、稳定 **Ingress**（JSON 载荷）、Etcd **实例发现**与 **逻辑元数据**（服务类型 / WS 路由，protojson）；**§11** Bootstrap + Watch、降级与健康；**§11.12** 实现与可靠性（须完整落实，禁止静默缺省关键路径） |
| [superpowers/specs/2026-04-15-lobby-service-design.md](./superpowers/specs/2026-04-15-lobby-service-design.md) | **Lobby** 服务：活动域 gRPC、无状态扩展、与 **Gateway** 分工；配置管线指向专项规格；**§6** 配置 JSON 摘要 / Go 示意、`lobby.v1` 与 **Ingress**、**ConfigLoader/Snapshot** 草图 |
| [superpowers/specs/2026-04-16-lobby-config-csv-and-tooling-spec.md](./superpowers/specs/2026-04-16-lobby-config-csv-and-tooling-spec.md) | **Lobby 策划 CSV（`sheet_v1`）** 与 **`json_payload`**、空值默认、打表工具流水线；**§4** 含 **CSV 示例与转换后 JSON 示例** |
| [superpowers/specs/2026-04-17-datasource-routing-and-pools.md](./superpowers/specs/2026-04-17-datasource-routing-and-pools.md) | **MySQL / Redis 命名池**；**`routing_key`** + **LPM** + **策略 A**；**§11** 实现：不热更、`pkg` 复用、路由逻辑共用；**§7** 迁移 |
| [superpowers/plans/2026-04-13-gateway-ingress-implementation-plan.md](./superpowers/plans/2026-04-13-gateway-ingress-implementation-plan.md) | Gateway Ingress **M0 落地**（proto → User 分发 → 网关 `WrapIngress`） |
| [superpowers/plans/2026-04-13-gateway-etcd-meta-implementation-plan.md](./superpowers/plans/2026-04-13-gateway-etcd-meta-implementation-plan.md) | 网关 **Etcd 元数据**（类型 + 路由 protojson）、Bootstrap、**Watch**、`AtomicRouter`、**seed-gateway-meta**（与设计 **§11、§11.12** 对齐） |
| [superpowers/plans/2026-04-15-lobby-service-implementation-plan.md](./superpowers/plans/2026-04-15-lobby-service-implementation-plan.md) | **Lobby** 实现计划：**CSV/`json_payload`/打表** 见 [专项规格](./superpowers/specs/2026-04-16-lobby-config-csv-and-tooling-spec.md)；MySQL **`scope`/`title`/`tags`/`start_time`/`end_time`**（**§C**）；**§C.1 `config_id` 命名**；proto、迁移、bootstrap、**Redis**、Ingress、Task 7 |

后续可在本表追加：部署规范、配置与染色专题、各服务边界说明等。
