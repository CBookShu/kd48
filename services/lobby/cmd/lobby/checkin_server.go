// services/lobby/cmd/lobby/checkin_server.go
package main

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
func (s *CheckinService) Checkin(ctx context.Context, req *lobbyv1.CheckinRequest) (*lobbyv1.CheckinData, error) {
	userID, ok := ctx.Value("user_id").(int64)
	if !ok {
		return nil, status.Errorf(codes.Code(commonv1.ErrorCode_USER_NOT_AUTHENTICATED), "用户未认证")
	}

	if s.period == nil {
		return nil, status.Errorf(codes.Code(commonv1.ErrorCode_CHECKIN_PERIOD_NOT_ACTIVE), "签到期未开启")
	}

	// 获取用户签到状态
	checkinStatus, err := s.checkinStore.GetStatus(ctx, userID)
	if err != nil {
		slog.ErrorContext(ctx, "failed to get checkin status", "user_id", userID, "error", err)
		return nil, status.Errorf(codes.Code(commonv1.ErrorCode_INTERNAL_ERROR), "获取签到状态失败")
	}

	// 检查是否需要重置（新期开始）
	if checkinStatus.PeriodID != s.period.PeriodID {
		checkinStatus = &checkin.UserCheckinStatus{
			PeriodID:    s.period.PeriodID,
			ClaimedDays: []int{},
		}
	}

	// 检查今日是否已签到
	today := time.Now().Format("2006-01-02")
	if checkinStatus.LastCheckinDate == today {
		return nil, status.Errorf(codes.Code(commonv1.ErrorCode_CHECKIN_ALREADY_TODAY), "今日已签到")
	}

	// 计算连续天数
	yesterday := time.Now().AddDate(0, 0, -1).Format("2006-01-02")
	if checkinStatus.LastCheckinDate == yesterday {
		checkinStatus.ContinuousDays++
	} else {
		checkinStatus.ContinuousDays = 1
	}

	// 计算奖励
	day := len(checkinStatus.ClaimedDays) + 1
	rewards := s.calculator.GetDailyReward(day)

	// 检查连续奖励
	if s.calculator.HasContinuousReward(checkinStatus.ContinuousDays) {
		continuousReward := s.calculator.GetContinuousReward(checkinStatus.ContinuousDays)
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
	checkinStatus.LastCheckinDate = today
	checkinStatus.ClaimedDays = append(checkinStatus.ClaimedDays, day)
	if err := s.checkinStore.UpdateStatus(ctx, userID, checkinStatus); err != nil {
		slog.ErrorContext(ctx, "failed to update checkin status", "user_id", userID, "error", err)
		return nil, status.Errorf(codes.Code(commonv1.ErrorCode_INTERNAL_ERROR), "更新签到状态失败")
	}

	return &lobbyv1.CheckinData{
		ContinuousDays: int32(checkinStatus.ContinuousDays),
		Rewards:        rewards,
	}, nil
}

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
	checkinStatus, err := s.checkinStore.GetStatus(ctx, userID)
	if err != nil {
		slog.ErrorContext(ctx, "failed to get checkin status", "user_id", userID, "error", err)
		return nil, status.Errorf(codes.Code(commonv1.ErrorCode_INTERNAL_ERROR), "获取签到状态失败")
	}

	// 检查是否需要重置
	if checkinStatus.PeriodID != s.period.PeriodID {
		checkinStatus = &checkin.UserCheckinStatus{
			PeriodID:    s.period.PeriodID,
			ClaimedDays: []int{},
		}
	}

	// 构建响应
	today := time.Now().Format("2006-01-02")
	data := &lobbyv1.CheckinStatusData{
		PeriodId:          s.period.PeriodID,
		PeriodName:        s.period.PeriodName,
		TodayChecked:      checkinStatus.LastCheckinDate == today,
		ContinuousDays:    int32(checkinStatus.ContinuousDays),
		TotalDays:         int32(len(checkinStatus.ClaimedDays)),
		ClaimedContinuous: getClaimedContinuous(checkinStatus.ClaimedDays, s.calculator),
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
