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
	heartbeat       *HeartbeatManager
	connections     map[string]*websocket.Conn // clientID → conn
	userConnections map[uint32]string           // userID → clientID
	connMu          sync.RWMutex
	stopCh          chan struct{}
	metrics         ConnectionMetrics
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
		heartbeat:       NewHeartbeatManager(hbConfig),
		connections:     make(map[string]*websocket.Conn),
		userConnections: make(map[uint32]string),
		stopCh:          make(chan struct{}),
		metrics:         ConnectionMetrics{},
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

		// Also clean up userConnections (find userID by clientID)
		for uid, cid := range cm.userConnections {
			if cid == clientID {
				delete(cm.userConnections, uid)
				break
			}
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

// checkAllConnections 检查所有连接的超时状态（包括心跳超时和空闲超时）
func (cm *ConnectionManager) checkAllConnections() {
	cm.connMu.Lock()
	defer cm.connMu.Unlock()

	heartbeatTimeoutClients := make([]string, 0)
	idleTimeoutClients := make([]string, 0)

	for clientID := range cm.connections {
		// 检查心跳超时
		if cm.heartbeat.CheckTimeout(clientID) {
			heartbeatTimeoutClients = append(heartbeatTimeoutClients, clientID)
			continue
		}

		// 检查空闲超时（如果有配置）
		if cm.heartbeat.config.IdleTimeout > 0 {
			state := cm.heartbeat.GetState(clientID)
			if state != nil && !state.lastActivity.IsZero() {
				if time.Since(state.lastActivity) > cm.heartbeat.config.IdleTimeout {
					idleTimeoutClients = append(idleTimeoutClients, clientID)
				}
			}
		}
	}

	// 断开心跳超时的连接
	for _, clientID := range heartbeatTimeoutClients {
		if conn, exists := cm.connections[clientID]; exists {
			cm.disconnectClientUnlocked(clientID, conn, "heartbeat timeout")
			cm.metrics.HeartbeatFailures++
		}
	}

	// 断开空闲超时的连接
	for _, clientID := range idleTimeoutClients {
		if conn, exists := cm.connections[clientID]; exists {
			cm.disconnectClientUnlocked(clientID, conn, "idle timeout")
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

// RecordActivity 记录客户端活动（用于空闲检测）
func (cm *ConnectionManager) RecordActivity(clientID string) {
	cm.heartbeat.RecordActivity(clientID)
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

// RegisterUserConnection 登录成功后关联 userID 与 clientID
func (cm *ConnectionManager) RegisterUserConnection(userID uint32, clientID string) {
	cm.connMu.Lock()
	defer cm.connMu.Unlock()
	cm.userConnections[userID] = clientID
	slog.Debug("user connection registered", "user_id", userID, "client_id", clientID)
}

// GetUserClientID 根据 userID 查找 clientID
func (cm *ConnectionManager) GetUserClientID(userID uint32) (string, bool) {
	cm.connMu.RLock()
	defer cm.connMu.RUnlock()
	clientID, exists := cm.userConnections[userID]
	return clientID, exists
}

// DisconnectByUserID 按 userID 断开连接（顶号时调用）
func (cm *ConnectionManager) DisconnectByUserID(userID uint32, reason string) {
	cm.connMu.Lock()
	clientID, exists := cm.userConnections[userID]
	if !exists {
		cm.connMu.Unlock()
		return // 该网关实例没有这个用户的连接
	}

	conn, connExists := cm.connections[clientID]
	if !connExists || conn == nil {
		// 没有实际连接，只清理映射
		delete(cm.connections, clientID)
		delete(cm.userConnections, userID)
		cm.metrics.ActiveConnections--
		cm.metrics.DisconnectedCount++
		if cm.metrics.ActiveConnections < 0 {
			cm.metrics.ActiveConnections = 0
		}
		cm.connMu.Unlock()
		return
	}

	// 从 maps 中移除，这样在发送消息时不会被其他操作干扰
	delete(cm.connections, clientID)
	delete(cm.userConnections, userID)
	cm.metrics.ActiveConnections--
	cm.metrics.DisconnectedCount++
	if cm.metrics.ActiveConnections < 0 {
		cm.metrics.ActiveConnections = 0
	}
	cm.connMu.Unlock()

	// 现在不持有锁，安全地进行 WebSocket 操作
	// 1. 先发送业务消息（让客户端显示友好提示）
	kickMsg := `{"method":"session_kicked","code":1008,"msg":"session replaced"}`
	if err := conn.WriteMessage(websocket.TextMessage, []byte(kickMsg)); err != nil {
		slog.Debug("failed to send kick message", "user_id", userID, "error", err)
	}

	// 2. 短暂延迟确保消息发送
	time.Sleep(100 * time.Millisecond)

	// 3. 发送 WebSocket Close 消息
	if err := conn.WriteMessage(websocket.CloseMessage,
		websocket.FormatCloseMessage(websocket.ClosePolicyViolation, reason)); err != nil {
		slog.Debug("failed to send close message", "user_id", userID, "error", err)
	}
	conn.Close()

	slog.Info("user disconnected by session invalidate",
		"user_id", userID, "reason", reason)
}
