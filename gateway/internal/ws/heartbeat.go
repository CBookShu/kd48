package ws

import (
	"sync"
	"time"
)

// HeartbeatConfig 心跳配置
type HeartbeatConfig struct {
	Interval     time.Duration // 心跳间隔
	Timeout      time.Duration // 心跳超时
	MaxMissed    int           // 最大丢失次数
}

// HeartbeatManager 心跳管理器
// 负责记录客户端的心跳状态并检测连接超时
type HeartbeatManager struct {
	config      HeartbeatConfig
	connections map[string]*connectionState
	mu          sync.RWMutex
}

// connectionState 连接状态
type connectionState struct {
	lastPing     time.Time // 上次收到客户端Ping的时间
	lastPong     time.Time // 上次客户端收到服务器Pong的时间（或客户端响应的时间）
	missedCount  int       // 连续超时次数
	isAlive      bool      // 连接是否存活
}

// NewHeartbeatManager 创建心跳管理器
func NewHeartbeatManager(config HeartbeatConfig) *HeartbeatManager {
	return &HeartbeatManager{
		config:      config,
		connections: make(map[string]*connectionState),
	}
}

// RecordPing 记录客户端发送的Ping
// 客户端主动向服务器发送Ping，服务器收到后记录lastPing时间
// 正常心跳流程：客户端发送Ping -> 服务器记录lastPing并回复Pong -> 客户端收到Pong后记录lastPong
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

// RecordPong 记录客户端响应的心跳Pong
// 服务器收到客户端的Ping后回复Pong，客户端收到Pong后记录lastPong
// 这表示客户端成功收到了服务器的响应，连接是活跃的
func (hm *HeartbeatManager) RecordPong(clientID string) {
	hm.mu.Lock()
	defer hm.mu.Unlock()

	if state, exists := hm.connections[clientID]; exists {
		state.lastPong = time.Now()
		state.missedCount = 0
	}
}

// GetState 获取连接状态（返回副本，避免并发安全问题）
func (hm *HeartbeatManager) GetState(clientID string) *connectionState {
	hm.mu.RLock()
	defer hm.mu.RUnlock()

	if state, exists := hm.connections[clientID]; exists {
		// 返回副本而不是内部指针，避免竞态条件
		copy := *state
		return &copy
	}
	return nil
}

// CheckTimeout 检查连接是否超时
// 检查 lastPong 时间：如果超过 Timeout 时间没有收到客户端的 Pong 响应，则认为连接可能已断开
func (hm *HeartbeatManager) CheckTimeout(clientID string) bool {
	hm.mu.Lock()
	defer hm.mu.Unlock()

	state, exists := hm.connections[clientID]
	if !exists || !state.isAlive {
		return false
	}

	// 检查是否超过超时时间（基于 lastPong 判断）
	// 如果 lastPong 为零值，说明从未收到过 Pong，使用 lastPing 判断
	checkTime := state.lastPong
	if checkTime.IsZero() {
		checkTime = state.lastPing
	}

	if time.Since(checkTime) > hm.config.Timeout {
		state.missedCount++
		if state.missedCount >= hm.config.MaxMissed {
			state.isAlive = false
			return true // 需要断开连接
		}
	}

	return false
}