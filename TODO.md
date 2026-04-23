# kd48 TODO

> 最后更新: 2026-04-23

---

## 已完成

### Config-Loader 打表工具 (2026-04-21)

- [x] CSV 解析器（三行头格式）
- [x] 类型解析（int32, int64, string, time, arrays, maps）
- [x] 数据校验（snake_case 列名、重复检测）
- [x] JSON payload 生成
- [x] MySQL 写入器
- [x] Redis 通知发布器
- [x] Go struct 代码生成（含 ConfigTime 类型）
- [x] 管道执行器
- [x] CLI 主程序

**相关文件**:
- `tools/config-loader/` - 完整工具实现
- `tools/config-loader/testdata/example.csv` - 示例 CSV

**验证命令**:
```bash
cd tools/config-loader && go test ./... -v
go build ./cmd/config-loader
```

---

## 已完成

### WebSocket 心跳与连接管理 (2026-04-20)

- [x] 服务端收到 Ping 后回复 Pong（RFC 6455 合规）
- [x] RecordActivity 记录活动时间用于超时检测
- [x] 配置参数化（server_timeout: 90s, check_interval: 5s）
- [x] 单元测试覆盖
- [x] **HeartbeatManager** 心跳状态管理（Ping/Pong/超时检测）
- [x] **ConnectionManager** 连接生命周期管理（注册/注销/断开）
- [x] 空闲连接自动断开（IdleTimeout 配置）
- [x] 连接统计指标（TotalConnections/ActiveConnections/HeartbeatFailures）

**相关文件**:
- `gateway/internal/ws/heartbeat.go`
- `gateway/internal/ws/connection_manager.go`
- `gateway/internal/ws/handler.go`

**验证命令**:
```bash
go test ./gateway/internal/ws/... -v
go build ./gateway/...
```

---

## 已完成

### Lobby 服务核心骨架 (2026-04-19)

- [x] Task 1: lobby.v1 Proto 与代码生成（Ping RPC）
- [x] Task 2: MySQL 迁移（lobby_config_revision 表）
- [x] Task 3: 服务骨架（main.go、server.go、ingress.go）
- [x] gRPC 服务注册到 etcd（kd48/lobby-service）
- [x] GatewayIngress 分发实现
- [x] bufconn 单元测试
- [x] go.work 集成

**相关文档**:
- `docs/superpowers/specs/2026-04-15-lobby-service-design.md`
- `docs/superpowers/plans/2026-04-15-lobby-service-implementation-plan.md`

**验证命令**:
```bash
go test ./services/lobby/... -v
go build ./services/lobby/...
```

---

## 已完成

### 网关 Etcd 元数据与 Watch 热更新 (2026-04-13 ~ 2026-04-18)

- [x] `GatewayRouteSpec.establishes_session` 字段
- [x] `AtomicRouter` 无锁路由快照
- [x] Bootstrap：Range + 校验 + gRPC Dial 池（物理空 key 时 fail-fast）
- [x] Watch：`kd48/meta/` 前缀监听 + debounce 全量重建
- [x] draining：旧连接延迟 30s 后 Close
- [x] Handler：`public` 鉴权白名单 + `establishes_session` 会话标记
- [x] seed-gateway-meta 工具
- [x] 单元测试覆盖

**相关文档**:
- `docs/superpowers/specs/2026-04-13-gateway-backend-connection-design.md`（§11）
- `docs/superpowers/plans/2026-04-13-gateway-etcd-meta-implementation-plan.md`

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

#### P0: 顶号/踢人机制 🟢 基本完成

| 组件 | 文件 | 状态 |
|------|------|------|
| User 服务 - Lua 原子操作 | `services/user/cmd/user/server.go` | ✅ `loginSessionLua` 双向映射原子更新 |
| User 服务 - 发布失效通知 | `services/user/cmd/user/server.go` | ✅ 发布到 `kd48:session:invalidate` 频道 |
| Gateway - Session 订阅器 | `gateway/internal/ws/session_subscriber.go` | ✅ 完整订阅和处理逻辑 |
| Gateway - ConnectionManager | `gateway/internal/ws/connection_manager.go` | ✅ `DisconnectByUserID` 强制断开 |
| Gateway - Handler 注册映射 | `gateway/internal/ws/handler.go` | ✅ 登录后调用 `RegisterUserConnection` |

**待完善**:
- [ ] Ping 方法返回实际 ConfigRevision（当前硬编码为 0，需从 ConfigStore 获取）

---

#### P0: Lobby 服务 Task 4-6 🟢 已完成

| Task | 状态 | 说明 |
|------|------|------|
| Task 4: 配置加载器 | ✅ | `services/lobby/internal/config/loader.go` - 从 MySQL 加载配置 |
| Task 5: Redis 变更通知 | ✅ | `services/lobby/internal/config/watcher.go` - 订阅 `kd48:lobby:config:notify` |
| Task 6: GatewayIngress | ✅ | `services/lobby/cmd/lobby/ingress.go` - 已注册 `/lobby.v1.LobbyService/Ping` |

**验证命令**:
```bash
go test ./services/lobby/... -v
go test ./gateway/internal/ws/... -v
```

---

## 待处理

### 中优先级

#### P1: Config-Loader Go 代码生成优化 ✅ 已完成

- [x] 包名从 ConfigName 推导（每个配置独立包）- `-go-package` 参数
- [x] 输出目录按配置名组织（如 `generated/checkin/`, `generated/reward/`）- `-go-out` 参数
- [x] TimeFormat 每包独立定义，避免全局常量冲突

**相关文件**: `tools/config-loader/internal/generator/go.go`

#### P1: Config-Loader Lobby 接入设计 ✅ 已完成

- [x] TypedStore 类型安全存储（`services/lobby/internal/config/store.go`）
- [x] Register 泛型注册函数（`services/lobby/internal/config/registry.go`）
- [x] 配置加载器从 MySQL 加载（`services/lobby/internal/config/loader.go`）
- [x] 热更新订阅（`services/lobby/internal/config/watcher.go`）

**相关文档**: `docs/superpowers/specs/2026-04-22-lobby-config-loading-design.md`

---

### 待处理

#### P1: 签到活动（打通基础设施） ⏱️ 3-4天

> 用签到活动驱动基础设施搭建，验证全链路

**业务功能**:
- [ ] 签到配置表设计（CSV 格式 + MySQL 表）
- [ ] Proto 扩展：`lobby.v1.Checkin` RPC
- [ ] 签到逻辑实现（每日签到、连续天数、奖励发放）
- [ ] 接入 Config-Loader 打表

**基础设施（同步完成）**:
- [ ] OTel 链路追踪接入（Jaeger）
- [ ] Prometheus 指标采集（QPS、延迟）
- [ ] docker-compose 监控栈（Jaeger + Prometheus + Grafana）

**验证**:
- [ ] 单元测试
- [ ] 链路追踪可查
- [ ] 指标可观测

---

#### P1: Web 客户端（演示验证） ⏱️ 2-3天

> 原生 HTML/JS 客户端，演示签到功能

- [ ] WebSocket 消息协议定义
- [ ] Gateway WebSocket 支持
- [ ] 注册页面
- [ ] 登录页面
- [ ] 签到页面（展示连续天数、奖励）
- [ ] 端到端验证

**技术栈**: 原生 HTML/JS，无构建工具

---

#### P1: 网关多服务验证 ⏱️ 1-2天
- [ ] 测试同时运行 User 和 Lobby 服务
- [ ] 验证动态路由更新
- [ ] 文档化多服务接入流程

**说明**: 代码已支持多服务（`AtomicRouter`、`Manager`），需实际运行验证。

#### P1: 统一API响应格式 ⏱️ 2-3天
- [ ] 设计通用 `ApiResponse` proto
- [ ] 统一错误码和消息格式
- [ ] 更新现有服务响应格式

#### P1: 监控体系完善 ⏱️ 3-4天
- [x] OTel 基础（`pkg/otelkit`）
- [x] 连接指标（`ConnectionMetrics`）
- [ ] Prometheus 集成
- [ ] Grafana 仪表板
- [ ] 告警规则

**说明**: 基础监控已完成，生产级监控待完善。

---

### 低优先级

#### P2: 流量控制和染色 ⏱️ 3-4天
- [ ] 实现基于令牌桶的限流
- [ ] 支持请求染色和AB测试
- [ ] 配置管理界面

#### P2: 负载均衡扩展 ⏱️ 2-3天
- [ ] 实现最少连接、一致性哈希等算法
- [ ] 支持动态权重调整
- [ ] 性能测试和对比

#### P2: 部署构建优化 ⏱️ 2-3天
- [x] GitHub Actions CI 基础（`.github/workflows/ci.yml`）
- [ ] 添加 lobby 服务构建到 CI
- [ ] Dockerfile 和 Docker 镜像构建
- [ ] 健康检查和就绪探针

#### P3: 代码质量改进
- [x] 方法名长度合理（无明显过长问题）
- [x] 测试覆盖：gateway/ws 45.3%, lobby/config 87.4%
- [ ] user 服务测试覆盖（当前 11.6%）
- [ ] golangci-lint 配置
- [ ] 完善代码文档

---

### 未完成的实现计划

| 计划文档 | 状态 | 说明 |
|----------|------|------|
| `2026-04-15-lobby-service-implementation-plan.md` | ✅ 已完成 | Task 1-6 全部实现 |
| `2026-04-18-datasource-routing-implementation-plan.md` | ✅ 已完成 | dsroute 核心已实现，Router/Loader/LPM 全部就绪 |
| `2026-04-13-gateway-ingress-implementation-plan.md` | ✅ 已完成 | 网关 Ingress（含 `GatewayIngress`） |
| `2026-04-13-gateway-etcd-meta-implementation-plan.md` | ✅ 已完成 | 网关 Etcd 元数据（含 Watch 热更新、Bootstrap、draining） |
| `2026-04-20-heartbeat-fix-plan.md` | ✅ 已完成 | 心跳与连接管理 |

### 规格文档状态

| 规格文档 | 状态 | 说明 |
|----------|------|------|
| `2026-04-17-datasource-routing-and-pools.md` | ✅ 已完成 | dsroute 包已实现 |
| `2026-04-20-heartbeat-design.md` | ✅ 已完成 | 心跳与连接管理设计 |
| `2026-04-15-lobby-service-design.md` | ✅ 已完成 | Task 1-6 全部实现 |
| `2026-04-16-lobby-config-csv-and-tooling-spec.md` | 待实现 | 大厅配置与工具 |
| `2026-04-13-gateway-backend-connection-design.md` | ✅ 已完成 | 网关后端连接 |
| `2026-04-13-kd48-roadmap.md` | 路线图 | M0-M5+ 阶段规划 |

---

## 路线图概览 (from kd48-roadmap.md)

| 阶段 | 产品叙事 | 技术交付 | 状态 |
|------|----------|----------|------|
| **M0** | 统一入口、账号与会话、可观测、本地可跑 | 网关 WS、gRPC、User、Etcd、MySQL、Redis、OTel | ✅ 基本完成 |
| **M1** | 架子可扩、协作不断档 | compose/README、第二服务接入套路、最小 CI | ✅ 已完成（Lobby 骨架验证第二服务接入套路） |
| **M2** | 大厅可用 | 大厅无状态服务 + DB/缓存；染色/AB/打表最小可用 | 🟡 进行中 |
| **M3** | 房间：归属、匹配、开桌前退房与重连 | 房间有状态服务池 | 待开始 |
| **M4** | 游戏内对局 | 有状态游戏/回合服务 | 待开始 |
| **M5+** | 活动深化、轻量客户端 | 依赖前置阶段 | 待开始 |

---

## 杂项

(无)
