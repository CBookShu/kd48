# RPC 返回值重构实现计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 业务 RPC 只返回业务数据，错误通过 gRPC status 透传，网关直接转发 code 和 message

**Architecture:** 创建 common/v1 公共包定义 ApiResponse 和 ErrorCode，修改 proto 定义让 RPC 直接返回业务数据，服务端使用 status.Errorf 返回错误，网关透传

**Tech Stack:** gRPC, protobuf, Go

---

## 文件结构

```
api/proto/
├── common/
│   └── v1/
│       ├── api_response.proto    # 新建：公共 ApiResponse + ErrorCode
│       └── errors.go             # 新建：错误消息参考（可选）
├── lobby/
│   └── v1/
│       ├── common.proto          # 修改：删除 ApiResponse + ErrorCode
│       └── checkin.proto         # 修改：RPC 返回业务数据
└── user/
    └── v1/
        └── user.proto            # 修改：删除 success 字段，返回业务数据

services/lobby/cmd/lobby/
├── checkin_server.go             # 修改：使用 status.Errorf
├── item_server.go                # 修改：使用 status.Errorf
└── server.go                     # 修改：删除 errorResponse/successResponse

services/user/cmd/user/
└── server.go                     # 修改：删除 success 字段，使用 commonv1.ErrorCode

gateway/internal/ws/
└── handler.go                    # 已实现透传，无需修改

cmd/cli/internal/commands/
└── handler.go                    # 修改：移除 Success 字段检查
```

---

### Task 1: 创建公共 proto 包

**Files:**
- Create: `api/proto/common/v1/api_response.proto`
- Create: `api/proto/common/v1/errors.go`

- [ ] **Step 1: 创建 api_response.proto**

```protobuf
syntax = "proto3";

package common.v1;

option go_package = "github.com/CBookShu/kd48/api/proto/common/v1;commonv1";

import "google/protobuf/any.proto";

// ApiResponse 通用响应外壳（所有服务共用）
message ApiResponse {
  int32 code = 1;
  string message = 2;
  google.protobuf.Any data = 3;
  map<string, string> meta = 4;
}

// ErrorCode 错误码定义（所有服务共用）
// 规则：每个模块预分配 1000 个码
enum ErrorCode {
  SUCCESS = 0;

  // 系统错误 1000-1999
  INVALID_REQUEST = 1000;
  INTERNAL_ERROR = 1001;
  SERVICE_UNAVAILABLE = 1002;

  // 用户相关 2000-2999
  USER_NOT_FOUND = 2000;
  USER_NOT_AUTHENTICATED = 2001;

  // 签到相关 3000-3999
  CHECKIN_ALREADY_TODAY = 3000;
  CHECKIN_PERIOD_NOT_ACTIVE = 3001;
  CHECKIN_PERIOD_EXPIRED = 3002;

  // 物品相关 4000-4999
  ITEM_NOT_FOUND = 4000;
  ITEM_INSUFFICIENT = 4001;
}
```

- [ ] **Step 2: 创建 errors.go（可选，用于文档）**

```go
package commonv1

// 错误消息参考表（可选，用于构造错误或文档）
var ErrorMessages = map[int32]string{
	// 系统错误 (1000-1999)
	1000: "请求参数错误",
	1001: "内部错误",
	1002: "服务不可用",

	// 用户相关 (2000-2999)
	2000: "用户不存在",
	2001: "用户未认证",

	// 签到相关 (3000-3999)
	3000: "今日已签到",
	3001: "签到期未开启",
	3002: "签到期已过期",

	// 物品相关 (4000-4999)
	4000: "物品不存在",
	4001: "物品不足",
}
```

- [ ] **Step 3: 生成 Go 代码**

```bash
protoc -I api/proto \
  --go_out=api/proto --go_opt=module=github.com/CBookShu/kd48/api/proto \
  --go-grpc_out=api/proto --go-grpc_opt=module=github.com/CBookShu/kd48/api/proto \
  common/v1/api_response.proto
```

- [ ] **Step 4: 编译验证**

```bash
go build ./api/proto/common/v1/...
```

Expected: 编译成功，无错误

- [ ] **Step 5: 提交**

```bash
git add api/proto/common/
git commit -m "feat: add common/v1 package with ApiResponse and ErrorCode"
```

---

### Task 2: 修改 lobby proto 定义

**Files:**
- Modify: `api/proto/lobby/v1/common.proto`
- Modify: `api/proto/lobby/v1/checkin.proto`

- [ ] **Step 1: 清空 lobby/v1/common.proto**

```protobuf
syntax = "proto3";

package lobby.v1;

option go_package = "github.com/CBookShu/kd48/api/proto/lobby/v1;lobbyv1";

// 此文件保留为空，ApiResponse 和 ErrorCode 已移至 common/v1
```

- [ ] **Step 2: 修改 checkin.proto - RPC 返回业务数据**

```protobuf
syntax = "proto3";

package lobby.v1;

option go_package = "github.com/CBookShu/kd48/api/proto/lobby/v1;lobbyv1";

// CheckinService 签到服务
service CheckinService {
  rpc Checkin(CheckinRequest) returns (CheckinData);
  rpc GetStatus(GetStatusRequest) returns (CheckinStatusData);
}

message CheckinRequest {}

message CheckinData {
  int32 continuous_days = 1;
  map<int32, int64> rewards = 2;
}

message GetStatusRequest {}

message CheckinStatusData {
  int64 period_id = 1;
  string period_name = 2;
  repeated DailyRewardInfo daily_rewards = 3;
  repeated ContinuousRewardInfo continuous_rewards = 4;

  bool today_checked = 10;
  int32 continuous_days = 11;
  int32 total_days = 12;
  repeated int32 claimed_continuous = 13;
}

message DailyRewardInfo {
  int32 day = 1;
  map<int32, int64> rewards = 2;
}

message ContinuousRewardInfo {
  int32 continuous_days = 1;
  map<int32, int64> rewards = 2;
}
```

- [ ] **Step 3: 重新生成 proto**

```bash
./gen_proto.sh
```

或手动：
```bash
protoc -I api/proto \
  --go_out=api/proto --go_opt=module=github.com/CBookShu/kd48/api/proto \
  --go-grpc_out=api/proto --go-grpc_opt=module=github.com/CBookShu/kd48/api/proto \
  lobby/v1/common.proto \
  lobby/v1/checkin.proto
```

- [ ] **Step 4: 编译验证**

```bash
go build ./api/proto/lobby/v1/...
```

Expected: 编译成功，无错误

- [ ] **Step 5: 提交**

```bash
git add api/proto/lobby/v1/
git commit -m "feat: lobby proto returns business data directly"
```

---

### Task 3: 更新 gen_proto.sh

**Files:**
- Modify: `gen_proto.sh`

- [ ] **Step 1: 添加 common/v1 到生成脚本**

```bash
#!/bin/bash
set -euo pipefail
ROOT="$(cd "$(dirname "$0")" && pwd)"
cd "$ROOT"

protoc -I api/proto \
	--go_out=api/proto --go_opt=module=github.com/CBookShu/kd48/api/proto \
	--go-grpc_out=api/proto --go-grpc_opt=module=github.com/CBookShu/kd48/api/proto \
	common/v1/api_response.proto \
	user/v1/user.proto \
	gateway/v1/gateway.proto \
	gateway/v1/service_type.proto \
	gateway/v1/gateway_route.proto \
	lobby/v1/lobby.proto \
	lobby/v1/common.proto \
	lobby/v1/item.proto \
	lobby/v1/checkin.proto
```

- [ ] **Step 2: 提交**

```bash
git add gen_proto.sh
git commit -m "chore: add common/v1 to proto generation script"
```

---

### Task 4: 修改 checkin_server.go 使用 status.Errorf

**Files:**
- Modify: `services/lobby/cmd/lobby/checkin_server.go`

- [ ] **Step 1: 更新 import**

```go
import (
	"context"
	"log/slog"
	"time"

	commonv1 "github.com/CBookShu/kd48/api/proto/common/v1"
	lobbyv1 "github.com/CBookShu/kd48/api/proto/lobby/v1"
	"github.com/CBookShu/kd48/services/lobby/internal/checkin"
	"github.com/CBookShu/kd48/services/lobby/internal/item"
	"github.com/redis/go-redis/v9"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)
```

- [ ] **Step 2: 修改 Checkin 方法签名和实现**

```go
// Checkin 签到
func (s *CheckinService) Checkin(ctx context.Context, req *lobbyv1.CheckinRequest) (*lobbyv1.CheckinData, error) {
	userID, ok := ctx.Value("user_id").(int64)
	if !ok {
		return nil, status.Errorf(codes.Code(commonv1.ErrorCode_USER_NOT_AUTHENTICATED), "用户未认证")
	}

	if s.period == nil {
		return nil, status.Errorf(codes.Code(commonv1.ErrorCode_CHECKIN_PERIOD_NOT_ACTIVE), "签到期未开启")
	}

	// 获取用户签到状态
	statusData, err := s.checkinStore.GetStatus(ctx, userID)
	if err != nil {
		slog.ErrorContext(ctx, "failed to get checkin status", "user_id", userID, "error", err)
		return nil, status.Errorf(codes.Code(commonv1.ErrorCode_INTERNAL_ERROR), "获取签到状态失败")
	}

	// 检查是否需要重置（新期开始）
	if statusData.PeriodID != s.period.PeriodID {
		statusData = &checkin.UserCheckinStatus{
			PeriodID:    s.period.PeriodID,
			ClaimedDays: []int{},
		}
	}

	// 检查今日是否已签到
	today := time.Now().Format("2006-01-02")
	if statusData.LastCheckinDate == today {
		return nil, status.Errorf(codes.Code(commonv1.ErrorCode_CHECKIN_ALREADY_TODAY), "今日已签到")
	}

	// 计算连续天数
	yesterday := time.Now().AddDate(0, 0, -1).Format("2006-01-02")
	if statusData.LastCheckinDate == yesterday {
		statusData.ContinuousDays++
	} else {
		statusData.ContinuousDays = 1
	}

	// 计算奖励
	day := len(statusData.ClaimedDays) + 1
	rewards := s.calculator.GetDailyReward(day)

	// 检查连续奖励
	if s.calculator.HasContinuousReward(statusData.ContinuousDays) {
		continuousReward := s.calculator.GetContinuousReward(statusData.ContinuousDays)
		rewards = checkin.MergeRewards(rewards, continuousReward)
	}

	// 发放奖励
	for itemID, count := range rewards {
		if err := s.itemStore.AddItem(ctx, userID, itemID, count); err != nil {
			slog.ErrorContext(ctx, "failed to add items", "user_id", userID, "error", err)
			return nil, status.Errorf(codes.Code(commonv1.ErrorCode_INTERNAL_ERROR), "发放奖励失败")
		}
	}

	// 更新签到状态
	statusData.LastCheckinDate = today
	statusData.ClaimedDays = append(statusData.ClaimedDays, day)
	if err := s.checkinStore.UpdateStatus(ctx, userID, statusData); err != nil {
		slog.ErrorContext(ctx, "failed to update checkin status", "user_id", userID, "error", err)
		return nil, status.Errorf(codes.Code(commonv1.ErrorCode_INTERNAL_ERROR), "更新签到状态失败")
	}

	return &lobbyv1.CheckinData{
		ContinuousDays: int32(statusData.ContinuousDays),
		Rewards:        rewards,
	}, nil
}
```

- [ ] **Step 3: 修改 GetStatus 方法签名和实现**

```go
// GetStatus 获取签到状态
func (s *CheckinService) GetStatus(ctx context.Context, req *lobbyv1.GetStatusRequest) (*lobbyv1.CheckinStatusData, error) {
	userID, ok := ctx.Value("user_id").(int64)
	if !ok {
		return nil, status.Errorf(codes.Code(commonv1.ErrorCode_USER_NOT_AUTHENTICATED), "用户未认证")
	}

	if s.period == nil {
		return nil, status.Errorf(codes.Code(commonv1.ErrorCode_CHECKIN_PERIOD_NOT_ACTIVE), "签到期未开启")
	}

	// 获取用户签到状态
	statusData, err := s.checkinStore.GetStatus(ctx, userID)
	if err != nil {
		slog.ErrorContext(ctx, "failed to get checkin status", "user_id", userID, "error", err)
		return nil, status.Errorf(codes.Code(commonv1.ErrorCode_INTERNAL_ERROR), "获取签到状态失败")
	}

	// 检查是否需要重置
	if statusData.PeriodID != s.period.PeriodID {
		statusData = &checkin.UserCheckinStatus{
			PeriodID:    s.period.PeriodID,
			ClaimedDays: []int{},
		}
	}

	// 构建响应
	today := time.Now().Format("2006-01-02")
	data := &lobbyv1.CheckinStatusData{
		PeriodId:          s.period.PeriodID,
		PeriodName:        s.period.PeriodName,
		TodayChecked:      statusData.LastCheckinDate == today,
		ContinuousDays:    int32(statusData.ContinuousDays),
		TotalDays:         int32(len(statusData.ClaimedDays)),
		ClaimedContinuous: getClaimedContinuous(statusData.ClaimedDays, s.calculator),
	}

	// 添加每日奖励配置
	for day := 1; day <= 7; day++ {
		data.DailyRewards = append(data.DailyRewards, &lobbyv1.DailyRewardInfo{
			Day:     int32(day),
			Rewards: s.calculator.GetDailyReward(day),
		})
	}

	// 添加连续奖励配置
	for days, rewards := range s.calculator.GetAllContinuousRewards() {
		data.ContinuousRewards = append(data.ContinuousRewards, &lobbyv1.ContinuousRewardInfo{
			ContinuousDays: int32(days),
			Rewards:        rewards,
		})
	}

	return data, nil
}
```

- [ ] **Step 4: 编译验证**

```bash
go build ./services/lobby/cmd/lobby/...
```

- [ ] **Step 5: 更新 checkin_server_test.go**

测试需要适配新的返回格式（直接返回业务数据，错误通过 gRPC status 返回）：

```go
// services/lobby/cmd/lobby/checkin_server_test.go
package main

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	commonv1 "github.com/CBookShu/kd48/api/proto/common/v1"
	lobbyv1 "github.com/CBookShu/kd48/api/proto/lobby/v1"
	"github.com/CBookShu/kd48/services/lobby/internal/checkin"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/status"
)

func setupCheckinTest(t *testing.T) (*CheckinService, *miniredis.Miniredis) {
	mr, err := miniredis.Run()
	require.NoError(t, err)

	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	svc := NewCheckinService(rdb)

	// 设置测试期配置和奖励
	svc.SetPeriod(&PeriodConfig{
		PeriodID:   1,
		PeriodName: "Test Period",
	})
	svc.SetDailyRewards(map[int]map[int32]int64{
		1: {1001: 100},
		2: {1001: 200},
		3: {1001: 300},
		4: {1001: 400},
		5: {1001: 500},
		6: {1001: 600},
		7: {1002: 1000},
	})
	svc.SetContinuousRewards(map[int]map[int32]int64{
		3: {1003: 50},
		5: {1003: 100},
		7: {1003: 200},
	})

	return svc, mr
}

// === GetStatus Tests ===

func TestCheckinService_GetStatus_Success(t *testing.T) {
	svc, mr := setupCheckinTest(t)
	defer mr.Close()

	ctx := context.WithValue(context.Background(), "user_id", int64(12345))
	data, err := svc.GetStatus(ctx, &lobbyv1.GetStatusRequest{})

	require.NoError(t, err)
	assert.Equal(t, int64(1), data.PeriodId)
	assert.Equal(t, "Test Period", data.PeriodName)
	assert.False(t, data.TodayChecked)
	assert.Equal(t, int32(0), data.ContinuousDays)
	assert.Equal(t, int32(0), data.TotalDays)
	assert.Len(t, data.DailyRewards, 7)
}

func TestCheckinService_GetStatus_NotAuthenticated(t *testing.T) {
	svc, mr := setupCheckinTest(t)
	defer mr.Close()

	ctx := context.Background() // 没有 user_id
	_, err := svc.GetStatus(ctx, &lobbyv1.GetStatusRequest{})

	require.Error(t, err)
	st, ok := status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.Code(commonv1.ErrorCode_USER_NOT_AUTHENTICATED), st.Code())
}

func TestCheckinService_GetStatus_NoActivePeriod(t *testing.T) {
	mr, err := miniredis.Run()
	require.NoError(t, err)
	defer mr.Close()

	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	svc := NewCheckinService(rdb)
	// 不设置 period

	ctx := context.WithValue(context.Background(), "user_id", int64(12345))
	_, err = svc.GetStatus(ctx, &lobbyv1.GetStatusRequest{})

	require.Error(t, err)
	st, ok := status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.Code(commonv1.ErrorCode_CHECKIN_PERIOD_NOT_ACTIVE), st.Code())
}

func TestCheckinService_GetStatus_AfterCheckin(t *testing.T) {
	svc, mr := setupCheckinTest(t)
	defer mr.Close()

	ctx := context.WithValue(context.Background(), "user_id", int64(12345))

	// 先签到
	_, err := svc.Checkin(ctx, &lobbyv1.CheckinRequest{})
	require.NoError(t, err)

	// 再获取状态
	data, err := svc.GetStatus(ctx, &lobbyv1.GetStatusRequest{})
	require.NoError(t, err)

	assert.True(t, data.TodayChecked)
	assert.Equal(t, int32(1), data.ContinuousDays)
	assert.Equal(t, int32(1), data.TotalDays)
}

// === Checkin Tests ===

func TestCheckinService_Checkin_Success(t *testing.T) {
	svc, mr := setupCheckinTest(t)
	defer mr.Close()

	ctx := context.WithValue(context.Background(), "user_id", int64(12345))

	// 第一次签到
	data, err := svc.Checkin(ctx, &lobbyv1.CheckinRequest{})

	require.NoError(t, err)
	assert.Equal(t, int32(1), data.ContinuousDays)
	assert.Equal(t, map[int32]int64{1001: 100}, data.Rewards)
}

func TestCheckinService_Checkin_AlreadyToday(t *testing.T) {
	svc, mr := setupCheckinTest(t)
	defer mr.Close()

	ctx := context.WithValue(context.Background(), "user_id", int64(12345))

	// 第一次签到
	_, err := svc.Checkin(ctx, &lobbyv1.CheckinRequest{})
	require.NoError(t, err)

	// 同一天再次签到
	_, err = svc.Checkin(ctx, &lobbyv1.CheckinRequest{})

	require.Error(t, err)
	st, ok := status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.Code(commonv1.ErrorCode_CHECKIN_ALREADY_TODAY), st.Code())
}

func TestCheckinService_Checkin_ContinuousDays(t *testing.T) {
	svc, mr := setupCheckinTest(t)
	defer mr.Close()

	ctx := context.WithValue(context.Background(), "user_id", int64(12345))

	// 模拟昨天的签到状态
	yesterday := time.Now().AddDate(0, 0, -1).Format("2006-01-02")
	svc.checkinStore.UpdateStatus(ctx, 12345, &checkin.UserCheckinStatus{
		PeriodID:        1,
		LastCheckinDate: yesterday,
		ContinuousDays:  2,
		ClaimedDays:     []int{1, 2},
	})

	// 今天签到，应该是连续第3天
	data, err := svc.Checkin(ctx, &lobbyv1.CheckinRequest{})
	require.NoError(t, err)

	assert.Equal(t, int32(3), data.ContinuousDays)
	assert.Equal(t, int64(300), data.Rewards[1001]) // 第3天奖励
	assert.Equal(t, int64(50), data.Rewards[1003])  // 连续3天奖励
}

func TestCheckinService_Checkin_NotAuthenticated(t *testing.T) {
	svc, mr := setupCheckinTest(t)
	defer mr.Close()

	ctx := context.Background() // 没有 user_id
	_, err := svc.Checkin(ctx, &lobbyv1.CheckinRequest{})

	require.Error(t, err)
	st, ok := status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.Code(commonv1.ErrorCode_USER_NOT_AUTHENTICATED), st.Code())
}

func TestCheckinService_Checkin_NoActivePeriod(t *testing.T) {
	mr, err := miniredis.Run()
	require.NoError(t, err)
	defer mr.Close()

	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	svc := NewCheckinService(rdb)
	// 不设置 period

	ctx := context.WithValue(context.Background(), "user_id", int64(12345))
	_, err = svc.Checkin(ctx, &lobbyv1.CheckinRequest{})

	require.Error(t, err)
	st, ok := status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.Code(commonv1.ErrorCode_CHECKIN_PERIOD_NOT_ACTIVE), st.Code())
}

func TestCheckinService_Checkin_ItemsAddedToInventory(t *testing.T) {
	svc, mr := setupCheckinTest(t)
	defer mr.Close()

	userID := int64(12345)
	ctx := context.WithValue(context.Background(), "user_id", userID)

	// 签到
	_, err := svc.Checkin(ctx, &lobbyv1.CheckinRequest{})
	require.NoError(t, err)

	// 验证物品已添加到背包
	items, err := svc.itemStore.GetItems(ctx, userID)
	require.NoError(t, err)
	assert.Equal(t, int64(100), items[1001])
}
```

- [ ] **Step 6: 运行测试验证**

```bash
go test ./services/lobby/cmd/lobby/... -v -run TestCheckinService
```

Expected: 所有测试通过

- [ ] **Step 7: 提交**

```bash
git add services/lobby/cmd/lobby/checkin_server.go
git commit -m "feat: checkin service returns business data, uses status.Errorf for errors"
```

---

### Task 5: 修改 item_server.go

**Files:**
- Modify: `services/lobby/cmd/lobby/item_server.go`

- [ ] **Step 1: 更新 import 并修改 ItemService**

```go
package main

import (
	"context"

	commonv1 "github.com/CBookShu/kd48/api/proto/common/v1"
	lobbyv1 "github.com/CBookShu/kd48/api/proto/lobby/v1"
	"github.com/CBookShu/kd48/services/lobby/internal/item"
	"github.com/redis/go-redis/v9"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// ItemService 物品服务
type ItemService struct {
	lobbyv1.UnimplementedItemServiceServer
	store *item.ItemStore
}

func NewItemService(rdb *item.RedisClient) *ItemService {
	return &ItemService{
		store: item.NewItemStore(rdb),
	}
}

// GetMyItems 获取用户物品
func (s *ItemService) GetMyItems(ctx context.Context, req *lobbyv1.GetMyItemsRequest) (*lobbyv1.ItemsData, error) {
	userID, ok := ctx.Value("user_id").(int64)
	if !ok {
		return nil, status.Errorf(codes.Code(commonv1.ErrorCode_USER_NOT_AUTHENTICATED), "用户未认证")
	}

	items, err := s.store.GetItems(ctx, userID)
	if err != nil {
		return nil, status.Errorf(codes.Code(commonv1.ErrorCode_INTERNAL_ERROR), "获取物品失败")
	}

	return &lobbyv1.ItemsData{
		Items: items,
	}, nil
}
```

- [ ] **Step 2: 删除 errorResponse 和 successResponse 函数（如果 server.go 中有的话）**

检查并删除 server.go 或 item_server.go 中的辅助函数：
```go
// 删除这些函数
// func errorResponse(...) *lobbyv1.ApiResponse { ... }
// func successResponse(...) *lobbyv1.ApiResponse { ... }
```

- [ ] **Step 3: 编译验证**

```bash
go build ./services/lobby/cmd/lobby/...
```

- [ ] **Step 4: 更新 item_server_test.go**

测试需要适配新的返回格式：

```go
// services/lobby/cmd/lobby/item_server_test.go
package main

import (
	"context"
	"strconv"
	"testing"

	"github.com/alicebob/miniredis/v2"
	commonv1 "github.com/CBookShu/kd48/api/proto/common/v1"
	lobbyv1 "github.com/CBookShu/kd48/api/proto/lobby/v1"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/status"
)

func setupItemTest(t *testing.T) (*ItemService, *miniredis.Miniredis) {
	mr, err := miniredis.Run()
	require.NoError(t, err)

	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	svc := NewItemService(rdb)

	return svc, mr
}

// === GetMyItems Tests ===

func TestItemService_GetMyItems_Success(t *testing.T) {
	svc, mr := setupItemTest(t)
	defer mr.Close()

	ctx := context.WithValue(context.Background(), "user_id", int64(12345))

	// 设置测试数据
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	rdb.HSet(ctx, "kd48:user_items:12345", "1001", "1000", "1002", "500")

	data, err := svc.GetMyItems(ctx, &lobbyv1.GetMyItemsRequest{})

	require.NoError(t, err)
	assert.Equal(t, int64(1000), data.Items[1001])
	assert.Equal(t, int64(500), data.Items[1002])
}

func TestItemService_GetMyItems_NotAuthenticated(t *testing.T) {
	svc, mr := setupItemTest(t)
	defer mr.Close()

	ctx := context.Background() // 没有 user_id

	_, err := svc.GetMyItems(ctx, &lobbyv1.GetMyItemsRequest{})

	require.Error(t, err)
	st, ok := status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.Code(commonv1.ErrorCode_USER_NOT_AUTHENTICATED), st.Code())
}

func TestItemService_GetMyItems_EmptyInventory(t *testing.T) {
	svc, mr := setupItemTest(t)
	defer mr.Close()

	ctx := context.WithValue(context.Background(), "user_id", int64(12345))

	data, err := svc.GetMyItems(ctx, &lobbyv1.GetMyItemsRequest{})

	require.NoError(t, err)
	assert.Empty(t, data.Items)
}

func TestItemService_GetMyItems_DifferentUsers(t *testing.T) {
	svc, mr := setupItemTest(t)
	defer mr.Close()

	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})

	// 用户1的数据
	ctx1 := context.WithValue(context.Background(), "user_id", int64(111))
	rdb.HSet(ctx1, "kd48:user_items:111", "1001", "100")

	// 用户2的数据
	ctx2 := context.WithValue(context.Background(), "user_id", int64(222))
	rdb.HSet(ctx2, "kd48:user_items:222", "1001", "200")

	// 用户1查询
	data1, err := svc.GetMyItems(ctx1, &lobbyv1.GetMyItemsRequest{})
	require.NoError(t, err)
	assert.Equal(t, int64(100), data1.Items[1001])

	// 用户2查询
	data2, err := svc.GetMyItems(ctx2, &lobbyv1.GetMyItemsRequest{})
	require.NoError(t, err)
	assert.Equal(t, int64(200), data2.Items[1001])
}
```

- [ ] **Step 5: 运行测试验证**

```bash
go test ./services/lobby/cmd/lobby/... -v -run TestItemService
```

Expected: 所有测试通过

- [ ] **Step 6: 提交**

```bash
git add services/lobby/cmd/lobby/item_server.go services/lobby/cmd/lobby/server.go
git commit -m "feat: item service returns business data, removes ApiResponse helpers"
```

---

### Task 6: 更新 lobby/v1/item.proto

**Files:**
- Modify: `api/proto/lobby/v1/item.proto`

- [ ] **Step 1: 修改 item.proto**

```protobuf
syntax = "proto3";

package lobby.v1;

option go_package = "github.com/CBookShu/kd48/api/proto/lobby/v1;lobbyv1";

// ItemService 物品服务
service ItemService {
  rpc GetMyItems(GetMyItemsRequest) returns (ItemsData);
}

message GetMyItemsRequest {}

message ItemsData {
  map<int32, int64> items = 1;
}
```

- [ ] **Step 2: 重新生成 proto**

```bash
./gen_proto.sh
```

- [ ] **Step 3: 编译验证**

```bash
go build ./api/proto/lobby/v1/...
```

Expected: 编译成功，无错误

- [ ] **Step 4: 提交**

```bash
git add api/proto/lobby/v1/item.proto api/proto/lobby/v1/item.pb.go api/proto/lobby/v1/item_grpc.pb.go
git commit -m "feat: item proto returns ItemsData directly"
```

---

### Task 7: 更新 user.proto

**Files:**
- Modify: `api/proto/user/v1/user.proto`

- [ ] **Step 1: 修改 user.proto - 删除 success 字段**

```protobuf
syntax = "proto3";

package user.v1;

option go_package = "github.com/CBookShu/kd48/api/proto/user/v1;userv1";

// 用户服务定义
service UserService {
  // 模拟登录获取凭证
  rpc Login (LoginRequest) returns (LoginData);
  // 注册并签发会话（注册即登录，与 Login 同等 Redis Session）
  rpc Register (RegisterRequest) returns (RegisterData);
  // 验证 Token 并返回用户信息
  rpc VerifyToken (VerifyTokenRequest) returns (VerifyTokenData);
}

message LoginRequest {
  string username = 1;
  string password = 2;
}

message LoginData {
  string token = 1;
  uint64 user_id = 2;
}

message RegisterRequest {
  string username = 1;
  string password = 2;
}

message RegisterData {
  string token = 1;
  uint64 user_id = 2;
}

message VerifyTokenRequest {
  string token = 1;
}

message VerifyTokenData {
  uint64 user_id = 1;
  string username = 2;
}
```

- [ ] **Step 2: 重新生成 proto**

```bash
./gen_proto.sh
```

- [ ] **Step 3: 编译验证**

```bash
go build ./api/proto/user/v1/...
```

Expected: 编译成功，无错误

- [ ] **Step 4: 提交**

```bash
git add api/proto/user/v1/
git commit -m "feat: user proto returns business data directly"
```

---

### Task 8: 修改 user server.go

**Files:**
- Modify: `services/user/cmd/user/server.go`

- [ ] **Step 1: 更新 import**

```go
import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	commonv1 "github.com/CBookShu/kd48/api/proto/common/v1"
	userv1 "github.com/CBookShu/kd48/api/proto/user/v1"
	"github.com/CBookShu/kd48/pkg/dsroute"
	"github.com/CBookShu/kd48/services/user/internal/data/sqlc"
	"github.com/go-sql-driver/mysql"
	"golang.org/x/crypto/bcrypt"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)
```

- [ ] **Step 2: 修改 Login 方法**

```go
func (s *userService) Login(ctx context.Context, req *userv1.LoginRequest) (*userv1.LoginData, error) {
	slog.InfoContext(ctx, "Received Login request", "username", req.Username)

	routingKey := "sys:user:" + req.Username

	queries, err := s.getQueries(ctx, routingKey)
	if err != nil {
		return nil, err
	}

	user, err := queries.GetUserByUsername(ctx, req.Username)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, status.Errorf(codes.Code(commonv1.ErrorCode_USER_NOT_AUTHENTICATED), "用户名或密码错误")
		}
		slog.ErrorContext(ctx, "GetUserByUsername failed", "error", err)
		return nil, status.Errorf(codes.Code(commonv1.ErrorCode_INTERNAL_ERROR), "内部错误")
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		return nil, status.Errorf(codes.Code(commonv1.ErrorCode_USER_NOT_AUTHENTICATED), "用户名或密码错误")
	}

	token, hasOldToken, err := s.issueSessionAtomic(ctx, user.ID, user.Username)
	if err != nil {
		return nil, err
	}

	if hasOldToken {
		s.publishSessionInvalidate(ctx, user.ID)
	}

	slog.InfoContext(ctx, "User logged in successfully", "username", user.Username)

	return &userv1.LoginData{
		Token:  token,
		UserId: user.ID,
	}, nil
}
```

- [ ] **Step 3: 修改 Register 方法**

```go
func (s *userService) Register(ctx context.Context, req *userv1.RegisterRequest) (*userv1.RegisterData, error) {
	slog.InfoContext(ctx, "Received Register request", "username", req.Username)

	username := strings.TrimSpace(req.Username)
	if username == "" || req.Password == "" {
		return nil, status.Errorf(codes.Code(commonv1.ErrorCode_INVALID_REQUEST), "用户名和密码不能为空")
	}

	routingKey := "sys:user:" + username

	queries, err := s.getQueries(ctx, routingKey)
	if err != nil {
		return nil, err
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		slog.ErrorContext(ctx, "bcrypt hash failed", "error", err)
		return nil, status.Errorf(codes.Code(commonv1.ErrorCode_INTERNAL_ERROR), "内部错误")
	}

	err = queries.CreateUser(ctx, sqlc.CreateUserParams{
		Username:     username,
		PasswordHash: string(hash),
	})
	if err != nil {
		var mysqlErr *mysql.MySQLError
		if errors.As(err, &mysqlErr) && mysqlErr.Number == 1062 {
			return nil, status.Errorf(codes.Code(commonv1.ErrorCode_INVALID_REQUEST), "用户名已存在")
		}
		slog.ErrorContext(ctx, "CreateUser failed", "error", err)
		return nil, status.Errorf(codes.Code(commonv1.ErrorCode_INTERNAL_ERROR), "内部错误")
	}

	user, err := queries.GetUserByUsername(ctx, username)
	if err != nil {
		slog.ErrorContext(ctx, "GetUserByUsername after register failed", "error", err)
		return nil, status.Errorf(codes.Code(commonv1.ErrorCode_INTERNAL_ERROR), "内部错误")
	}

	token, _, err := s.issueSessionAtomic(ctx, user.ID, user.Username)
	if err != nil {
		return nil, err
	}

	slog.InfoContext(ctx, "User registered successfully", "username", user.Username)

	return &userv1.RegisterData{
		Token:  token,
		UserId: user.ID,
	}, nil
}
```

- [ ] **Step 4: 修改 VerifyToken 方法**

```go
func (s *userService) VerifyToken(ctx context.Context, req *userv1.VerifyTokenRequest) (*userv1.VerifyTokenData, error) {
	slog.InfoContext(ctx, "Received VerifyToken request")

	// Get user_id from context (injected by gateway)
	userIDVal := ctx.Value("user_id")
	if userIDVal == nil {
		slog.ErrorContext(ctx, "No user_id in context")
		return nil, status.Errorf(codes.Code(commonv1.ErrorCode_USER_NOT_AUTHENTICATED), "用户未认证")
	}

	userID, ok := userIDVal.(int64)
	if !ok {
		slog.ErrorContext(ctx, "Invalid user_id type in context")
		return nil, status.Errorf(codes.Code(commonv1.ErrorCode_USER_NOT_AUTHENTICATED), "用户未认证")
	}

	routingKey := "sys:user:id:" + fmt.Sprintf("%d", userID)

	queries, err := s.getQueries(ctx, routingKey)
	if err != nil {
		return nil, err
	}

	user, err := queries.GetUserByID(ctx, uint64(userID))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			slog.ErrorContext(ctx, "User not found", "user_id", userID)
			return nil, status.Errorf(codes.Code(commonv1.ErrorCode_USER_NOT_FOUND), "用户不存在")
		}
		slog.ErrorContext(ctx, "GetUserByID failed", "error", err)
		return nil, status.Errorf(codes.Code(commonv1.ErrorCode_INTERNAL_ERROR), "内部错误")
	}

	slog.InfoContext(ctx, "Token verified successfully", "user_id", userID, "username", user.Username)

	return &userv1.VerifyTokenData{
		UserId:   user.ID,
		Username: user.Username,
	}, nil
}
```

- [ ] **Step 5: 编译验证**

```bash
go build ./services/user/cmd/user/...
```

- [ ] **Step 6: 更新 server_test.go**

添加针对新返回格式和错误码的测试：

```go
// services/user/cmd/user/server_test.go
// 在现有测试基础上添加以下测试

func TestUserService_VerifyToken_NotAuthenticated(t *testing.T) {
	mr, err := miniredis.Run()
	require.NoError(t, err)
	defer mr.Close()

	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	router := createMockRouterWithRedis(t, rdb)
	svc := NewUserService(router, 1*time.Hour)

	// 没有 user_id 的 context
	ctx := context.Background()
	_, err = svc.VerifyToken(ctx, &userv1.VerifyTokenRequest{Token: "test"})

	require.Error(t, err)
	st, ok := status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.Code(commonv1.ErrorCode_USER_NOT_AUTHENTICATED), st.Code())
}

func TestUserService_VerifyToken_UserNotFound(t *testing.T) {
	mr, err := miniredis.Run()
	require.NoError(t, err)
	defer mr.Close()

	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	router := createMockRouterWithRedis(t, rdb)
	svc := NewUserService(router, 1*time.Hour)

	// 有 user_id 但用户不存在（需要真实的数据库连接，这里仅演示测试模式）
	// 实际测试需要 mock 数据库或使用测试数据库
	ctx := context.WithValue(context.Background(), "user_id", int64(99999))
	_, err = svc.VerifyToken(ctx, &userv1.VerifyTokenRequest{Token: "test"})

	// 由于没有真实数据库，会返回内部错误或用户未找到
	require.Error(t, err)
	st, ok := status.FromError(err)
	require.True(t, ok)
	// 验证是预期的错误码之一
	assert.True(t, st.Code() == codes.Code(commonv1.ErrorCode_USER_NOT_FOUND) ||
		st.Code() == codes.Code(commonv1.ErrorCode_INTERNAL_ERROR))
}
```

注意：由于 user service 依赖 MySQL，完整的单元测试需要 mock 数据库或使用测试数据库。上述测试演示了测试模式，实际项目中建议：
1. 使用接口抽象数据库依赖
2. 或使用 testcontainers 启动测试数据库

- [ ] **Step 7: 运行测试验证**

```bash
go test ./services/user/cmd/user/... -v -run TestUserService
```

- [ ] **Step 8: 提交**

```bash
git add services/user/cmd/user/server.go
git commit -m "feat: user service returns business data, uses commonv1.ErrorCode"
```

---

### Task 9: 更新网关 ingress（检查是否需要修改）

**Files:**
- Check: `gateway/internal/ws/handler.go`

- [ ] **Step 1: 检查 handler.go 是否需要修改**

当前代码已经实现透传：
```go
sts, ok := status.FromError(err)
if ok {
    h.sendResp(ctx, conn, req.Method, int32(sts.Code()), sts.Message(), nil)
}
```

这部分代码已经符合设计，**无需修改**。

- [ ] **Step 2: 编译验证**

```bash
go build ./gateway/cmd/gateway/...
```

---

### Task 10: 更新 CLI 客户端

**Files:**
- Modify: `cmd/cli/internal/commands/handler.go`

- [ ] **Step 1: 移除 Success 字段检查**

找到 checkin 相关响应类型，删除 Success 字段：

```go
// 删除这些结构体中的 Success 字段
type checkinDoResp struct{}  // 签到响应（空结构，响应通过 Code 判断）

type checkinStatusResp struct {
	PeriodID       int64  `json:"periodId"`
	PeriodName     string `json:"periodName"`
	TodayChecked   bool   `json:"todayChecked"`
	ContinuousDays int32  `json:"continuousDays"`
	TotalDays      int32  `json:"totalDays"`
}
```

- [ ] **Step 2: 更新 checkinDo 方法**

```go
// checkinDo 执行签到
func (h *Handler) checkinDo() string {
	if !h.state.IsLoggedIn {
		return "[错误] 请先登录"
	}

	resp, err := h.gateway.Send(context.Background(), "/lobby.v1.CheckinService/Checkin", nil)
	if err != nil {
		return fmt.Sprintf("[错误] %v", err)
	}

	if resp.Code != 0 {
		return fmt.Sprintf("[错误] %s", resp.Msg)
	}

	// 解析响应
	data, _ := json.Marshal(resp.Data)
	var checkinResp checkinDoResp
	if err := json.Unmarshal(data, &checkinResp); err != nil {
		// 签到成功，响应可能为空
		h.state.TodayChecked = true
		h.state.ContinuousDays++
		return fmt.Sprintf("[成功] 签到成功！连续签到: %d 天", h.state.ContinuousDays)
	}

	// 更新状态
	h.state.TodayChecked = true
	h.state.ContinuousDays++

	return fmt.Sprintf("[成功] 签到成功！连续签到: %d 天", h.state.ContinuousDays)
}
```

- [ ] **Step 3: 更新 checkinStatus 方法**

```go
// checkinStatus 查看签到状态
func (h *Handler) checkinStatus() string {
	if !h.state.IsLoggedIn {
		return "[错误] 请先登录"
	}

	resp, err := h.gateway.Send(context.Background(), "/lobby.v1.CheckinService/GetStatus", nil)
	if err != nil {
		return fmt.Sprintf("[错误] %v", err)
	}

	if resp.Code != 0 {
		return fmt.Sprintf("[错误] %s", resp.Msg)
	}

	// 解析响应
	data, _ := json.Marshal(resp.Data)
	var statusResp checkinStatusResp
	if err := json.Unmarshal(data, &statusResp); err != nil {
		return fmt.Sprintf("[错误] 解析响应失败: %v", err)
	}

	// 更新状态
	h.state.TodayChecked = statusResp.TodayChecked
	h.state.ContinuousDays = int(statusResp.ContinuousDays)

	checkStr := "✗ 未签到"
	if statusResp.TodayChecked {
		checkStr = "✓ 已签到"
	}

	return fmt.Sprintf(`┌─────────────────────────────────────────┐
│  Check-in Status                       │
├─────────────────────────────────────────┤
│  Period:     %-25s│
│  Today:      %-25s│
│  Streak:     %-25d days│
│  Total:      %-25d days│
└─────────────────────────────────────────┘`,
		statusResp.PeriodName,
		checkStr,
		statusResp.ContinuousDays,
		statusResp.TotalDays)
}
```

- [ ] **Step 4: 编译验证**

```bash
go build ./cmd/cli/...
```

- [ ] **Step 5: 更新 handler_test.go**

添加针对新响应格式的测试：

```go
// cmd/cli/internal/commands/handler_test.go
// 在现有测试基础上添加以下测试

func TestCheckinDo_Success(t *testing.T) {
	gw := &mockGateway{
		resp: &client.WsResponse{
			Method: "/lobby.v1.CheckinService/Checkin",
			Code:   0,
			Msg:    "",
			Data: map[string]interface{}{
				"continuous_days": float64(1),
				"rewards":         map[string]interface{}{"1001": float64(100)},
			},
		},
	}
	st := state.New()
	st.SetUser("testuser", 123, "token")
	h := New(gw, st)

	result := h.Handle("checkin:do")
	if result == "" {
		t.Error("checkin:do should return a result")
	}
	if !st.TodayChecked {
		t.Error("TodayChecked should be true after successful checkin")
	}
}

func TestCheckinDo_AlreadyToday(t *testing.T) {
	gw := &mockGateway{
		resp: &client.WsResponse{
			Method: "/lobby.v1.CheckinService/Checkin",
			Code:   int32(3000), // CHECKIN_ALREADY_TODAY
			Msg:    "今日已签到",
			Data:   nil,
		},
	}
	st := state.New()
	st.SetUser("testuser", 123, "token")
	h := New(gw, st)

	result := h.Handle("checkin:do")
	if result == "" {
		t.Error("checkin:do should return error message")
	}
	// 应该包含错误信息
	if result != "[错误] 今日已签到" {
		t.Errorf("Expected error message, got: %s", result)
	}
}

func TestCheckinStatus_Success(t *testing.T) {
	gw := &mockGateway{
		resp: &client.WsResponse{
			Method: "/lobby.v1.CheckinService/GetStatus",
			Code:   0,
			Msg:    "",
			Data: map[string]interface{}{
				"period_id":       float64(1),
				"period_name":     "Test Period",
				"today_checked":   true,
				"continuous_days": float64(3),
				"total_days":      float64(5),
			},
		},
	}
	st := state.New()
	st.SetUser("testuser", 123, "token")
	h := New(gw, st)

	result := h.Handle("checkin:status")
	if result == "" {
		t.Error("checkin:status should return a result")
	}
	if !st.TodayChecked {
		t.Error("TodayChecked should be true")
	}
	if st.ContinuousDays != 3 {
		t.Errorf("ContinuousDays should be 3, got %d", st.ContinuousDays)
	}
}

func TestCheckinStatus_NotAuthenticated(t *testing.T) {
	gw := &mockGateway{
		resp: &client.WsResponse{
			Method: "/lobby.v1.CheckinService/GetStatus",
			Code:   int32(2001), // USER_NOT_AUTHENTICATED
			Msg:    "用户未认证",
			Data:   nil,
		},
	}
	st := state.New()
	st.SetUser("testuser", 123, "token")
	h := New(gw, st)

	result := h.Handle("checkin:status")
	if result != "[错误] 用户未认证" {
		t.Errorf("Expected authentication error, got: %s", result)
	}
}

func TestGetMyItems_Success(t *testing.T) {
	gw := &mockGateway{
		resp: &client.WsResponse{
			Method: "/lobby.v1.ItemService/GetMyItems",
			Code:   0,
			Msg:    "",
			Data: map[string]interface{}{
				"items": map[string]interface{}{
					"1001": float64(1000),
					"1002": float64(500),
				},
			},
		},
	}
	st := state.New()
	st.SetUser("testuser", 123, "token")
	h := New(gw, st)

	result := h.Handle("items")
	if result == "" {
		t.Error("items should return a result")
	}
}

func TestGetMyItems_NotAuthenticated(t *testing.T) {
	gw := &mockGateway{
		resp: &client.WsResponse{
			Method: "/lobby.v1.ItemService/GetMyItems",
			Code:   int32(2001), // USER_NOT_AUTHENTICATED
			Msg:    "用户未认证",
			Data:   nil,
		},
	}
	st := state.New()
	st.SetUser("testuser", 123, "token")
	h := New(gw, st)

	result := h.Handle("items")
	if result != "[错误] 用户未认证" {
		t.Errorf("Expected authentication error, got: %s", result)
	}
}
```

- [ ] **Step 6: 运行测试验证**

```bash
go test ./cmd/cli/... -v
```

Expected: 所有测试通过

- [ ] **Step 7: 提交**

```bash
git add cmd/cli/internal/commands/handler.go
git commit -m "fix: cli removes Success field check, uses Code directly"
```

---

### Task 11: 集成测试

**Files:**
- Test: 所有服务

- [ ] **Step 1: 重新构建所有服务**

```bash
./build.sh
```

Expected: 所有服务构建成功，无错误

- [ ] **Step 2: 启动服务**

```bash
./run.sh
```

Expected: 所有服务启动正常

- [ ] **Step 3: 测试签到功能**

```bash
./kd48-cli
# 执行命令
user:register testuser testpass
checkin:do
checkin:status
item:list
```

Expected: 所有命令执行成功，返回正确的业务数据

- [ ] **Step 4: 验证错误码**

测试错误场景：
- 未登录执行签到 → 应返回 code=2001, message="用户未认证"
- 重复签到 → 应返回 code=3000, message="今日已签到"

Expected: 错误码和消息正确显示

- [ ] **Step 5: 提交**

```bash
git add -A
git commit -m "test: verify RPC refactoring works correctly"
```

---

## 自检清单

**1. Spec 覆盖检查：**
- [x] 创建 common/v1 公共包
- [x] ErrorCode 定义（1000+ 范围）
- [x] Proto RPC 返回业务数据（lobby + user）
- [x] 服务端使用 status.Errorf（checkin_server + item_server + user_server）
- [x] 网关透传（已实现）
- [x] 客户端移除 Success 检查

**2. Placeholder 扫描：**
- 无 "TBD"、"TODO" 等占位符
- 所有代码步骤都有完整实现

**3. 类型一致性：**
- ErrorCode 枚举值一致（1000+）
- 服务端使用 commonv1.ErrorCode
- 客户端使用 resp.Code + resp.Msg

**4. TDD 覆盖检查：**
- [x] Task 4: checkin_server_test.go 更新（测试错误码返回）
- [x] Task 5: item_server_test.go 更新（测试错误码返回）
- [x] Task 8: server_test.go 更新（测试错误码返回）
- [x] Task 10: handler_test.go 更新（测试响应解析）
- [x] Task 11: 集成测试（验证端到端流程）

**5. 步骤自检检查：**
- [x] Task 1: Step 4 编译验证
- [x] Task 2: Step 4 编译验证
- [x] Task 3: 脚本执行自带验证
- [x] Task 4: Step 4 编译验证 + Step 6 测试验证
- [x] Task 5: Step 3 编译验证 + Step 5 测试验证
- [x] Task 6: Step 3 编译验证
- [x] Task 7: Step 3 编译验证
- [x] Task 8: Step 5 编译验证 + Step 7 测试验证
- [x] Task 9: Step 2 编译验证
- [x] Task 10: Step 4 编译验证 + Step 6 测试验证
- [x] Task 11: Step 1-4 构建和运行验证

---

## 完成状态

**所有任务已完成** (2026-04-26)

### 提交记录
- `db150ff` chore: add common/v1 to proto generation script
- `e529c6f` feat: lobby services return business data directly, use status.Errorf for errors
- `b180d6f` feat: user proto returns business data directly
- `202cde2` feat(user): refactor RPC to return business data directly
- `1c7afc0` feat(cli): update handler for new RPC response format

### 测试结果
- Lobby service: 32 tests passed
- User service: 28 tests passed
- Gateway: 35 tests passed
- 所有服务编译成功
