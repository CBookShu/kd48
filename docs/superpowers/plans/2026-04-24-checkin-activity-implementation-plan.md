# 签到活动实现计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 实现签到活动功能，包括配置打表、签到服务、物品系统、观测环境和 Web 客户端演示。

**Architecture:** 扩展 Lobby 服务，新增 CheckinService 和 ItemService。配置通过 Config-Loader 打表，玩家数据使用 Redis 缓存 + MySQL 同步持久化。观测环境使用 Jaeger + Prometheus + Grafana。

**Tech Stack:** Go, gRPC, Protobuf, Redis, MySQL, OTel, Docker Compose, 原生 HTML/JS

---

## 文件结构

```
api/proto/lobby/v1/
├── common.proto             # 新增：ApiResponse 通用响应
├── checkin.proto            # 新增：签到服务 Proto
├── item.proto               # 新增：物品服务 Proto
└── lobby.proto              # 现有

services/lobby/
├── cmd/lobby/
│   ├── main.go              # 修改：注册新服务
│   ├── server.go            # 现有
│   ├── checkin_server.go    # 新增：CheckinService 实现
│   └── item_server.go       # 新增：ItemService 实现
├── internal/
│   ├── config/              # 现有
│   ├── checkin/
│   │   ├── store.go         # 新增：签到状态存储
│   │   ├── store_test.go    # 新增
│   │   ├── reward.go        # 新增：奖励发放逻辑
│   │   └── reward_test.go   # 新增
│   └── item/
│       ├── store.go         # 新增：物品存储
│       └── store_test.go    # 新增
└── migrations/
    └── 001_create_checkin_tables.sql  # 新增

tools/config-loader/testdata/
├── checkin_period.csv       # 新增
├── checkin_daily_reward.csv # 新增
├── checkin_continuous_reward.csv  # 新增
└── item_config.csv          # 新增

docker/
├── docker-compose.yml       # 修改：添加观测组件
├── prometheus.yml           # 新增
└── grafana/
    └── dashboards/
        └── lobby.json       # 新增

pkg/otelkit/
└── tracer.go                # 修改：支持 OTLP 导出

web/
├── index.html               # 新增
├── login.html               # 新增
├── register.html            # 新增
├── checkin.html             # 新增
├── items.html               # 新增
├── css/style.css            # 新增
└── js/
    ├── api.js               # 新增
    ├── ws.js                # 新增
    └── checkin.js           # 新增
```

---

## Task 1: Proto 定义与代码生成

**Files:**
- Create: `api/proto/lobby/v1/common.proto`
- Create: `api/proto/lobby/v1/checkin.proto`
- Create: `api/proto/lobby/v1/item.proto`
- Modify: `api/Makefile` (如需要)

- [ ] **Step 1: 创建 common.proto - 通用响应**

```protobuf
// api/proto/lobby/v1/common.proto
syntax = "proto3";

package lobby.v1;

option go_package = "github.com/CBookShu/kd48/api/proto/lobby/v1;lobbyv1";

import "google/protobuf/any.proto";

// ApiResponse 通用响应外壳
message ApiResponse {
  int32 code = 1;
  string message = 2;
  google.protobuf.Any data = 3;
  map<string, string> meta = 4;
}

// ErrorCode 错误码定义
enum ErrorCode {
  SUCCESS = 0;
  
  // 通用错误 1-99
  INVALID_REQUEST = 1;
  INTERNAL_ERROR = 2;
  SERVICE_UNAVAILABLE = 3;
  
  // 用户相关 100-199
  USER_NOT_FOUND = 100;
  USER_NOT_AUTHENTICATED = 101;
  
  // 签到相关 200-299
  CHECKIN_ALREADY_TODAY = 200;
  CHECKIN_PERIOD_NOT_ACTIVE = 201;
  CHECKIN_PERIOD_EXPIRED = 202;
  
  // 物品相关 300-399
  ITEM_NOT_FOUND = 300;
  ITEM_INSUFFICIENT = 301;
}
```

- [ ] **Step 2: 创建 item.proto - 物品服务**

```protobuf
// api/proto/lobby/v1/item.proto
syntax = "proto3";

package lobby.v1;

option go_package = "github.com/CBookShu/kd48/api/proto/lobby/v1;lobbyv1";

import "api/proto/lobby/v1/common.proto";

// ItemService 物品服务
service ItemService {
  rpc GetMyItems(GetMyItemsRequest) returns (ApiResponse);
}

message GetMyItemsRequest {}

message MyItemsData {
  map<int32, int64> items = 1;  // item_id -> count
}
```

- [ ] **Step 3: 创建 checkin.proto - 签到服务**

```protobuf
// api/proto/lobby/v1/checkin.proto
syntax = "proto3";

package lobby.v1;

option go_package = "github.com/CBookShu/kd48/api/proto/lobby/v1;lobbyv1";

import "api/proto/lobby/v1/common.proto";

// CheckinService 签到服务
service CheckinService {
  rpc Checkin(CheckinRequest) returns (ApiResponse);
  rpc GetStatus(GetStatusRequest) returns (ApiResponse);
}

message CheckinRequest {}

message CheckinData {
  int32 continuous_days = 1;
  map<int32, int64> rewards = 2;  // 本次获得的奖励
}

message GetStatusRequest {}

message CheckinStatusData {
  // 当前期配置
  int64 period_id = 1;
  string period_name = 2;
  repeated DailyRewardInfo daily_rewards = 3;
  repeated ContinuousRewardInfo continuous_rewards = 4;
  
  // 玩家状态
  bool today_checked = 10;
  int32 continuous_days = 11;
  int32 total_days = 12;
  repeated int32 claimed_continuous = 13;  // 已领取的连续奖励天数
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

- [ ] **Step 4: 生成 Go 代码**

Run: `./gen_proto.sh`

Expected: 生成 `api/proto/lobby/v1/*.pb.go` 文件

- [ ] **Step 5: 提交**

```bash
git add api/proto/lobby/v1/
git commit -m "feat(proto): add common, checkin, item proto definitions"
```

---

## Task 2: MySQL 迁移脚本

**Files:**
- Create: `services/lobby/migrations/001_create_checkin_tables.sql`

- [ ] **Step 1: 创建迁移脚本**

```sql
-- services/lobby/migrations/001_create_checkin_tables.sql

-- 签到期配置表
CREATE TABLE IF NOT EXISTS checkin_period (
    period_id BIGINT PRIMARY KEY,
    period_name VARCHAR(255) NOT NULL,
    start_time DATETIME NOT NULL,
    end_time DATETIME NOT NULL,
    status VARCHAR(32) NOT NULL DEFAULT 'active',
    INDEX idx_status_time (status, start_time, end_time)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- 每日签到奖励表
CREATE TABLE IF NOT EXISTS checkin_daily_reward (
    day INT PRIMARY KEY,
    rewards JSON NOT NULL,  -- {"1001": 100, "1002": 10}
    INDEX idx_day (day)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- 连续签到奖励表
CREATE TABLE IF NOT EXISTS checkin_continuous_reward (
    continuous_days INT PRIMARY KEY,
    rewards JSON NOT NULL,  -- {"2001": 1}
    INDEX idx_days (continuous_days)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- 物品配置表
CREATE TABLE IF NOT EXISTS item_config (
    item_id INT PRIMARY KEY,
    item_name VARCHAR(255) NOT NULL,
    item_type VARCHAR(64) NOT NULL,
    description VARCHAR(512),
    icon VARCHAR(255),
    INDEX idx_type (item_type)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- 玩家物品表
CREATE TABLE IF NOT EXISTS user_items (
    user_id BIGINT NOT NULL,
    item_id INT NOT NULL,
    count BIGINT NOT NULL DEFAULT 0,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    PRIMARY KEY (user_id, item_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- 玩家签到数据表
CREATE TABLE IF NOT EXISTS user_checkin (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    user_id BIGINT NOT NULL,
    period_id BIGINT NOT NULL,
    last_checkin_date DATE NOT NULL,
    continuous_days INT NOT NULL DEFAULT 0,
    claimed_days JSON,  -- [1,2,3,5,7]
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    UNIQUE KEY uk_user (user_id),
    INDEX idx_period (period_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
```

- [ ] **Step 2: 执行迁移**

Run: 
```bash
migrate -path ./services/lobby/migrations -database "mysql://root:root@tcp(localhost:3306)/kd48?parseTime=true&loc=Local" up
```

Expected: 创建成功

- [ ] **Step 3: 提交**

```bash
git add services/lobby/migrations/
git commit -m "feat(db): add checkin tables migration"
```

---

## Task 3: 配置数据 CSV 示例

**Files:**
- Create: `tools/config-loader/testdata/checkin_period.csv`
- Create: `tools/config-loader/testdata/checkin_daily_reward.csv`
- Create: `tools/config-loader/testdata/checkin_continuous_reward.csv`
- Create: `tools/config-loader/testdata/item_config.csv`

- [ ] **Step 1: 创建 checkin_period.csv**

```csv
期ID,期名称,开始时间,结束时间,状态
period_id,period_name,start_time,end_time,status
int64,string,time,time,string
1,日常签到,2026-01-01 00:00:00,2026-12-31 23:59:59,active
```

- [ ] **Step 2: 创建 checkin_daily_reward.csv**

```csv
天数,奖励物品映射
day,rewards
int32,map<int32,int64>
1,1001:100
2,1001:150
3,1001:200
4,1001:250
5,1001:300
6,1001:350
7,1001:500|2001:1
```

- [ ] **Step 3: 创建 checkin_continuous_reward.csv**

```csv
连续天数,奖励物品映射
continuous_days,rewards
int32,map<int32,int64>
7,2001:1
14,2001:2
30,2002:1
```

- [ ] **Step 4: 创建 item_config.csv**

```csv
物品ID,物品名称,物品类型,描述,图标
item_id,item_name,item_type,description,icon
int32,string,string,string,string
1001,金币,currency,游戏金币,icon_coin
1002,钻石,currency,充值钻石,icon_diamond
2001,礼盒,item,普通礼盒,icon_gift_box
2002,传说礼盒,item,传说礼盒,icon_legendary
```

- [ ] **Step 5: 提交**

```bash
git add tools/config-loader/testdata/
git commit -m "feat(config): add checkin config csv files"
```

---

## Task 4: 物品存储 Store

**Files:**
- Create: `services/lobby/internal/item/store.go`
- Create: `services/lobby/internal/item/store_test.go`

- [ ] **Step 1: 写失败测试**

```go
// services/lobby/internal/item/store_test.go
package item

import (
	"context"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
)

func TestItemStore_GetItems(t *testing.T) {
	mr, err := miniredis.Run()
	assert.NoError(t, err)
	defer mr.Close()

	rdb := redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})

	store := NewItemStore(rdb)
	ctx := context.Background()

	// 空用户返回空 map
	items, err := store.GetItems(ctx, 12345)
	assert.NoError(t, err)
	assert.Empty(t, items)
}

func TestItemStore_AddItem(t *testing.T) {
	mr, err := miniredis.Run()
	assert.NoError(t, err)
	defer mr.Close()

	rdb := redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})

	store := NewItemStore(rdb)
	ctx := context.Background()

	// 添加物品
	err = store.AddItem(ctx, 12345, 1001, 100)
	assert.NoError(t, err)

	// 验证
	items, err := store.GetItems(ctx, 12345)
	assert.NoError(t, err)
	assert.Equal(t, int64(100), items[1001])
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `go test ./services/lobby/internal/item/... -v`

Expected: FAIL (store.go 不存在)

- [ ] **Step 3: 实现 ItemStore**

```go
// services/lobby/internal/item/store.go
package item

import (
	"context"
	"fmt"

	"github.com/redis/go-redis/v9"
)

// ItemStore 物品存储
type ItemStore struct {
	rdb *redis.Client
}

// NewItemStore 创建物品存储
func NewItemStore(rdb *redis.Client) *ItemStore {
	return &ItemStore{rdb: rdb}
}

// redisKey 生成 Redis key
func redisKey(userID int64) string {
	return fmt.Sprintf("kd48:user_items:%d", userID)
}

// GetItems 获取用户所有物品
func (s *ItemStore) GetItems(ctx context.Context, userID int64) (map[int32]int64, error) {
	key := redisKey(userID)
	result, err := s.rdb.HGetAll(ctx, key).Result()
	if err != nil {
		return nil, fmt.Errorf("redis hgetall: %w", err)
	}

	items := make(map[int32]int64)
	for itemIDStr, countStr := range result {
		var itemID int32
		var count int64
		fmt.Sscanf(itemIDStr, "%d", &itemID)
		fmt.Sscanf(countStr, "%d", &count)
		items[itemID] = count
	}

	return items, nil
}

// AddItem 添加物品（返回新数量）
func (s *ItemStore) AddItem(ctx context.Context, userID int64, itemID int32, count int64) error {
	key := redisKey(userID)
	field := fmt.Sprintf("%d", itemID)
	
	_, err := s.rdb.HIncrBy(ctx, key, field, count).Result()
	if err != nil {
		return fmt.Errorf("redis hincrby: %w", err)
	}

	return nil
}

// SetItem 设置物品数量
func (s *ItemStore) SetItem(ctx context.Context, userID int64, itemID int32, count int64) error {
	key := redisKey(userID)
	field := fmt.Sprintf("%d", itemID)
	
	_, err := s.rdb.HSet(ctx, key, field, count).Result()
	if err != nil {
		return fmt.Errorf("redis hset: %w", err)
	}

	return nil
}
```

- [ ] **Step 4: 运行测试确认通过**

Run: `go test ./services/lobby/internal/item/... -v`

Expected: PASS

- [ ] **Step 5: 提交**

```bash
git add services/lobby/internal/item/
git commit -m "feat(item): add item store with redis hash"
```

---

## Task 5: 签到存储 Store

**Files:**
- Create: `services/lobby/internal/checkin/store.go`
- Create: `services/lobby/internal/checkin/store_test.go`

- [ ] **Step 1: 写失败测试**

```go
// services/lobby/internal/checkin/store_test.go
package checkin

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
)

func TestCheckinStore_GetStatus(t *testing.T) {
	mr, err := miniredis.Run()
	assert.NoError(t, err)
	defer mr.Close()

	rdb := redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})

	store := NewCheckinStore(rdb)
	ctx := context.Background()

	// 新用户返回默认状态
	status, err := store.GetStatus(ctx, 12345)
	assert.NoError(t, err)
	assert.Equal(t, int64(0), status.PeriodID)
	assert.Equal(t, 0, status.ContinuousDays)
}

func TestCheckinStore_UpdateStatus(t *testing.T) {
	mr, err := miniredis.Run()
	assert.NoError(t, err)
	defer mr.Close()

	rdb := redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})

	store := NewCheckinStore(rdb)
	ctx := context.Background()

	// 更新状态
	status := &UserCheckinStatus{
		PeriodID:         1,
		LastCheckinDate:  time.Now().Format("2006-01-02"),
		ContinuousDays:   5,
		ClaimedDays:      []int{1, 2, 3, 4, 5},
	}

	err = store.UpdateStatus(ctx, 12345, status)
	assert.NoError(t, err)

	// 验证
	got, err := store.GetStatus(ctx, 12345)
	assert.NoError(t, err)
	assert.Equal(t, int64(1), got.PeriodID)
	assert.Equal(t, 5, got.ContinuousDays)
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `go test ./services/lobby/internal/checkin/... -v`

Expected: FAIL

- [ ] **Step 3: 实现 CheckinStore**

```go
// services/lobby/internal/checkin/store.go
package checkin

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// UserCheckinStatus 用户签到状态
type UserCheckinStatus struct {
	PeriodID         int64  `json:"period_id"`
	LastCheckinDate  string `json:"last_checkin_date"`
	ContinuousDays   int    `json:"continuous_days"`
	ClaimedDays      []int  `json:"claimed_days"`
}

// CheckinStore 签到存储
type CheckinStore struct {
	rdb *redis.Client
}

// NewCheckinStore 创建签到存储
func NewCheckinStore(rdb *redis.Client) *CheckinStore {
	return &CheckinStore{rdb: rdb}
}

// redisKey 生成 Redis key
func redisKey(userID int64) string {
	return fmt.Sprintf("kd48:checkin:%d", userID)
}

// GetStatus 获取用户签到状态
func (s *CheckinStore) GetStatus(ctx context.Context, userID int64) (*UserCheckinStatus, error) {
	key := redisKey(userID)
	data, err := s.rdb.Get(ctx, key).Result()
	if err == redis.Nil {
		// 新用户返回默认状态
		return &UserCheckinStatus{
			PeriodID:        0,
			LastCheckinDate: "",
			ContinuousDays:  0,
			ClaimedDays:     []int{},
		}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("redis get: %w", err)
	}

	var status UserCheckinStatus
	if err := json.Unmarshal([]byte(data), &status); err != nil {
		return nil, fmt.Errorf("json unmarshal: %w", err)
	}

	return &status, nil
}

// UpdateStatus 更新用户签到状态
func (s *CheckinStore) UpdateStatus(ctx context.Context, userID int64, status *UserCheckinStatus) error {
	key := redisKey(userID)
	data, err := json.Marshal(status)
	if err != nil {
		return fmt.Errorf("json marshal: %w", err)
	}

	// 设置 7 天过期
	_, err = s.rdb.Set(ctx, key, data, 7*24*time.Hour).Result()
	if err != nil {
		return fmt.Errorf("redis set: %w", err)
	}

	return nil
}

// ResetStatus 重置签到状态（新期开始）
func (s *CheckinStore) ResetStatus(ctx context.Context, userID int64, periodID int64) error {
	status := &UserCheckinStatus{
		PeriodID:        periodID,
		LastCheckinDate: "",
		ContinuousDays:  0,
		ClaimedDays:     []int{},
	}
	return s.UpdateStatus(ctx, userID, status)
}
```

- [ ] **Step 4: 运行测试确认通过**

Run: `go test ./services/lobby/internal/checkin/... -v`

Expected: PASS

- [ ] **Step 5: 提交**

```bash
git add services/lobby/internal/checkin/
git commit -m "feat(checkin): add checkin store with redis"
```

---

## Task 6: 奖励发放逻辑

**Files:**
- Create: `services/lobby/internal/checkin/reward.go`
- Create: `services/lobby/internal/checkin/reward_test.go`

- [ ] **Step 1: 写失败测试**

```go
// services/lobby/internal/checkin/reward_test.go
package checkin

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRewardCalculator_GetDailyReward(t *testing.T) {
	calculator := NewRewardCalculator()

	// 添加测试配置
	calculator.SetDailyRewards(map[int]map[int32]int64{
		1: {1001: 100},
		7: {1001: 500, 2001: 1},
	})

	rewards := calculator.GetDailyReward(1)
	assert.Equal(t, int64(100), rewards[1001])

	rewards = calculator.GetDailyReward(7)
	assert.Equal(t, int64(500), rewards[1001])
	assert.Equal(t, int64(1), rewards[2001])
}

func TestRewardCalculator_GetContinuousReward(t *testing.T) {
	calculator := NewRewardCalculator()

	calculator.SetContinuousRewards(map[int]map[int32]int64{
		7:  {2001: 1},
		30: {2002: 1},
	})

	rewards := calculator.GetContinuousReward(7)
	assert.Equal(t, int64(1), rewards[2001])

	rewards = calculator.GetContinuousReward(30)
	assert.Equal(t, int64(1), rewards[2002])

	// 不存在的天数返回空
	rewards = calculator.GetContinuousReward(5)
	assert.Empty(t, rewards)
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `go test ./services/lobby/internal/checkin/... -v`

Expected: FAIL

- [ ] **Step 3: 实现 RewardCalculator**

```go
// services/lobby/internal/checkin/reward.go
package checkin

// RewardCalculator 奖励计算器
type RewardCalculator struct {
	dailyRewards      map[int]map[int32]int64      // day -> item_id -> count
	continuousRewards map[int]map[int32]int64      // continuous_days -> item_id -> count
}

// NewRewardCalculator 创建奖励计算器
func NewRewardCalculator() *RewardCalculator {
	return &RewardCalculator{
		dailyRewards:      make(map[int]map[int32]int64),
		continuousRewards: make(map[int]map[int32]int64),
	}
}

// SetDailyRewards 设置每日奖励配置
func (c *RewardCalculator) SetDailyRewards(rewards map[int]map[int32]int64) {
	c.dailyRewards = rewards
}

// SetContinuousRewards 设置连续奖励配置
func (c *RewardCalculator) SetContinuousRewards(rewards map[int]map[int32]int64) {
	c.continuousRewards = rewards
}

// GetDailyReward 获取指定天数的每日奖励
func (c *RewardCalculator) GetDailyReward(day int) map[int32]int64 {
	if rewards, ok := c.dailyRewards[day]; ok {
		return rewards
	}
	return map[int32]int64{}
}

// GetContinuousReward 获取指定连续天数的奖励
func (c *RewardCalculator) GetContinuousReward(continuousDays int) map[int32]int64 {
	if rewards, ok := c.continuousRewards[continuousDays]; ok {
		return rewards
	}
	return map[int32]int64{}
}

// HasContinuousReward 检查是否有连续奖励
func (c *RewardCalculator) HasContinuousReward(continuousDays int) bool {
	_, ok := c.continuousRewards[continuousDays]
	return ok
}

// MergeRewards 合并两个奖励 map
func MergeRewards(base, add map[int32]int64) map[int32]int64 {
	result := make(map[int32]int64)
	for k, v := range base {
		result[k] = v
	}
	for k, v := range add {
		result[k] += v
	}
	return result
}
```

- [ ] **Step 4: 运行测试确认通过**

Run: `go test ./services/lobby/internal/checkin/... -v`

Expected: PASS

- [ ] **Step 5: 提交**

```bash
git add services/lobby/internal/checkin/reward.go services/lobby/internal/checkin/reward_test.go
git commit -m "feat(checkin): add reward calculator"
```

---

## Task 7: ItemService 实现

**Files:**
- Create: `services/lobby/cmd/lobby/item_server.go`
- Modify: `services/lobby/cmd/lobby/main.go`

- [ ] **Step 1: 写失败测试**

```go
// services/lobby/cmd/lobby/item_server_test.go
package main

import (
	"context"
	"testing"

	"github.com/alicebob/miniredis/v2"
	lobbyv1 "github.com/CBookShu/kd48/api/proto/lobby/v1"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"google.golang.org/protobuf/types/known/anypb"
)

func TestItemService_GetMyItems(t *testing.T) {
	mr, err := miniredis.Run()
	assert.NoError(t, err)
	defer mr.Close()

	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	svc := NewItemService(rdb)
	ctx := context.Background()

	// 设置测试数据
	rdb.HSet(ctx, "kd48:user_items:12345", "1001", "1000", "1002", "500")

	// 调用
	resp, err := svc.GetMyItems(ctx, &lobbyv1.GetMyItemsRequest{})
	assert.NoError(t, err)
	assert.Equal(t, int32(0), resp.Code)

	// 解析 data
	var data lobbyv1.MyItemsData
	err = resp.Data.UnmarshalTo(&data)
	assert.NoError(t, err)
	assert.Equal(t, int64(1000), data.Items[1001])
	assert.Equal(t, int64(500), data.Items[1002])
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `go test ./services/lobby/cmd/lobby/... -v`

Expected: FAIL

- [ ] **Step 3: 实现 ItemService**

```go
// services/lobby/cmd/lobby/item_server.go
package main

import (
	"context"
	"encoding/json"

	lobbyv1 "github.com/CBookShu/kd48/api/proto/lobby/v1"
	"github.com/CBookShu/kd48/services/lobby/internal/item"
	"github.com/redis/go-redis/v9"
	"google.golang.org/protobuf/types/known/anypb"
)

// ItemService 物品服务
type ItemService struct {
	lobbyv1.UnimplementedItemServiceServer
	store *item.ItemStore
}

// NewItemService 创建物品服务
func NewItemService(rdb *redis.Client) *ItemService {
	return &ItemService{
		store: item.NewItemStore(rdb),
	}
}

// GetMyItems 获取我的物品
func (s *ItemService) GetMyItems(ctx context.Context, req *lobbyv1.GetMyItemsRequest) (*lobbyv1.ApiResponse, error) {
	// 从 context 获取 user_id（由 Gateway 注入）
	userID, ok := ctx.Value("user_id").(int64)
	if !ok {
		return errorResponse(lobbyv1.ErrorCode_USER_NOT_AUTHENTICATED, "user not authenticated"), nil
	}

	items, err := s.store.GetItems(ctx, userID)
	if err != nil {
		return errorResponse(lobbyv1.ErrorCode_INTERNAL_ERROR, "failed to get items"), nil
	}

	data := &lobbyv1.MyItemsData{
		Items: items,
	}

	return successResponse(data)
}

// successResponse 构造成功响应
func successResponse(data protobuf.Message) (*lobbyv1.ApiResponse, error) {
	anyData, err := anypb.New(data)
	if err != nil {
		return errorResponse(lobbyv1.ErrorCode_INTERNAL_ERROR, "failed to marshal response"), nil
	}

	return &lobbyv1.ApiResponse{
		Code:    int32(lobbyv1.ErrorCode_SUCCESS),
		Message: "success",
		Data:    anyData,
	}, nil
}

// errorResponse 构造错误响应
func errorResponse(code lobbyv1.ErrorCode, message string) *lobbyv1.ApiResponse {
	return &lobbyv1.ApiResponse{
		Code:    int32(code),
		Message: message,
	}
}
```

- [ ] **Step 4: 运行测试确认通过**

Run: `go test ./services/lobby/cmd/lobby/... -v`

Expected: PASS

- [ ] **Step 5: 在 main.go 注册服务**

修改 `services/lobby/cmd/lobby/main.go`，在 gRPC server 注册部分添加：

```go
// 在 lobbyv1.RegisterLobbyServiceServer 之后添加
itemSvc := NewItemService(redisPools["default"])
lobbyv1.RegisterItemServiceServer(s, itemSvc)
```

- [ ] **Step 6: 验证编译通过**

Run: `go build ./services/lobby/...`

Expected: 成功

- [ ] **Step 7: 提交**

```bash
git add services/lobby/cmd/lobby/
git commit -m "feat(item): implement ItemService with GetMyItems"
```

---

## Task 8: CheckinService 实现

**Files:**
- Create: `services/lobby/cmd/lobby/checkin_server.go`
- Modify: `services/lobby/cmd/lobby/main.go`

- [ ] **Step 1: 写失败测试**

```go
// services/lobby/cmd/lobby/checkin_server_test.go
package main

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	lobbyv1 "github.com/CBookShu/kd48/api/proto/lobby/v1"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
)

func TestCheckinService_Checkin(t *testing.T) {
	mr, err := miniredis.Run()
	assert.NoError(t, err)
	defer mr.Close()

	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	svc := NewCheckinService(rdb)
	
	// 设置测试配置
	svc.SetDailyRewards(map[int]map[int32]int64{1: {1001: 100}})
	
	ctx := context.WithValue(context.Background(), "user_id", int64(12345))

	// 第一次签到
	resp, err := svc.Checkin(ctx, &lobbyv1.CheckinRequest{})
	assert.NoError(t, err)
	assert.Equal(t, int32(0), resp.Code)

	var data lobbyv1.CheckinData
	err = resp.Data.UnmarshalTo(&data)
	assert.NoError(t, err)
	assert.Equal(t, int32(1), data.ContinuousDays)
	assert.Equal(t, int64(100), data.Rewards[1001])

	// 重复签到
	resp, err = svc.Checkin(ctx, &lobbyv1.CheckinRequest{})
	assert.NoError(t, err)
	assert.Equal(t, int32(lobbyv1.ErrorCode_CHECKIN_ALREADY_TODAY), resp.Code)
}

func TestCheckinService_GetStatus(t *testing.T) {
	mr, err := miniredis.Run()
	assert.NoError(t, err)
	defer mr.Close()

	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	svc := NewCheckinService(rdb)
	svc.SetPeriod(&PeriodConfig{PeriodID: 1, PeriodName: "test"})
	svc.SetDailyRewards(map[int]map[int32]int64{1: {1001: 100}})

	ctx := context.WithValue(context.Background(), "user_id", int64(12345))

	resp, err := svc.GetStatus(ctx, &lobbyv1.GetStatusRequest{})
	assert.NoError(t, err)
	assert.Equal(t, int32(0), resp.Code)

	var data lobbyv1.CheckinStatusData
	err = resp.Data.UnmarshalTo(&data)
	assert.NoError(t, err)
	assert.Equal(t, int64(1), data.PeriodId)
	assert.False(t, data.TodayChecked)
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `go test ./services/lobby/cmd/lobby/... -v`

Expected: FAIL

- [ ] **Step 3: 实现 CheckinService**

```go
// services/lobby/cmd/lobby/checkin_server.go
package main

import (
	"context"
	"time"

	lobbyv1 "github.com/CBookShu/kd48/api/proto/lobby/v1"
	"github.com/CBookShu/kd48/services/lobby/internal/checkin"
	"github.com/CBookShu/kd48/services/lobby/internal/item"
	"github.com/redis/go-redis/v9"
	"google.golang.org/protobuf/types/known/anypb"
)

// PeriodConfig 期配置
type PeriodConfig struct {
	PeriodID   int64
	PeriodName string
}

// CheckinService 签到服务
type CheckinService struct {
	lobbyv1.UnimplementedCheckinServiceServer
	checkinStore *checkin.CheckinStore
	itemStore    *item.ItemStore
	calculator   *checkin.RewardCalculator
	period       *PeriodConfig
}

// NewCheckinService 创建签到服务
func NewCheckinService(rdb *redis.Client) *CheckinService {
	return &CheckinService{
		checkinStore: checkin.NewCheckinStore(rdb),
		itemStore:    item.NewItemStore(rdb),
		calculator:   checkin.NewRewardCalculator(),
	}
}

// SetPeriod 设置当前期配置
func (s *CheckinService) SetPeriod(p *PeriodConfig) {
	s.period = p
}

// SetDailyRewards 设置每日奖励配置
func (s *CheckinService) SetDailyRewards(rewards map[int]map[int32]int64) {
	s.calculator.SetDailyRewards(rewards)
}

// SetContinuousRewards 设置连续奖励配置
func (s *CheckinService) SetContinuousRewards(rewards map[int]map[int32]int64) {
	s.calculator.SetContinuousRewards(rewards)
}

// Checkin 签到
func (s *CheckinService) Checkin(ctx context.Context, req *lobbyv1.CheckinRequest) (*lobbyv1.ApiResponse, error) {
	userID, ok := ctx.Value("user_id").(int64)
	if !ok {
		return errorResponse(lobbyv1.ErrorCode_USER_NOT_AUTHENTICATED, "user not authenticated"), nil
	}

	if s.period == nil {
		return errorResponse(lobbyv1.ErrorCode_CHECKIN_PERIOD_NOT_ACTIVE, "no active period"), nil
	}

	// 获取用户签到状态
	status, err := s.checkinStore.GetStatus(ctx, userID)
	if err != nil {
		return errorResponse(lobbyv1.ErrorCode_INTERNAL_ERROR, "failed to get status"), nil
	}

	// 检查是否需要重置（新期开始）
	if status.PeriodID != s.period.PeriodID {
		status = &checkin.UserCheckinStatus{
			PeriodID:    s.period.PeriodID,
			ClaimedDays: []int{},
		}
	}

	// 检查今日是否已签到
	today := time.Now().Format("2006-01-02")
	if status.LastCheckinDate == today {
		return errorResponse(lobbyv1.ErrorCode_CHECKIN_ALREADY_TODAY, "already checked in today"), nil
	}

	// 计算连续天数
	yesterday := time.Now().AddDate(0, 0, -1).Format("2006-01-02")
	if status.LastCheckinDate == yesterday {
		status.ContinuousDays++
	} else {
		status.ContinuousDays = 1
	}

	// 计算奖励
	day := len(status.ClaimedDays) + 1
	rewards := s.calculator.GetDailyReward(day)

	// 检查连续奖励
	if s.calculator.HasContinuousReward(status.ContinuousDays) {
		continuousReward := s.calculator.GetContinuousReward(status.ContinuousDays)
		rewards = checkin.MergeRewards(rewards, continuousReward)
	}

	// 发放奖励
	for itemID, count := range rewards {
		if err := s.itemStore.AddItem(ctx, userID, itemID, count); err != nil {
			return errorResponse(lobbyv1.ErrorCode_INTERNAL_ERROR, "failed to add items"), nil
		}
	}

	// 更新签到状态
	status.LastCheckinDate = today
	status.ClaimedDays = append(status.ClaimedDays, day)
	if err := s.checkinStore.UpdateStatus(ctx, userID, status); err != nil {
		return errorResponse(lobbyv1.ErrorCode_INTERNAL_ERROR, "failed to update status"), nil
	}

	data := &lobbyv1.CheckinData{
		ContinuousDays: int32(status.ContinuousDays),
		Rewards:        rewards,
	}

	return successResponse(data)
}

// GetStatus 获取签到状态
func (s *CheckinService) GetStatus(ctx context.Context, req *lobbyv1.GetStatusRequest) (*lobbyv1.ApiResponse, error) {
	userID, ok := ctx.Value("user_id").(int64)
	if !ok {
		return errorResponse(lobbyv1.ErrorCode_USER_NOT_AUTHENTICATED, "user not authenticated"), nil
	}

	if s.period == nil {
		return errorResponse(lobbyv1.ErrorCode_CHECKIN_PERIOD_NOT_ACTIVE, "no active period"), nil
	}

	// 获取用户签到状态
	status, err := s.checkinStore.GetStatus(ctx, userID)
	if err != nil {
		return errorResponse(lobbyv1.ErrorCode_INTERNAL_ERROR, "failed to get status"), nil
	}

	// 检查是否需要重置
	if status.PeriodID != s.period.PeriodID {
		status = &checkin.UserCheckinStatus{
			PeriodID:    s.period.PeriodID,
			ClaimedDays: []int{},
		}
	}

	// 构建响应
	today := time.Now().Format("2006-01-02")
	data := &lobbyv1.CheckinStatusData{
		PeriodId:         s.period.PeriodID,
		PeriodName:       s.period.PeriodName,
		TodayChecked:     status.LastCheckinDate == today,
		ContinuousDays:   int32(status.ContinuousDays),
		TotalDays:        int32(len(status.ClaimedDays)),
		ClaimedContinuous: getClaimedContinuous(status.ClaimedDays, s.calculator),
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

	return successResponse(data)
}

// getClaimedContinuous 获取已领取的连续奖励天数
func getClaimedContinuous(claimedDays []int, calc *checkin.RewardCalculator) []int32 {
	var result []int32
	for _, day := range claimedDays {
		if calc.HasContinuousReward(day) {
			result = append(result, int32(day))
		}
	}
	return result
}
```

- [ ] **Step 4: 补充 RewardCalculator 方法**

在 `services/lobby/internal/checkin/reward.go` 添加：

```go
// GetAllContinuousRewards 获取所有连续奖励配置
func (c *RewardCalculator) GetAllContinuousRewards() map[int]map[int32]int64 {
	return c.continuousRewards
}
```

- [ ] **Step 5: 运行测试确认通过**

Run: `go test ./services/lobby/cmd/lobby/... -v`

Expected: PASS

- [ ] **Step 6: 在 main.go 注册服务**

修改 `services/lobby/cmd/lobby/main.go`，添加：

```go
// 在 itemSvc 注册之后添加
checkinSvc := NewCheckinService(redisPools["default"])
lobbyv1.RegisterCheckinServiceServer(s, checkinSvc)
```

- [ ] **Step 7: 验证编译通过**

Run: `go build ./services/lobby/...`

Expected: 成功

- [ ] **Step 8: 提交**

```bash
git add services/lobby/
git commit -m "feat(checkin): implement CheckinService with Checkin and GetStatus"
```

---

## Task 9: 观测环境 - Docker Compose

**Files:**
- Modify: `docker-compose.yml`
- Create: `docker/prometheus.yml`

- [ ] **Step 1: 修改 docker-compose.yml 添加观测组件**

在现有 `docker-compose.yml` 末尾添加：

```yaml
  # 观测组件
  jaeger:
    image: jaegertracing/all-in-one:1.58
    environment:
      - COLLECTOR_OTLP_ENABLED=true
    ports:
      - "16686:16686"   # Jaeger UI
      - "4318:4318"     # OTLP HTTP
      - "4317:4317"     # OTLP gRPC

  prometheus:
    image: prom/prometheus:v2.51.0
    ports:
      - "9090:9090"
    volumes:
      - ./docker/prometheus.yml:/etc/prometheus/prometheus.yml
      - prometheus_data:/prometheus
    command:
      - '--config.file=/etc/prometheus/prometheus.yml'
      - '--storage.tsdb.path=/prometheus'

  grafana:
    image: grafana/grafana:10.4.0
    ports:
      - "3000:3000"
    environment:
      - GF_SECURITY_ADMIN_PASSWORD=admin
    volumes:
      - grafana_data:/var/lib/grafana
    depends_on:
      - prometheus

volumes:
  # 在现有 volumes 部分添加
  prometheus_data:
  grafana_data:
```

- [ ] **Step 2: 创建 prometheus.yml**

```yaml
# docker/prometheus.yml
global:
  scrape_interval: 15s

scrape_configs:
  - job_name: 'prometheus'
    static_configs:
      - targets: ['localhost:9090']

  - job_name: 'gateway'
    static_configs:
      - targets: ['host.docker.internal:8080']

  - job_name: 'lobby'
    static_configs:
      - targets: ['host.docker.internal:9001']

  - job_name: 'user'
    static_configs:
      - targets: ['host.docker.internal:9000']
```

- [ ] **Step 3: 验证配置**

Run: `docker-compose config`

Expected: 配置解析成功

- [ ] **Step 4: 提交**

```bash
git add docker-compose.yml docker/
git commit -m "feat(observability): add jaeger, prometheus, grafana to docker-compose"
```

---

## Task 10: OTel OTLP 导出

**Files:**
- Modify: `pkg/otelkit/tracer.go`

- [ ] **Step 1: 检查现有 otelkit 实现**

Run: `cat pkg/otelkit/tracer.go`

- [ ] **Step 2: 修改支持 OTLP 导出**

```go
// pkg/otelkit/tracer.go
package otelkit

import (
	"context"
	"os"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	"go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.24.0"
)

// InitTracer 初始化 Tracer，支持 OTLP 导出
func InitTracer(serviceName string) (func(context.Context) error, error) {
	// 检查是否配置了 OTLP endpoint
	otlpEndpoint := os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")
	
	var exporter trace.SpanExporter
	var err error

	if otlpEndpoint != "" {
		// 使用 OTLP HTTP 导出
		exporter, err = otlptracehttp.New(context.Background(),
			otlptracehttp.WithEndpoint(otlpEndpoint),
			otlptracehttp.WithInsecure(),
		)
	} else {
		// 默认使用 stdout（开发环境）
		exporter, err = NewStdoutExporter()
	}

	if err != nil {
		return nil, err
	}

	res, err := resource.Merge(
		resource.Default(),
		resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceName(serviceName),
		),
	)
	if err != nil {
		return nil, err
	}

	tp := trace.NewTracerProvider(
		trace.WithBatcher(exporter),
		trace.WithResource(res),
	)

	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	return tp.Shutdown, nil
}

// NewStdoutExporter 创建 stdout 导出器（用于开发）
func NewStdoutExporter() (trace.SpanExporter, error) {
	// 简化版本，直接返回 noop
	return trace.NewNoopTracerProvider(), nil
}
```

- [ ] **Step 3: 验证编译**

Run: `go build ./pkg/otelkit/...`

Expected: 成功

- [ ] **Step 4: 提交**

```bash
git add pkg/otelkit/
git commit -m "feat(otelkit): support OTLP exporter for jaeger"
```

---

## Task 11: Web 客户端 - 基础结构

**Files:**
- Create: `web/index.html`
- Create: `web/login.html`
- Create: `web/register.html`
- Create: `web/css/style.css`
- Create: `web/js/api.js`

- [ ] **Step 1: 创建 style.css**

```css
/* web/css/style.css */
* {
  margin: 0;
  padding: 0;
  box-sizing: border-box;
}

body {
  font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
  background: #f5f5f5;
  min-height: 100vh;
}

.container {
  max-width: 800px;
  margin: 0 auto;
  padding: 20px;
}

.card {
  background: white;
  border-radius: 8px;
  padding: 20px;
  margin-bottom: 20px;
  box-shadow: 0 2px 4px rgba(0,0,0,0.1);
}

.btn {
  display: inline-block;
  padding: 10px 20px;
  background: #007bff;
  color: white;
  border: none;
  border-radius: 4px;
  cursor: pointer;
  font-size: 16px;
}

.btn:hover {
  background: #0056b3;
}

.btn:disabled {
  background: #ccc;
  cursor: not-allowed;
}

input {
  width: 100%;
  padding: 10px;
  border: 1px solid #ddd;
  border-radius: 4px;
  margin-bottom: 10px;
  font-size: 16px;
}

.nav {
  background: #333;
  padding: 10px 20px;
  margin-bottom: 20px;
}

.nav a {
  color: white;
  text-decoration: none;
  margin-right: 20px;
}

.nav a:hover {
  text-decoration: underline;
}

.reward-item {
  display: inline-block;
  background: #e8f5e9;
  padding: 5px 10px;
  border-radius: 4px;
  margin: 5px;
}

.day-box {
  display: inline-block;
  width: 60px;
  height: 60px;
  text-align: center;
  line-height: 60px;
  border: 2px solid #ddd;
  border-radius: 8px;
  margin: 5px;
}

.day-box.checked {
  background: #4caf50;
  color: white;
  border-color: #4caf50;
}

.day-box.current {
  border-color: #007bff;
}
```

- [ ] **Step 2: 创建 api.js**

```javascript
// web/js/api.js
const API_BASE = 'http://localhost:8080';

// 调用 gRPC 服务（通过 Gateway Ingress）
async function callApi(service, method, data, token) {
  const response = await fetch(`${API_BASE}/api/${service}/${method}`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      ...(token ? { 'Authorization': `Bearer ${token}` } : {})
    },
    body: JSON.stringify(data)
  });

  return response.json();
}

// 用户注册
async function register(username, password) {
  return callApi('user.v1.UserService', 'Register', {
    username: username,
    password: password
  });
}

// 用户登录
async function login(username, password) {
  return callApi('user.v1.UserService', 'Login', {
    username: username,
    password: password
  });
}

// 获取签到状态
async function getCheckinStatus(token) {
  return callApi('lobby.v1.CheckinService', 'GetStatus', {}, token);
}

// 签到
async function checkin(token) {
  return callApi('lobby.v1.CheckinService', 'Checkin', {}, token);
}

// 获取我的物品
async function getMyItems(token) {
  return callApi('lobby.v1.ItemService', 'GetMyItems', {}, token);
}

// 保存 token
function saveToken(token) {
  localStorage.setItem('token', token);
}

// 获取 token
function getToken() {
  return localStorage.getItem('token');
}

// 清除 token
function clearToken() {
  localStorage.removeItem('token');
}

// 检查登录状态
function checkAuth() {
  const token = getToken();
  if (!token) {
    window.location.href = '/login.html';
    return false;
  }
  return true;
}
```

- [ ] **Step 3: 创建 index.html**

```html
<!-- web/index.html -->
<!DOCTYPE html>
<html>
<head>
  <meta charset="UTF-8">
  <title>kd48 - 游戏平台</title>
  <link rel="stylesheet" href="/css/style.css">
</head>
<body>
  <nav class="nav">
    <a href="/">首页</a>
    <a href="/checkin.html">签到</a>
    <a href="/items.html">背包</a>
    <a href="#" onclick="logout()">退出</a>
  </nav>

  <div class="container">
    <div class="card">
      <h1>欢迎来到 kd48</h1>
      <p>一个简单的游戏平台演示</p>
    </div>
  </div>

  <script src="/js/api.js"></script>
  <script>
    if (!checkAuth()) {
      // 未登录会自动跳转
    }

    function logout() {
      clearToken();
      window.location.href = '/login.html';
    }
  </script>
</body>
</html>
```

- [ ] **Step 4: 创建 login.html**

```html
<!-- web/login.html -->
<!DOCTYPE html>
<html>
<head>
  <meta charset="UTF-8">
  <title>登录 - kd48</title>
  <link rel="stylesheet" href="/css/style.css">
</head>
<body>
  <div class="container">
    <div class="card">
      <h2>登录</h2>
      <form id="loginForm">
        <input type="text" id="username" placeholder="用户名" required>
        <input type="password" id="password" placeholder="密码" required>
        <button type="submit" class="btn">登录</button>
      </form>
      <p style="margin-top: 10px;">
        没有账号？<a href="/register.html">注册</a>
      </p>
      <p id="error" style="color: red;"></p>
    </div>
  </div>

  <script src="/js/api.js"></script>
  <script>
    document.getElementById('loginForm').addEventListener('submit', async (e) => {
      e.preventDefault();
      const username = document.getElementById('username').value;
      const password = document.getElementById('password').value;
      
      try {
        const resp = await login(username, password);
        if (resp.code === 0) {
          saveToken(resp.data.token);
          window.location.href = '/';
        } else {
          document.getElementById('error').textContent = resp.message;
        }
      } catch (err) {
        document.getElementById('error').textContent = '网络错误';
      }
    });

    // 已登录则跳转首页
    if (getToken()) {
      window.location.href = '/';
    }
  </script>
</body>
</html>
```

- [ ] **Step 5: 创建 register.html**

```html
<!-- web/register.html -->
<!DOCTYPE html>
<html>
<head>
  <meta charset="UTF-8">
  <title>注册 - kd48</title>
  <link rel="stylesheet" href="/css/style.css">
</head>
<body>
  <div class="container">
    <div class="card">
      <h2>注册</h2>
      <form id="registerForm">
        <input type="text" id="username" placeholder="用户名" required>
        <input type="password" id="password" placeholder="密码" required>
        <input type="password" id="password2" placeholder="确认密码" required>
        <button type="submit" class="btn">注册</button>
      </form>
      <p style="margin-top: 10px;">
        已有账号？<a href="/login.html">登录</a>
      </p>
      <p id="error" style="color: red;"></p>
    </div>
  </div>

  <script src="/js/api.js"></script>
  <script>
    document.getElementById('registerForm').addEventListener('submit', async (e) => {
      e.preventDefault();
      const username = document.getElementById('username').value;
      const password = document.getElementById('password').value;
      const password2 = document.getElementById('password2').value;
      
      if (password !== password2) {
        document.getElementById('error').textContent = '密码不一致';
        return;
      }

      try {
        const resp = await register(username, password);
        if (resp.code === 0) {
          alert('注册成功，请登录');
          window.location.href = '/login.html';
        } else {
          document.getElementById('error').textContent = resp.message;
        }
      } catch (err) {
        document.getElementById('error').textContent = '网络错误';
      }
    });
  </script>
</body>
</html>
```

- [ ] **Step 6: 提交**

```bash
git add web/
git commit -m "feat(web): add base structure with login/register pages"
```

---

## Task 12: Web 客户端 - 签到页面

**Files:**
- Create: `web/checkin.html`
- Create: `web/js/checkin.js`

- [ ] **Step 1: 创建 checkin.js**

```javascript
// web/js/checkin.js

// 渲染签到状态
function renderStatus(data) {
  const statusDiv = document.getElementById('status');
  
  statusDiv.innerHTML = `
    <p>连续签到: <strong>${data.continuous_days}</strong> 天</p>
    <p>累计签到: <strong>${data.total_days}</strong> 天</p>
    <p>本期: ${data.period_name}</p>
  `;

  // 渲染日期格子
  const daysDiv = document.getElementById('days');
  daysDiv.innerHTML = '';
  
  for (let i = 1; i <= 7; i++) {
    const dayBox = document.createElement('div');
    dayBox.className = 'day-box';
    dayBox.textContent = `第${i}天`;
    
    if (i <= data.total_days) {
      dayBox.classList.add('checked');
    }
    if (i === data.total_days + 1 && !data.today_checked) {
      dayBox.classList.add('current');
    }
    
    daysDiv.appendChild(dayBox);
  }

  // 更新按钮状态
  const btn = document.getElementById('checkinBtn');
  if (data.today_checked) {
    btn.disabled = true;
    btn.textContent = '今日已签到';
  } else {
    btn.disabled = false;
    btn.textContent = '立即签到';
  }
}

// 渲染奖励结果
function renderReward(data) {
  const resultDiv = document.getElementById('result');
  resultDiv.innerHTML = '<h3>签到成功！</h3>';
  
  const rewardsDiv = document.createElement('div');
  for (const [itemId, count] of Object.entries(data.rewards)) {
    const span = document.createElement('span');
    span.className = 'reward-item';
    span.textContent = `物品${itemId} x ${count}`;
    rewardsDiv.appendChild(span);
  }
  resultDiv.appendChild(rewardsDiv);
}

// 加载签到状态
async function loadStatus() {
  try {
    const resp = await getCheckinStatus(getToken());
    if (resp.code === 0) {
      renderStatus(resp.data);
    } else {
      alert(resp.message);
    }
  } catch (err) {
    console.error('加载状态失败:', err);
  }
}

// 执行签到
async function doCheckin() {
  const btn = document.getElementById('checkinBtn');
  btn.disabled = true;
  
  try {
    const resp = await checkin(getToken());
    if (resp.code === 0) {
      renderReward(resp.data);
      await loadStatus();
    } else if (resp.code === 200) {
      alert('今日已签到');
    } else {
      alert(resp.message);
    }
  } catch (err) {
    console.error('签到失败:', err);
    alert('网络错误');
  } finally {
    btn.disabled = false;
  }
}
```

- [ ] **Step 2: 创建 checkin.html**

```html
<!-- web/checkin.html -->
<!DOCTYPE html>
<html>
<head>
  <meta charset="UTF-8">
  <title>签到 - kd48</title>
  <link rel="stylesheet" href="/css/style.css">
</head>
<body>
  <nav class="nav">
    <a href="/">首页</a>
    <a href="/checkin.html">签到</a>
    <a href="/items.html">背包</a>
    <a href="#" onclick="logout()">退出</a>
  </nav>

  <div class="container">
    <div class="card">
      <h2>每日签到</h2>
      <div id="status">加载中...</div>
    </div>

    <div class="card">
      <div id="days"></div>
    </div>

    <div class="card">
      <button id="checkinBtn" class="btn" onclick="doCheckin()">立即签到</button>
    </div>

    <div class="card" id="result"></div>
  </div>

  <script src="/js/api.js"></script>
  <script src="/js/checkin.js"></script>
  <script>
    if (!checkAuth()) {
      // 未登录会自动跳转
    } else {
      loadStatus();
    }

    function logout() {
      clearToken();
      window.location.href = '/login.html';
    }
  </script>
</body>
</html>
```

- [ ] **Step 3: 提交**

```bash
git add web/checkin.html web/js/checkin.js
git commit -m "feat(web): add checkin page with status and reward display"
```

---

## Task 13: Web 客户端 - 背包页面

**Files:**
- Create: `web/items.html`

- [ ] **Step 1: 创建 items.html**

```html
<!-- web/items.html -->
<!DOCTYPE html>
<html>
<head>
  <meta charset="UTF-8">
  <title>背包 - kd48</title>
  <link rel="stylesheet" href="/css/style.css">
</head>
<body>
  <nav class="nav">
    <a href="/">首页</a>
    <a href="/checkin.html">签到</a>
    <a href="/items.html">背包</a>
    <a href="#" onclick="logout()">退出</a>
  </nav>

  <div class="container">
    <div class="card">
      <h2>我的道具</h2>
      <div id="items">加载中...</div>
    </div>
  </div>

  <script src="/js/api.js"></script>
  <script>
    if (!checkAuth()) {
      // 未登录会自动跳转
    } else {
      loadItems();
    }

    async function loadItems() {
      try {
        const resp = await getMyItems(getToken());
        const itemsDiv = document.getElementById('items');
        
        if (resp.code === 0 && Object.keys(resp.data.items).length > 0) {
          itemsDiv.innerHTML = '';
          for (const [itemId, count] of Object.entries(resp.data.items)) {
            const itemDiv = document.createElement('div');
            itemDiv.className = 'card';
            itemDiv.innerHTML = `
              <p><strong>物品ID: ${itemId}</strong></p>
              <p>数量: ${count}</p>
            `;
            itemsDiv.appendChild(itemDiv);
          }
        } else if (resp.code === 0) {
          itemsDiv.innerHTML = '<p>暂无道具</p>';
        } else {
          itemsDiv.innerHTML = `<p style="color: red;">${resp.message}</p>`;
        }
      } catch (err) {
        document.getElementById('items').innerHTML = '<p style="color: red;">网络错误</p>';
      }
    }

    function logout() {
      clearToken();
      window.location.href = '/login.html';
    }
  </script>
</body>
</html>
```

- [ ] **Step 2: 提交**

```bash
git add web/items.html
git commit -m "feat(web): add items page for displaying user items"
```

---

## Task 14: 更新 TODO.md

**Files:**
- Modify: `TODO.md`

- [ ] **Step 1: 更新 TODO.md 标记签到任务进行中**

在 TODO.md 中，将签到活动任务标记为已完成：
- P1: 签到活动（打通基础设施）→ ✅ 已完成

- [ ] **Step 2: 提交**

```bash
git add TODO.md
git commit -m "docs: mark checkin activity as completed"
```

---

## 验证清单

实现完成后，按以下步骤验证：

1. **启动服务**
   ```bash
   docker-compose up -d
   ./build.sh
   ./gateway.bin &
   ./lobby.bin &
   ```

2. **测试签到 API**
   ```bash
   # 注册用户
   curl -X POST http://localhost:8080/api/user.v1.UserService/Register \
     -H "Content-Type: application/json" \
     -d '{"username":"test","password":"test123"}'
   
   # 登录获取 token
   curl -X POST http://localhost:8080/api/user.v1.UserService/Login \
     -H "Content-Type: application/json" \
     -d '{"username":"test","password":"test123"}'
   
   # 获取签到状态
   curl -X POST http://localhost:8080/api/lobby.v1.CheckinService/GetStatus \
     -H "Authorization: Bearer <token>"
   
   # 签到
   curl -X POST http://localhost:8080/api/lobby.v1.CheckinService/Checkin \
     -H "Authorization: Bearer <token>"
   ```

3. **测试 Web 客户端**
   - 访问 http://localhost:8080/web/
   - 注册 → 登录 → 签到 → 查看背包

4. **验证观测环境**
   - Jaeger: http://localhost:16686
   - Prometheus: http://localhost:9090
   - Grafana: http://localhost:3000 (admin/admin)
