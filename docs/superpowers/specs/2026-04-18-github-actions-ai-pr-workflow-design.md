# GitHub Actions AI PR 自动化工作流设计

## 批准记录（人类门闩）

- **状态**：已批准
- **批准范围**：全文
- **批准人 / 日期**：用户 / 2026-04-18
- **TDD**：不适用（CI/CD 配置）
- **Subagent**：按任务拆分

---

## 1. 目标

实现本地提交代码后，通过 `gh` CLI 创建 PR，GitHub Actions 自动执行 CI 检查和 AI 代码审查，人工确认后合并。

---

## 2. 整体流程

```
┌─────────────────┐     ┌─────────────────────────────────┐
│  本地开发完成    │────►│  gh pr create --fill            │
│  git push       │     │  (自动生成 PR 标题和描述)        │
└─────────────────┘     └───────────────┬─────────────────┘
                                        │
                                        ▼
                        ┌─────────────────────────────────┐
                        │  GitHub Actions 触发            │
                        │  ┌───────────────────────────┐  │
                        │  │ Job 1: CI 检查            │  │
                        │  │ - go build ./...          │  │
                        │  │ - go test ./...           │  │
                        │  └─────────────┬─────────────┘  │
                        │                │ 通过            │
                        │  ┌─────────────▼─────────────┐  │
                        │  │ Job 2: AI 分析            │  │
                        │  │ - 获取 PR diff            │  │
                        │  │ - 调用自定义 AI API       │  │
                        │  │ - 生成审查评论            │  │
                        │  └─────────────┬─────────────┘  │
                        └────────────────┼─────────────────┘
                                         │
                                         ▼
                        ┌─────────────────────────────────┐
                        │  PR 评论展示结果                │
                        │  - CI 状态                      │
                        │  - AI 审查结果                  │
                        │  - 是否可以合并                 │
                        └───────────────┬─────────────────┘
                                        │
                                        ▼
                        ┌─────────────────────────────────┐
                        │  人工确认，点击 Merge 按钮      │
                        └─────────────────────────────────┘
```

---

## 3. 本地命令

```bash
# 推送分支并创建 PR
git push -u origin <branch-name>
gh pr create --fill
```

`--fill` 参数会自动从 commit message 生成 PR 标题和描述。

---

## 4. AI 功能

| 功能 | 触发时机 | 输出 |
|------|----------|------|
| **PR 描述生成** | PR 创建时 | 自动填充标题和描述 |
| **代码审查** | PR 创建/更新时 | 评论中列出问题、建议 |
| **测试失败分析** | CI 失败时 | 评论中分析原因、给修复建议 |

---

## 5. AI 审查输出示例

```markdown
## 🤖 AI 代码审查

### 代码质量
- ✅ 代码风格符合规范
- ✅ 无明显安全风险

### 潜在问题
1. **server.go:52** - `db.Ping` 未处理超时
   建议添加: `ctx, cancel := context.WithTimeout(ctx, 5*time.Second)`

2. **loader.go:88** - Watch 错误仅记录日志，可能丢失事件

### 建议
- 考虑为 RouteLoader 添加重连机制

---

## ✅ CI 状态

- Build: ✅ 通过
- Test: ✅ 27/27 通过

---

> **状态: 可以合并** - 请人工确认后点击 Merge
```

---

## 6. 文件结构

```
.github/
└── workflows/
    ├── ci.yml           # CI 检查 (build + test)
    └── ai-review.yml    # AI 代码审查
```

---

## 7. GitHub Secrets 配置

在 GitHub 仓库 Settings → Secrets and variables → Actions 中添加：

| Secret 名称 | 说明 | 示例 |
|-------------|------|------|
| `AI_API_URL` | 自定义 AI API 地址 | `https://api.example.com/v1` |
| `AI_API_KEY` | API 密钥 | `sk-xxx` |
| `AI_MODEL` | 模型名称 | `gpt-4` |

---

## 8. 合并条件

- ✅ CI 检查通过（build + test）
- ✅ AI 审查完成
- ⚠️ **人工确认后手动点击 Merge**

---

## 9. 技术实现

### 9.1 CI Workflow (ci.yml)

```yaml
name: CI

on:
  pull_request:
    branches: [main, master]

jobs:
  build-and-test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      
      - uses: actions/setup-go@v5
        with:
          go-version: '1.26'
      
      - name: Build
        run: go build ./...
      
      - name: Test
        run: go test ./... -v
```

### 9.2 AI Review Workflow (ai-review.yml)

```yaml
name: AI Review

on:
  pull_request:
    branches: [main, master]

jobs:
  ai-review:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
        with:
          fetch-depth: 0
      
      - name: Get PR diff
        id: diff
        run: |
          git diff origin/main...HEAD > pr.diff
          echo "diff<<EOF" >> $GITHUB_OUTPUT
          cat pr.diff >> $GITHUB_OUTPUT
          echo "EOF" >> $GITHUB_OUTPUT
      
      - name: AI Analysis
        id: ai
        env:
          AI_API_URL: ${{ secrets.AI_API_URL }}
          AI_API_KEY: ${{ secrets.AI_API_KEY }}
          AI_MODEL: ${{ secrets.AI_MODEL }}
        run: |
          # 调用 AI API 分析代码
          response=$(curl -s -X POST "${AI_API_URL}/chat/completions" \
            -H "Authorization: Bearer ${AI_API_KEY}" \
            -H "Content-Type: application/json" \
            -d "{
              \"model\": \"${AI_MODEL}\",
              \"messages\": [
                {\"role\": \"system\", \"content\": \"你是一个代码审查专家\"},
                {\"role\": \"user\", \"content\": \"请审查以下代码变更:\\n${{ steps.diff.outputs.diff }}\"}
              ]
            }")
          
          # 提取分析结果并保存
          echo "$response" | jq -r '.choices[0].message.content' > review.md
      
      - name: Comment on PR
        uses: actions/github-script@v7
        with:
          script: |
            const fs = require('fs');
            const review = fs.readFileSync('review.md', 'utf8');
            await github.rest.issues.createComment({
              owner: context.repo.owner,
              repo: context.repo.repo,
              issue_number: context.issue.number,
              body: `## 🤖 AI 代码审查\n\n${review}`
            });
```

---

## 10. 前置条件

- [ ] 安装 `gh` CLI (`brew install gh`)
- [ ] 登录 GitHub (`gh auth login`)
- [ ] 配置 AI API Secrets

---

## 11. 后续扩展

- 自动标签分类（bug/feature/docs）
- 代码覆盖率检查
- 多语言支持
