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

func (hm *HeartbeatManager) RecordPing(clientID string) {
	hm.mu.Lock()
	defer hm.mu.Unlock()

	if state, exists := hm.connections[clientID]; exists {
		state.lastPing = time.Now()
		state.missedCount = 0
	} else {
		hm.connections[clientID] = &connectionState{
			lastPing:    time.Now(),
			lastPong:    time.Time{},
			missedCount: 0,
			isAlive:     true,
		}
	}
}

func (hm *HeartbeatManager) GetState(clientID string) *connectionState {
	hm.mu.RLock()
	defer hm.mu.RUnlock()

	return hm.connections[clientID]
}

func (hm *HeartbeatManager) CheckTimeout(clientID string) bool {
	hm.mu.Lock()
	defer hm.mu.Unlock()

	state, exists := hm.connections[clientID]
	if !exists || !state.isAlive {
		return false
	}

	// 检查是否超过超时时间
	if time.Since(state.lastPing) > hm.config.Timeout {
		state.missedCount++
		if state.missedCount >= hm.config.MaxMissed {
			state.isAlive = false
			return true // 需要断开连接
		}
	}

	return false
}