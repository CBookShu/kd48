// services/lobby/cmd/lobby/item_server_test.go
package main

import (
	"context"
	"testing"

	"github.com/alicebob/miniredis/v2"
	lobbyv1 "github.com/CBookShu/kd48/api/proto/lobby/v1"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
)

func TestItemService_GetMyItems(t *testing.T) {
	mr, err := miniredis.Run()
	assert.NoError(t, err)
	defer mr.Close()

	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	svc := NewItemService(rdb)

	// Create context with user_id
	ctx := context.WithValue(context.Background(), "user_id", int64(12345))

	// 设置测试数据
	rdb.HSet(ctx, "kd48:user_items:12345", "1001", "1000", "1002", "500")

	// 调用
	resp, err := svc.GetMyItems(ctx, &lobbyv1.GetMyItemsRequest{})
	assert.NoError(t, err)
	assert.Equal(t, int32(0), resp.Code)

	// 解析 data
	var data lobbyv1.MyItemsData
	err = resp.Data.UnmarshalTo(&data)
	assert.NoError(t, err)
	assert.Equal(t, int64(1000), data.Items[1001])
	assert.Equal(t, int64(500), data.Items[1002])
}

func TestItemService_GetMyItems_NotAuthenticated(t *testing.T) {
	mr, err := miniredis.Run()
	assert.NoError(t, err)
	defer mr.Close()

	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	svc := NewItemService(rdb)

	// Create context without user_id
	ctx := context.Background()

	// 调用
	resp, err := svc.GetMyItems(ctx, &lobbyv1.GetMyItemsRequest{})
	assert.NoError(t, err)
	assert.Equal(t, int32(lobbyv1.ErrorCode_USER_NOT_AUTHENTICATED), resp.Code)
}
