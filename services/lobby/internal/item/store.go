// services/lobby/internal/item/store.go
package item

import (
	"context"
	"fmt"
	"strconv"

	"github.com/redis/go-redis/v9"
)

// ItemStore 物品存储
type ItemStore struct {
	rdb *redis.Client
}

// NewItemStore 创建物品存储
func NewItemStore(rdb *redis.Client) *ItemStore {
	return &ItemStore{rdb: rdb}
}

// redisKey 生成 Redis key
func redisKey(userID int64) string {
	return fmt.Sprintf("kd48:user_items:%d", userID)
}

// GetItems 获取用户所有物品
func (s *ItemStore) GetItems(ctx context.Context, userID int64) (map[int32]int64, error) {
	key := redisKey(userID)
	result, err := s.rdb.HGetAll(ctx, key).Result()
	if err != nil {
		return nil, fmt.Errorf("redis hgetall: %w", err)
	}

	items := make(map[int32]int64)
	for itemIDStr, countStr := range result {
		itemID, err := strconv.ParseInt(itemIDStr, 10, 32)
		if err != nil {
			return nil, fmt.Errorf("parse item_id %q: %w", itemIDStr, err)
		}
		count, err := strconv.ParseInt(countStr, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("parse count %q: %w", countStr, err)
		}
		items[int32(itemID)] = count
	}

	return items, nil
}

// AddItem 添加物品（返回新数量）
func (s *ItemStore) AddItem(ctx context.Context, userID int64, itemID int32, count int64) error {
	key := redisKey(userID)
	field := fmt.Sprintf("%d", itemID)

	_, err := s.rdb.HIncrBy(ctx, key, field, count).Result()
	if err != nil {
		return fmt.Errorf("redis hincrby: %w", err)
	}

	return nil
}

// SetItem 设置物品数量
func (s *ItemStore) SetItem(ctx context.Context, userID int64, itemID int32, count int64) error {
	key := redisKey(userID)
	field := fmt.Sprintf("%d", itemID)

	_, err := s.rdb.HSet(ctx, key, field, count).Result()
	if err != nil {
		return fmt.Errorf("redis hset: %w", err)
	}

	return nil
}
