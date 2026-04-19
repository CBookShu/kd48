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

## 冲突处理

若用户指令与本文件冲突，**应先简要说明冲突** 并 **请求用户二选一**（遵循用户指令 **或** 遵循本仓库流程），**不得静默忽略** 本文件。
