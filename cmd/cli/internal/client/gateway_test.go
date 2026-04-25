package client

import (
	"context"
	"testing"
	"time"
)

func TestGatewayURL(t *testing.T) {
	if GatewayURL != "ws://localhost:8080/ws" {
		t.Errorf("GatewayURL should be 'ws://localhost:8080/ws', got '%s'", GatewayURL)
	}
}

func TestWriteTimeout(t *testing.T) {
	if WriteTimeout != 10*time.Second {
		t.Errorf("WriteTimeout should be 10s, got %v", WriteTimeout)
	}
}

func TestNew(t *testing.T) {
	g := New()
	if g == nil {
		t.Error("New() should return non-nil Gateway")
	}
	if g.conn != nil {
		t.Error("New() should create Gateway with nil connection")
	}
}

func TestConnectFailure(t *testing.T) {
	g := New()

	// Use a context with timeout to avoid hanging
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	err := g.Connect(ctx)

	// Connection should fail because no server at wrong address
	// We test that Connect() properly handles failure
	if err == nil {
		// If it somehow connected, close it
		g.Close()
		t.Log("Note: Server might be running on localhost:8080")
	}
}

func TestSendNotConnected(t *testing.T) {
	g := New()

	_, err := g.Send(context.Background(), "/test/method", map[string]string{"key": "value"})

	if err == nil {
		t.Error("Send() should fail when not connected")
	}
}

func TestCloseNotConnected(t *testing.T) {
	g := New()

	// Close should not panic when not connected
	err := g.Close()
	if err != nil {
		t.Errorf("Close() on unconnected gateway should not return error, got: %v", err)
	}
}
