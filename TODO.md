# kd48 TODO

> 最后更新: 2026-04-18

---

## 已完成

### AI PR Review 工具切换 (2026-04-18)

- [x] 切换到 `presubmit/ai-reviewer` action
- [x] 复用现有 secrets: `AI_API_KEY`, `AI_API_URL`, `AI_MODEL`
- [x] Pin action 到 SHA (`c8604503`) 防供应链攻击
- [x] 添加 `AI_API_URL` 验证
- [x] 支持 `reopened` 事件触发
- [x] 创建 PR: https://github.com/CBookShu/kd48/pull/XX

**相关文件**:
- `.github/workflows/ai-review.yml`

### dbpool 增强 (2026-04-17 ~ 2026-04-18)

- [x] Protobuf 定义 (`api/proto/dsroute/v1/routing.proto`)
- [x] 连接池配置扩展 (MySQL: ConnMaxLifetime/ConnMaxIdleTime, Redis: MinIdleConns/Timeouts)
- [x] RouteLoader 实现 (etcd Get/Watch)
- [x] Router 原子更新支持
- [x] main.go 集成
- [x] 集成测试

**相关文档**:
- `docs/superpowers/specs/2026-04-17-dbpool-enhancement-design.md`
- `docs/superpowers/plans/2026-04-17-dbpool-enhancement-plan.md`

### UserService MySQL 路由集成 (2026-04-18)

- [x] server.go 改造 (getQueries 辅助方法)
- [x] main.go 改造 (移除固定 queries)
- [x] 单元测试
- [x] routingKey 格式: `{category}:{table}:{key}` (如 `sys:user:alice`)

**相关文档**:
- `docs/superpowers/specs/2026-04-18-user-service-mysql-routing-design.md`
- `docs/superpowers/plans/2026-04-18-user-service-mysql-routing-plan.md`

---

## 待处理

### 高优先级

(无)

### 未完成的实现计划

| 计划文档 | 状态 | 说明 |
|----------|------|------|
| `2026-04-18-datasource-routing-implementation-plan.md` | 部分 | dsroute 核心已实现，但规格文档中的多数据源配置可能未完全接入 |
| `2026-04-15-lobby-service-implementation-plan.md` | 待执行 | 大厅服务 |
| `2026-04-13-gateway-ingress-implementation-plan.md` | 待确认 | 网关 Ingress |
| `2026-04-13-gateway-etcd-meta-implementation-plan.md` | 待确认 | 网关 Etcd 元数据 |

### 规格文档待实现

| 规格文档 | 状态 | 说明 |
|----------|------|------|
| `2026-04-17-datasource-routing-and-pools.md` | 已完成 | dsroute 包已实现 |
| `2026-04-15-lobby-service-design.md` | 待实现 | 大厅服务设计 |
| `2026-04-16-lobby-config-csv-and-tooling-spec.md` | 待实现 | 大厅配置与工具 |
| `2026-04-13-gateway-backend-connection-design.md` | 待确认 | 网关后端连接 |
| `2026-04-13-kd48-roadmap.md` | 路线图 | M0-M5+ 阶段规划 |

---

## 路线图概览 (from kd48-roadmap.md)

| 阶段 | 产品叙事 | 技术交付 | 状态 |
|------|----------|----------|------|
| **M0** | 统一入口、账号与会话、可观测、本地可跑 | 网关 WS、gRPC、User、Etcd、MySQL、Redis、OTel | ✅ 基本完成 |
| **M1** | 架子可扩、协作不断档 | compose/README、第二服务接入套路、最小 CI | 待开始 |
| **M2** | 大厅可用 | 大厅无状态服务 + DB/缓存；染色/AB/打表最小可用 | 待开始 |
| **M3** | 房间：归属、匹配、开桌前退房与重连 | 房间有状态服务池 | 待开始 |
| **M4** | 游戏内对局 | 有状态游戏/回合服务 | 待开始 |
| **M5+** | 活动深化、轻量客户端 | 依赖前置阶段 | 待开始 |

---

## 杂项

(无)
