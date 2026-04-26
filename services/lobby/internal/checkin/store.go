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
func redisKey(userID uint32) string {
	return fmt.Sprintf("kd48:checkin:%d", userID)
}

// GetStatus 获取用户签到状态
func (s *CheckinStore) GetStatus(ctx context.Context, userID uint32) (*UserCheckinStatus, error) {
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
func (s *CheckinStore) UpdateStatus(ctx context.Context, userID uint32, status *UserCheckinStatus) error {
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
func (s *CheckinStore) ResetStatus(ctx context.Context, userID uint32, periodID int64) error {
	status := &UserCheckinStatus{
		PeriodID:        periodID,
		LastCheckinDate: "",
		ContinuousDays:  0,
		ClaimedDays:     []int{},
	}
	return s.UpdateStatus(ctx, userID, status)
}
