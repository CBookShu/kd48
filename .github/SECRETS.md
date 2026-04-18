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
