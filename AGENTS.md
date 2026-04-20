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
brainstorming → writing-plans → TDD开发 → verification → code-review → 完成
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
