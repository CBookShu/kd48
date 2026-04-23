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
