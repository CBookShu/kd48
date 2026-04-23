// services/lobby/cmd/lobby/checkin_server.go
package main

import (
	"context"
	"log/slog"
	"time"

	lobbyv1 "github.com/CBookShu/kd48/api/proto/lobby/v1"
	"github.com/CBookShu/kd48/services/lobby/internal/checkin"
	"github.com/CBookShu/kd48/services/lobby/internal/item"
	"github.com/redis/go-redis/v9"
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
		slog.ErrorContext(ctx, "failed to get checkin status", "user_id", userID, "error", err)
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
			slog.ErrorContext(ctx, "failed to add items", "user_id", userID, "error", err)
			return errorResponse(lobbyv1.ErrorCode_INTERNAL_ERROR, "failed to add items"), nil
		}
	}

	// 更新签到状态
	status.LastCheckinDate = today
	status.ClaimedDays = append(status.ClaimedDays, day)
	if err := s.checkinStore.UpdateStatus(ctx, userID, status); err != nil {
		slog.ErrorContext(ctx, "failed to update checkin status", "user_id", userID, "error", err)
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
		slog.ErrorContext(ctx, "failed to get checkin status", "user_id", userID, "error", err)
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
		PeriodId:          s.period.PeriodID,
		PeriodName:        s.period.PeriodName,
		TodayChecked:      status.LastCheckinDate == today,
		ContinuousDays:    int32(status.ContinuousDays),
		TotalDays:         int32(len(status.ClaimedDays)),
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
