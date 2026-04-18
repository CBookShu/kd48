# kd48 协作与 Agent 流程（强制）

本仓库要求 **Cursor / Claude 等 Agent** 与 **人类贡献者** 共同遵守下列流程。与 [superpowers](https://github.com/obra/superpowers) 技能一致处已写明；**冲突时以本文件 + 用户当轮明确指令为准**。

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

## 1. 讨论与设计阶段（brainstorming HARD GATE）

- 当用户表示 **仍在讨论方案、收敛设计、不写实现** 时：**禁止** 修改业务代码、配置、proto、生成脚本；**仅** 允许更新 **`docs/superpowers/specs/`** 等设计文档，且须在对话中征得 **「可以落盘」** 类明确许可后再改文档。
- **未** 获得用户对设计/范围的明确批准前，**不** 进入实现。

---

## 2. 实现计划批准门闩（writing-plans）

凡 **多步骤、多文件、或行为变更** 的实现工作：

1. 必须存在 **`docs/superpowers/plans/YYYY-MM-DD-<feature>.md`**（或用户指定的等价路径），且含 **Goal / Architecture / 分 Task + checkbox**。
2. **禁止** 在对话中 **未出现用户明确的计划批准** 的情况下开始写生产代码。用户批准示例（满足其一即可）：
   - 「**批准该实现计划，从 Task N 开始**」
   - 「**可以按 `2026-…-plan.md` 执行**」
3. 计划文件 **建议** 在文首 Goal 下方包含 **「批准记录」** 占位（见 §6 模板），人类批准后可手写日期与范围。

**单处 typo、单文件纯注释** 等极小改动可豁免整份计划，但须在对话中说明豁免理由。

---

## 3. TDD 门闩（test-driven-development）

对 **新功能、bug 修复、行为变更、重构**：

- **禁止** 先写生产代码再补测试作为默认路径。
- **必须**：**先写会失败的测试** → 运行确认失败 → **最小实现** 使测试通过 → 重构。
- **例外** 仅当用户在当轮对话中 **明文声明**（例如：「本步豁免 TDD，仅生成/配置」）——仅限生成代码、纯配置、一次性脚本等。

集成/Etcd 等难以单测的部分：**仍须** 先有 **可失败** 的测试策略（接口 mock、fake、`testing.Short()` 跳过集成、或独立 `*_integration_test.go`），再在对话中说明；**禁止**「整段实现写完再随便加一个测试」作为糊弄。

---

## 4. 多任务实现（subagent-driven-development）

当实现计划含 **多个独立 Task** 时：

- **默认** 应按 superpowers **subagent-driven-development**：**一 Task 一 subagent**（或用户要求的 **executing-plans** 并行会话），并在任务后做 **规格符合性审查** 与 **代码质量审查**。
- **禁止** 在无理由的情况下，在 **单一会话内** 连续吞掉整份计划而跳过上述分工（除非用户 **明确** 说「本仓库本步由单会话执行、豁免 subagent」）。

---

## 5. 完成前验证（verification-before-completion）

- **禁止** 在未运行约定测试/构建并 **核对输出** 前，声称「已完成」「CI 会通过」。
- 默认命令：`go test ./...`（在 `go.work` 根目录）及受影响模块的 `go build`；若项目另有脚本，以计划或用户指令为准。

---

## 6. 计划文首「批准记录」模板（建议粘贴到各 plan）

```markdown
## 批准记录（人类门闩）

- **状态**：待批准 / 已批准
- **批准范围**：例如「全文」或「Task 3～5」
- **批准人 / 日期**：（手填）
- **TDD**：强制 / 豁免（若豁免须写理由）
- **Subagent**：按任务拆分 / 本步单会话豁免（若豁免须写理由）
```

---

## 7. 文档与规格索引

- 路线图与设计：`docs/README.md`
- 网关与 Etcd meta 等专题：见 `docs/superpowers/specs/` 与 `docs/superpowers/plans/`

---

## 8. 违反本文件时的处理

Agent 若发现用户指令与本文件冲突，**应先简要说明冲突** 并 **请求用户二选一**（遵循用户指令 **或** 遵循本仓库流程），**不得静默忽略** 本文件。
