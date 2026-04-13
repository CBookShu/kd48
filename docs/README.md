# kd48 项目文档

本目录用于存放 **结构化文档**（路线图、专题规格、设计记录等），与仓库根目录 **`spec.md`**（当前 M0 技术基座 **草稿**）并行维护。

**演进目标**：持续充实 `docs/` 后，由本体系 **逐步承接** 根目录 `spec.md` 的「总规格」角色；迁移期在 `spec.md` 顶部保留指向本文档的索引即可（详见路线图 **§1.5**）。

## 文档索引

| 文档 | 说明 |
|------|------|
| [superpowers/specs/2026-04-13-kd48-roadmap.md](./superpowers/specs/2026-04-13-kd48-roadmap.md) | 产品与技术面长期路线图（大厅/游戏内、无状态与有状态服务、阶段划分等） |
| [superpowers/specs/2026-04-13-gateway-backend-connection-design.md](./superpowers/specs/2026-04-13-gateway-backend-connection-design.md) | 网关与后端连接、Etcd 发现、稳定 Ingress 协议（JSON 载荷）与迁移建议 |
| [superpowers/plans/2026-04-13-gateway-ingress-implementation-plan.md](./superpowers/plans/2026-04-13-gateway-ingress-implementation-plan.md) | Gateway Ingress 落地实现计划（proto → user → 网关） |

后续可在本表追加：部署规范、配置与染色专题、各服务边界说明等。
