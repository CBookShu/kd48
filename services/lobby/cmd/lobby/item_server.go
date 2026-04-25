// services/lobby/cmd/lobby/item_server.go
package main

import (
	"context"

	commonv1 "github.com/CBookShu/kd48/api/proto/common/v1"
	lobbyv1 "github.com/CBookShu/kd48/api/proto/lobby/v1"
	"github.com/CBookShu/kd48/services/lobby/internal/item"
	"github.com/redis/go-redis/v9"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// ItemService 物品服务
type ItemService struct {
	lobbyv1.UnimplementedItemServiceServer
	store *item.ItemStore
}

// NewItemService 创建物品服务
func NewItemService(rdb *redis.Client) *ItemService {
	return &ItemService{
		store: item.NewItemStore(rdb),
	}
}

// GetMyItems 获取我的物品
func (s *ItemService) GetMyItems(ctx context.Context, req *lobbyv1.GetMyItemsRequest) (*lobbyv1.MyItemsData, error) {
	// 从 context 获取 user_id（由 Gateway 注入）
	userID, ok := ctx.Value("user_id").(int64)
	if !ok {
		return nil, status.Errorf(codes.Code(commonv1.ErrorCode_USER_NOT_AUTHENTICATED), "用户未认证")
	}

	items, err := s.store.GetItems(ctx, userID)
	if err != nil {
		return nil, status.Errorf(codes.Code(commonv1.ErrorCode_INTERNAL_ERROR), "获取物品失败")
	}

	return &lobbyv1.MyItemsData{
		Items: items,
	}, nil
}
