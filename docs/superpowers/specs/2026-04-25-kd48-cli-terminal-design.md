# KD48 CLI Terminal 设计文档

> **创建日期**: 2026-04-25
> **用途**: 替代原有 Web 前端的命令行终端程序
> **技术栈**: Go + WebSocket (Gateway 连接)

---

## 1. 目标

创建一个交互式命令行终端程序，模拟玩家完整操作流程：
- 登录/注册
- 每日签到
- 查看签到状态
- 查看背包物品

通过 WebSocket 连接 Gateway，真实模拟客户端行为。

---

## 2. 架构

```
┌─────────────────────────────────────────────────────────┐
│                     CLI Terminal                        │
│  ┌─────────────┐   ┌────────────┐   ┌───────────────┐  │
│  │  REPL Loop  │ → │  Commands  │ → │ WebSocket     │  │
│  │  (readline) │   │  Handler   │   │ Client        │  │
│  └─────────────┘   └────────────┘   └───────────────┘  │
└─────────────────────────────────────────────────────────┘
                                              │
                                              ▼
                               ┌─────────────────────────┐
                               │   WebSocket Gateway     │
                               │   (localhost:8080)      │
                               └─────────────────────────┘
```

### 文件结构

```
cmd/cli/
├── main.go                    # 入口，REPL 主循环
├── go.mod
└── go.sum

internal/
├── repl/
│   └── repl.go               # REPL 循环，命令解析
├── client/
│   └── gateway.go            # WebSocket 连接 Gateway
└── commands/
    └── handler.go            # 命令处理逻辑
```

---

## 3. 功能列表

### 3.1 连接管理

- 启动时自动连接 Gateway WebSocket
- 连接失败提示并退出
- 心跳保活（可选）

### 3.2 命令列表

| 命令 | 参数 | 登录前 | 登录后 | 说明 |
|------|------|--------|--------|------|
| `login` | `username password` | ✓ | ✓ | 登录，登录后切换用户 |
| `register` | `username password` | ✓ | ✓ | 注册新账户 |
| `checkin` | - | ✗ | ✓ | 每日签到 |
| `status` | - | ✗ | ✓ | 查看签到状态（详细）|
| `items` | - | ✗ | ✓ | 查看背包物品 |
| `logout` | - | ✗ | ✓ | 登出 |
| `help` | - | ✓ | ✓ | 显示帮助 |
| `quit` / `exit` | - | ✓ | ✓ | 退出程序 |

### 3.3 状态管理

程序维护内部状态：
- `isLoggedIn`: 是否已登录
- `username`: 当前用户名
- `userID`: 当前用户 ID
- `token`: 登录 token
- `todayChecked`: 今日是否已签到
- `continuousDays`: 连续签到天数

---

## 4. 交互设计

### 4.1 启动欢迎界面

```
══════════════════════════════════════════════════════════
  Welcome to KD48 CLI Terminal
══════════════════════════════════════════════════════════
  Available Commands:
    login <username> <password>    - Login
    register <username> <password> - Register new account
    help                           - Show all commands
    quit / exit                    - Exit
══════════════════════════════════════════════════════════
  Status: Not logged in
────────────────────────────────────────────────────────────────
kd48>
```

### 4.2 登录后界面

```
══════════════════════════════════════════════════════════
  Welcome to KD48 CLI Terminal
══════════════════════════════════════════════════════════
  Available Commands:
    login <username> <password>    - Switch account
    register <username> <password> - Register new account
    checkin                        - Daily check-in
    status                         - View check-in status
    items                          - View your items
    logout                         - Logout
    help                           - Show all commands
    quit / exit                    - Exit
══════════════════════════════════════════════════════════
  Status: Logged in as [john] | Checked: No | Streak: 3 days
────────────────────────────────────────────────────────────────
kd48>
```

### 4.3 命令执行示例

```
kd48> login john 123456
[发送请求...]
[成功] 已登录，当前用户: john

kd48> checkin
[发送请求...]
[成功] 签到成功！连续签到: 1 天
获得奖励: {金币: 100}

kd48> items
[发送请求...]
[成功] 背包物品:
  - 1001: 100  (金币)
  - 1002: 50   (体力)

kd48> status
┌─────────────────────────────────────────┐
│  Player Status                          │
├─────────────────────────────────────────┤
│  Username:    john                      │
│  User ID:     12345                     │
│  Check-in:    ✓ Done today              │
│  Streak:      1 consecutive day         │
│  Total Days:  1                         │
│  Next Reward: 6 days (500 coins)        │
└─────────────────────────────────────────┘

kd48> quit
Goodbye!
```

### 4.4 错误提示

```
kd48> login
[错误] 用法: login <username> <password>

kd48> checkin
[错误] 请先登录

kd48> unknowncommand
[错误] 未知命令，输入 'help' 查看可用命令

kd48> login wronguser wrongpass
[错误] 用户名或密码错误
```

---

## 5. WebSocket 消息格式

### 请求格式

通过 Gateway 的 Ingress API 发送 JSON：

```json
{
  "route": "/lobby.v1.CheckinService/Checkin",
  "jsonPayload": "{\"token\":\"<token>\"}"
}
```

### 响应格式

```json
{
  "code": 0,
  "msg": "success",
  "data": "..."
}
```

---

## 6. 依赖

- Go 1.21+
- gorilla/websocket - WebSocket 客户端
- readline - REPL 输入（带 Tab 补全）
- 项目内部 proto 生成代码

---

## 7. 实现顺序

1. 创建项目结构，添加依赖
2. 实现 WebSocket 客户端
3. 实现 REPL 循环
4. 实现命令处理器
5. 测试完整流程

---

## 8. 边界情况处理

| 场景 | 处理 |
|------|------|
| Gateway 未启动 | 提示 "无法连接 Gateway，请确保服务已启动" |
| 网络断开 | 提示 "连接已断开，输入 'quit' 退出" |
| Token 过期 | 自动登出，提示 "登录已过期，请重新登录" |
| 签到已领取 | 提示 "今日已签到" |
| 参数错误 | 提示正确用法 |

---

## 9. 非功能性要求

- 响应时间显示（可选）
- 日志输出（可选）
- 彩色输出（可选，提升体验）
