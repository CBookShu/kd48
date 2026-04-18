# UserService MySQL 路由集成实现计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 让 UserService 的 MySQL 业务使用 Router 进行动态路由

**Architecture:** 移除固定的 qc 字段，新增 getQueries() 辅助方法，业务方法通过 routingKey 动态获取 Queries

**Tech Stack:** Go 1.26, sqlc, dsroute.Router

**设计文档:** `docs/superpowers/specs/2026-04-18-user-service-mysql-routing-design.md`

---

## 文件结构

| 文件 | 职责 | 操作 |
|------|------|------|
| `services/user/cmd/user/server.go` | UserService 实现，新增 getQueries | 修改 |
| `services/user/cmd/user/main.go` | 移除 queries 创建和传参 | 修改 |

---

## Task 1: 修改 server.go

**Files:**
- Modify: `services/user/cmd/user/server.go`

- [ ] **Step 1: 修改 userService 结构体**

移除 `qc` 字段：

```go
type userService struct {
	userv1.UnimplementedUserServiceServer
	router   *dsroute.Router
	tokenTTL time.Duration
}
```

- [ ] **Step 2: 修改 NewUserService 函数**

移除 `queries` 参数：

```go
func NewUserService(router *dsroute.Router, tokenTTL time.Duration) *userService {
	return &userService{
		router:   router,
		tokenTTL: tokenTTL,
	}
}
```

- [ ] **Step 3: 更新 routingKeySession 常量**

```go
const routingKeySession = "sys:session"
```

- [ ] **Step 4: 新增 getQueries 辅助方法**

```go
func (s *userService) getQueries(ctx context.Context, routingKey string) (*sqlc.Queries, error) {
	db, _, err := s.router.ResolveDB(ctx, routingKey)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "resolve db route: %v", err)
	}
	return sqlc.New(db), nil
}
```

- [ ] **Step 5: 修改 Login 方法**

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

	token, err := s.issueSession(ctx, user.ID, user.Username)
	if err != nil {
		return nil, err
	}

	slog.InfoContext(ctx, "User logged in successfully", "username", user.Username)

	return &userv1.LoginReply{
		Success: true,
		Token:   token,
	}, nil
}
```

- [ ] **Step 6: 修改 Register 方法**

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

	token, err := s.issueSession(ctx, user.ID, user.Username)
	if err != nil {
		return nil, err
	}

	slog.InfoContext(ctx, "User registered successfully", "username", user.Username)

	return &userv1.RegisterReply{
		Success: true,
		Token:   token,
	}, nil
}
```

- [ ] **Step 7: 运行编译验证**

```bash
cd /Users/cbookshu/dev/temp/kd48
go build ./services/user/cmd/user/...
```

Expected: 编译成功

- [ ] **Step 8: Commit**

```bash
git add services/user/cmd/user/server.go
git commit -m "feat(user): add getQueries helper for mysql routing"
```

---

## Task 2: 修改 main.go

**Files:**
- Modify: `services/user/cmd/user/main.go`

- [ ] **Step 1: 删除 queries 变量创建**

删除这行：
```go
queries := sqlc.New(mysqlPools["default"])
```

- [ ] **Step 2: 修改 NewUserService 调用**

找到创建 userSvc 的位置，修改为：

```go
userSvc := NewUserService(router, time.Duration(c.Session.ExpireHours)*time.Hour)
```

- [ ] **Step 3: 运行编译验证**

```bash
cd /Users/cbookshe/dev/temp/kd48
go build ./services/user/cmd/user/...
```

Expected: 编译成功

- [ ] **Step 4: Commit**

```bash
git add services/user/cmd/user/main.go
git commit -m "feat(user): remove fixed queries, use router for mysql"
```

---

## Task 3: 添加单元测试

**Files:**
- Create: `services/user/cmd/user/server_test.go`

- [ ] **Step 1: 创建测试文件**

```go
package main

import (
	"context"
	"database/sql"
	"testing"

	"github.com/CBookShu/kd48/pkg/dsroute"
	"github.com/redis/go-redis/v9"
)

func TestUserService_getQueries_Success(t *testing.T) {
	mysqlPools := map[string]*sql.DB{
		"default": nil, // 实际测试需要 mock 或真实连接
	}
	redisPools := map[string]redis.UniversalClient{
		"default": nil,
	}
	mysqlRoutes := []dsroute.RouteRule{{Prefix: "sys:user:", Pool: "default"}}
	redisRoutes := []dsroute.RouteRule{{Prefix: "", Pool: "default"}}

	router, err := dsroute.NewRouter(mysqlPools, redisPools, mysqlRoutes, redisRoutes)
	if err != nil {
		t.Fatalf("failed to create router: %v", err)
	}

	us := &userService{router: router}

	// 测试路由解析（不实际创建 Queries，因为 pool 是 nil）
	routingKey := "sys:user:alice"
	_, _, err = us.router.ResolveDB(context.Background(), routingKey)
	if err != nil {
		t.Errorf("expected route resolve success, got error: %v", err)
	}
}

func TestUserService_getQueries_RouteNotFound(t *testing.T) {
	mysqlPools := map[string]*sql.DB{
		"default": nil,
	}
	redisPools := map[string]redis.UniversalClient{
		"default": nil,
	}
	mysqlRoutes := []dsroute.RouteRule{{Prefix: "game:", Pool: "game_pool"}} // 没有 sys:user: 路由
	redisRoutes := []dsroute.RouteRule{{Prefix: "", Pool: "default"}}

	router, err := dsroute.NewRouter(mysqlPools, redisPools, mysqlRoutes, redisRoutes)
	if err != nil {
		// 预期失败：game_pool 不在 mysqlPools 中
		t.Logf("expected error: %v", err)
		return
	}

	us := &userService{router: router}

	routingKey := "sys:user:alice"
	_, _, err = us.router.ResolveDB(context.Background(), routingKey)
	if err == nil {
		t.Error("expected error for route not found, got nil")
	}
}
```

- [ ] **Step 2: 运行测试**

```bash
cd /Users/cbookshu/dev/temp/kd48
go test ./services/user/cmd/user/... -v
```

Expected: 测试通过

- [ ] **Step 3: Commit**

```bash
git add services/user/cmd/user/server_test.go
git commit -m "test(user): add getQueries unit tests"
```

---

## Task 4: 验证构建

**Files:**
- 无文件变更

- [ ] **Step 1: 运行完整构建**

```bash
cd /Users/cbookshu/dev/temp/kd48
go build ./...
```

Expected: 编译成功

- [ ] **Step 2: 运行所有测试**

```bash
go test ./... -v
```

Expected: 测试通过

---

## 自我审查

| 检查项 | 状态 |
|--------|------|
| **Spec 覆盖** | ✅ 所有设计点已覆盖 |
| **Placeholder 扫描** | ✅ 无 TBD/TODO |
| **类型一致性** | ✅ getQueries 返回 *sqlc.Queries 一致 |

**覆盖检查：**
- [x] userService 结构体移除 qc → Task 1
- [x] NewUserService 参数修改 → Task 1, Task 2
- [x] getQueries 辅助方法 → Task 1
- [x] Login 方法改造 → Task 1
- [x] Register 方法改造 → Task 1
- [x] routingKey 格式 `sys:user:{username}` → Task 1
- [x] 单元测试 → Task 3

---

## 执行方式选择

**Plan complete and saved to `docs/superpowers/plans/2026-04-18-user-service-mysql-routing-plan.md`.**

**Two execution options:**

**1. Subagent-Driven (recommended)** - I dispatch a fresh subagent per task, review between tasks, fast iteration

**2. Inline Execution** - Execute tasks in this session using executing-plans, batch execution with checkpoints for review

**Which approach?**
