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