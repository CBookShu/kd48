# Session 失效与顶号机制设计

**日期**: 2026-04-21
**状态**: 已批准
**作者**: Claude

---

## 1. 概述

实现单终端策略：同一账号只能一个设备在线，新登录自动踢掉旧设备。

---

## 2. 产品策略

- **单终端模式**：同一账号同时只能一个设备在线
- 新登录时自动踢掉旧设备，无需用户手动操作
- 旧设备收到 WebSocket 关闭消息，提示 "session replaced"

---

## 3. Session 存储结构（双向映射）

| Redis Key | Value | TTL | 说明 |
|-----------|-------|-----|------|
| `user:session:{token}` | `{userID}:{username}` | Session TTL | 现有，Session 数据 |
| `user:{userID}:session` | `{token}` | Session TTL | 新增，当前有效 token |

---

## 4. 登录流程（User 服务）

```
1. 验证用户名密码
2. 查询 Redis: GET user:{userID}:session → 旧 token
3. 如有旧 token：
   - DEL user:session:{旧token}
   - DEL user:{userID}:session
   - PUBLISH kd48:session:invalidate {"user_id": userID}
4. 生成新 token（32字节随机）
5. SETEX user:session:{新token} {ttl} "{userID}:{username}"
6. SETEX user:{userID}:session {ttl} "{新token}"
7. 返回 token 给客户端
```

---

## 5. 网关订阅配置

| 配置项 | 值 |
|--------|------|
| 频道 | `kd48:session:invalidate` |
| 消息格式 | JSON `{"user_id": <int64>}` |

网关启动时订阅该频道，收到消息后执行断开流程。

---

## 6. ConnectionManager 改造

### 6.1 新增字段

```go
type ConnectionManager struct {
    heartbeat      *HeartbeatManager
    connections    map[string]*websocket.Conn  // clientID → conn（现有）
    userConnections map[int64]string           // userID → clientID（新增）
    connMu         sync.RWMutex
    stopCh         chan struct{}
    metrics        ConnectionMetrics
}
```

### 6.2 新增方法

```go
// RegisterUserConnection 登录成功后关联 userID 与 clientID
func (cm *ConnectionManager) RegisterUserConnection(userID int64, clientID string)

// GetUserClientID 根据 userID 查找 clientID
func (cm *ConnectionManager) GetUserClientID(userID int64) (string, bool)

// DisconnectByUserID 按 userID 断开连接（顶号时调用）
func (cm *ConnectionManager) DisconnectByUserID(userID int64, reason string)
```

### 6.3 DisconnectByUserID 实现

```go
func (cm *ConnectionManager) DisconnectByUserID(userID int64, reason string) {
    cm.connMu.Lock()
    defer cm.connMu.Unlock()

    clientID, exists := cm.userConnections[userID]
    if !exists {
        return // 该网关实例没有这个用户的连接
    }

    conn, exists := cm.connections[clientID]
    if exists && conn != nil {
        // 发送关闭消息
        conn.WriteMessage(websocket.CloseMessage,
            websocket.FormatCloseMessage(websocket.ClosePolicyViolation, reason))
        conn.Close()
    }

    // 清理映射
    delete(cm.connections, clientID)
    delete(cm.userConnections, userID)
    cm.metrics.ActiveConnections--
    cm.metrics.DisconnectedCount++

    slog.Info("user disconnected by session invalidate",
        "user_id", userID, "reason", reason)
}
```

---

## 7. Handler 改造

### 7.1 clientMeta 结构（已预留）

```go
type clientMeta struct {
    connID          uint64
    conn            *websocket.Conn
    clientID        string
    isAuthenticated bool
    userID          int64  // 登录成功后填充
    token           string // 登录成功后填充
}
```

### 7.2 登录成功后处理

Handler 处理登录响应时：
1. 解析响应中的 userID
2. 更新 `meta.userID = userID`
3. 调用 `connManager.RegisterUserConnection(userID, clientID)`

---

## 8. 网关启动订阅流程

### 8.1 main.go 新增

```go
// 创建 Redis 客户端（用于订阅）
rdb := redis.NewClient(&redis.Options{...})

// 启动 Session 失效订阅
sessionSubscriber := NewSessionInvalidationSubscriber(rdb, connManager)
go sessionSubscriber.Start(ctx)
```

### 8.2 SessionInvalidationSubscriber

```go
type SessionInvalidationSubscriber struct {
    rdb         *redis.Client
    connManager *ConnectionManager
    channel     string
}

func (s *SessionInvalidationSubscriber) Start(ctx context.Context) {
    pubsub := s.rdb.Subscribe(ctx, s.channel)
    defer pubsub.Close()

    ch := pubsub.Channel()
    for {
        select {
        case <-ctx.Done():
            return
        case msg := <-ch:
            var data struct { UserID int64 `json:"user_id"` }
            if err := json.Unmarshal([]byte(msg.Payload), &data); err != nil {
                slog.Warn("invalid session invalidate message", "error", err)
                continue
            }
            s.connManager.DisconnectByUserID(data.UserID, "session replaced")
        }
    }
}
```

---

## 9. 错误处理

| 场景 | 处理 |
|------|------|
| Redis 连接失败 | 网关启动失败，记录错误 |
| Pub/Sub 断开 | 自动重连（redis-go 内置） |
| 消息格式错误 | 记录警告日志，跳过该消息 |
| userID 无对应连接 | 忽略（其他网关实例处理） |

---

## 10. 测试要点

- [ ] 登录创建双向 Session 映射
- [ ] 新登录删除旧 Session 并发布通知
- [ ] 网关收到通知后断开对应连接
- [ ] 连接断开后清理两个映射表
- [ ] Session TTL 过期后自动清理

---

## 11. 涉及文件

| 文件 | 改动 |
|------|------|
| `services/user/cmd/user/server.go` | 双向映射 + Pub/Sub |
| `services/user/cmd/user/main.go` | 配置 Redis Pub/Sub 频道 |
| `gateway/cmd/gateway/main.go` | 启动 Redis 订阅 |
| `gateway/internal/ws/connection_manager.go` | 新增 userConnections + DisconnectByUserID |
| `gateway/internal/ws/handler.go` | 登录后存储 userID |
| `gateway/internal/ws/session_subscriber.go` | 新增订阅器（可选，也可内联在 main.go） |
| `pkg/conf/conf.go` | 新增 session invalidate channel 配置（可选） |

---

## 12. 后续 TODO

- 推送系统设计（通用 Pub/Sub 框架）
- 踢人管理 API（运营后台）
- 多终端模式（可选支持）