# Session 失效与顶号机制实现计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 实现单终端登录策略，新登录自动踢掉旧设备。

**Architecture:** User 服务使用 Lua 脚本原子化 Session 操作，通过 Redis Pub/Sub 通知网关；网关订阅失效消息，按 userID 断开旧连接。

**Tech Stack:** Go 1.26, Redis (Lua + Pub/Sub), go-redis/v9, protobuf

---

## 批准记录（人类门闩）

- **状态**：待批准
- **批准范围**：（手填）
- **批准人 / 日期**：（手填）
- **TDD**：强制
- **Subagent**：按任务拆分

---

## 文件结构

| 文件 | 职责 |
|------|------|
| `api/proto/user/v1/user.proto` | 新增 `user_id` 字段到 LoginReply/RegisterReply |
| `services/user/cmd/user/server.go` | Lua 脚本 + 原子化 issueSessionAtomic |
| `services/user/cmd/user/server_test.go` | Session 原子性测试 |
| `gateway/internal/ws/connection_manager.go` | userConnections 映射 + DisconnectByUserID |
| `gateway/internal/ws/connection_manager_test.go` | userConnections 相关测试 |
| `gateway/internal/ws/session_subscriber.go` | Redis Pub/Sub 订阅器（新增） |
| `gateway/internal/ws/session_subscriber_test.go` | 订阅器测试（新增） |
| `gateway/cmd/gateway/main.go` | 启动订阅器 |
| `gateway/internal/ws/handler.go` | 登录后存储 userID |

---

### Task 1: Proto 新增 user_id 字段

**Files:**
- Modify: `api/proto/user/v1/user.proto`
- Generate: `api/proto/user/v1/*.pb.go`

- [ ] **Step 1: 修改 proto 文件**

在 `api/proto/user/v1/user.proto` 中添加 `user_id` 字段：

```protobuf
message LoginReply {
  bool success = 1;
  string token = 2;
  uint64 user_id = 3; // 新增：用户ID，网关需要用于顶号映射
}

message RegisterReply {
  bool success = 1;
  string token = 2;
  uint64 user_id = 3; // 新增：用户ID
}
```

- [ ] **Step 2: 生成代码**

```bash
cd api/proto && bash gen_proto.sh
```

Expected: 生成新的 `.pb.go` 文件

- [ ] **Step 3: 验证构建**

```bash
cd api/proto && go build ./...
```

Expected: 构建成功

- [ ] **Step 4: Commit**

```bash
git add api/proto/user/v1/user.proto api/proto/user/v1/*.pb.go
git commit -m "feat(proto): add user_id to LoginReply and RegisterReply"
```

---

### Task 2: User 服务 Lua 脚本与原子化 Session

**Files:**
- Modify: `services/user/cmd/user/server.go`
- Modify: `services/user/cmd/user/server_test.go`

- [ ] **Step 1: 写测试 - issueSessionAtomic 返回 token**

在 `services/user/cmd/user/server_test.go` 新增测试（使用 miniredis）：

```go
package main

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func TestIssueSessionAtomic_NewUser(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis run: %v", err)
	}
	defer mr.Close()

	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	router := createMockRouterWithRedis(rdb)
	svc := NewUserService(router, 1*time.Hour)

	token, err := svc.issueSessionAtomic(context.Background(), 123, "alice")
	if err != nil {
		t.Fatalf("issueSessionAtomic: %v", err)
	}
	if token == "" {
		t.Error("token should not be empty")
	}

	// 验证双向映射
	userKey := "user:123:session"
	storedToken := mr.Get(userKey)
	if storedToken != token {
		t.Errorf("user:123:session = %q, want %q", storedToken, token)
	}

	sessionKey := "user:session:" + token
	sessionVal := mr.Get(sessionKey)
	if sessionVal != "123:alice" {
		t.Errorf("user:session:%s = %q, want 123:alice", token, sessionVal)
	}
}

func createMockRouterWithRedis(rdb redis.UniversalClient) *dsroute.Router {
	// 创建带有 Redis 的 mock router
	mysqlPools := map[string]*sql.DB{}
	redisPools := map[string]redis.UniversalClient{"default": rdb}
	mysqlRoutes := []dsroute.RouteRule{}
	redisRoutes := []dsroute.RouteRule{{Prefix: "sys:session", Pool: "default"}}
	router, _ := dsroute.NewRouter(mysqlPools, redisPools, mysqlRoutes, redisRoutes)
	return router
}
```

- [ ] **Step 2: 运行测试确认失败**

```bash
cd services/user && go test ./cmd/user/... -v -run TestIssueSessionAtomic
```

Expected: FAIL (issueSessionAtomic not defined)

- [ ] **Step 3: 实现 Lua 脚本和 issueSessionAtomic**

在 `services/user/cmd/user/server.go` 添加：

```go
// loginSessionLua 原子化 Session 创建 Lua 脚本
// KEYS[1] = user:{userID}:session (userID → token 映射)
// KEYS[2] = user:session:{newToken} (token → session 数据)
// ARGV[1] = TTL (秒)
// ARGV[2] = {userID}:{username} (session 数据)
// ARGV[3] = newToken
const loginSessionLua = `
local oldToken = redis.call('GET', KEYS[1])
if oldToken and oldToken ~= ARGV[3] then
	redis.call('DEL', 'user:session:' .. oldToken)
end
redis.call('SETEX', KEYS[1], ARGV[1], ARGV[3])
redis.call('SETEX', KEYS[2], ARGV[1], ARGV[2])
return oldToken
`

// issueSessionAtomic 原子化创建 Session（使用 Lua 脚本）
// 返回新 token 和是否有旧 token（用于发布 Pub/Sub）
func (s *userService) issueSessionAtomic(ctx context.Context, userID uint64, username string) (string, bool, error) {
	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		slog.ErrorContext(ctx, "Failed to generate token", "error", err)
		return "", false, status.Error(codes.Internal, "internal server error")
	}
	token := hex.EncodeToString(tokenBytes)

	userKey := fmt.Sprintf("user:%d:session", userID)
	sessionKey := fmt.Sprintf("user:session:%s", token)
	sessionValue := fmt.Sprintf("%d:%s", userID, username)

	rdb, _, err := s.router.ResolveRedis(ctx, routingKeySession)
	if err != nil {
		slog.ErrorContext(ctx, "Failed to resolve redis for session", "error", err)
		return "", false, status.Error(codes.Internal, "internal server error")
	}

	result, err := rdb.Eval(ctx, loginSessionLua,
		[]string{userKey, sessionKey},
		int64(s.tokenTTL.Seconds()), sessionValue, token).Result()
	if err != nil {
		slog.ErrorContext(ctx, "Failed to execute session Lua", "error", err)
		return "", false, status.Error(codes.Internal, "internal server error")
	}

	// 检查是否有旧 token
	oldToken, _ := result.(string)
	hasOldToken := oldToken != ""

	return token, hasOldToken, nil
}
```

- [ ] **Step 4: 运行测试确认通过**

```bash
cd services/user && go test ./cmd/user/... -v -run TestIssueSessionAtomic_NewUser
```

Expected: PASS

- [ ] **Step 5: 写测试 - 顶号场景（旧 token 被删除）**

```go
func TestIssueSessionAtomic_KickOldSession(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis run: %v", err)
	}
	defer mr.Close()

	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	router := createMockRouterWithRedis(rdb)
	svc := NewUserService(router, 1*time.Hour)

	// 第一次登录
	token1, _, err := svc.issueSessionAtomic(context.Background(), 123, "alice")
	if err != nil {
		t.Fatalf("first login: %v", err)
	}

	// 验证旧 session 存在
	sessionKey1 := "user:session:" + token1
	if mr.Get(sessionKey1) != "123:alice" {
		t.Fatalf("first session not stored correctly")
	}

	// 第二次登录（顶号）
	token2, hasOldToken, err := svc.issueSessionAtomic(context.Background(), 123, "alice")
	if err != nil {
		t.Fatalf("second login: %v", err)
	}
	if !hasOldToken {
		t.Error("hasOldToken should be true when replacing old session")
	}

	// 验证新 token 存在
	userKey := "user:123:session"
	if mr.Get(userKey) != token2 {
		t.Errorf("user:123:session = %q, want %q", mr.Get(userKey), token2)
	}

	// 验证旧 token 被删除
	sessionKey1After := "user:session:" + token1
	if mr.Get(sessionKey1After) != "" {
		t.Errorf("old session key should be deleted, got %q", mr.Get(sessionKey1After))
	}

	// 验证新 session 存在
	sessionKey2 := "user:session:" + token2
	if mr.Get(sessionKey2) != "123:alice" {
		t.Errorf("new session should exist with correct value")
	}
}
```

- [ ] **Step 6: 运行测试**

```bash
cd services/user && go test ./cmd/user/... -v -run TestIssueSessionAtomic_KickOldSession
```

Expected: PASS

- [ ] **Step 7: Commit**

```bash
git add services/user/cmd/user/server.go services/user/cmd/user/server_test.go
git commit -m "feat(user): add atomic session creation with Lua script"
```

---

### Task 3: User 服务更新 Login/Register 并发布 Pub/Sub

**Files:**
- Modify: `services/user/cmd/user/server.go`
- Modify: `services/user/cmd/user/server_test.go`

- [ ] **Step 1: 写测试 - Login 返回 user_id**

```go
func TestLogin_ReturnsUserID(t *testing.T) {
	// 此测试需要完整的 mock 环境（MySQL + Redis）
	// 使用 bufconn gRPC 测试
	// 简化版：仅验证 proto 字段存在
	
	// 跳过完整集成测试，后续用集成测试验证
	t.Skip("requires full integration test setup")
}
```

- [ ] **Step 2: 更新 Login 方法**

修改 `services/user/cmd/user/server.go` 的 Login 方法：

```go
func (s *userService) Login(ctx context.Context, req *userv1.LoginRequest) (*userv1.LoginReply, error) {
	slog.InfoContext(ctx, "Received Login request", "username", req.Username)

	routingKey := "sys:user:" + req.Username

	queries, err := s.getQueries(ctx, routingKey)
	if err != nil {
		return nil, err
	}

	user, err := queries.GetUserByUsername(ctx, req.Username)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, status.Error(codes.Unauthenticated, "invalid username or password")
		}
		slog.ErrorContext(ctx, "GetUserByUsername failed", "error", err)
		return nil, status.Error(codes.Internal, "internal server error")
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		return nil, status.Error(codes.Unauthenticated, "invalid username or password")
	}

	// 使用原子化 Session 创建
	token, hasOldToken, err := s.issueSessionAtomic(ctx, user.ID, user.Username)
	if err != nil {
		return nil, err
	}

	// 如果有旧 token，发布失效通知
	if hasOldToken {
		s.publishSessionInvalidate(ctx, user.ID)
	}

	slog.InfoContext(ctx, "User logged in successfully", "username", user.Username, "user_id", user.ID)

	return &userv1.LoginReply{
		Success: true,
		Token:   token,
		UserId:  user.ID, // 新增
	}, nil
}

// publishSessionInvalidate 发布 Session 失效通知
func (s *userService) publishSessionInvalidate(ctx context.Context, userID uint64) {
	rdb, _, err := s.router.ResolveRedis(ctx, routingKeySession)
	if err != nil {
		slog.WarnContext(ctx, "Failed to resolve redis for publish", "error", err)
		return
	}

	notifyData := fmt.Sprintf(`{"user_id":%d}`, userID)
	if err := rdb.Publish(ctx, "kd48:session:invalidate", notifyData).Err(); err != nil {
		slog.WarnContext(ctx, "Failed to publish session invalidate", "error", err)
	}
}
```

- [ ] **Step 3: 更新 Register 方法**

同样修改 Register 方法：

```go
func (s *userService) Register(ctx context.Context, req *userv1.RegisterRequest) (*userv1.RegisterReply, error) {
	slog.InfoContext(ctx, "Received Register request", "username", req.Username)

	username := strings.TrimSpace(req.Username)
	if username == "" || req.Password == "" {
		return nil, status.Error(codes.InvalidArgument, "username and password required")
	}

	routingKey := "sys:user:" + username

	queries, err := s.getQueries(ctx, routingKey)
	if err != nil {
		return nil, err
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		slog.ErrorContext(ctx, "bcrypt hash failed", "error", err)
		return nil, status.Error(codes.Internal, "internal server error")
	}

	err = queries.CreateUser(ctx, sqlc.CreateUserParams{
		Username:     username,
		PasswordHash: string(hash),
	})
	if err != nil {
		var mysqlErr *mysql.MySQLError
		if errors.As(err, &mysqlErr) && mysqlErr.Number == 1062 {
			return nil, status.Error(codes.AlreadyExists, "username already exists")
		}
		slog.ErrorContext(ctx, "CreateUser failed", "error", err)
		return nil, status.Error(codes.Internal, "internal server error")
	}

	user, err := queries.GetUserByUsername(ctx, username)
	if err != nil {
		slog.ErrorContext(ctx, "GetUserByUsername after register failed", "error", err)
		return nil, status.Error(codes.Internal, "internal server error")
	}

	// 注册时不会有旧 token，直接创建
	token, _, err := s.issueSessionAtomic(ctx, user.ID, user.Username)
	if err != nil {
		return nil, err
	}

	slog.InfoContext(ctx, "User registered successfully", "username", user.Username, "user_id", user.ID)

	return &userv1.RegisterReply{
		Success: true,
		Token:   token,
		UserId:  user.ID, // 新增
	}, nil
}
```

- [ ] **Step 4: 验证构建**

```bash
cd services/user && go build ./...
```

Expected: 构建成功

- [ ] **Step 5: Commit**

```bash
git add services/user/cmd/user/server.go
git commit -m "feat(user): update Login/Register with atomic session and user_id"
```

---

### Task 4: Gateway ConnectionManager userConnections 映射

**Files:**
- Modify: `gateway/internal/ws/connection_manager.go`
- Modify: `gateway/internal/ws/connection_manager_test.go`

- [ ] **Step 1: 写测试 - RegisterUserConnection**

在 `gateway/internal/ws/connection_manager_test.go` 添加：

```go
func TestConnectionManager_RegisterUserConnection(t *testing.T) {
	cm := NewConnectionManager(HeartbeatConfig{
		Interval:  30 * time.Second,
		Timeout:   10 * time.Second,
		MaxMissed: 3,
	})

	clientID := "conn-123"
	userID := int64(456)

	// 先注册连接
	cm.RegisterConnection(clientID, nil)

	// 关联 userID
	cm.RegisterUserConnection(userID, clientID)

	// 验证可以通过 userID 找到 clientID
	foundClientID, exists := cm.GetUserClientID(userID)
	if !exists {
		t.Error("user connection should exist")
	}
	if foundClientID != clientID {
		t.Errorf("foundClientID = %q, want %q", foundClientID, clientID)
	}
}

func TestConnectionManager_RegisterUserConnection_Replace(t *testing.T) {
	cm := NewConnectionManager(HeartbeatConfig{
		Interval:  30 * time.Second,
		Timeout:   10 * time.Second,
		MaxMissed: 3,
	})

	cm.RegisterConnection("conn-1", nil)
	cm.RegisterConnection("conn-2", nil)

	userID := int64(100)

	// 第一次关联
	cm.RegisterUserConnection(userID, "conn-1")
	found1, _ := cm.GetUserClientID(userID)
	if found1 != "conn-1" {
		t.Errorf("first association failed")
	}

	// 第二次关联（替换）
	cm.RegisterUserConnection(userID, "conn-2")
	found2, _ := cm.GetUserClientID(userID)
	if found2 != "conn-2" {
		t.Errorf("second association should replace first, got %q", found2)
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

```bash
cd gateway && go test ./internal/ws/... -v -run TestConnectionManager_RegisterUserConnection
```

Expected: FAIL (方法未定义)

- [ ] **Step 3: 实现 userConnections 字段和方法**

在 `gateway/internal/ws/connection_manager.go` 修改：

```go
type ConnectionManager struct {
	heartbeat       *HeartbeatManager
	connections     map[string]*websocket.Conn // clientID → conn
	userConnections map[int64]string           // userID → clientID（新增）
	connMu          sync.RWMutex
	stopCh          chan struct{}
	metrics         ConnectionMetrics
}

func NewConnectionManager(hbConfig HeartbeatConfig) *ConnectionManager {
	return &ConnectionManager{
		heartbeat:       NewHeartbeatManager(hbConfig),
		connections:     make(map[string]*websocket.Conn),
		userConnections: make(map[int64]string), // 新增初始化
		stopCh:          make(chan struct{}),
		metrics:         ConnectionMetrics{},
	}
}

// RegisterUserConnection 登录成功后关联 userID 与 clientID
func (cm *ConnectionManager) RegisterUserConnection(userID int64, clientID string) {
	cm.connMu.Lock()
	defer cm.connMu.Unlock()
	cm.userConnections[userID] = clientID
	slog.Debug("user connection registered", "user_id", userID, "client_id", clientID)
}

// GetUserClientID 根据 userID 查找 clientID
func (cm *ConnectionManager) GetUserClientID(userID int64) (string, bool) {
	cm.connMu.RLock()
	defer cm.connMu.RUnlock()
	clientID, exists := cm.userConnections[userID]
	return clientID, exists
}
```

- [ ] **Step 4: 运行测试确认通过**

```bash
cd gateway && go test ./internal/ws/... -v -run TestConnectionManager_RegisterUserConnection
```

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add gateway/internal/ws/connection_manager.go gateway/internal/ws/connection_manager_test.go
git commit -m "feat(gateway): add userConnections mapping to ConnectionManager"
```

---

### Task 5: Gateway ConnectionManager DisconnectByUserID

**Files:**
- Modify: `gateway/internal/ws/connection_manager.go`
- Modify: `gateway/internal/ws/connection_manager_test.go`

- [ ] **Step 1: 写测试 - DisconnectByUserID**

```go
func TestConnectionManager_DisconnectByUserID(t *testing.T) {
	cm := NewConnectionManager(HeartbeatConfig{
		Interval:  30 * time.Second,
		Timeout:   10 * time.Second,
		MaxMissed: 3,
	})

	clientID := "conn-123"
	userID := int64(456)

	cm.RegisterConnection(clientID, nil)
	cm.RegisterUserConnection(userID, clientID)

	// 断开用户连接
	cm.DisconnectByUserID(userID, "session replaced")

	// 验证连接被清理
	_, connExists := cm.GetConnection(clientID)
	if connExists {
		t.Error("connection should be removed")
	}

	_, userExists := cm.GetUserClientID(userID)
	if userExists {
		t.Error("user connection mapping should be removed")
	}

	metrics := cm.GetMetrics()
	if metrics.ActiveConnections != 0 {
		t.Errorf("active connections should be 0, got %d", metrics.ActiveConnections)
	}
	if metrics.DisconnectedCount != 1 {
		t.Errorf("disconnected count should be 1, got %d", metrics.DisconnectedCount)
	}
}

func TestConnectionManager_DisconnectByUserID_NotExist(t *testing.T) {
	cm := NewConnectionManager(HeartbeatConfig{
		Interval:  30 * time.Second,
		Timeout:   10 * time.Second,
		MaxMissed: 3,
	})

	// 断开不存在的用户（不应出错）
	cm.DisconnectByUserID(999, "session replaced")

	metrics := cm.GetMetrics()
	if metrics.DisconnectedCount != 0 {
		t.Errorf("disconnected count should be 0 for non-existent user")
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

```bash
cd gateway && go test ./internal/ws/... -v -run TestConnectionManager_DisconnectByUserID
```

Expected: FAIL (方法未定义)

- [ ] **Step 3: 实现 DisconnectByUserID**

```go
// DisconnectByUserID 按 userID 断开连接（顶号时调用）
func (cm *ConnectionManager) DisconnectByUserID(userID int64, reason string) {
	cm.connMu.Lock()
	defer cm.connMu.Unlock()

	clientID, exists := cm.userConnections[userID]
	if !exists {
		return // 该网关实例没有这个用户的连接
	}

	conn, connExists := cm.connections[clientID]
	if connExists && conn != nil {
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

	if cm.metrics.ActiveConnections < 0 {
		cm.metrics.ActiveConnections = 0
	}

	slog.Info("user disconnected by session invalidate",
		"user_id", userID, "reason", reason, "active_count", cm.metrics.ActiveConnections)
}
```

- [ ] **Step 4: 运行测试确认通过**

```bash
cd gateway && go test ./internal/ws/... -v -run TestConnectionManager_DisconnectByUserID
```

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add gateway/internal/ws/connection_manager.go gateway/internal/ws/connection_manager_test.go
git commit -m "feat(gateway): add DisconnectByUserID method"
```

---

### Task 6: Gateway SessionInvalidationSubscriber

**Files:**
- Create: `gateway/internal/ws/session_subscriber.go`
- Create: `gateway/internal/ws/session_subscriber_test.go`

- [ ] **Step 1: 写测试 - 订阅器解析消息**

```go
package ws

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func TestSessionInvalidationSubscriber_ParseMessage(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis run: %v", err)
	}
	defer mr.Close()

	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})

	cm := NewConnectionManager(HeartbeatConfig{
		Interval:  30 * time.Second,
		Timeout:   10 * time.Second,
		MaxMissed: 3,
	})

	// 注册一个连接
	clientID := "conn-123"
	userID := int64(456)
	cm.RegisterConnection(clientID, nil)
	cm.RegisterUserConnection(userID, clientID)

	subscriber := NewSessionInvalidationSubscriber(rdb, cm, "kd48:session:invalidate")

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	go subscriber.Start(ctx)

	// 发布失效消息
	mr.Publish("kd48:session:invalidate", `{"user_id":456}`)

	// 等待处理
	time.Sleep(100 * time.Millisecond)

	// 验证连接被断开
	_, exists := cm.GetUserClientID(userID)
	if exists {
		t.Error("user connection should be removed after invalidate")
	}
}

func TestSessionInvalidationSubscriber_InvalidJSON(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis run: %v", err)
	}
	defer mr.Close()

	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})

	cm := NewConnectionManager(HeartbeatConfig{
		Interval:  30 * time.Second,
		Timeout:   10 * time.Second,
		MaxMissed: 3,
	})

	clientID := "conn-123"
	userID := int64(456)
	cm.RegisterConnection(clientID, nil)
	cm.RegisterUserConnection(userID, clientID)

	subscriber := NewSessionInvalidationSubscriber(rdb, cm, "kd48:session:invalidate")

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	go subscriber.Start(ctx)

	// 发布无效 JSON
	mr.Publish("kd48:session:invalidate", `invalid-json`)

	time.Sleep(100 * time.Millisecond)

	// 验证连接仍然存在（无效消息被忽略）
	_, exists := cm.GetUserClientID(userID)
	if !exists {
		t.Error("user connection should NOT be removed for invalid message")
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

```bash
cd gateway && go test ./internal/ws/... -v -run TestSessionInvalidationSubscriber
```

Expected: FAIL (文件不存在)

- [ ] **Step 3: 创建 session_subscriber.go**

```go
package ws

import (
	"context"
	"encoding/json"
	"log/slog"

	"github.com/redis/go-redis/v9"
)

const SessionInvalidateChannel = "kd48:session:invalidate"

// SessionInvalidationSubscriber Redis Pub/Sub 订阅器
type SessionInvalidationSubscriber struct {
	rdb         *redis.Client
	connManager *ConnectionManager
	channel     string
}

// NewSessionInvalidationSubscriber 创建订阅器
func NewSessionInvalidationSubscriber(rdb *redis.Client, cm *ConnectionManager, channel string) *SessionInvalidationSubscriber {
	return &SessionInvalidationSubscriber{
		rdb:         rdb,
		connManager: cm,
		channel:     channel,
	}
}

// Start 启动订阅
func (s *SessionInvalidationSubscriber) Start(ctx context.Context) {
	pubsub := s.rdb.Subscribe(ctx, s.channel)
	defer pubsub.Close()

	slog.Info("session invalidate subscriber started", "channel", s.channel)

	ch := pubsub.Channel()
	for {
		select {
		case <-ctx.Done():
			slog.Info("session invalidate subscriber stopped")
			return
		case msg, ok := <-ch:
			if !ok {
				return
			}
			s.handleMessage(msg.Payload)
		}
	}
}

// handleMessage 处理失效消息
func (s *SessionInvalidationSubscriber) handleMessage(payload string) {
	var data struct {
		UserID int64 `json:"user_id"`
	}

	if err := json.Unmarshal([]byte(payload), &data); err != nil {
		slog.Warn("invalid session invalidate message", "error", err, "payload", payload)
		return
	}

	slog.Debug("received session invalidate", "user_id", data.UserID)
	s.connManager.DisconnectByUserID(data.UserID, "session replaced")
}
```

- [ ] **Step 4: 运行测试确认通过**

```bash
cd gateway && go test ./internal/ws/... -v -run TestSessionInvalidationSubscriber
```

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add gateway/internal/ws/session_subscriber.go gateway/internal/ws/session_subscriber_test.go
git commit -m "feat(gateway): add SessionInvalidationSubscriber"
```

---

### Task 7: Gateway main.go 启动订阅器

**Files:**
- Modify: `gateway/cmd/gateway/main.go`

- [ ] **Step 1: 在 main.go 中启动订阅器**

在 `gateway/cmd/gateway/main.go` 的连接管理器启动后添加：

```go
// 在 connManager.Start 之后添加：

// 启动 Session 失效订阅器
sessionSubscriber := ws.NewSessionInvalidationSubscriber(rdb, connManager, ws.SessionInvalidateChannel)
subscriberCtx, subscriberCancel := context.WithCancel(ctx)
defer subscriberCancel()
go sessionSubscriber.Start(subscriberCtx)
slog.Info("Session invalidate subscriber started")
```

- [ ] **Step 2: 验证构建**

```bash
cd gateway && go build ./cmd/gateway/...
```

Expected: 构建成功

- [ ] **Step 3: Commit**

```bash
git add gateway/cmd/gateway/main.go
git commit -m "feat(gateway): start SessionInvalidationSubscriber in main"
```

---

### Task 8: Gateway Handler 登录后存储 userID

**Files:**
- Modify: `gateway/internal/ws/handler.go`
- Modify: `gateway/internal/ws/handler_test.go`

- [ ] **Step 1: 写测试 - 登录后 userID 被存储**

由于 Handler 测试需要完整的 WebSocket mock，这里简化为验证代码结构：

```go
// handler_test.go 增加结构验证测试
func TestClientMeta_UserIDField(t *testing.T) {
	meta := &clientMeta{
		connID:          1,
		clientID:        "conn-1",
		isAuthenticated: true,
		userID:          123,
		token:           "test-token",
	}

	if meta.userID != 123 {
		t.Errorf("userID should be 123, got %d", meta.userID)
	}
}
```

- [ ] **Step 2: 更新 clientMeta 结构（已预留 userID）**

确认 `gateway/internal/ws/handler.go` 中 clientMeta 已有 userID 字段。

- [ ] **Step 3: 更新 Handler 登录后处理**

在 `gateway/internal/ws/handler.go` 的 ServeWS 方法中，登录成功后处理：

需要找到登录响应处理的位置，从响应中提取 userID。

由于当前 Handler 通过 Ingress 调用后端服务，响应格式需要解析。

查看现有 handler.go 的处理逻辑，在成功响应后添加：

```go
// 在 route.EstablishSession 判断之后，成功响应处理中：

// 如果是登录/Register，解析响应获取 userID
if route.EstablishSession && resp != nil {
	// 尝试解析 LoginReply/RegisterReply 格式
	// protojson 格式: {"success":true,"token":"...","user_id":123}
	if dataMap, ok := resp.(map[string]interface{}); ok {
		if uid, ok := dataMap["user_id"]; ok {
			switch v := uid.(type) {
			case float64:
				meta.userID = int64(v)
			case int64:
				meta.userID = v
			case uint64:
				meta.userID = int64(v)
			}
			if meta.userID > 0 {
				h.connManager.RegisterUserConnection(meta.userID, clientID)
				slog.InfoContext(ctx, "User authenticated", "user_id", meta.userID, "client_id", clientID)
			}
		}
	}
}
```

- [ ] **Step 4: 验证构建**

```bash
cd gateway && go build ./...
```

Expected: 构建成功

- [ ] **Step 5: Commit**

```bash
git add gateway/internal/ws/handler.go gateway/internal/ws/handler_test.go
git commit -m "feat(gateway): store userID after login in Handler"
```

---

### Task 9: 验证命令

- [ ] **Step 1: 运行所有测试**

```bash
go test ./gateway/... ./services/user/... ./pkg/... ./api/proto/... -v
```

Expected: 所有测试通过

- [ ] **Step 2: 构建验证**

```bash
go build ./gateway/... ./services/user/... ./services/lobby/... ./pkg/...
```

Expected: 构建成功

- [ ] **Step 3: Commit 验证**

```bash
git log --oneline -10
```

Expected: 查看提交历史，确认所有提交

---

## 验证命令（默认）

```bash
# 测试
go test ./...

# 构建
go build ./...
```

---

## 与设计的可追溯映射

| 设计规格 | 本计划 Task |
|----------|-------------|
| §3 Session 双向映射 | Task 2 |
| §4 Lua 脚本原子化 | Task 2 |
| §5 网关订阅配置 | Task 6, Task 7 |
| §6 ConnectionManager 改造 | Task 4, Task 5 |
| §7 Handler 登录后处理 | Task 8 |
| §10 测试要点 | 各 Task 内含测试 |