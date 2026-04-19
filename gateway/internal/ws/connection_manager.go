package ws

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/gofiber/contrib/websocket"
)

// ConnectionManager 管理所有WebSocket连接
// 负责连接注册、心跳监控、超时断开

type ConnectionManager struct {
	heartbeat      *HeartbeatManager
	connections    map[string]*websocket.Conn
	connMu         sync.RWMutex
	stopCh         chan struct{}
	metrics        ConnectionMetrics
}

// ConnectionMetrics 连接统计指标
type ConnectionMetrics struct {
	TotalConnections   int64
	ActiveConnections  int64
	DisconnectedCount  int64
	HeartbeatFailures  int64
}

// NewConnectionManager 创建连接管理器
func NewConnectionManager(hbConfig HeartbeatConfig) *ConnectionManager {
	return &ConnectionManager{
		heartbeat:   NewHeartbeatManager(hbConfig),
		connections: make(map[string]*websocket.Conn),
		stopCh:      make(chan struct{}),
		metrics:     ConnectionMetrics{},
	}
}

// GetMetrics 获取当前连接指标（线程安全）
func (cm *ConnectionManager) GetMetrics() ConnectionMetrics {
	cm.connMu.RLock()
	defer cm.connMu.RUnlock()
	return cm.metrics
}

// RegisterConnection 注册新连接
// 如果clientID已存在，返回false（防止重复连接）
func (cm *ConnectionManager) RegisterConnection(clientID string, conn *websocket.Conn) bool {
	cm.connMu.Lock()
	defer cm.connMu.Unlock()

	// 检查是否已经存在
	if _, exists := cm.connections[clientID]; exists {
		return false
	}

	cm.connections[clientID] = conn
	cm.metrics.TotalConnections++
	cm.metrics.ActiveConnections++

	// 初始化心跳状态
	cm.heartbeat.RecordPing(clientID)

	slog.Info("connection registered",
		"client_id", clientID,
		"active_count", cm.metrics.ActiveConnections,
		"total_count", cm.metrics.TotalConnections,
	)

	return true
}

// UnregisterConnection 注销连接
func (cm *ConnectionManager) UnregisterConnection(clientID string) {
	cm.connMu.Lock()
	defer cm.connMu.Unlock()

	if _, exists := cm.connections[clientID]; exists {
		delete(cm.connections, clientID)
		cm.metrics.ActiveConnections--

		if cm.metrics.ActiveConnections < 0 {
			cm.metrics.ActiveConnections = 0
		}

		slog.Info("connection unregistered",
			"client_id", clientID,
			"active_count", cm.metrics.ActiveConnections,
		)
	}
}

// Start 启动连接管理器的后台定时任务
func (cm *ConnectionManager) Start(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Second) // 检查间隔
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			slog.Info("connection manager stopping: context done")
			return
		case <-cm.stopCh:
			slog.Info("connection manager stopping: stop signal received")
			return
		case <-ticker.C:
			cm.checkAllConnections()
		}
	}
}

// Stop 停止连接管理器
func (cm *ConnectionManager) Stop() {
	select {
	case <-cm.stopCh:
		// 已经关闭
	default:
		close(cm.stopCh)
	}
}

// checkAllConnections 检查所有连接的超时状态
func (cm *ConnectionManager) checkAllConnections() {
	cm.connMu.Lock()
	defer cm.connMu.Unlock()

	toDisconnect := make([]string, 0)

	for clientID := range cm.connections {
		if cm.heartbeat.CheckTimeout(clientID) {
			toDisconnect = append(toDisconnect, clientID)
		}
	}

	// 断开超时的连接
	for _, clientID := range toDisconnect {
		if conn, exists := cm.connections[clientID]; exists {
			cm.disconnectClientUnlocked(clientID, conn, "heartbeat timeout")
		}
	}
}

// disconnectClientUnlocked 断开客户端连接（内部使用，不持有锁）
func (cm *ConnectionManager) disconnectClientUnlocked(clientID string, conn *websocket.Conn, reason string) {
	if conn != nil {
		// 发送关闭消息
		if err := conn.WriteMessage(websocket.CloseMessage,
			websocket.FormatCloseMessage(websocket.CloseNormalClosure, reason)); err != nil {
			slog.Debug("failed to send close message", "client_id", clientID, "error", err)
		}
		conn.Close()
	}

	delete(cm.connections, clientID)
	cm.metrics.ActiveConnections--
	cm.metrics.DisconnectedCount++

	if cm.metrics.ActiveConnections < 0 {
		cm.metrics.ActiveConnections = 0
	}

	slog.Info("client disconnected",
		"client_id", clientID,
		"reason", reason,
		"active_count", cm.metrics.ActiveConnections,
	)
}

// GetConnection 获取指定客户端的连接
func (cm *ConnectionManager) GetConnection(clientID string) (*websocket.Conn, bool) {
	cm.connMu.RLock()
	defer cm.connMu.RUnlock()
	conn, exists := cm.connections[clientID]
	return conn, exists
}

// RecordPing 记录客户端ping（提供给外部调用）
func (cm *ConnectionManager) RecordPing(clientID string) {
	cm.heartbeat.RecordPing(clientID)
}

// RecordPong 记录客户端pong（提供给外部调用）
func (cm *ConnectionManager) RecordPong(clientID string) {
	cm.heartbeat.RecordPong(clientID)
}

// GetHeartbeatState 获取心跳状态（返回副本）
func (cm *ConnectionManager) GetHeartbeatState(clientID string) *connectionState {
	return cm.heartbeat.GetState(clientID)
}

// GetActiveConnectionCount 获取活跃连接数
func (cm *ConnectionManager) GetActiveConnectionCount() int {
	cm.connMu.RLock()
	defer cm.connMu.RUnlock()
	return len(cm.connections)
}
