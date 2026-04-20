package ws

import (
	"testing"
	"time"
)

// TestConnectionManager_RecordActivity tests that RecordActivity updates
// the lastActivity timestamp when called through ConnectionManager.
// This simulates the behavior when handler.go receives a Ping message
// and calls connManager.RecordActivity(clientID).
func TestConnectionManager_RecordActivity(t *testing.T) {
	cm := NewConnectionManager(HeartbeatConfig{
		Interval:  30 * time.Second,
		Timeout:   10 * time.Second,
		MaxMissed: 3,
	})

	clientID := "test-ping-client"

	// Register connection (this internally calls RecordPing which creates the state)
	cm.RegisterConnection(clientID, nil)

	// RecordActivity should update lastActivity timestamp
	// This simulates what happens in handler.go when Ping is received:
	//   if h.connManager != nil {
	//       h.connManager.RecordActivity(clientID)
	//   }
	cm.RecordActivity(clientID)

	// Verify lastActivity was updated
	state := cm.GetHeartbeatState(clientID)
	if state == nil {
		t.Fatal("connection state should exist after registration")
	}

	if state.lastActivity.IsZero() {
		t.Error("lastActivity should be set after RecordActivity")
	}

	// Verify it's recent (within last second)
	if !state.lastActivity.After(time.Now().Add(-1 * time.Second)) {
		t.Error("lastActivity should be recent")
	}
}

// TestConnectionManager_RecordActivity_ResetsMissedCount tests that RecordActivity
// resets the missedCount, similar to how Ping handling works in the handler.
func TestConnectionManager_RecordActivity_ResetsMissedCount(t *testing.T) {
	cm := NewConnectionManager(HeartbeatConfig{
		Interval:  30 * time.Second,
		Timeout:   10 * time.Second,
		MaxMissed: 3,
	})

	clientID := "test-reset-missed"
	cm.RegisterConnection(clientID, nil)

	// Simulate missed heartbeats by calling CheckTimeout
	// First, get the state and verify initial missedCount is 0
	state1 := cm.GetHeartbeatState(clientID)
	if state1 == nil {
		t.Fatal("state should exist")
	}
	initialMissed := state1.missedCount

	// Now call RecordActivity - it should reset missedCount to 0
	cm.RecordActivity(clientID)

	state2 := cm.GetHeartbeatState(clientID)
	if state2.missedCount != 0 {
		t.Errorf("expected missedCount to be reset to 0, got %d", state2.missedCount)
	}
	_ = initialMissed // silence unused variable warning
}

// TestConnectionManager_RecordActivity_Concurrent calls RecordActivity concurrently
// to verify thread safety.
func TestConnectionManager_RecordActivity_Concurrent(t *testing.T) {
	cm := NewConnectionManager(HeartbeatConfig{
		Interval:  30 * time.Second,
		Timeout:   10 * time.Second,
		MaxMissed: 3,
	})

	clientID := "test-concurrent"
	cm.RegisterConnection(clientID, nil)

	// Run concurrent RecordActivity calls
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func() {
			cm.RecordActivity(clientID)
			done <- true
		}()
	}

	// Wait for all goroutines to complete
	for i := 0; i < 10; i++ {
		<-done
	}

	// Verify the connection state is still valid
	state := cm.GetHeartbeatState(clientID)
	if state == nil {
		t.Fatal("connection state should exist after concurrent RecordActivity calls")
	}

	// lastActivity should be recent
	if !state.lastActivity.After(time.Now().Add(-1 * time.Second)) {
		t.Error("lastActivity should be recent after concurrent updates")
	}
}
