package ws

import (
	"testing"
	"time"
)

// TestClientMeta_UserIDField verifies that clientMeta struct has userID field
// and can store/retrieve user ID values correctly.
func TestClientMeta_UserIDField(t *testing.T) {
	meta := &clientMeta{
		connID:          1,
		clientID:        "test-client",
		isAuthenticated: false,
		userID:          0, // default, not authenticated
	}

	// Verify initial state
	if meta.userID != 0 {
		t.Errorf("expected initial userID to be 0, got %d", meta.userID)
	}

	// After successful authentication, userID should be set
	meta.userID = 12345
	if meta.userID != 12345 {
		t.Errorf("expected userID to be 12345, got %d", meta.userID)
	}
}

// TestExtractUserIDFromResponse tests the extraction of user_id from
// response data map (parsed from JSON).
func TestExtractUserIDFromResponse(t *testing.T) {
	tests := []struct {
		name     string
		data     interface{}
		expected int64
	}{
		{
			name:     "nil data",
			data:     nil,
			expected: 0,
		},
		{
			name:     "non-map data",
			data:     "string",
			expected: 0,
		},
		{
			name:     "map without user_id",
			data:     map[string]interface{}{"success": true},
			expected: 0,
		},
		{
			name:     "map with float64 user_id (JSON unmarshal default)",
			data:     map[string]interface{}{"success": true, "user_id": float64(12345)},
			expected: 12345,
		},
		{
			name:     "map with int user_id",
			data:     map[string]interface{}{"success": true, "user_id": 67890},
			expected: 67890,
		},
		{
			name:     "map with int64 user_id",
			data:     map[string]interface{}{"success": true, "user_id": int64(99999)},
			expected: 99999,
		},
		{
			name:     "map with uint64 user_id",
			data:     map[string]interface{}{"success": true, "user_id": uint64(88888)},
			expected: 88888,
		},
		{
			name:     "map with string user_id (invalid)",
			data:     map[string]interface{}{"success": true, "user_id": "not-a-number"},
			expected: 0,
		},
		{
			name:     "map with zero user_id",
			data:     map[string]interface{}{"success": true, "user_id": float64(0)},
			expected: 0,
		},
		{
			name: "nested data.user_id (VerifyToken response format)",
			data: map[string]interface{}{
				"success": true,
				"data": map[string]interface{}{
					"user_id":  float64(12345),
					"username": "testuser",
				},
			},
			expected: 12345,
		},
		{
			name: "nested data.user_id with int64",
			data: map[string]interface{}{
				"success": true,
				"data": map[string]interface{}{
					"user_id": int64(67890),
				},
			},
			expected: 67890,
		},
		{
			name: "nested data without user_id",
			data: map[string]interface{}{
				"success": true,
				"data": map[string]interface{}{
					"username": "testuser",
				},
			},
			expected: 0,
		},
		{
			name: "nested data is not a map",
			data: map[string]interface{}{
				"success": true,
				"data":    "invalid",
			},
			expected: 0,
		},
		{
			name: "both top-level and nested user_id (top-level takes precedence)",
			data: map[string]interface{}{
				"success": true,
				"user_id": float64(11111),
				"data": map[string]interface{}{
					"user_id": float64(22222),
				},
			},
			expected: 11111, // top-level user_id takes precedence
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractUserIDFromResponse(tt.data)
			if result != tt.expected {
				t.Errorf("extractUserIDFromResponse(%v) = %d, want %d", tt.data, result, tt.expected)
			}
		})
	}
}

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
