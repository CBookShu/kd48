package ws

import (
	"sync"
	"time"
)

type HeartbeatConfig struct {
	Interval     time.Duration // 心跳间隔
	Timeout      time.Duration // 心跳超时
	MaxMissed    int           // 最大丢失次数
}

type HeartbeatManager struct {
	config      HeartbeatConfig
	connections map[string]*connectionState
	mu          sync.RWMutex
}

type connectionState struct {
	lastPing     time.Time
	lastPong     time.Time
	missedCount  int
	isAlive      bool
}

func NewHeartbeatManager(config HeartbeatConfig) *HeartbeatManager {
	return &HeartbeatManager{
		config:      config,
		connections: make(map[string]*connectionState),
	}
}