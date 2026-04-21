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

### 4.1 并发竞争问题

两个设备同时登录同一账号可能导致单终端策略失效：

```
T1: A GET user:{userID}:session → 空
T2: B GET user:{userID}:session → 空（A还没写入）
T3-T6: A、B 各自写入 token → 都获得有效 session
```

**解决方案**：使用 **Lua 脚本原子化** 整个 Session 操作。

### 4.2 Lua 脚本定义

```lua
-- login_session.lua
-- KEYS[1] = user:{userID}:session       (userID → token 映射)
-- KEYS[2] = user:session:{newToken}     (token → session 数据)
-- ARGV[1] = TTL (秒)
-- ARGV[2] = {userID}:{username}         (session 数据)
-- ARGV[3] = newToken                    (新生成的 token)

local oldToken = redis.call('GET', KEYS[1])
if oldToken and oldToken ~= ARGV[3] then
    -- 删除旧 token 的 session 数据
    redis.call('DEL', 'user:session:' .. oldToken)
end

-- 设置新的双向映射
redis.call('SETEX', KEYS[1], ARGV[1], ARGV[3])     -- userID → token
redis.call('SETEX', KEYS[2], ARGV[1], ARGV[2])     -- token → session数据

return oldToken  -- 返回旧 token（如有），用于发布 Pub/Sub
```

### 4.3 登录流程

```
1. 验证用户名密码
2. 生成新 token（32字节随机）
3. 执行 Lua 脚本（原子操作）：
   - 获取旧 token
   - 删除旧 session 数据（如有）
   - 设置新 session 双向映射
   - 返回旧 token
4. 如 Lua 返回旧 token：
   - PUBLISH kd48:session:invalidate {"user_id": userID}
5. 返回新 token 给客户端
```

### 4.4 Go 调用示例

```go
const loginSessionLua = `
local oldToken = redis.call('GET', KEYS[1])
if oldToken and oldToken ~= ARGV[3] then
    redis.call('DEL', 'user:session:' .. oldToken)
end
redis.call('SETEX', KEYS[1], ARGV[1], ARGV[3])
redis.call('SETEX', KEYS[2], ARGV[1], ARGV[2])
return oldToken
`

func (s *userService) issueSessionAtomic(ctx context.Context, userID uint64, username string) (string, error) {
    token := generateToken()
    sessionKey := fmt.Sprintf("user:session:%s", token)
    userKey := fmt.Sprintf("user:%d:session", userID)
    sessionValue := fmt.Sprintf("%d:%s", userID, username)
    
    result, err := s.rdb.Eval(ctx, loginSessionLua, 
        []string{userKey, sessionKey},
        int64(s.tokenTTL.Seconds()), sessionValue, token).Result()
    
    if err != nil {
        return "", err
    }
    
    // 如果有旧 token，发布失效通知
    if oldToken, ok := result.(string); ok && oldToken != "" {
        notifyData := fmt.Sprintf(`{"user_id":%d}`, userID)
        s.rdb.Publish(ctx, "kd48:session:invalidate", notifyData)
    }
    
    return token, nil
}
```

### 4.5 原子性保证

Redis Lua 脚本特性：
- **单线程执行**：脚本执行期间不处理其他命令
- **原子性**：整个脚本要么全部执行，要么全部不执行
- **无竞争**：多个并发登录请求串行化执行

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
- [ ] **并发登录原子性测试**：两个请求同时登录，验证只有一个获得有效 session

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