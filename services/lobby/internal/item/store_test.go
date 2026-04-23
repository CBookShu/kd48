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
