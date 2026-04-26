package ws

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func TestSessionInvalidationSubscriber_ParseMessage(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis run: %v", err)
	}
	defer mr.Close()

	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})

	cm := NewConnectionManager(HeartbeatConfig{
		Interval:  30 * time.Second,
		Timeout:   10 * time.Second,
		MaxMissed: 3,
	})

	// 注册一个连接
	clientID := "conn-123"
	userID := uint32(456)
	cm.RegisterConnection(clientID, nil)
	cm.RegisterUserConnection(userID, clientID)

	subscriber := NewSessionInvalidationSubscriber(rdb, cm, "kd48:session:invalidate")

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	go subscriber.Start(ctx)

	// 等待订阅器启动
	time.Sleep(50 * time.Millisecond)

	// 使用 miniredis 的 Publish 方法发布失效消息
	mr.Publish("kd48:session:invalidate", `{"user_id":456}`)

	// 等待处理
	time.Sleep(100 * time.Millisecond)

	// 验证连接被断开
	_, exists := cm.GetUserClientID(userID)
	if exists {
		t.Error("user connection should be removed after invalidate")
	}
}

func TestSessionInvalidationSubscriber_InvalidJSON(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis run: %v", err)
	}
	defer mr.Close()

	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})

	cm := NewConnectionManager(HeartbeatConfig{
		Interval:  30 * time.Second,
		Timeout:   10 * time.Second,
		MaxMissed: 3,
	})

	clientID := "conn-123"
	userID := uint32(456)
	cm.RegisterConnection(clientID, nil)
	cm.RegisterUserConnection(userID, clientID)

	subscriber := NewSessionInvalidationSubscriber(rdb, cm, "kd48:session:invalidate")

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	go subscriber.Start(ctx)

	// 等待订阅器启动
	time.Sleep(50 * time.Millisecond)

	// 发布无效 JSON
	mr.Publish("kd48:session:invalidate", `invalid-json`)

	time.Sleep(100 * time.Millisecond)

	// 验证连接仍然存在（无效消息被忽略）
	_, exists := cm.GetUserClientID(userID)
	if !exists {
		t.Error("user connection should NOT be removed for invalid message")
	}
}
