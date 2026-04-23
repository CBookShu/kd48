// services/lobby/cmd/lobby/item_server.go
package main

import (
	"context"
	"log/slog"

	lobbyv1 "github.com/CBookShu/kd48/api/proto/lobby/v1"
	"github.com/CBookShu/kd48/services/lobby/internal/item"
	"github.com/redis/go-redis/v9"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
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
func (s *ItemService) GetMyItems(ctx context.Context, req *lobbyv1.GetMyItemsRequest) (*lobbyv1.ApiResponse, error) {
	// 从 context 获取 user_id（由 Gateway 注入）
	userID, ok := ctx.Value("user_id").(int64)
	if !ok {
		return errorResponse(lobbyv1.ErrorCode_USER_NOT_AUTHENTICATED, "user not authenticated"), nil
	}

	items, err := s.store.GetItems(ctx, userID)
	if err != nil {
		slog.ErrorContext(ctx, "failed to get items", "user_id", userID, "error", err)
		return errorResponse(lobbyv1.ErrorCode_INTERNAL_ERROR, "failed to get items"), nil
	}

	data := &lobbyv1.MyItemsData{
		Items: items,
	}

	return successResponse(data)
}

// successResponse 构造成功响应
func successResponse(data proto.Message) (*lobbyv1.ApiResponse, error) {
	anyData, err := anypb.New(data)
	if err != nil {
		return errorResponse(lobbyv1.ErrorCode_INTERNAL_ERROR, "failed to marshal response"), nil
	}

	return &lobbyv1.ApiResponse{
		Code:    int32(lobbyv1.ErrorCode_SUCCESS),
		Message: "success",
		Data:    anyData,
	}, nil
}

// errorResponse 构造错误响应
func errorResponse(code lobbyv1.ErrorCode, message string) *lobbyv1.ApiResponse {
	return &lobbyv1.ApiResponse{
		Code:    int32(code),
		Message: message,
	}
}
