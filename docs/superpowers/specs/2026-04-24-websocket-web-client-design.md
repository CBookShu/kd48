# WebSocket Web 客户端设计文档

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**目标：** 将 Web 客户端从 HTTP REST API 改为使用 WebSocket 进行所有后端通信。

**架构：** 单一 WebSocket 连接配合 Promise 风格的 API 封装。Web 客户端建立一个 WebSocket 连接到网关，用于所有 API 调用（登录、注册、签到、物品）。通过匹配请求和响应的 `method` 字段进行消息关联。

**技术栈：** JavaScript (ES6+)、WebSocket API、Promise/async-await

---

## 问题描述

当前 Web 客户端 (`web/js/api.js`) 使用 HTTP REST API：
```javascript
fetch('http://localhost:8080/api/user.v1.UserService/Login', {...})
```

但网关只提供：
- `/health` - 健康检查
- `/ws` - WebSocket 端点，用于 gRPC 调用
- `/web/*` - 静态文件服务

没有 `/api/*` HTTP REST 端点。Web 客户端需要使用 WebSocket 进行所有通信。

**额外问题：** 网关的 `WrapIngress` 函数没有将 `user_id` 注入到 context 中，但后端服务期望通过 `ctx.Value("user_id")` 获取用户 ID。这个问题必须修复。

---

## 解决方案：WebSocket API 客户端

### 架构概览

```
┌─────────────────────────────────────────────────────────┐
│  Web 浏览器                                              │
│  ┌─────────────────────────────────────────────────┐   │
│  │  api.js (WsClient)                              │   │
│  │  - connect() → 建立 WebSocket 连接              │   │
│  │  - call(method, payload) → Promise              │   │
│  │  - 通过 method 字段匹配请求/响应                 │   │
│  └─────────────────────────────────────────────────┘   │
│           │                                              │
│  ┌────────┴───────┬───────────┬───────────┐            │
│  │  login.html    │ checkin.html │ items.html │        │
│  │  register.html │              │           │         │
│  └────────────────┴───────────┴───────────┘            │
└─────────────────────────────────────────────────────────┘
                         │ WebSocket
                         ▼
┌─────────────────────────────────────────────────────────┐
│  网关 (:8080)                                            │
│  /ws → WebSocket Handler → 路由 → 后端 gRPC             │
└─────────────────────────────────────────────────────────┘
```

### 消息协议

**请求格式：**
```json
{
  "method": "/user.v1.UserService/Login",
  "payload": "{\"username\":\"alice\",\"password\":\"secret\"}"
}
```

**响应格式：**
```json
{
  "method": "/user.v1.UserService/Login",
  "code": 0,
  "msg": "success",
  "data": {"success": true, "token": "abc...", "user_id": 123}
}
```

**错误响应：**
```json
{
  "method": "/user.v1.UserService/Login",
  "code": 16,
  "msg": "用户名或密码错误",
  "data": null
}
```

### WsClient 类

```javascript
class WsClient {
  constructor(url)
  connect()                    // 建立 WebSocket 连接
  disconnect()                 // 关闭连接
  isConnected()                // 检查连接状态
  call(method, payload)        // 通用 API 调用，返回 Promise

  // 便捷方法
  login(username, password)
  register(username, password)
  getCheckinStatus()
  checkin()
  getMyItems()

  // Token 管理
  saveToken(token)
  getToken()
  clearToken()
}
```

### 连接流程

**未认证页面（login.html, register.html）：**
1. 页面加载
2. 创建 WsClient 实例
3. 调用 `connect()` 建立 WebSocket 连接
4. 调用 `login()` 或 `register()`
5. 成功后保存 token 并跳转

**已认证页面（checkin.html, items.html, index.html）：**
1. 页面加载
2. 检查是否存在 token
3. 如果没有 token，跳转到登录页
4. 创建 WsClient 实例
5. 调用 `connect()` 建立 WebSocket 连接
6. 发起已认证的 API 调用

### 错误处理

| 错误码 | 含义 | 处理方式 |
|--------|------|----------|
| 0 | 成功 | 处理数据 |
| 16 | 未认证 | 跳转到登录页 |
| 3 | 参数错误 | 显示错误信息 |
| 6 | 已存在 | 显示"用户名已存在" |
| 其他 | 其他错误 | 显示错误信息 |

| 场景 | 行为 |
|------|------|
| 连接失败 | 显示错误信息并提供重试按钮 |
| 请求超时 (5秒) | 显示"请求超时"错误 |
| WebSocket 关闭 | 显示"连接断开"错误 |

---

## 后端改动

### 1. `gateway/internal/ws/wrapper.go` (修改)

**问题：** `WrapIngress` 没有将 `user_id` 注入到 context 中，但后端服务期望 `ctx.Value("user_id")`。

**修复：** 更新 `WrapIngress`，在用户已认证时将 `meta.userID` 注入到 context。

```go
func WrapIngress(cli gatewayv1.GatewayIngressClient, route string) WsHandlerFunc {
    return func(ctx context.Context, payload []byte, meta *clientMeta) (*WsHandlerResult, error) {
        // 如果已认证，将 user_id 注入到 context
        if meta.userID > 0 {
            ctx = context.WithValue(ctx, "user_id", meta.userID)
        }

        reply, err := cli.Call(ctx, &gatewayv1.IngressRequest{
            Route:       route,
            JsonPayload: payload,
        })
        if err != nil {
            return nil, err
        }
        return &WsHandlerResult{JSON: reply.GetJsonPayload()}, nil
    }
}
```

---

## 前端改动

### 1. `web/js/api.js` (重写)

**新实现：**
- WsClient 类，包含 WebSocket 连接管理
- Promise 风格的 `call()` 方法，用于请求/响应关联
- 便捷方法：`login()`、`register()`、`checkin()`、`getCheckinStatus()`、`getMyItems()`
- Token 存储在 localStorage
- 连接状态管理

### 2. `web/login.html` (修改)

**改动：**
- 页面加载时创建 WsClient 实例
- 调用 `wsClient.login()` 替代 `login()` (HTTP fetch)
- 处理连接错误
- 成功后：保存 token，跳转到 `/`

### 3. `web/register.html` (修改)

**改动：**
- 页面加载时创建 WsClient 实例
- 调用 `wsClient.register()` 替代 `register()` (HTTP fetch)
- 处理连接错误
- 成功后：跳转到登录页

### 4. `web/checkin.html` (修改)

**改动：**
- 页面加载时创建 WsClient 实例
- 调用 `wsClient.getCheckinStatus()` 替代 `getCheckinStatus()`
- 调用 `wsClient.checkin()` 替代 `checkin()`
- 处理连接错误和认证错误

### 5. `web/items.html` (修改)

**改动：**
- 页面加载时创建 WsClient 实例
- 调用 `wsClient.getMyItems()` 替代 `getMyItems()`
- 处理连接错误和认证错误

### 6. `web/index.html` (修改)

**改动：**
- 页面加载时创建 WsClient 实例
- 更新认证检查，使用新的 token 管理

---

## 测试策略

### 手动测试

1. **注册流程：**
   - 打开 `/register.html`
   - 输入用户名和密码
   - 提交表单
   - 验证注册成功
   - 验证跳转到登录页

2. **登录流程：**
   - 打开 `/login.html`
   - 输入已注册的凭据
   - 提交表单
   - 验证登录成功
   - 验证跳转到首页
   - 验证 token 存储在 localStorage

3. **签到流程：**
   - 导航到 `/checkin.html`
   - 验证状态正确加载
   - 点击签到按钮
   - 验证奖励显示正确

4. **物品流程：**
   - 导航到 `/items.html`
   - 验证物品正确加载

5. **错误情况：**
   - 尝试用已存在的用户名注册
   - 尝试用错误的密码登录
   - 断开网络，验证错误处理

---

## 约束

- 每个页面单一 WebSocket 连接
- 不使用消息 ID 关联（使用 method 字段匹配）
- Token 存储在 localStorage
- 连接超时：5 秒
- 请求超时：5 秒

---

## 不在范围内

- WebSocket 重连（未来增强）
- 客户端心跳/ping-pong（服务端处理）
- 多并发请求关联（当前协议使用 method 匹配）
