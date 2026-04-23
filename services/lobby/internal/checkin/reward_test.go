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
