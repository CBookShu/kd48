# kd48 WebSocket 心跳机制设计文档

**日期**: 2026-04-20  
**作者**: Claude  
**状态**: 已批准待实施  
**工作树**: todo-analysis

---

## 1. 概述

本文档定义 kd48 网关 WebSocket 连接的心跳机制设计。核心原则是**服务端被动、客户端主动**，通过将保活责任分散到客户端来降低服务端负载，同时确保 WebSocket 协议合规。

---

## 2. 设计目标

| 目标 | 说明 |
|------|------|
| **协议合规** | 严格遵守 WebSocket RFC 6455，服务端收到 Ping 必须回复 Pong |
| **负载分散** | 心跳保活责任完全由客户端承担，服务端只做被动响应 |
| **连接稳定** | 能够及时检测死连接并清理，避免资源泄漏 |
| **简单可靠** | 基于活动时间检测，避免复杂状态机，易于理解和维护 |

---

## 3. 协议规范

### 3.1 WebSocket Ping/Pong 机制

根据 [RFC 6455 Section 5.5.2](https://datatracker.ietf.org/doc/html/rfc6455#section-5.5.2)：

- **Ping Frame**: 一方发送，用于探测对方是否存活
- **Pong Frame**: 收到 Ping 后必须回复 Pong（协议强制）
- **自动回复**: `fasthttp/websocket` 不会自动回复 Pong，需要手动处理

### 3.2 当前实现问题

现有代码(`handler.go:99-104`)存在协议违规：

```go
if msgType == websocket.PingMessage {
    if h.connManager != nil {
        h.connManager.RecordActivity(clientID)
    }
    continue  // ❌ 违反协议：未回复 Pong
}
```

**风险**: 某些 WebSocket 客户端在收不到 Pong 时会主动断开连接。

---

## 4. 设计方案

### 4.1 架构概览

```
┌──────────────┐                    ┌──────────────┐
│    Client    │                    │    Server    │
└──────┬───────┘                    └──────┬───────┘
       │                                   │
       │  ① WebSocket Ping Frame           │
       │ ────────────────────────────────> │
       │                                   │
       │  ② WebSocket Pong Frame           │
       │ <──────────────────────────────── │
       │    (协议强制回复)                  │
       │                                   │
       │  ③ Record lastActivity            │
       │    (内部计时)                     │
       │                                   │
       │         ... 时间流逝 ...           │
       │                                   │
       │  ④ 超过 server_timeout 无活动      │
       │    服务端发送 Close 并断开         │
       │ <────────────────────X (Close)    │
       │                                   │
       │  ⑤ Client 重连（如需要）           │
       │      ...                           │
```

### 4.2 组件职责

| 组件 | 职责 | 位置 |
|------|------|------|
| **WebSocket Client** | 每 `client_interval` 发送 Ping Frame | 客户端实现 |
| **Handler.ServeWS** | 收到 Ping → 回复 Pong → 记录活动时间 | `handler.go` |
| **HeartbeatManager** | 管理 `lastActivity` 时间戳，检测超时 | `heartbeat.go` |
| **ConnectionManager** | 定时扫描所有连接，断开超时连接 | `connection_manager.go` |

### 4.3 服务端行为

**被动接收心跳**:
1. 接收客户端发送的 `Ping Frame`
2. 立即回复 `Pong Frame`（RFC 6455 要求）
3. 调用 `RecordActivity()` 更新活动时间戳

**超时检测**:
1. `ConnectionManager` 每 `check_interval` 扫描所有连接
2. 对超过 `server_timeout` 未活动的连接，主动断开
3. 断开时发送 WebSocket Close Frame（带原因说明）

**异常处理**:
- 客户端不发 Ping：超时后服务端断开连接
- 网络中断：超时检测后清理资源
- 大量连接：扫描间隔和超时时间可配置

### 4.4 客户端行为（建议实现）

**心跳发送**:
- 每 30 秒发送一次 `Ping Frame`
- 帧内容可为空（`[]byte{}`）

**超时处理**:
- 发送 Ping 后，5 秒内必须收到 Pong
- 超时未收到：认为连接已死，主动断开并重连

**连接恢复**:
- 检测到断开后，按指数退避策略重连
- 重连成功后重新鉴权（如需要）

---

## 5. 配置参数

### 5.1 服务端配置

```yaml
# config.yaml 建议配置
gateway:
  heartbeat:
    server_timeout: 90s    # 无活动超时时间
    check_interval: 5s     # 服务端扫描间隔
```

| 参数 | 值 | 说明 |
|------|-----|------|
| `server_timeout` | 90s | 超过此时间无 Ping/业务消息，服务端断开连接 |
| `check_interval` | 5s | 服务端扫描间隔，检查是否有超时连接 |

**超时设置计算**:
```
client_interval × 3 < server_timeout < client_interval × 4
```
推荐: 30s × 3 = 90s，给客户端 3 次心跳重试机会

### 5.2 客户端推荐配置

| 参数 | 推荐值 | 说明 |
|------|--------|------|
| `client_interval` | 30s | 心跳发送间隔 |
| `pong_wait_timeout` | 5s | 等待 Pong 回复超时 |
| `reconnect_delay` | 1s, 2s, 4s... | 重连退避策略 |

---

## 6. 代码变更

### 6.1 文件: `gateway/internal/ws/handler.go`

**位置**: `ServeWS` 方法，消息类型判断处

**变更前**:
```go
// 处理 Ping 消息（客户端主动发送的 Ping）
if msgType == websocket.PingMessage {
    if h.connManager != nil {
        h.connManager.RecordActivity(clientID)
    }
    continue
}
```

**变更后**:
```go
// 处理 Ping 消息（客户端主动发送的 Ping）
// RFC 6455: 服务端收到 Ping 必须回复 Pong
if msgType == websocket.PingMessage {
    // 回复 Pong（协议强制）
    _ = conn.WriteControl(websocket.PongMessage, []byte{},
        time.Now().Add(1*time.Second))

    // 记录活动时间（用于超时检测）
    if h.connManager != nil {
        h.connManager.RecordActivity(clientID)
    }
    continue
}
```

**变更说明**:
- 新增 Pong 回复（使用 `WriteControl` 确保及时发送）
- 其余逻辑（活动记录、超时检测）已存在无需修改

### 6.2 文件: `config.yaml`

**新增**:
```yaml
gateway:
  heartbeat:
    server_timeout: 90s
    check_interval: 5s
```

---

## 7. 与现有代码集成

### 7.1 现有已存在组件

以下组件已在 `todo-analysis` 分支实现：

| 组件 | 文件 | 状态 |
|------|------|------|
| `HeartbeatManager` | `heartbeat.go` | ✅ 已实现 |
| `ConnectionManager` | `connection_manager.go` | ✅ 已实现 |
| 单元测试 | `*_test.go` | ✅ 已覆盖 |

### 7.2 集成点

```
handler.go ServeWS()
    │
    ├── 收到 Ping Frame
    ├── WriteControl(Pong)  ← 本次新增
    └── connManager.RecordActivity()
            │
            ▼
    heartbeat.RecordActivity() (更新 lastActivity)
            │
            ▼
    ConnectionManager.Start() (后台 goroutine)
            │
            ├── ticker (5s)
            ├── CheckTimeout() (检查 lastActivity)
            └── disconnectClientUnlocked() (超时时断开)
```

---

## 8. 测试策略

### 8.1 单元测试

| 测试项 | 说明 |
|--------|------|
| `TestHeartbeatManager_CheckTimeout` | 验证超时检测逻辑（已存在） |
| `TestConnectionManager_HeartbeatTimeout` | 验证连接断开逻辑（已存在） |
| `TestHandler_PingPong` | 新增：验证收到 Ping 回复 Pong |

### 8.2 集成测试

1. **心跳保活测试**:
   - 客户端每 30s 发送 Ping
   - 验证服务端 90s 内不主动断开

2. **超时断开测试**:
   - 客户端停止发送心跳
   - 验证服务端 90-95s 后断开连接

3. **协议合规测试**:
   - 抓包验证 Pong 帧正确回复

---

## 9. 风险与缓解

| 风险 | 可能性 | 影响 | 缓解措施 |
|------|--------|------|----------|
| 某些客户端不实现 Ping | 中 | 高 | 任何业务消息也调用 `RecordActivity`，业务活跃时不强制心跳 |
| 网络延迟导致误判超时 | 低 | 中 | `server_timeout` 设置为 3×心跳间隔，留有裕量 |
| 大量连接扫描开销 | 低 | 低 | `check_interval=5s` 足够稀疏，连接数高时可调大 |

---

## 10. 成功标准

### 10.1 功能标准
- [ ] 服务端收到 Ping 立即回复 Pong（抓包验证）
- [ ] 客户端心跳保活，连接不被误断
- [ ] 客户端停发心跳后，服务端 90s 内断开
- [ ] 断开的连接资源被正确清理

### 10.2 性能标准
- [ ] 服务端不主动发起任何网络请求
- [ ] 心跳处理代码路径简洁（无复杂计算）
- [ ] 5000 并发连接下扫描无明显 CPU 峰值

---

## 11. 附录

### 11.1 相关文档
- [RFC 6455 - WebSocket Protocol](https://datatracker.ietf.org/doc/html/rfc6455)
- `2026-04-19-todo-analysis-and-next-steps-design.md` - 原始分析文档

### 11.2 相关文件
- `gateway/internal/ws/handler.go` - 心跳处理入口
- `gateway/internal/ws/heartbeat.go` - 心跳管理器
- `gateway/internal/ws/connection_manager.go` - 连接管理器
- `gateway/internal/ws/*_test.go` - 测试文件

### 11.3 变更范围
| 文件 | 变更类型 | 行数 |
|------|----------|------|
| `handler.go` | 新增 Pong 回复 | +3 ~ +5 行 |
| `config.yaml` | 新增配置项 | +4 行 |
| 测试文件 | 新增测试用例 | 约 50 行 |

---

**审批记录**:
- 2026-04-20: 设计确认（用户确认采用方案C，服务端被动模式）
