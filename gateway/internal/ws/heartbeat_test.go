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