package rediskit

import (
	"context"
	"fmt"
	"time"

	"github.com/CBookShu/kd48/pkg/conf"
	"github.com/redis/go-redis/v9"
)

// NewClient 创建并验证 Redis 连接
func NewClient(c conf.RedisConf) (*redis.Client, error) {
	rdb := redis.NewClient(&redis.Options{
		Addr:     c.Addr,
		Password: c.Password,
		DB:       c.DB,
	})

	// 带超时的 Ping 验证连接可用性
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	if err := rdb.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("redis ping failed: %w", err)
	}

	return rdb, nil
}
