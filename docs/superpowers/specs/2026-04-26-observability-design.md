# kd48 可观测性体系设计规格

> **文档类型**：设计规格 (Design Spec)  
> **日期**：2026-04-26  
> **状态**：已批准待实施  
> **关联计划**：`docs/superpowers/plans/2026-04-26-observability-implementation-plan.md`

---

## 1. 背景与目标

### 1.1 现状

kd48 项目已具备以下可观测性基础：

- ✅ **OpenTelemetry Tracing**：`pkg/otelkit` 包实现，支持 Jaeger 导出
- ✅ **Docker 观测栈**：Jaeger (16686) + Prometheus (9090) + Grafana (3000)
- ✅ **日志系统**：slog + zap + trace_id 注入
- ✅ **连接指标**：`gateway/internal/ws/ConnectionMetrics`
- ✅ **gRPC 追踪**：`otelgrpc` 中间件已集成

### 1.2 目标

构建生产级可观测性体系，覆盖：

1. **Metrics**：统一指标暴露和收集
2. **Dashboard**：可视化监控面板
3. **Alerting**：基于 SLO 的告警机制

---

## 2. 整体架构

```
┌─────────────────────────────────────────────────────────────┐
│                        kd48 服务集群                          │
├──────────────┬──────────────┬──────────────┬───────────────┤
│   Gateway    │    User      │    Lobby     │   (未来服务)   │
│   :8080      │    :9000     │    :9001     │               │
├──────────────┴──────────────┴──────────────┴───────────────┤
│                    Prometheus Client                         │
│                    /metrics 端点                             │
├─────────────────────────────────────────────────────────────┤
│                    Prometheus Server                         │
│                    :9090 (docker-compose)                    │
├─────────────────────────────────────────────────────────────┤
│                    Grafana                                   │
│                    :3000 (docker-compose)                    │
└─────────────────────────────────────────────────────────────┘
```

---

## 3. 指标规范

### 3.1 命名约定

所有指标前缀：`kd48_`

格式：`kd48_<domain>_<entity>_<metric>_<unit>`

- `domain`: http, grpc, ws, db, auth, checkin...
- `entity`: requests, connections, pool...
- `metric`: total, duration, active...
- `unit`: seconds, bytes (可选)

### 3.2 技术指标（所有服务通用）

| 指标名 | 类型 | 标签 | 说明 | 数据源 |
|--------|------|------|------|--------|
| `kd48_http_requests_total` | Counter | service, method, path, status | HTTP 请求总数 | Fiber 中间件 |
| `kd48_http_request_duration_seconds` | Histogram | service, method, path | 请求延迟分布 | Fiber 中间件 |
| `kd48_grpc_requests_total` | Counter | service, method, status | gRPC 请求总数 | gRPC 拦截器 |
| `kd48_grpc_request_duration_seconds` | Histogram | service, method | gRPC 延迟分布 | gRPC 拦截器 |
| `kd48_db_pool_connections_active` | Gauge | service, db_type, pool_name | 活跃连接数 | dsroute.Router |
| `kd48_db_pool_connections_idle` | Gauge | service, db_type, pool_name | 空闲连接数 | dsroute.Router |
| `kd48_db_pool_wait_duration_seconds` | Histogram | service, db_type | 等待连接耗时 | dsroute.Router |

### 3.3 业务指标（Gateway 特有）

| 指标名 | 类型 | 标签 | 说明 | 埋点位置 |
|--------|------|------|------|----------|
| `kd48_ws_connections_active` | Gauge | - | 当前活跃 WS 连接 | ConnectionManager |
| `kd48_ws_connections_total` | Counter | - | 累计 WS 连接数 | handler.go |
| `kd48_auth_login_total` | Counter | result=[success/failure] | 登录次数 | user/server.go |
| `kd48_auth_register_total` | Counter | result=[success/failure] | 注册次数 | user/server.go |

### 3.4 业务指标（Lobby 特有）

| 指标名 | 类型 | 标签 | 说明 | 埋点位置 |
|--------|------|------|------|----------|
| `kd48_checkin_total` | Counter | result=[success/failure] | 签到次数 | checkin/server.go |
| `kd48_item_granted_total` | Counter | item_type | 物品发放次数 | item/server.go |
| `kd48_config_revision` | Gauge | config_name | 当前配置版本 | config/store.go |

---

## 4. 实施阶段

### Phase 1：核心指标（2 天）

**目标**：所有服务暴露基础技术指标

**任务清单**：

- [ ] **4.1.1** 创建 `pkg/metrics` 包
  - 定义指标注册表
  - HTTP 中间件自动记录请求指标
  - gRPC 拦截器自动记录调用指标

- [ ] **4.1.2** 扩展 `pkg/dsroute`
  - 为 `Router` 添加连接池指标暴露
  - 监控 MySQL/Redis 连接池状态

- [ ] **4.1.3** Gateway 指标端点
  - 添加 `:8080/metrics`
  - 复用 `ConnectionManager.GetMetrics()`
  - 集成 Fiber Prometheus 中间件

- [ ] **4.1.4** User Service 指标端点
  - 添加 `:9000/metrics`
  - gRPC 拦截器指标

- [ ] **4.1.5** Lobby Service 指标端点
  - 添加 `:9001/metrics`
  - gRPC 拦截器指标

- [ ] **4.1.6** 验证 Prometheus 抓取
  - 更新 `docker/prometheus.yml`
  - 确认所有端点可访问

**验收标准**：
- `curl localhost:8080/metrics` 返回 Prometheus 格式数据
- Prometheus UI (localhost:9090/targets) 显示所有服务 UP

### Phase 2：业务指标 + Dashboard（1-2 天）

**目标**：手动埋点业务指标，创建 Grafana Dashboard

**任务清单**：

- [ ] **4.2.1** WebSocket 连接指标
  - 连接建立/断开计数器
  - 活跃连接 Gauge（已有，需暴露）

- [ ] **4.2.2** 认证业务指标
  - 登录成功/失败计数器
  - 注册成功/失败计数器

- [ ] **4.2.3** 签到活动指标
  - 签到次数计数器
  - 按活动类型分组

- [ ] **4.2.4** Dashboard 1：服务概览
  - QPS (按服务)
  - 延迟 P99 (按服务)
  - 错误率
  - 服务健康状态

- [ ] **4.2.5** Dashboard 2：WebSocket 监控
  - 活跃连接数
  - 连接数趋势
  - 心跳失败率

- [ ] **4.2.6** Dashboard 3：业务指标
  - 登录/注册趋势
  - 签到活动参与
  - 物品发放统计

**验收标准**：
- 3 个 Dashboard 可正常访问
- 指标数据实时更新

### Phase 3：告警（可选，1 天）

**目标**：基于 SLO 的告警机制

**任务清单**：

- [ ] **4.3.1** 定义 SLO
  - 可用性：99.9% (每月停机 < 43分钟)
  - 延迟：P99 < 500ms
  - 错误率：< 0.1%

- [ ] **4.3.2** 配置告警规则
  - 服务宕机告警
  - 错误率突增告警
  - 延迟超标告警

- [ ] **4.3.3** 告警通知
  - 配置通知渠道（日志文件先行）

---

## 5. 技术选型

| 组件 | 选型 | 版本 | 理由 |
|------|------|------|------|
| Metrics Client | `github.com/prometheus/client_golang` | v1.19+ | Go Prometheus 标准实现 |
| HTTP 指标 | `github.com/ansrivas/fiberprometheus` | v2.7+ | Fiber 框架专用 |
| gRPC 指标 | `github.com/grpc-ecosystem/go-grpc-prometheus` | v1.2+ | gRPC 生态标准 |
| Dashboard | Grafana | 10.4+ | 已部署，无需变更 |
| 告警 | Prometheus Alertmanager | v0.27+ | 与 Prometheus 原生集成 |

---

## 6. 与现有系统集成

### 6.1 与 dsroute 集成

在 `pkg/dsroute/router.go` 中添加指标收集：

```go
// ResolveDB 时记录等待时间
func (r *Router) ResolveDB(ctx context.Context, routingKey string) (*sql.DB, string, error) {
    start := time.Now()
    db, pool, err := r.resolveDBInternal(ctx, routingKey)
    metrics.DBPoolWaitDuration.Observe(time.Since(start).Seconds())
    return db, pool, err
}
```

### 6.2 与 WebSocket 集成

复用 `ConnectionManager` 现有指标：

```go
func (cm *ConnectionManager) CollectMetrics(ch chan<- prometheus.Metric) {
    metrics := cm.GetMetrics()
    ch <- prometheus.MustNewConstMetric(
        wsConnectionsActiveDesc,
        prometheus.GaugeValue,
        float64(metrics.ActiveConnections),
    )
}
```

### 6.3 与 OTel 集成（可选）

未来可将 Prometheus 指标同时导出为 OTel 格式：

```go
// 使用 otel-prometheus-bridge
bridge, err := prometheus.NewExporter()
```

---

## 7. Dashboard 详细设计

### 7.1 服务概览 Dashboard

**URL**：`http://localhost:3000/d/kd48-overview`

**面板布局**：

```
┌──────────────────────────────────────────────────────┐
│  刷新：5s  │  时间范围：Last 1 hour                    │
├──────────────────────────────────────────────────────┤
│  [QPS - 折线图]           │  [延迟 P99 - 折线图]       │
│  - gateway/user/lobby     │  - gateway/user/lobby      │
├──────────────────────────────────────────────────────┤
│  [错误率 - 折线图]         │  [活跃连接 - 数字面板]      │
│  - 4xx/5xx 分离           │  - WebSocket 连接数         │
├──────────────────────────────────────────────────────┤
│  [服务健康 - 状态灯]                                    │
│  - 绿色：健康  红色：异常                                │
└──────────────────────────────────────────────────────┘
```

### 7.2 WebSocket 监控 Dashboard

**URL**：`http://localhost:3000/d/kd48-websocket`

**面板布局**：

```
┌──────────────────────────────────────────────────────┐
│  活跃连接：1,234    今日累计：12,345    峰值：1,500   │
├──────────────────────────────────────────────────────┤
│  [连接数趋势 - 面积图 - 过去1小时]                       │
├──────────────────────────────────────────────────────┤
│  [连接时长分布 - 直方图]   │  [心跳失败率 - 折线图]      │
└──────────────────────────────────────────────────────┘
```

### 7.3 业务指标 Dashboard

**URL**：`http://localhost:3000/d/kd48-business`

**面板布局**：

```
┌──────────────────────────────────────────────────────┐
│  [登录趋势 - 折线图]        │  [注册趋势 - 折线图]       │
│  - 成功/失败分离            │  - 成功/失败分离           │
├──────────────────────────────────────────────────────┤
│  [签到统计 - 柱状图]        │  [物品发放 - 饼图]         │
│  - 按活动分组               │  - 按类型分组              │
└──────────────────────────────────────────────────────┘
```

---

## 8. 安全考虑

### 8.1 指标端点暴露

- `/metrics` 端点**不经过**认证（Prometheus 需要访问）
- 仅暴露聚合指标，不暴露敏感信息
- 不在指标中包含用户 ID、Token 等隐私数据

### 8.2 标签基数控制

- `path` 标签使用路由模板（如 `/api/users/:id`），而非实际路径
- 避免高基数标签（如 user_id、session_id）
- 预估标签组合数 < 10,000

---

## 9. 性能影响

### 9.1 开销预估

| 指标类型 | 额外开销 | 说明 |
|----------|----------|------|
| HTTP 中间件 | ~1-2% CPU | 每个请求计数 |
| gRPC 拦截器 | ~0.5% CPU | 每个 RPC 调用计数 |
| 业务指标 | 可忽略 | 低频操作（登录、签到） |
| 连接池指标 | 可忽略 | 定期采集 |

### 9.2 采样策略

- 技术指标：全量采集
- 业务指标：全量采集（低频）
- 追踪数据：保持现有采样率

---

## 10. 附录

### 10.1 参考实现

- Prometheus Go Client: https://github.com/prometheus/client_golang
- Fiber Prometheus: https://github.com/ansrivas/fiberprometheus
- Grafana Dashboard JSON 导出/导入

### 10.2 相关文档

- `docs/superpowers/specs/2026-04-13-gateway-backend-connection-design.md` - 网关设计
- `docs/superpowers/specs/2026-04-17-datasource-routing-and-pools.md` - 数据源路由
- `docs/superpowers/specs/2026-04-23-checkin-activity-design.md` - 签到活动设计

### 10.3 验收检查清单

- [ ] 所有服务 `/metrics` 端点可访问
- [ ] Prometheus 成功抓取所有端点
- [ ] 3 个 Grafana Dashboard 可正常显示
- [ ] 关键指标数据验证正确
- [ ] 服务重启后指标持续正常
- [ ] 无敏感信息泄露

---

## 批准记录

- **状态**：已批准
- **批准范围**：全文
- **批准人 / 日期**：用户确认 / 2026-04-26
- **TDD**：强制
- **Subagent**：按任务拆分（Phase 1/2/3 分别执行）
