# GitHub Actions AI PR 工作流实现计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 实现 GitHub Actions CI 检查和 AI 代码审查自动化工作流

**Architecture:** 创建 .github/workflows/ 目录，添加 ci.yml 和 ai-review.yml 两个 workflow 文件

**Tech Stack:** GitHub Actions, GitHub CLI (gh), OpenAI 兼容 API

**设计文档:** `docs/superpowers/specs/2026-04-18-github-actions-ai-pr-workflow-design.md`

---

## 文件结构

| 文件 | 职责 | 操作 |
|------|------|------|
| `.github/workflows/ci.yml` | CI 检查 (build + test) | 新建 |
| `.github/workflows/ai-review.yml` | AI 代码审查 | 新建 |

---

## Task 1: 创建 CI Workflow

**Files:**
- Create: `.github/workflows/ci.yml`

- [ ] **Step 1: 创建 .github/workflows 目录**

```bash
mkdir -p .github/workflows
```

- [ ] **Step 2: 创建 ci.yml 文件**

```yaml
name: CI

on:
  pull_request:
    branches: [main, master]
  push:
    branches: [main, master]

jobs:
  build-and-test:
    runs-on: ubuntu-latest
    steps:
      - name: Checkout
        uses: actions/checkout@v4
      
      - name: Setup Go
        uses: actions/setup-go@v5
        with:
          go-version: '1.26'
          cache: true
      
      - name: Build
        run: go build ./...
      
      - name: Test
        run: go test ./... -v -timeout 10m
```

- [ ] **Step 3: 验证文件创建**

```bash
ls -la .github/workflows/ci.yml
```

Expected: 文件存在

- [ ] **Step 4: Commit**

```bash
git add .github/workflows/ci.yml
git commit -m "feat(ci): add GitHub Actions CI workflow"
```

---

## Task 2: 创建 AI Review Workflow

**Files:**
- Create: `.github/workflows/ai-review.yml`

- [ ] **Step 1: 创建 ai-review.yml 文件**

```yaml
name: AI Review

on:
  pull_request:
    branches: [main, master]
    types: [opened, synchronize, reopened]

permissions:
  contents: read
  pull-requests: write
  issues: write

jobs:
  ai-review:
    runs-on: ubuntu-latest
    steps:
      - name: Checkout
        uses: actions/checkout@v4
        with:
          fetch-depth: 0
      
      - name: Get PR diff
        id: diff
        run: |
          git fetch origin ${{ github.base_ref }}
          git diff origin/${{ github.base_ref }}...HEAD > pr.diff
          echo "diff_length=$(wc -l < pr.diff)" >> $GITHUB_OUTPUT
      
      - name: AI Code Review
        if: steps.diff.outputs.diff_length != '0'
        id: ai
        env:
          AI_API_URL: ${{ secrets.AI_API_URL }}
          AI_API_KEY: ${{ secrets.AI_API_KEY }}
          AI_MODEL: ${{ secrets.AI_MODEL }}
        run: |
          # 准备 diff 内容
          DIFF_CONTENT=$(cat pr.diff | head -c 30000)
          
          # 构建 prompt
          PROMPT="请审查以下代码变更，从以下角度分析：
          1. 代码质量和风格
          2. 潜在的 bug 或安全问题
          3. 性能问题
          4. 可读性和可维护性
          
          请用中文回复，格式如下：
          ## 代码质量
          - ✅ 或 ❌ 评价
          
          ## 潜在问题
          1. 文件:行号 - 问题描述
          
          ## 建议
          - 改进建议
          
          代码变更：
          \`\`\`diff
          ${DIFF_CONTENT}
          \`\`\`"
          
          # 调用 AI API
          RESPONSE=$(curl -s -X POST "${AI_API_URL}/chat/completions" \
            -H "Authorization: Bearer ${AI_API_KEY}" \
            -H "Content-Type: application/json" \
            -d "{
              \"model\": \"${AI_MODEL}\",
              \"messages\": [
                {\"role\": \"system\", \"content\": \"你是一个专业的代码审查专家，擅长发现代码问题和给出改进建议。\"},
                {\"role\": \"user\", \"content\": \"${PROMPT}\"}
              ],
              \"temperature\": 0.3
            }")
          
          # 提取结果
          echo "$RESPONSE" | jq -r '.choices[0].message.content' > review_result.md
          
          # 检查是否有错误
          if [ "$(echo "$RESPONSE" | jq -r '.error')" != "null" ]; then
            echo "AI API Error: $(echo "$RESPONSE" | jq -r '.error.message')"
            exit 1
          fi
      
      - name: Comment on PR
        if: steps.diff.outputs.diff_length != '0'
        uses: actions/github-script@v7
        with:
          script: |
            const fs = require('fs');
            const review = fs.readFileSync('review_result.md', 'utf8');
            
            const body = `## 🤖 AI 代码审查
            
            ${review}
            
            ---
            
            > **注意**: AI 审查仅供参考，请结合实际情况判断。
            > 如有问题，请人工确认后点击 Merge 按钮。`;
            
            await github.rest.issues.createComment({
              owner: context.repo.owner,
              repo: context.repo.repo,
              issue_number: context.issue.number,
              body: body
            });
      
      - name: Review Summary
        if: steps.diff.outputs.diff_length == '0'
        run: |
          echo "No diff found, skipping AI review"
```

- [ ] **Step 2: 验证文件创建**

```bash
ls -la .github/workflows/ai-review.yml
```

Expected: 文件存在

- [ ] **Step 3: Commit**

```bash
git add .github/workflows/ai-review.yml
git commit -m "feat(ci): add AI code review workflow"
```

---

## Task 3: 添加 Secrets 配置说明

**Files:**
- Create: `.github/SECRETS.md`

- [ ] **Step 1: 创建 Secrets 配置说明文档**

```markdown
# GitHub Secrets 配置

本仓库的 GitHub Actions 需要配置以下 Secrets。

## 配置方法

1. 进入 GitHub 仓库页面
2. 点击 Settings → Secrets and variables → Actions
3. 点击 "New repository secret" 添加以下 Secrets

## 必需 Secrets

| 名称 | 说明 | 示例 |
|------|------|------|
| `AI_API_URL` | AI API 地址（OpenAI 兼容协议） | `https://api.openai.com/v1` |
| `AI_API_KEY` | API 密钥 | `sk-xxx` |
| `AI_MODEL` | 模型名称 | `gpt-4` |

## 验证

配置完成后，创建一个 PR 测试 AI Review 是否正常工作。
```

- [ ] **Step 2: Commit**

```bash
git add .github/SECRETS.md
git commit -m "docs: add GitHub Secrets configuration guide"
```

---

## Task 4: 验证并推送

**Files:**
- 无新文件

- [ ] **Step 1: 验证所有文件存在**

```bash
ls -la .github/workflows/
ls -la .github/SECRETS.md
```

Expected: ci.yml, ai-review.yml, SECRETS.md 都存在

- [ ] **Step 2: 推送到远程**

```bash
git push -u origin feat/github-actions-ai-pr
```

- [ ] **Step 3: 创建 PR**

```bash
gh pr create --title "feat: add GitHub Actions CI and AI review workflow" --body "$(cat <<'EOF'
## Summary

This PR adds GitHub Actions automation for CI and AI code review:

1. **CI Workflow** (`ci.yml`)
   - Build check: `go build ./...`
   - Test check: `go test ./...`

2. **AI Review Workflow** (`ai-review.yml`)
   - Automatic code review on PR creation/update
   - Uses OpenAI-compatible API
   - Comments on PR with review results

3. **Secrets Guide** (`SECRETS.md`)
   - Configuration instructions for AI API

## Test Plan

- [ ] Configure GitHub Secrets (AI_API_URL, AI_API_KEY, AI_MODEL)
- [ ] Merge this PR
- [ ] Create a test PR to verify workflows work correctly

## Required Secrets

| Secret | Description |
|--------|-------------|
| `AI_API_URL` | AI API URL (OpenAI compatible) |
| `AI_API_KEY` | API Key |
| `AI_MODEL` | Model name |
EOF
)"
```

---

## 自我审查

| 检查项 | 状态 |
|--------|------|
| **Spec 覆盖** | ✅ CI + AI Review + Secrets 文档 |
| **Placeholder 扫描** | ✅ 无 TBD/TODO |
| **类型一致性** | ✅ 无类型冲突 |

**覆盖检查：**
- [x] CI Workflow → Task 1
- [x] AI Review Workflow → Task 2
- [x] Secrets 配置说明 → Task 3
- [x] 推送和创建 PR → Task 4

---

## 执行方式选择

**Plan complete and saved to `docs/superpowers/plans/2026-04-18-github-actions-ai-pr-workflow-plan.md`.**

**Two execution options:**

**1. Subagent-Driven (recommended)** - I dispatch a fresh subagent per task, review between tasks, fast iteration

**2. Inline Execution** - Execute tasks in this session using executing-plans, batch execution with checkpoints for review

**Which approach?**
