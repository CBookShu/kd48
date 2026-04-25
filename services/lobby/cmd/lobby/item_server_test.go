// services/lobby/cmd/lobby/item_server_test.go
package main

import (
	"context"
	"strconv"
	"testing"

	"github.com/alicebob/miniredis/v2"
	commonv1 "github.com/CBookShu/kd48/api/proto/common/v1"
	lobbyv1 "github.com/CBookShu/kd48/api/proto/lobby/v1"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func setupItemTest(t *testing.T) (*ItemService, *miniredis.Miniredis) {
	mr, err := miniredis.Run()
	require.NoError(t, err)

	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	svc := NewItemService(rdb)

	return svc, mr
}

// === GetMyItems Tests ===

func TestItemService_GetMyItems_Success(t *testing.T) {
	svc, mr := setupItemTest(t)
	defer mr.Close()

	ctx := context.WithValue(context.Background(), "user_id", int64(12345))

	// 设置测试数据
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	rdb.HSet(ctx, "kd48:user_items:12345", "1001", "1000", "1002", "500")

	data, err := svc.GetMyItems(ctx, &lobbyv1.GetMyItemsRequest{})

	require.NoError(t, err)
	assert.Equal(t, int64(1000), data.Items[1001])
	assert.Equal(t, int64(500), data.Items[1002])
}

func TestItemService_GetMyItems_NotAuthenticated(t *testing.T) {
	svc, mr := setupItemTest(t)
	defer mr.Close()

	ctx := context.Background() // 没有 user_id

	_, err := svc.GetMyItems(ctx, &lobbyv1.GetMyItemsRequest{})

	require.Error(t, err)
	st, ok := status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.Code(commonv1.ErrorCode_USER_NOT_AUTHENTICATED), st.Code())
}

func TestItemService_GetMyItems_EmptyInventory(t *testing.T) {
	svc, mr := setupItemTest(t)
	defer mr.Close()

	ctx := context.WithValue(context.Background(), "user_id", int64(12345))

	data, err := svc.GetMyItems(ctx, &lobbyv1.GetMyItemsRequest{})

	require.NoError(t, err)
	assert.Empty(t, data.Items)
}

func TestItemService_GetMyItems_DifferentUsers(t *testing.T) {
	svc, mr := setupItemTest(t)
	defer mr.Close()

	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})

	// 用户1的数据
	ctx1 := context.WithValue(context.Background(), "user_id", int64(111))
	rdb.HSet(ctx1, "kd48:user_items:111", "1001", "100")

	// 用户2的数据
	ctx2 := context.WithValue(context.Background(), "user_id", int64(222))
	rdb.HSet(ctx2, "kd48:user_items:222", "1001", "200")

	// 用户1查询
	data1, err := svc.GetMyItems(ctx1, &lobbyv1.GetMyItemsRequest{})
	require.NoError(t, err)
	assert.Equal(t, int64(100), data1.Items[1001])

	// 用户2查询
	data2, err := svc.GetMyItems(ctx2, &lobbyv1.GetMyItemsRequest{})
	require.NoError(t, err)
	assert.Equal(t, int64(200), data2.Items[1001])
}

func TestItemService_GetMyItems_LargeNumbers(t *testing.T) {
	svc, mr := setupItemTest(t)
	defer mr.Close()

	ctx := context.WithValue(context.Background(), "user_id", int64(12345))

	// 设置大数值物品
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	rdb.HSet(ctx, "kd48:user_items:12345", "1001", "999999999999")

	data, err := svc.GetMyItems(ctx, &lobbyv1.GetMyItemsRequest{})

	require.NoError(t, err)
	assert.Equal(t, int64(999999999999), data.Items[1001])
}

func TestItemService_GetMyItems_ManyItems(t *testing.T) {
	svc, mr := setupItemTest(t)
	defer mr.Close()

	ctx := context.WithValue(context.Background(), "user_id", int64(12345))

	// 设置大量物品
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	fields := make(map[string]interface{})
	for i := 1001; i <= 1100; i++ {
		fields[strconv.Itoa(i)] = (i - 1000) * 10
	}
	rdb.HSet(ctx, "kd48:user_items:12345", fields)

	data, err := svc.GetMyItems(ctx, &lobbyv1.GetMyItemsRequest{})

	require.NoError(t, err)
	assert.Len(t, data.Items, 100)
}
