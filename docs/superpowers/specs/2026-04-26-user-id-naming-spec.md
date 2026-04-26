# UserID 字段命名与类型规范

## 问题描述

当前系统中 `user_id` 字段存在两个问题：

### 1. 命名不一致

| 层级 | 当前格式 | 说明 |
|------|---------|------|
| Proto 定义 | `user_id` | snake_case |
| protojson 输出 | `userId` | camelCase（protojson 默认行为） |
| CLI 解析 | `userId` | 与 protojson 一致 |
| Gateway context key | `"user_id"` | snake_case 字符串 |
| Redis Pub/Sub | `{"user_id": 123}` | snake_case |

### 2. 类型选择不当

当前使用 `uint64`，导致 protojson 输出字符串：
```json
{"user_id": "123"}  // 字符串，前端处理不便
```

`uint32` 输出数字：
```json
{"user_id": 123}  // 数字，更自然
```

`uint32` 范围 0 ~ 42亿，足够任何规模用户系统。

---

## 规范决策

### 决策1：全局使用 snake_case

**理由：**
1. Proto 定义已使用 snake_case
2. Redis Pub/Sub 已使用 snake_case
3. 数据库字段也是 snake_case
4. 后端服务 context key 已是 snake_case

### 决策2：类型改为 uint32

**理由：**
1. 42亿足够（微信13亿用户）
2. JSON 输出是数字，前端处理更自然
3. 不需要字符串转 int64 的辅助函数

---

## 统一规范

| 层级 | 格式 | 类型 |
|------|------|------|
| Proto 定义 | `user_id` (snake_case) | `uint32` |
| JSON 输出 | `user_id` (snake_case) | 数字 |
| Context Key | `pkg/contextkey.UserIDKey` | `uint32` |
| Redis Pub/Sub | `user_id` | 数字 |
| Go 代码 | `userID` | `uint32` |

---

## 新增：全局 Context Key 定义

### 文件：`pkg/contextkey/contextkey.go`

```go
// Package contextkey 定义全局 context.Value 键，避免跨服务键冲突。
package contextkey

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

### 使用方式

**写入 context：**
```go
import "github.com/CBookShu/kd48/pkg/contextkey"

ctx = contextkey.WithUserID(ctx, userID)
```

**读取 context：**
```go
import "github.com/CBookShu/kd48/pkg/contextkey"

userID, ok := contextkey.GetUserID(ctx)
if !ok {
    return status.Errorf(codes.Unauthenticated, "用户未认证")
}
```

### 设计理由

1. **类型安全**：typed key 避免 string key 的潜在冲突
2. **统一入口**：所有服务使用同一个包，避免定义分散
3. **封装函数**：`GetUserID` 和 `WithUserID` 隐藏类型断言细节
4. **易于维护**：修改 key 类型只需改一处

---

## 需要修改的文件

### 0. 新增 `pkg/contextkey/contextkey.go`

创建全局 context key 定义包（见上方「新增：全局 Context Key 定义」章节）。

### 1. `api/proto/user/v1/user.proto`

```protobuf
// 修改前
message LoginData {
  string token = 1;
  uint64 user_id = 2;
}

message RegisterData {
  string token = 1;
  uint64 user_id = 2;
}

message VerifyTokenData {
  uint64 user_id = 1;
  string username = 2;
}

// 修改后
message LoginData {
  string token = 1;
  uint32 user_id = 2;
}

message RegisterData {
  string token = 1;
  uint32 user_id = 2;
}

message VerifyTokenData {
  uint32 user_id = 1;
  string username = 2;
}
```

修改后重新生成：
```bash
buf generate
```

### 2. `gateway/internal/ws/response_data.go`

```go
// 修改前
marshaler := protojson.MarshalOptions{EmitUnpopulated: true}

// 修改后
marshaler := protojson.MarshalOptions{
    EmitUnpopulated: true,
    UseProtoNames:   true,  // 使用 proto 字段名（snake_case）
}
```

### 3. `gateway/internal/ws/handler.go`

**修改 `clientMeta`：**
```go
type clientMeta struct {
    connID          uint64
    conn            *websocket.Conn
    clientID        string
    isAuthenticated bool
    userID          uint32  // int64 → uint32
    token           string
}
```

**删除 `extractUserIDFromResponse` 函数，简化为：**
```go
// 7. 登录成功后提取 user_id 并注册用户连接映射
if route.EstablishSession {
    meta.isAuthenticated = true

    if dataMap, ok := data.(map[string]interface{}); ok {
        if userID := extractUint32(dataMap["user_id"]); userID > 0 {
            meta.userID = userID
            if h.connManager != nil {
                h.connManager.RegisterUserConnection(userID, clientID)
                slog.InfoContext(ctx, "User connection registered",
                    "user_id", userID, "client_id", clientID)
            }
        }
    }
}

// 辅助函数：提取 uint32（JSON 数字解析为 float64）
func extractUint32(v interface{}) uint32 {
    switch val := v.(type) {
    case float64:
        return uint32(val)
    case int:
        return uint32(val)
    case uint32:
        return val
    default:
        return 0
    }
}
```

### 4. `gateway/internal/ws/connection_manager.go`

```go
// 修改前
userConnections map[int64]string

func (cm *ConnectionManager) RegisterUserConnection(userID int64, clientID string)
func (cm *ConnectionManager) GetUserClientID(userID int64) (string, bool)
func (cm *ConnectionManager) DisconnectByUserID(userID int64, reason string)

// 修改后
userConnections map[uint32]string

func (cm *ConnectionManager) RegisterUserConnection(userID uint32, clientID string)
func (cm *ConnectionManager) GetUserClientID(userID uint32) (string, bool)
func (cm *ConnectionManager) DisconnectByUserID(userID uint32, reason string)
```

### 5. `gateway/internal/ws/session_subscriber.go`

```go
// 修改前
var data struct { UserID int64 `json:"user_id"` }

// 修改后
var data struct { UserID uint32 `json:"user_id"` }
```

### 6. `cmd/cli/internal/commands/handler.go`

```go
// 修改前
type userLoginResp struct {
    Token  string      `json:"token"`
    UserID int64String `json:"userId"`
}

type userRegisterResp struct {
    Token  string      `json:"token"`
    UserID int64String `json:"userId"`
}

// 修改后
type userLoginResp struct {
    Token  string `json:"token"`
    UserID uint32 `json:"user_id"`
}

type userRegisterResp struct {
    Token  string `json:"token"`
    UserID uint32 `json:"user_id"`
}
```

删除 `int64String` 类型（不再需要）。

### 7. `services/user/cmd/user/server.go`

**函数签名修改：**
```go
// 修改前
func (s *userService) issueSessionAtomic(ctx context.Context, userID uint64, username string) ...
func (s *userService) publishSessionInvalidate(ctx context.Context, userID uint64)

// 修改后
func (s *userService) issueSessionAtomic(ctx context.Context, userID uint32, username string) ...
func (s *userService) publishSessionInvalidate(ctx context.Context, userID uint32)
```

**调用处无需修改**（sqlc 生成类型也变为 uint32，类型完全匹配）：
```go
// user.ID 现在是 uint32，与函数参数类型一致
token, hasOldToken, err := s.issueSessionAtomic(ctx, user.ID, user.Username)
s.publishSessionInvalidate(ctx, user.ID)

// 返回值也无需转换
return &userv1.LoginData{
    Token:  token,
    UserId: user.ID,  // uint32，类型匹配
}
```

### 8. `services/user/cmd/user/ingress.go`

```go
// 修改前
var userID uint32
var username string
if _, err := fmt.Sscanf(sessionValue, "%d:%s", &userID, &username); err != nil {

// Create context with user_id for the service
ctxWithUser := context.WithValue(ctx, "user_id", userID)

// 修改后
import "github.com/CBookShu/kd48/pkg/contextkey"

var userID uint32
var username string
if _, err := fmt.Sscanf(sessionValue, "%d:%s", &userID, &username); err != nil {

// Create context with user_id for the service
ctxWithUser := contextkey.WithUserID(ctx, userID)
```

### 9. `services/user/cmd/user/server.go` - Context Value 类型断言

```go
// 修改前
userIDVal := ctx.Value("user_id")
if userIDVal == nil {
    return nil, status.Errorf(codes.Unauthenticated, "用户未认证")
}
userID, ok := userIDVal.(uint32)
if !ok {
    return nil, status.Errorf(codes.Unauthenticated, "用户未认证")
}

// 修改后
import "github.com/CBookShu/kd48/pkg/contextkey"

userID, ok := contextkey.GetUserID(ctx)
if !ok {
    return nil, status.Errorf(codes.Unauthenticated, "用户未认证")
}
```

### 10. `services/user/cmd/user/ingress.go` - VerifyToken 路由中读取 context

```go
// 修改前
userIDVal := ctx.Value("user_id")
if userIDVal != nil {

// 修改后
import "github.com/CBookShu/kd48/pkg/contextkey"

_, ok := contextkey.GetUserID(ctx)
if ok {
```

### 11. `services/user/cmd/user/ingress_test.go`

```go
// 修改前
assert.Equal(t, "123", got["userId"])

// 修改后
assert.Equal(t, float64(123), got["user_id"])  // JSON 数字解析为 float64
```

### 12. `services/lobby/internal/checkin/store.go`

```go
// 修改前
func redisKey(userID int64) string {
func (s *CheckinStore) GetStatus(ctx context.Context, userID int64) (*UserCheckinStatus, error)
func (s *CheckinStore) UpdateStatus(ctx context.Context, userID int64, status *UserCheckinStatus) error
func (s *CheckinStore) ResetStatus(ctx context.Context, userID int64, periodID int64) error

// 修改后
func redisKey(userID uint32) string {
func (s *CheckinStore) GetStatus(ctx context.Context, userID uint32) (*UserCheckinStatus, error)
func (s *CheckinStore) UpdateStatus(ctx context.Context, userID uint32, status *UserCheckinStatus) error
func (s *CheckinStore) ResetStatus(ctx context.Context, userID uint32, periodID int64) error
```

### 13. `services/lobby/internal/item/store.go`

```go
// 修改前
func redisKey(userID int64) string {
func (s *ItemStore) GetItems(ctx context.Context, userID int64) (map[int32]int64, error)
func (s *ItemStore) AddItem(ctx context.Context, userID int64, itemID int32, count int64) error
func (s *ItemStore) SetItem(ctx context.Context, userID int64, itemID int32, count int64) error

// 修改后
func redisKey(userID uint32) string {
func (s *ItemStore) GetItems(ctx context.Context, userID uint32) (map[int32]int64, error)
func (s *ItemStore) AddItem(ctx context.Context, userID uint32, itemID int32, count int64) error
func (s *ItemStore) SetItem(ctx context.Context, userID uint32, itemID int32, count int64) error
```

### 14. `services/lobby/cmd/lobby/ingress.go`

**删除局部定义，改用公共包：**
```go
// 修改前
type contextKey string
const userIDKey contextKey = "user_id"

// 修改后
import "github.com/CBookShu/kd48/pkg/contextkey"

// 删除上述定义，直接使用 contextkey.UserIDKey
```

**注入 user_id 到 context：**
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

### 15. `services/lobby/cmd/lobby/checkin_server.go`

```go
// 修改前
userID, ok := ctx.Value(userIDKey).(uint32)

// 修改后
import "github.com/CBookShu/kd48/pkg/contextkey"

userID, ok := contextkey.GetUserID(ctx)
if !ok {
    return nil, status.Errorf(codes.Code(commonv1.ErrorCode_USER_NOT_AUTHENTICATED), "用户未认证")
}
```

### 16. `services/lobby/cmd/lobby/item_server.go`

```go
// 修改前
userID, ok := ctx.Value(userIDKey).(uint32)

// 修改后
import "github.com/CBookShu/kd48/pkg/contextkey"

userID, ok := contextkey.GetUserID(ctx)
if !ok {
    return nil, status.Errorf(codes.Code(commonv1.ErrorCode_USER_NOT_AUTHENTICATED), "用户未认证")
}
```

### 17. Gateway 测试文件

以下测试文件中的 `int64` 类型需要改为 `uint32`：
- `gateway/internal/ws/connection_manager_test.go`
- `gateway/internal/ws/session_subscriber_test.go`
- `gateway/internal/ws/handler_test.go`

---

## 数据流验证

### 修改后的数据流

```
Login 返回：
{"token": "xxx", "user_id": 123}  ← snake_case + 数字

VerifyToken 返回：
{"user_id": 123, "username": "test"}  ← snake_case + 数字
```

### 无需字符串转换

`uint32` 在 JSON 中是数字，`json.Unmarshal` 解析为 `float64`：
```go
var data map[string]interface{}
json.Unmarshal([]byte(`{"user_id": 123}`), &data)
// data["user_id"] == float64(123)
```

---

## 数据库层面

### 现有 Schema

```sql
-- services/user/migrations/000001_init_users.up.sql
CREATE TABLE `users` (
    `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    ...
);

-- services/lobby/migrations/001_create_checkin_tables.sql
CREATE TABLE IF NOT EXISTS user_items (
    user_id BIGINT NOT NULL,
    ...
);

CREATE TABLE IF NOT EXISTS user_checkin (
    user_id BIGINT NOT NULL,
    ...
);
```

### 决策：数据库字段改为 INT UNSIGNED

**理由：**
1. 类型必须明确匹配，不依赖隐式兼容
2. `INT UNSIGNED` 范围 0 ~ 42亿，足够用户系统
3. 与 Proto `uint32` 完全对应

### 需要新增迁移脚本

**文件：`services/user/migrations/000002_change_user_id_to_int.up.sql`**

```sql
ALTER TABLE `users` MODIFY `id` INT UNSIGNED NOT NULL AUTO_INCREMENT;
```

**文件：`services/user/migrations/000002_change_user_id_to_int.down.sql`**

```sql
ALTER TABLE `users` MODIFY `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT;
```

**文件：`services/lobby/migrations/002_change_user_id_to_int.up.sql`**

```sql
ALTER TABLE `user_items` MODIFY `user_id` INT UNSIGNED NOT NULL;
ALTER TABLE `user_checkin` MODIFY `user_id` INT UNSIGNED NOT NULL;
```

**文件：`services/lobby/migrations/002_change_user_id_to_int.down.sql`**

```sql
ALTER TABLE `user_items` MODIFY `user_id` BIGINT NOT NULL;
ALTER TABLE `user_checkin` MODIFY `user_id` BIGINT NOT NULL;
```

### sqlc 重新生成

修改数据库后，重新生成 sqlc 代码：

```bash
cd services/user && sqlc generate
cd services/lobby && sqlc generate
```

生成后的 `models.go` 将变为：
```go
type User struct {
    ID uint32 `json:"id"`  // uint64 → uint32
    ...
}
```

### 无需手动类型转换

数据库、sqlc、Proto 类型完全一致：
- 数据库：`INT UNSIGNED`
- sqlc：`uint32`
- Proto：`uint32`
- Go 业务代码：`uint32`

---

## 测试验证

### 单元测试

```go
func TestProtojsonSnakeCaseUint32(t *testing.T) {
    d := &userv1.LoginData{Token: "test", UserId: 123}
    marshaler := protojson.MarshalOptions{
        EmitUnpopulated: true,
        UseProtoNames:   true,
    }
    b, err := marshaler.Marshal(d)
    require.NoError(t, err)

    // snake_case + 数字
    assert.Contains(t, string(b), `"user_id":123`)
    assert.NotContains(t, string(b), `"userId"`)
    assert.NotContains(t, string(b), `"user_id":"123"`)  // 不是字符串
    }
```

### Context Key 类型安全问题修复

**问题：** 测试文件使用字符串 `"user_id"` 作为 context key，但生产代码改用 `pkg/contextkey.UserIDKey`。

**原因：** Go 的 `context.Value` 比较是类型敏感的：
```go
context.WithValue(ctx, "user_id", value)       // key 类型是 string
context.WithValue(ctx, contextkey.UserIDKey, value)  // key 类型是 contextKey
// 这两个 key 不相等，无法互相访问
```

**修复原则：** 所有代码（生产 + 测试）统一使用 `pkg/contextkey` 包。

#### 需要修复的测试文件

**1. `services/lobby/cmd/lobby/checkin_server_test.go`**

所有 `context.WithValue(context.Background(), "user_id", ...)` 改为：
```go
import "github.com/CBookShu/kd48/pkg/contextkey"

ctx := contextkey.WithUserID(context.Background(), uint32(12345))
```

涉及行号：56, 90, 103, 124, 138, 157, 182, 204, 226, 239, 265

**2. `services/lobby/cmd/lobby/item_server_test.go`**

所有 `context.WithValue(context.Background(), "user_id", ...)` 改为：
```go
import "github.com/CBookShu/kd48/pkg/contextkey"

ctx := contextkey.WithUserID(context.Background(), uint32(12345))
```

涉及行号：35, 66, 81, 85, 103, 119

**3. `services/lobby/cmd/lobby/ingress_test.go`**

Mock 服务中提取 user_id 的方式改为：
```go
// 修改前
if userID, ok := ctx.Value("user_id").(uint32); ok {

// 修改后
import "github.com/CBookShu/kd48/pkg/contextkey"

if userID, ok := contextkey.GetUserID(ctx); ok {
```

涉及行号：43, 57, 78

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

涉及行号：107-117, 124-135, 141-152

**4. `services/user/cmd/user/` 下的测试文件**

如果存在使用 `ctx.Value("user_id")` 的测试，同样改为 `contextkey.GetUserID(ctx)`。

### 集成测试

1. CLI 登录成功
2. 验证 `user:whoami` 显示正确用户 ID
3. 验证顶号踢人功能正常

---

## 变更影响

| 组件 | 影响范围 |
|------|---------|
| 公共包 | 新增 `pkg/contextkey/contextkey.go` |
| Proto | 3个 message 的 user_id 类型 |
| 数据库 | 3张表的 user_id 字段类型 |
| sqlc | 重新生成，类型自动变为 uint32 |
| Gateway | protojson 配置 + 类型定义 + 4个文件 |
| CLI | JSON tag + 类型定义 |
| User Service | ingress + server + 测试（改用 contextkey 包） |
| Lobby Service | ingress 删除局部定义 + store + server + 测试（改用 contextkey 包） |
| 测试 | 所有测试文件改用 `contextkey.WithUserID/GetUserID` |

---

## 执行步骤

### 阶段1：创建公共包

1. **创建全局 Context Key 包**
   - 创建 `pkg/contextkey/contextkey.go`
   - 定义 `UserIDKey`、`GetUserID()`、`WithUserID()`

### 阶段2：修改 Proto 并重新生成

2. **修改 Proto 定义**
   - 修改 `api/proto/user/v1/user.proto`（uint64 → uint32）
   - 运行 `buf generate` 重新生成 Go 代码

### 阶段3：数据库迁移和 sqlc 重新生成

3. **执行数据库迁移**
   - User 服务：`000002_change_user_id_to_int.up.sql`
   - Lobby 服务：`002_change_user_id_to_int.up.sql`

4. **重新生成 sqlc**
   ```bash
   cd services/user && sqlc generate
   cd services/lobby && sqlc generate
   ```

### 阶段4：修改 Gateway

5. **修改 Gateway**
   - `response_data.go` 添加 `UseProtoNames: true`
   - `handler.go` 类型修改 + 删除 `extractUserIDFromResponse`
   - `connection_manager.go` 类型修改
   - `session_subscriber.go` 类型修改

### 阶段5：修改 CLI

6. **修改 CLI**
   - `handler.go` JSON tag + 类型定义

### 阶段6：修改 User Service

7. **修改 User Service**
   - `ingress.go` 改用 `contextkey.WithUserID()`
   - `server.go` 改用 `contextkey.GetUserID()`
   - 更新测试文件

### 阶段7：修改 Lobby Service

8. **修改 Lobby Service**
   - `ingress.go` 删除局部 `userIDKey`，改用 `contextkey` 包
   - `internal/checkin/store.go` 函数签名 `int64` → `uint32`
   - `internal/item/store.go` 函数签名 `int64` → `uint32`
   - `checkin_server.go` 改用 `contextkey.GetUserID()`
   - `item_server.go` 改用 `contextkey.GetUserID()`
   - 更新测试文件（见下方详细说明）

### 阶段8：更新所有测试文件

9. **更新测试文件**（全部改用 `contextkey` 包）

   **9.1 `services/lobby/cmd/lobby/checkin_server_test.go`**
   - `context.WithValue(..., "user_id", ...)` → `contextkey.WithUserID(...)`
   - 共 11 处

   **9.2 `services/lobby/cmd/lobby/item_server_test.go`**
   - `context.WithValue(..., "user_id", ...)` → `contextkey.WithUserID(...)`
   - 共 6 处

   **9.3 `services/lobby/cmd/lobby/ingress_test.go`**
   - Mock 中 `ctx.Value("user_id")` → `contextkey.GetUserID(ctx)`
   - 测试用例通过 `Baggage` 注入 user_id
   - 共 3 处 mock + 3 处测试用例

   **9.4 Gateway 测试文件**
   - `connection_manager_test.go`
   - `session_subscriber_test.go`
   - `handler_test.go`

   **9.5 User Service 测试文件**
   - `services/user/cmd/user/ingress_test.go`

### 阶段9：编译和测试验证

10. **编译验证**
    ```bash
    go build ./...
    ```

11. **运行测试**
    ```bash
    go test ./...
    ```

### 阶段10：集成测试

12. **CLI 集成测试**
    - 登录成功
    - `user:whoami` 显示正确用户 ID
    - 顶号踢人功能正常

---

## 已知问题

### CLI 登录成功/失败显示不一致（待调查）

**现象：** 用户报告 `user:login test 123` 显示"用户名或密码错误"，但服务端日志显示登录成功。

**调查结果：**

1. **服务端行为正确**
   - User Service 日志：`User logged in successfully {"username": "test"}`
   - Gateway 日志：`User connection registered {"user_id": "10", "client_id": "conn-22"}`
   - 直接 WebSocket 测试返回正确响应：`{"code": 0, "msg": "success", "data": {"token": "...", "user_id": 10}}`

2. **CLI 代码逻辑正确**
   ```go
   if resp.Code != 0 {
       return fmt.Sprintf("[错误] %s", resp.Msg)
   }
   // ... 解析响应并返回成功
   ```

3. **可能原因**
   - **时间窗口竞态**：用户在短时间多次输入命令，导致 session 状态变化
   - **输入缓冲区问题**：交互式终端的行缓冲可能导致命令被错误解析
   - **Session 替换**：重复登录触发 `session replaced` 错误

4. **验证结果**
   - 管道输入测试正常：`echo "user:login test 123" | go run ./cmd/cli/main.go` → `[成功] 已登录`
   - 直接 WebSocket 调用正常

**状态：** 待进一步调查，可能需要添加 CLI 调试日志

**建议修复：** 在 CLI 的 `gateway.Send()` 调用前后添加详细日志，记录请求和响应的完整内容
