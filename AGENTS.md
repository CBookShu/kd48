# kd48 项目规则

本项目使用 [superpowers](https://github.com/obra/superpowers) 技能体系，已全局安装。本文件仅补充项目特定规则。

---

## 开发命令速查

- **启动中间件**: `docker compose up -d`（Etcd:2379, MySQL:3306, Redis:6379）
- **配置**: `cp config.yaml gateway/config.yaml && cp config.yaml services/user/config.yaml`
- **日志目录**: `mkdir -p logs`
- **迁移**: `cd services/user && migrate -path ./migrations -database "mysql://root:root@tcp(localhost:3306)/kd48?parseTime=true&loc=Local" up`
- **测试**: `go test ./...`（根目录 go.work）
- **单测**: `go test ./gateway/internal/ws/... -v`
- **构建**: `go build ./...`
- **运行**: `go run ./gateway/cmd/gateway` / `go run ./services/user/cmd/user`
- **WS 入口**: `ws://localhost:8080/ws`

---

## 项目结构

- **Modules**: api/proto | gateway | pkg | services/user（go.work）
- **Go**: 1.26.1（见 go.work）
- **入口**: gateway/:8080, user 服务/:9000

---

## 项目特定约束

### 分支管理

- **禁止直接修改 master 分支**：所有变更必须从 master 拉出新分支，完成后再合并回 master
- 分支命名规范：`feature/<功能名>`、`fix/<问题名>`、`refactor/<重构名>`

### 数据源访问规范

- **MySQL/Redis 连接必须通过 `dsroute.Router` 解析**
- 禁止直接使用 map key 访问连接池（如 `mysqlPools["default"]`、`redisPools["default"]`）
- 必须定义 routing key，通过 `router.ResolveDB(ctx, routingKey)` 或 `router.ResolveRedis(ctx, routingKey)` 获取连接
- Routing Key 命名规范：`<服务名>:<业务域>`，如 `lobby:config`、`user:session`

#### 正确示例

```go
// ✅ 正确：通过 Router 解析
db, poolName, err := router.ResolveDB(ctx, "lobby:config")
if err != nil {
    return fmt.Errorf("resolve db: %w", err)
}

rdb, poolName, err := router.ResolveRedis(ctx, "lobby:config")
if err != nil {
    return fmt.Errorf("resolve redis: %w", err)
}
```

#### 错误示例

```go
// ❌ 错误：直接访问连接池
db := mysqlPools["default"]
rdb := redisPools["default"]

// ❌ 错误：服务内部暴露 pools map
func (s *lobbyService) getMySQLDB(name string) (*sql.DB, error) {
    db, ok := s.mysqlPools[name]  // 应改为使用 Router
    ...
}
```

#### 架构说明

- `main.go` 初始化连接池后，应创建 `dsroute.Router` 并注入服务
- 服务内部持有 `*dsroute.Router`，通过 routing key 解析连接
- 参考 `services/user/cmd/user/main.go` 的正确实现

#### 路由配置初始化

新服务添加 routing key 后，需要在 `gateway/cmd/seed-gateway-meta/main.go` 中初始化对应的路由规则：

```go
// 在 seed-gateway-meta 中添加路由配置
mysqlRoutes := []dsroute.RouteRule{
    {Prefix: "lobby:config-data", Pool: "default"},
}
redisRoutes := []dsroute.RouteRule{
    {Prefix: "lobby:config-notify", Pool: "default"},
}

// 写入 etcd
routesJSON, _ := json.Marshal(mysqlRoutes)
cli.Put(ctx, "kd48/routing/mysql_routes", string(routesJSON))
routesJSON, _ = json.Marshal(redisRoutes)
cli.Put(ctx, "kd48/routing/redis_routes", string(routesJSON))
```

这样部署时通过 `go run ./gateway/cmd/seed-gateway-meta` 即可导入必要的前置数据。

### 设计文档路径

- 设计文档：`docs/superpowers/specs/YYYY-MM-DD-<topic>-design.md`
- 实现计划：`docs/superpowers/plans/YYYY-MM-DD-<feature>-plan.md`

### 计划批准模板

实现计划文件须在 Goal 下方包含批准记录：

```markdown
## 批准记录（人类门闩）

- **状态**：待批准 / 已批准
- **批准范围**：例如「全文」或「Task 3～5」
- **批准人 / 日期**：（手填）
- **TDD**：强制 / 豁免（若豁免须写理由）
- **Subagent**：按任务拆分 / 本步单会话豁免（若豁免须写理由）
```

### 验证命令

- 测试：`go test ./...`（在 go.work 根目录）
- 构建：`go build ./...`

### TODO

- [ ] 添加真实数据库/Redis 集成测试（当前使用 sqlmock/miniredis 模拟）
  - 真实 MySQL 数据库集成测试
  - 真实 Redis Pub/Sub 集成测试
  - 配置热更新端到端测试
  - 路由配置变更后的连接切换测试

---

## 文档索引

- 路线图与设计：`docs/README.md`
- 设计文档：`docs/superpowers/specs/`
- 实现计划：`docs/superpowers/plans/`

---

## AI 执行规范

> 详细设计文档：`docs/superpowers/specs/2026-04-20-ai-execution-policy-design.md`

### 核心原则

**无豁免即完整流程** — 所有 AI 工作默认遵循 superpowers 全流程，只有明确列入豁免清单的场景可简化。

### 标准流程

```
brainstorming → using-git-worktrees → writing-plans → TDD开发 → verification → code-review → 完成
```

### 豁免清单（可跳过 brainstorming 和 writing-plans）

| 场景 | 仍需执行 |
|------|---------|
| 纯注释修改 | 验证文档生成正确 |
| 配置文件更新 | 验证配置可加载 |
| 测试补充 | TDD、测试通过 |
| 格式化/lint 修复 | 工具执行、验证通过 |
| 依赖升级（无 API 变更） | 构建/测试通过 |
| 简单重构（IDE 自动生成） | 验证行为未变 |
| 原型验证（不合并主分支） | 简要确认目标，豁免 review |

> 豁免使用：任务开始时由 AI 主动声明豁免理由，用户确认后进入简化流程。

### 质量门禁

**自动检查**（每次提交前）：
```bash
go build ./... && go test ./... && go vet ./...
```

**AI 必须主动报告**：
- 变更范围说明
- 测试覆盖情况
- review 检查清单完成情况（非豁免场景）

---

## 冲突处理

若用户指令与本文件冲突，**应先简要说明冲突** 并 **请求用户二选一**（遵循用户指令 **或** 遵循本仓库流程），**不得静默忽略** 本文件。
