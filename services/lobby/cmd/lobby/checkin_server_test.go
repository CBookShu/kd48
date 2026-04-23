// services/lobby/cmd/lobby/checkin_server_test.go
package main

import (
	"context"
	"testing"

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
	svc.SetPeriod(&PeriodConfig{PeriodID: 1, PeriodName: "test"})

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
