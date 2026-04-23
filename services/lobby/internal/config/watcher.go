package config

import (
	"context"
	"encoding/json"
	"log/slog"

	"github.com/CBookShu/kd48/pkg/dsroute"
	"github.com/redis/go-redis/v9"
)

// ConfigNotifyChannel Redis Pub/Sub 频道
const ConfigNotifyChannel = "kd48:lobby:config:notify"

// ConfigWatcher 订阅 Redis Pub/Sub 实现热更新
type ConfigWatcher struct {
	router     *dsroute.Router
	routingKey string
	loader     *ConfigLoader
	channel    string
}

// NewConfigWatcher 创建订阅器
func NewConfigWatcher(router *dsroute.Router, routingKey string, loader *ConfigLoader, channel string) *ConfigWatcher {
	return &ConfigWatcher{
		router:     router,
		routingKey: routingKey,
		loader:     loader,
		channel:    channel,
	}
}

// Start 启动订阅
func (w *ConfigWatcher) Start(ctx context.Context) {
	// 通过 Router 解析 Redis 连接
	rdb, poolName, err := w.router.ResolveRedis(ctx, w.routingKey)
	if err != nil {
		slog.Error("failed to resolve redis for config watcher", "error", err, "routingKey", w.routingKey)
		return
	}

	// 转换为 *redis.Client（Pub/Sub 需要具体类型）
	client, ok := rdb.(*redis.Client)
	if !ok {
		slog.Error("resolved redis is not a *redis.Client", "pool", poolName)
		return
	}

	pubsub := client.Subscribe(ctx, w.channel)
	defer pubsub.Close()

	// 检查订阅是否成功
	if _, err := pubsub.Receive(ctx); err != nil {
		slog.Error("failed to subscribe to config channel", "channel", w.channel, "error", err)
		return
	}

	slog.Info("config watcher started", "channel", w.channel, "pool", poolName)

	ch := pubsub.Channel()
	for {
		select {
		case <-ctx.Done():
			slog.Info("config watcher stopped")
			return
		case msg, ok := <-ch:
			if !ok {
				return
			}
			w.handleMessage(ctx, msg.Payload)
		}
	}
}

// handleMessage 处理热更新消息
func (w *ConfigWatcher) handleMessage(ctx context.Context, payload string) {
	var notify struct {
		ConfigName string `json:"config_name"`
		Revision   int64  `json:"revision"`
	}

	if err := json.Unmarshal([]byte(payload), &notify); err != nil {
		slog.Warn("invalid config notify message", "error", err, "payload", payload)
		return
	}

	if notify.ConfigName == "" {
		slog.Warn("config notify message missing config_name", "payload", payload)
		return
	}

	slog.Info("received config update",
		"config_name", notify.ConfigName,
		"revision", notify.Revision)

	if err := w.loader.LoadOne(ctx, notify.ConfigName); err != nil {
		slog.Error("failed to reload config",
			"config_name", notify.ConfigName,
			"error", err)
	}
}
