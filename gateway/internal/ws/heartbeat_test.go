package ws

import (
	"testing"
	"time"
)

func TestHeartbeatManager_New(t *testing.T) {
	config := HeartbeatConfig{
		Interval: 30 * time.Second,
		Timeout:  10 * time.Second,
		MaxMissed: 3,
	}
	hm := NewHeartbeatManager(config)

	if hm == nil {
		t.Fatal("HeartbeatManager should not be nil")
	}
}

func TestHeartbeatManager_RecordPing(t *testing.T) {
	hm := NewHeartbeatManager(HeartbeatConfig{
		Interval:  30 * time.Second,
		Timeout:   10 * time.Second,
		MaxMissed: 3,
	})

	clientID := "test-client-123"
	hm.RecordPing(clientID)

	state := hm.GetState(clientID)
	if state == nil {
		t.Fatal("connection state should exist after ping")
	}

	if !state.lastPing.After(time.Now().Add(-1*time.Second)) {
		t.Error("lastPing should be recent")
	}
}

func TestHeartbeatManager_CheckTimeout(t *testing.T) {
	hm := NewHeartbeatManager(HeartbeatConfig{
		Interval:  1 * time.Second,
		Timeout:   500 * time.Millisecond,
		MaxMissed: 2,
	})

	clientID := "test-client-456"
	hm.RecordPing(clientID)
	// 客户端收到服务器的Pong响应后记录lastPong
	hm.RecordPong(clientID)

	// 等待第一次超时
	time.Sleep(600 * time.Millisecond)

	// 第一次检查：应该增加missedCount，但还不会断开连接
	shouldDisconnect := hm.CheckTimeout(clientID)
	if shouldDisconnect {
		t.Error("should not disconnect after first timeout (MaxMissed=2)")
	}

	// 再次检查，应该触发第二次超时并断开连接
	shouldDisconnect = hm.CheckTimeout(clientID)
	if !shouldDisconnect {
		t.Error("should disconnect after second timeout")
	}
}

func TestHeartbeatManager_RecordPong(t *testing.T) {
	hm := NewHeartbeatManager(HeartbeatConfig{
		Interval:  30 * time.Second,
		Timeout:   10 * time.Second,
		MaxMissed: 3,
	})

	clientID := "test-client-pong"
	hm.RecordPing(clientID)

	// 初始状态：lastPong 为零值
	state := hm.GetState(clientID)
	if state == nil {
		t.Fatal("connection state should exist after ping")
	}
	if !state.lastPong.IsZero() {
		t.Error("lastPong should be zero before RecordPong")
	}

	// 记录 Pong
	hm.RecordPong(clientID)

	// 验证 lastPong 已更新
	state = hm.GetState(clientID)
	if state.lastPong.IsZero() {
		t.Error("lastPong should be set after RecordPong")
	}
	if !state.lastPong.After(time.Now().Add(-1 * time.Second)) {
		t.Error("lastPong should be recent")
	}
	if state.missedCount != 0 {
		t.Error("missedCount should be reset after RecordPong")
	}
}

func TestHeartbeatManager_GetStateReturnsCopy(t *testing.T) {
	hm := NewHeartbeatManager(HeartbeatConfig{
		Interval:  30 * time.Second,
		Timeout:   10 * time.Second,
		MaxMissed: 3,
	})

	clientID := "test-client-copy"
	hm.RecordPing(clientID)

	// 获取状态副本
	state1 := hm.GetState(clientID)
	if state1 == nil {
		t.Fatal("state should exist")
	}

	// 修改副本
	originalValue := state1.missedCount
	state1.missedCount = 999

	// 获取另一个副本，验证原数据未被修改
	state2 := hm.GetState(clientID)
	if state2.missedCount != originalValue {
		t.Error("GetState should return copy, original data should not be modified")
	}
}