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
		Timeout:   200 * time.Millisecond,
		MaxMissed: 2,
	})

	clientID := "test-client-456"
	// 先创建连接状态（通过 RecordPing，因为 RecordActivity 不会创建新连接）
	hm.RecordPing(clientID)

	// 等待第一次超时
	time.Sleep(300 * time.Millisecond)

	// 第一次检查：应该增加 missedCount 到 1，但还不会断开连接
	shouldDisconnect := hm.CheckTimeout(clientID)
	if shouldDisconnect {
		t.Error("should not disconnect after first timeout (MaxMissed=2, missedCount=1)")
	}

	// 验证 missedCount 已经增加
	state := hm.GetState(clientID)
	if state == nil {
		t.Fatal("state should exist")
	}
	if state.missedCount != 1 {
		t.Errorf("expected missedCount=1 after first check, got %d", state.missedCount)
	}

	// 再次调用 CheckTimeout - 由于时间仍然超过 Timeout，应该再次增加 missedCount
	shouldDisconnect = hm.CheckTimeout(clientID)
	if !shouldDisconnect {
		t.Errorf("should disconnect after second timeout (missedCount should be 2)")
	}
}

func TestHeartbeatManager_RecordActivity(t *testing.T) {
	hm := NewHeartbeatManager(HeartbeatConfig{
		Interval:  30 * time.Second,
		Timeout:   10 * time.Second,
		MaxMissed: 3,
	})

	clientID := "test-client-activity"
	// RecordActivity 只更新已存在的状态，需要先创建
	hm.RecordPing(clientID) // 先创建连接

	// 记录活动
	hm.RecordActivity(clientID)

	// 验证 lastActivity 已更新
	state := hm.GetState(clientID)
	if state == nil {
		t.Fatal("connection state should exist")
	}
	if state.lastActivity.IsZero() {
		t.Error("lastActivity should be set after RecordActivity")
	}
	if !state.lastActivity.After(time.Now().Add(-1 * time.Second)) {
		t.Error("lastActivity should be recent")
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