package ws

import (
	"context"
	"encoding/json"
	"log/slog"

	"github.com/redis/go-redis/v9"
)

const SessionInvalidateChannel = "kd48:session:invalidate"

// SessionInvalidationSubscriber Redis Pub/Sub 订阅器
type SessionInvalidationSubscriber struct {
	rdb         *redis.Client
	connManager *ConnectionManager
	channel     string
}

// NewSessionInvalidationSubscriber 创建订阅器
func NewSessionInvalidationSubscriber(rdb *redis.Client, cm *ConnectionManager, channel string) *SessionInvalidationSubscriber {
	return &SessionInvalidationSubscriber{
		rdb:         rdb,
		connManager: cm,
		channel:     channel,
	}
}

// Start 启动订阅
func (s *SessionInvalidationSubscriber) Start(ctx context.Context) {
	pubsub := s.rdb.Subscribe(ctx, s.channel)
	defer pubsub.Close()

	// Check subscription was successful
	if _, err := pubsub.Receive(ctx); err != nil {
		slog.Error("failed to subscribe to channel", "channel", s.channel, "error", err)
		return
	}

	slog.Info("session invalidate subscriber started", "channel", s.channel)

	ch := pubsub.Channel()
	for {
		select {
		case <-ctx.Done():
			slog.Info("session invalidate subscriber stopped")
			return
		case msg, ok := <-ch:
			if !ok {
				return
			}
			s.handleMessage(msg.Payload)
		}
	}
}

// handleMessage 处理失效消息
func (s *SessionInvalidationSubscriber) handleMessage(payload string) {
	var data struct {
		UserID int64 `json:"user_id"`
	}

	if err := json.Unmarshal([]byte(payload), &data); err != nil {
		slog.Warn("invalid session invalidate message", "error", err, "payload", payload)
		return
	}

	slog.Debug("received session invalidate", "user_id", data.UserID)
	s.connManager.DisconnectByUserID(data.UserID, "session replaced")
}
