package ws

import (
	"context"
	"testing"
	"time"
)

func TestConnectionManager_New(t *testing.T) {
	cm := NewConnectionManager(HeartbeatConfig{
		Interval:  30 * time.Second,
		Timeout:   10 * time.Second,
		MaxMissed: 3,
	})

	if cm == nil {
		t.Fatal("ConnectionManager should not be nil")
	}

	// 检查指标初始值
	metrics := cm.GetMetrics()
	if metrics.TotalConnections != 0 {
		t.Errorf("expected 0 total connections, got %d", metrics.TotalConnections)
	}
	if metrics.ActiveConnections != 0 {
		t.Errorf("expected 0 active connections, got %d", metrics.ActiveConnections)
	}
}

func TestConnectionManager_RegisterConnection(t *testing.T) {
	cm := NewConnectionManager(HeartbeatConfig{
		Interval:  30 * time.Second,
		Timeout:   10 * time.Second,
		MaxMissed: 3,
	})

	clientID := "test-client-789"

	// 模拟WebSocket连接（为nil也可以注册）
	registered := cm.RegisterConnection(clientID, nil)

	if !registered {
		t.Error("should successfully register connection")
	}

	metrics := cm.GetMetrics()
	if metrics.TotalConnections != 1 {
		t.Errorf("expected 1 total connection, got %d", metrics.TotalConnections)
	}
	if metrics.ActiveConnections != 1 {
		t.Errorf("expected 1 active connection, got %d", metrics.ActiveConnections)
	}

	// 验证重复注册会失败
	registered2 := cm.RegisterConnection(clientID, nil)
	if registered2 {
		t.Error("should reject duplicate connection registration")
	}

	// 验证总数不变
	metrics2 := cm.GetMetrics()
	if metrics2.TotalConnections != 1 {
		t.Errorf("expected still 1 total connection after duplicate, got %d", metrics2.TotalConnections)
	}
}

func TestConnectionManager_UnregisterConnection(t *testing.T) {
	cm := NewConnectionManager(HeartbeatConfig{
		Interval:  30 * time.Second,
		Timeout:   10 * time.Second,
		MaxMissed: 3,
	})

	clientID := "test-client-unregister"

	// 注册
	cm.RegisterConnection(clientID, nil)

	// 注销
	cm.UnregisterConnection(clientID)

	metrics := cm.GetMetrics()
	if metrics.ActiveConnections != 0 {
		t.Errorf("expected 0 active connections after unregister, got %d", metrics.ActiveConnections)
	}

	// 重复注销不应出错或影响计数
	cm.UnregisterConnection(clientID)
	cm.UnregisterConnection("non-existent")

	metrics2 := cm.GetMetrics()
	if metrics2.ActiveConnections != 0 {
		t.Errorf("expected 0 active connections, got %d", metrics2.ActiveConnections)
	}
}

func TestConnectionManager_StartStop(t *testing.T) {
	cm := NewConnectionManager(HeartbeatConfig{
		Interval:  100 * time.Millisecond,
		Timeout:   50 * time.Millisecond,
		MaxMissed: 2,
	})

	// 启动管理器
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	go cm.Start(ctx)

	// 等待一段时间确保定时器运行
	time.Sleep(150 * time.Millisecond)

	// 停止应该成功（虽然ctx已经过期）
	cm.Stop()
}

func TestConnectionManager_MultipleClients(t *testing.T) {
	cm := NewConnectionManager(HeartbeatConfig{
		Interval:  30 * time.Second,
		Timeout:   10 * time.Second,
		MaxMissed: 3,
	})

	// 注册多个客户端
	clientIDs := []string{"client-1", "client-2", "client-3"}
	for _, id := range clientIDs {
		if !cm.RegisterConnection(id, nil) {
			t.Fatalf("failed to register client %s", id)
		}
	}

	// 验证计数
	metrics := cm.GetMetrics()
	if metrics.TotalConnections != 3 {
		t.Errorf("expected 3 total connections, got %d", metrics.TotalConnections)
	}
	if metrics.ActiveConnections != 3 {
		t.Errorf("expected 3 active connections, got %d", metrics.ActiveConnections)
	}

	// 验证活跃连接数
	if count := cm.GetActiveConnectionCount(); count != 3 {
		t.Errorf("expected 3 active connection count, got %d", count)
	}

	// 注销一个
	cm.UnregisterConnection("client-2")

	// 验证计数
	metrics2 := cm.GetMetrics()
	if metrics2.ActiveConnections != 2 {
		t.Errorf("expected 2 active connections, got %d", metrics2.ActiveConnections)
	}
	// TotalConnections 不应该减少
	if metrics2.TotalConnections != 3 {
		t.Errorf("expected 3 total connections (cumulative), got %d", metrics2.TotalConnections)
	}
}

func TestConnectionManager_RecordPingThroughManager(t *testing.T) {
	cm := NewConnectionManager(HeartbeatConfig{
		Interval:  30 * time.Second,
		Timeout:   10 * time.Second,
		MaxMissed: 3,
	})

	clientID := "test-ping-client"
	cm.RegisterConnection(clientID, nil)

	// 通过 ConnectionManager 记录 ping
	cm.RecordPing(clientID)

	// 验证心跳状态
	state := cm.GetHeartbeatState(clientID)
	if state == nil {
		t.Fatal("heartbeat state should exist")
	}

	if state.lastPing.IsZero() {
		t.Error("lastPing should be set after RecordPing")
	}
}

func TestConnectionManager_HeartbeatTimeout(t *testing.T) {
	cm := NewConnectionManager(HeartbeatConfig{
		Interval:  1 * time.Second,
		Timeout:   300 * time.Millisecond,
		MaxMissed: 2,
	})

	clientID := "test-timeout-client"
	cm.RegisterConnection(clientID, nil)

	// 先发送一次 ping，设置 lastPing
	cm.RecordPing(clientID)

	// 等待超过超时时间
	time.Sleep(400 * time.Millisecond)

	// 第一次检查：应该增加 missedCount 到 1，但还不会断开
	shouldDisconnect1 := cm.heartbeat.CheckTimeout(clientID)
	if shouldDisconnect1 {
		t.Error("should NOT disconnect after first timeout check (MaxMissed=2, missedCount=1)")
	}

	// 验证 missedCount
	state1 := cm.GetHeartbeatState(clientID)
	if state1 == nil {
		t.Fatal("state should exist")
	}
	if state1.missedCount != 1 {
		t.Errorf("expected missedCount=1 after first check, got %d", state1.missedCount)
	}

	// 第二次检查：应该增加 missedCount 到 2 并触发断开
	shouldDisconnect2 := cm.heartbeat.CheckTimeout(clientID)
	if !shouldDisconnect2 {
		t.Error("should indicate disconnect after second timeout check (MaxMissed=2, missedCount=2)")
	}
}
