// services/lobby/cmd/lobby/checkin_server_test.go
package main

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	commonv1 "github.com/CBookShu/kd48/api/proto/common/v1"
	lobbyv1 "github.com/CBookShu/kd48/api/proto/lobby/v1"
	"github.com/CBookShu/kd48/pkg/contextkey"
	"github.com/CBookShu/kd48/services/lobby/internal/checkin"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
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
		7: {1002: 1000}, // 第7天特殊奖励
	})
	svc.SetContinuousRewards(map[int]map[int32]int64{
		3: {1003: 50},  // 连续3天
		5: {1003: 100}, // 连续5天
		7: {1003: 200}, // 连续7天
	})

	return svc, mr
}

// === GetStatus Tests ===

func TestCheckinService_GetStatus_Success(t *testing.T) {
	svc, mr := setupCheckinTest(t)
	defer mr.Close()

	ctx := contextkey.WithUserID(context.Background(), uint32(12345))
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

	ctx := contextkey.WithUserID(context.Background(), uint32(12345))
	_, err = svc.GetStatus(ctx, &lobbyv1.GetStatusRequest{})

	require.Error(t, err)
	st, ok := status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.Code(commonv1.ErrorCode_CHECKIN_PERIOD_NOT_ACTIVE), st.Code())
}

func TestCheckinService_GetStatus_AfterCheckin(t *testing.T) {
	svc, mr := setupCheckinTest(t)
	defer mr.Close()

	ctx := contextkey.WithUserID(context.Background(), uint32(12345))

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

	ctx := contextkey.WithUserID(context.Background(), uint32(12345))

	// 第一次签到
	data, err := svc.Checkin(ctx, &lobbyv1.CheckinRequest{})

	require.NoError(t, err)
	assert.Equal(t, int32(1), data.ContinuousDays)
	assert.Equal(t, map[int32]int64{1001: 100}, data.Rewards) // 第1天奖励
}

func TestCheckinService_Checkin_AlreadyToday(t *testing.T) {
	svc, mr := setupCheckinTest(t)
	defer mr.Close()

	ctx := contextkey.WithUserID(context.Background(), uint32(12345))

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

	ctx := contextkey.WithUserID(context.Background(), uint32(12345))

	// 模拟昨天的签到状态
	yesterday := time.Now().AddDate(0, 0, -1).Format("2006-01-02")
	svc.checkinStore.UpdateStatus(ctx, uint32(12345), &checkin.UserCheckinStatus{
		PeriodID:        1,
		LastCheckinDate: yesterday,
		ContinuousDays:  2,
		ClaimedDays:     []int{1, 2},
	})

	// 今天签到，应该是连续第3天
	data, err := svc.Checkin(ctx, &lobbyv1.CheckinRequest{})
	require.NoError(t, err)

	assert.Equal(t, int32(3), data.ContinuousDays)
	// 应该获得第3天奖励 + 连续3天奖励
	assert.Equal(t, int64(300), data.Rewards[1001]) // 第3天奖励
	assert.Equal(t, int64(50), data.Rewards[1003])  // 连续3天奖励
}

func TestCheckinService_Checkin_BreakContinuous(t *testing.T) {
	svc, mr := setupCheckinTest(t)
	defer mr.Close()

	ctx := contextkey.WithUserID(context.Background(), uint32(12345))

	// 模拟2天前的签到状态（断签）
	twoDaysAgo := time.Now().AddDate(0, 0, -2).Format("2006-01-02")
	svc.checkinStore.UpdateStatus(ctx, uint32(12345), &checkin.UserCheckinStatus{
		PeriodID:        1,
		LastCheckinDate: twoDaysAgo,
		ContinuousDays:  5,
		ClaimedDays:     []int{1, 2, 3, 4, 5},
	})

	// 今天签到，连续天数应该重置为1
	data, err := svc.Checkin(ctx, &lobbyv1.CheckinRequest{})
	require.NoError(t, err)

	assert.Equal(t, int32(1), data.ContinuousDays) // 连续天数重置为1
	// ClaimedDays 不会重置，所以获得第6天的奖励
	assert.Equal(t, int64(600), data.Rewards[1001])
	// 没有连续奖励（因为连续天数是1）
	assert.Equal(t, int64(0), data.Rewards[1003])
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

	ctx := contextkey.WithUserID(context.Background(), uint32(12345))
	_, err = svc.Checkin(ctx, &lobbyv1.CheckinRequest{})

	require.Error(t, err)
	st, ok := status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.Code(commonv1.ErrorCode_CHECKIN_PERIOD_NOT_ACTIVE), st.Code())
}

func TestCheckinService_Checkin_NewPeriodReset(t *testing.T) {
	svc, mr := setupCheckinTest(t)
	defer mr.Close()

	ctx := contextkey.WithUserID(context.Background(), uint32(12345))

	// 模拟旧期的签到状态
	svc.checkinStore.UpdateStatus(ctx, uint32(12345), &checkin.UserCheckinStatus{
		PeriodID:        999, // 旧期
		LastCheckinDate: time.Now().AddDate(0, 0, -1).Format("2006-01-02"),
		ContinuousDays:  10,
		ClaimedDays:     []int{1, 2, 3, 4, 5, 6, 7},
	})

	// 新期签到，应该重置
	data, err := svc.Checkin(ctx, &lobbyv1.CheckinRequest{})
	require.NoError(t, err)

	// 新期第一天签到
	assert.Equal(t, int32(1), data.ContinuousDays)
	assert.Equal(t, map[int32]int64{1001: 100}, data.Rewards)
}

// === Item Integration Tests ===

func TestCheckinService_Checkin_ItemsAddedToInventory(t *testing.T) {
	svc, mr := setupCheckinTest(t)
	defer mr.Close()

	userID := uint32(12345)
	ctx := contextkey.WithUserID(context.Background(), userID)

	// 签到
	_, err := svc.Checkin(ctx, &lobbyv1.CheckinRequest{})
	require.NoError(t, err)

	// 验证物品已添加到背包
	items, err := svc.itemStore.GetItems(ctx, userID)
	require.NoError(t, err)
	assert.Equal(t, int64(100), items[1001])
}
