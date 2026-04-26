# UserID 字段统一实现计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 统一使用 `pkg/contextkey` 包管理 `user_id` context key，确保类型安全。Proto 和大部分代码已完成 uint32 类型修改，核心任务是让所有服务使用统一的 contextkey 包。

**Architecture:** 创建全局 `pkg/contextkey` 包定义类型安全的 context key。Go 的 `context.Value` 是类型敏感的，`ctx.Value("user_id")` 和 `ctx.Value(contextkey.UserIDKey)` 不兼容，所有代码必须统一使用 `contextkey.GetUserID/WithUserID`。

**Tech Stack:** Go, Protobuf, protojson, MySQL, sqlc, Redis

---

## 当前状态分析

### ✅ 已完成的修改（无需修改）

| 文件 | 状态 |
|------|------|
| `api/proto/user/v1/user.proto` | ✅ `uint32 user_id` |
| `gateway/internal/ws/response_data.go` | ✅ `UseProtoNames: true` |
| `gateway/internal/ws/handler.go` | ✅ `clientMeta.userID uint32` |
| `gateway/internal/ws/connection_manager.go` | ✅ `map[uint32]string` |
| `gateway/internal/ws/session_subscriber.go` | ✅ `UserID uint32` |
| `gateway/internal/ws/wrapper.go` | ✅ 正确注入 baggage |
| `cmd/cli/internal/commands/handler.go` | ✅ `json:"user_id"` + `uint32` |
| `services/user/cmd/user/ingress.go` | ✅ `protojsonMarshaler` + `uint32` |
| `services/lobby/cmd/lobby/*_test.go` | ✅ 已使用 `uint32(12345)` |

### 🔴 需要修改的核心任务

**问题：所有代码仍使用字符串 `"user_id"` 或局部定义的 `userIDKey` 作为 context key，需要统一改用 `pkg/contextkey` 包。**

| 文件 | 当前问题 |
|------|---------|
| `services/user/cmd/user/ingress.go` | `context.WithValue(ctx, "user_id", ...)` 和 `ctx.Value("user_id")` |
| `services/user/cmd/user/server.go` | `ctx.Value("user_id")` |
| `services/lobby/cmd/lobby/ingress.go` | 局部定义 `const userIDKey contextKey = "user_id"` |
| `services/lobby/cmd/lobby/checkin_server.go` | `ctx.Value(userIDKey).(uint32)` |
| `services/lobby/cmd/lobby/item_server.go` | `ctx.Value(userIDKey).(uint32)` |
| `services/lobby/cmd/lobby/*_test.go` | `context.WithValue(ctx, "user_id", ...)` |
| `services/user/cmd/user/ingress_test.go` | 需要更新断言 |

---

## File Structure

| 文件 | 操作 | 说明 |
|------|------|------|
| `pkg/contextkey/contextkey.go` | 新建 | 全局 context key 定义 |
| `services/user/cmd/user/ingress.go` | 修改 | 使用 contextkey 包 |
| `services/user/cmd/user/server.go` | 修改 | 使用 contextkey 包 |
| `services/user/cmd/user/ingress_test.go` | 修改 | 断言更新 |
| `services/lobby/cmd/lobby/ingress.go` | 修改 | 删除局部 userIDKey，使用 contextkey |
| `services/lobby/cmd/lobby/checkin_server.go` | 修改 | 使用 contextkey.GetUserID |
| `services/lobby/cmd/lobby/item_server.go` | 修改 | 使用 contextkey.GetUserID |
| `services/lobby/cmd/lobby/checkin_server_test.go` | 修改 | 使用 contextkey.WithUserID |
| `services/lobby/cmd/lobby/ingress_test.go` | 修改 | 使用 contextkey + Baggage |
| `services/lobby/cmd/lobby/item_server_test.go` | 修改 | 使用 contextkey.WithUserID |
| `services/user/migrations/000002_change_user_id_to_int.up.sql` | 新建 | 数据库迁移 |
| `services/user/migrations/000002_change_user_id_to_int.down.sql` | 新建 | 回滚脚本 |
| `services/lobby/migrations/002_change_user_id_to_int.up.sql` | 新建 | 数据库迁移 |
| `services/lobby/migrations/002_change_user_id_to_int.down.sql` | 新建 | 回滚脚本 |

---

### Task 1: 创建全局 Context Key 包

**Files:**
- Create: `pkg/contextkey/contextkey.go`

**说明:** 创建全局 context key 包，确保所有服务使用类型安全的 context key。Go 的 `context.Value` 是类型敏感的，`ctx.Value("user_id")` 和 `ctx.Value(contextkey.UserIDKey)` 不兼容。

- [ ] **Step 1: 创建 pkg/contextkey/contextkey.go**

```go
// Package contextkey 定义全局 context.Value 键，避免跨服务键冲突。
package contextkey

import "context"

// contextKey 是 context.Value 键的类型。
type contextKey string

const (
	// UserIDKey 是用户 ID 在 context 中的键。
	// 所有服务必须使用此键存取 user_id，确保类型安全。
	UserIDKey contextKey = "user_id"
)

// GetUserID 从 context 中获取用户 ID。
// 返回 user_id 和是否存在的标志。
func GetUserID(ctx context.Context) (uint32, bool) {
	v := ctx.Value(UserIDKey)
	if v == nil {
		return 0, false
	}
	userID, ok := v.(uint32)
	return userID, ok
}

// WithUserID 返回一个包含用户 ID 的新 context。
func WithUserID(ctx context.Context, userID uint32) context.Context {
	return context.WithValue(ctx, UserIDKey, userID)
}
```

- [ ] **Step 2: 验证编译通过**

Run: `go build ./pkg/contextkey/...`

Expected: 编译成功

- [ ] **Step 3: 提交**

```bash
git add pkg/contextkey/contextkey.go
git commit -m "feat(contextkey): add global context key package for user_id

- Type-safe context keys across all services
- GetUserID/WithUserID helper functions
- Avoids string key collisions"
```

---

### Task 2: 更新 User Service ingress.go 使用 contextkey

**Files:**
- Modify: `services/user/cmd/user/ingress.go`

**当前代码问题:**
- Line 82: `userIDVal := ctx.Value("user_id")` - VerifyToken 路由检查
- Line 127: `context.WithValue(ctx, "user_id", userID)` - 设置 context

- [ ] **Step 1: 添加 import**

```go
import (
	"context"
	"fmt"

	gatewayv1 "github.com/CBookShu/kd48/api/proto/gateway/v1"
	userv1 "github.com/CBookShu/kd48/api/proto/user/v1"
	"github.com/CBookShu/kd48/pkg/contextkey"
	"github.com/redis/go-redis/v9"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"
)
```

- [ ] **Step 2: 修改 VerifyToken 路由检查 (Line 82-94)**

```go
// 修改前
userIDVal := ctx.Value("user_id")
if userIDVal != nil {
	// User already authenticated via gateway session
	out, err := s.inner.VerifyToken(ctx, &userv1.VerifyTokenRequest{})
	// ...
}

// 修改后
_, ok := contextkey.GetUserID(ctx)
if ok {
	// User already authenticated via gateway session
	out, err := s.inner.VerifyToken(ctx, &userv1.VerifyTokenRequest{})
	// ...
}
```

- [ ] **Step 3: 修改设置 context (Line 127)**

```go
// 修改前
ctxWithUser := context.WithValue(ctx, "user_id", userID)

// 修改后
ctxWithUser := contextkey.WithUserID(ctx, userID)
```

- [ ] **Step 4: 验证编译通过**

Run: `cd services/user && go build ./...`

Expected: 编译成功

- [ ] **Step 5: 提交**

```bash
git add services/user/cmd/user/ingress.go
git commit -m "refactor(user): use contextkey package for user_id context"
```

---

### Task 3: 更新 User Service server.go 使用 contextkey

**Files:**
- Modify: `services/user/cmd/user/server.go`

**当前代码问题:**
- Line 232-242: 手动类型断言 `ctx.Value("user_id").(uint32)`

- [ ] **Step 1: 添加 import**

```go
import "github.com/CBookShu/kd48/pkg/contextkey"
```

- [ ] **Step 2: 修改 VerifyToken 方法中的 context 读取 (Line 232-242)**

```go
// 修改前
userIDVal := ctx.Value("user_id")
if userIDVal == nil {
	slog.ErrorContext(ctx, "No user_id in context")
	return nil, status.Errorf(codes.Code(commonv1.ErrorCode_USER_NOT_AUTHENTICATED), "用户未认证")
}

userID, ok := userIDVal.(uint32)
if !ok {
	slog.ErrorContext(ctx, "Invalid user_id type in context")
	return nil, status.Errorf(codes.Code(commonv1.ErrorCode_USER_NOT_AUTHENTICATED), "用户未认证")
}

// 修改后
userID, ok := contextkey.GetUserID(ctx)
if !ok {
	slog.ErrorContext(ctx, "No user_id in context")
	return nil, status.Errorf(codes.Code(commonv1.ErrorCode_USER_NOT_AUTHENTICATED), "用户未认证")
}
```

- [ ] **Step 3: 验证编译通过**

Run: `cd services/user && go build ./...`

Expected: 编译成功

- [ ] **Step 4: 提交**

```bash
git add services/user/cmd/user/server.go
git commit -m "refactor(user): use contextkey.GetUserID in server"
```

---

### Task 4: 更新 User Service ingress_test.go 断言

**Files:**
- Modify: `services/user/cmd/user/ingress_test.go`

- [ ] **Step 1: 检查并更新断言**

如果测试中使用 `"userId"` 字符串断言，改为 `"user_id"` 和数字类型：

```go
// 修改前
assert.Equal(t, "123", got["userId"])

// 修改后
assert.Equal(t, float64(123), got["user_id"])  // JSON 数字解析为 float64
```

- [ ] **Step 2: 运行测试**

Run: `cd services/user && go test ./... -v`

Expected: PASS

- [ ] **Step 3: 提交**

```bash
git add services/user/cmd/user/ingress_test.go
git commit -m "test(user): update assertions for snake_case user_id"
```

---

### Task 5: 更新 Lobby Service ingress.go 使用 contextkey

**Files:**
- Modify: `services/lobby/cmd/lobby/ingress.go`

**当前代码问题:**
- Line 15-17: 局部定义 `type contextKey string` 和 `const userIDKey contextKey = "user_id"`
- Line 54: `context.WithValue(ctx, userIDKey, uint32(userID))`

- [ ] **Step 1: 删除局部定义，添加 import**

```go
package main

import (
	"context"
	"strconv"

	gatewayv1 "github.com/CBookShu/kd48/api/proto/gateway/v1"
	lobbyv1 "github.com/CBookShu/kd48/api/proto/lobby/v1"
	"github.com/CBookShu/kd48/pkg/contextkey"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"
)

// 删除以下两行：
// type contextKey string
// const userIDKey contextKey = "user_id"
```

- [ ] **Step 2: 修改设置 context (Line 54)**

```go
// 修改前
if userID, err := strconv.ParseUint(userIDStr, 10, 32); err == nil {
	ctx = context.WithValue(ctx, userIDKey, uint32(userID))
}

// 修改后
if userID, err := strconv.ParseUint(userIDStr, 10, 32); err == nil {
	ctx = contextkey.WithUserID(ctx, uint32(userID))
}
```

- [ ] **Step 3: 验证编译通过**

Run: `cd services/lobby && go build ./...`

Expected: 编译成功

- [ ] **Step 4: 提交**

```bash
git add services/lobby/cmd/lobby/ingress.go
git commit -m "refactor(lobby): use pkg/contextkey for user_id context key"
```

---

### Task 6: 更新 Lobby Service checkin_server.go 使用 contextkey

**Files:**
- Modify: `services/lobby/cmd/lobby/checkin_server.go`

**当前代码问题:**
- Line 59: `userID, ok := ctx.Value(userIDKey).(uint32)`
- Line 131: `userID, ok := ctx.Value(userIDKey).(uint32)`

- [ ] **Step 1: 添加 import**

```go
import "github.com/CBookShu/kd48/pkg/contextkey"
```

- [ ] **Step 2: 修改 Checkin 方法 (Line 59)**

```go
// 修改前
userID, ok := ctx.Value(userIDKey).(uint32)
if !ok {
	return nil, status.Errorf(codes.Code(commonv1.ErrorCode_USER_NOT_AUTHENTICATED), "用户未认证")
}

// 修改后
userID, ok := contextkey.GetUserID(ctx)
if !ok {
	return nil, status.Errorf(codes.Code(commonv1.ErrorCode_USER_NOT_AUTHENTICATED), "用户未认证")
}
```

- [ ] **Step 3: 修改 GetStatus 方法 (Line 131)**

```go
// 修改前
userID, ok := ctx.Value(userIDKey).(uint32)
if !ok {
	return nil, status.Errorf(codes.Code(commonv1.ErrorCode_USER_NOT_AUTHENTICATED), "用户未认证")
}

// 修改后
userID, ok := contextkey.GetUserID(ctx)
if !ok {
	return nil, status.Errorf(codes.Code(commonv1.ErrorCode_USER_NOT_AUTHENTICATED), "用户未认证")
}
```

- [ ] **Step 4: 验证编译通过**

Run: `cd services/lobby && go build ./...`

Expected: 编译成功

- [ ] **Step 5: 提交**

```bash
git add services/lobby/cmd/lobby/checkin_server.go
git commit -m "refactor(lobby): use contextkey.GetUserID in checkin_server"
```

---

### Task 7: 更新 Lobby Service item_server.go 使用 contextkey

**Files:**
- Modify: `services/lobby/cmd/lobby/item_server.go`

**当前代码问题:**
- Line 31: `userID, ok := ctx.Value(userIDKey).(uint32)`

- [ ] **Step 1: 添加 import**

```go
import "github.com/CBookShu/kd48/pkg/contextkey"
```

- [ ] **Step 2: 修改 GetMyItems 方法 (Line 31)**

```go
// 修改前
userID, ok := ctx.Value(userIDKey).(uint32)
if !ok {
	return nil, status.Errorf(codes.Code(commonv1.ErrorCode_USER_NOT_AUTHENTICATED), "用户未认证")
}

// 修改后
userID, ok := contextkey.GetUserID(ctx)
if !ok {
	return nil, status.Errorf(codes.Code(commonv1.ErrorCode_USER_NOT_AUTHENTICATED), "用户未认证")
}
```

- [ ] **Step 3: 验证编译通过**

Run: `cd services/lobby && go build ./...`

Expected: 编译成功

- [ ] **Step 4: 提交**

```bash
git add services/lobby/cmd/lobby/item_server.go
git commit -m "refactor(lobby): use contextkey.GetUserID in item_server"
```

---

### Task 8: 更新 Lobby Service 测试文件使用 contextkey

**Files:**
- Modify: `services/lobby/cmd/lobby/checkin_server_test.go`
- Modify: `services/lobby/cmd/lobby/ingress_test.go`
- Modify: `services/lobby/cmd/lobby/item_server_test.go`

**说明:** 测试必须使用 `contextkey.WithUserID` 而非 `context.WithValue(ctx, "user_id", ...)`，因为 Go 的 context.Value 是类型敏感的。

- [ ] **Step 1: 更新 checkin_server_test.go**

添加 import：

```go
import "github.com/CBookShu/kd48/pkg/contextkey"
```

将所有 `context.WithValue(context.Background(), "user_id", uint32(12345))` 改为：

```go
ctx := contextkey.WithUserID(context.Background(), uint32(12345))
```

涉及测试函数：`TestCheckinService_GetStatus_Success`, `TestCheckinService_Checkin_Success` 等

- [ ] **Step 2: 更新 item_server_test.go**

添加 import：

```go
import "github.com/CBookShu/kd48/pkg/contextkey"
```

将所有 `context.WithValue(context.Background(), "user_id", uint32(12345))` 改为：

```go
ctx := contextkey.WithUserID(context.Background(), uint32(12345))
```

涉及测试函数：`TestItemService_GetMyItems_Success` 等

- [ ] **Step 3: 更新 ingress_test.go**

Mock 服务中提取 user_id 的方式改为：

```go
// 修改前
if userID, ok := ctx.Value("user_id").(uint32); ok {

// 修改后
if userID, ok := contextkey.GetUserID(ctx); ok {
```

测试用例通过 `Baggage` 注入 user_id（ingress 会自动提取）：

```go
// 修改前（直接设置 context）
ctx := context.WithValue(context.Background(), "user_id", uint32(12345))
req := &gatewayv1.IngressRequest{Route: "...", JsonPayload: []byte(`{}`)}
ingress.Call(ctx, req)

// 修改后（通过 baggage 注入，ingress 会提取并设置 context）
req := &gatewayv1.IngressRequest{
	Route:       "...",
	JsonPayload: []byte(`{}`),
	Baggage:     map[string]string{"user_id": "12345"},
}
ingress.Call(context.Background(), req)
```

- [ ] **Step 4: 运行测试确认通过**

Run: `cd services/lobby && go test ./... -v`

Expected: PASS

- [ ] **Step 5: 提交测试更新**

```bash
git add services/lobby/cmd/lobby/*_test.go
git commit -m "test(lobby): use contextkey.WithUserID/GetUserID in tests"
```

---

### Task 9: 创建 User Service 数据库迁移脚本

**Files:**
- Create: `services/user/migrations/000002_change_user_id_to_int.up.sql`
- Create: `services/user/migrations/000002_change_user_id_to_int.down.sql`

- [ ] **Step 1: 创建 up 迁移脚本**

```sql
-- +migrate Up
ALTER TABLE `users` MODIFY `id` INT UNSIGNED NOT NULL AUTO_INCREMENT;
```

- [ ] **Step 2: 创建 down 迁移脚本**

```sql
-- +migrate Down
ALTER TABLE `users` MODIFY `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT;
```

- [ ] **Step 3: 提交迁移脚本**

```bash
git add services/user/migrations/000002_change_user_id_to_int.up.sql services/user/migrations/000002_change_user_id_to_int.down.sql
git commit -m "feat(user): add migration for user_id INT UNSIGNED"
```

---

### Task 10: 创建 Lobby Service 数据库迁移脚本

**Files:**
- Create: `services/lobby/migrations/002_change_user_id_to_int.up.sql`
- Create: `services/lobby/migrations/002_change_user_id_to_int.down.sql`

- [ ] **Step 1: 创建 up 迁移脚本**

```sql
-- +migrate Up
ALTER TABLE `user_items` MODIFY `user_id` INT UNSIGNED NOT NULL;
ALTER TABLE `user_checkin` MODIFY `user_id` INT UNSIGNED NOT NULL;
```

- [ ] **Step 2: 创建 down 迁移脚本**

```sql
-- +migrate Down
ALTER TABLE `user_items` MODIFY `user_id` BIGINT NOT NULL;
ALTER TABLE `user_checkin` MODIFY `user_id` BIGINT NOT NULL;
```

- [ ] **Step 3: 提交迁移脚本**

```bash
git add services/lobby/migrations/002_change_user_id_to_int.up.sql services/lobby/migrations/002_change_user_id_to_int.down.sql
git commit -m "feat(lobby): add migration for user_id INT UNSIGNED"
```

---

### Task 11: 全面编译和测试验证

**Files:**
- None (verification task)

- [ ] **Step 1: 编译所有服务**

Run: `go build ./...`

Expected: 编译成功

- [ ] **Step 2: 运行所有单元测试**

Run: `go test ./...`

Expected: 所有测试通过

- [ ] **Step 3: 验证 context key 类型安全**

确认所有使用 `user_id` context 的地方都通过 `contextkey.GetUserID/WithUserID` 访问。

---

### Task 12: 集成测试

**Files:**
- None (integration test)

- [ ] **Step 1: CLI 集成测试**

1. 使用 CLI 注册新用户
2. 验证登录成功
3. 执行签到操作
4. 验证顶号踢人功能

---

### Task 13: 最终提交

**Files:**
- None (final commit task)

- [ ] **Step 1: 确认所有修改已提交**

```bash
git status
git log --oneline -15
```

- [ ] **Step 2: 推送到远程仓库**

```bash
git push origin <branch-name>
```

---

## 验证清单

| 检查项 | 预期结果 | 状态 |
|--------|----------|------|
| pkg/contextkey 包 | 已创建，含 GetUserID/WithUserID | ✅ |
| User Service ingress | 使用 contextkey.WithUserID/GetUserID | ✅ |
| User Service server | 使用 contextkey.GetUserID | ✅ |
| Lobby Service ingress | 删除局部 userIDKey，使用 contextkey | ✅ |
| Lobby Service checkin_server | 使用 contextkey.GetUserID | ✅ |
| Lobby Service item_server | 使用 contextkey.GetUserID | ✅ |
| 所有测试文件 | 使用 contextkey.WithUserID/GetUserID | ✅ |
| 所有测试通过 | `go test ./...` ✓ | ✅ |
| 集成测试通过 | CLI 测试 | ✅ |

---

## 已知问题

### CLI 登录成功/失败显示不一致（待调查）

**现象：** 交互式 CLI 中 `user:login test 123` 有时显示"用户名或密码错误"，但服务端日志显示登录成功。

**调查结果：**

1. **服务端行为正确**
   - User Service 日志：`User logged in successfully {"username": "test"}`
   - Gateway 日志：`User connection registered {"user_id": "10", "client_id": "conn-22"}`
   - 直接 WebSocket 测试返回正确响应：`{"code": 0, "msg": "success", "data": {"token": "...", "user_id": 10}}`

2. **CLI 代码逻辑正确**
   - `if resp.Code != 0 { return fmt.Sprintf("[错误] %s", resp.Msg) }`

3. **可能原因**
   - **时间窗口竞态**：短时间多次输入命令，导致 session 状态变化
   - **输入缓冲区问题**：交互式终端的行缓冲可能导致命令被错误解析
   - **Session 替换**：重复登录触发 `session replaced` 错误

4. **验证结果**
   - 管道输入测试正常：`echo "user:login test 123" | go run ./cmd/cli/main.go` → `[成功] 已登录`
   - 直接 WebSocket 调用正常

**状态：** 待进一步调查，可能需要添加 CLI 调试日志

**建议修复：** 在 CLI 的 `gateway.Send()` 调用前后添加详细日志，记录请求和响应的完整内容
