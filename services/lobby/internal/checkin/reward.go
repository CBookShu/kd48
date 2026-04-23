// services/lobby/internal/checkin/reward.go
package checkin

// RewardCalculator 奖励计算器
type RewardCalculator struct {
	dailyRewards      map[int]map[int32]int64 // day -> item_id -> count
	continuousRewards map[int]map[int32]int64 // continuous_days -> item_id -> count
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

// GetAllContinuousRewards 获取所有连续奖励配置
func (c *RewardCalculator) GetAllContinuousRewards() map[int]map[int32]int64 {
	return c.continuousRewards
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
