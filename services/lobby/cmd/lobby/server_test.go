package main

import (
	"context"
	"database/sql"
	"net"
	"testing"

	lobbyv1 "github.com/CBookShu/kd48/api/proto/lobby/v1"
	"github.com/CBookShu/kd48/pkg/dsroute"
	"github.com/redis/go-redis/v9"
	"google.golang.org/grpc"
	"google.golang.org/grpc/test/bufconn"
)

// newTestLobbyRouter 创建测试用 Router
func newTestLobbyRouter(t *testing.T) *dsroute.Router {
	mysqlPools := map[string]*sql.DB{}
	redisPools := map[string]redis.UniversalClient{}

	router, err := dsroute.NewRouter(mysqlPools, redisPools, nil, nil, "lobby")
	if err != nil {
		t.Fatalf("failed to create test router: %v", err)
	}
	return router
}

func setupTestServer(t *testing.T) (lobbyv1.LobbyServiceClient, func()) {
	listener := bufconn.Listen(1024 * 1024)

	server := grpc.NewServer()

	// 创建测试 Router
	router := newTestLobbyRouter(t)

	lobbySvc := NewLobbyService(router)
	lobbyv1.RegisterLobbyServiceServer(server, lobbySvc)

	go func() {
		if err := server.Serve(listener); err != nil {
			t.Errorf("Server exited with error: %v", err)
		}
	}()

	bufDialer := func(context.Context, string) (net.Conn, error) {
		return listener.Dial()
	}

	ctx := context.Background()
	conn, err := grpc.DialContext(ctx, "bufnet",
		grpc.WithContextDialer(bufDialer),
		grpc.WithInsecure())
	if err != nil {
		t.Fatalf("Failed to dial bufnet: %v", err)
	}

	client := lobbyv1.NewLobbyServiceClient(conn)

	cleanup := func() {
		conn.Close()
		server.Stop()
		listener.Close()
	}

	return client, cleanup
}

func TestPing(t *testing.T) {
	client, cleanup := setupTestServer(t)
	defer cleanup()

	ctx := context.Background()
	req := &lobbyv1.PingRequest{
		ClientHint: stringPtr("test-client"),
	}

	resp, err := client.Ping(ctx, req)
	if err != nil {
		t.Fatalf("Ping failed: %v", err)
	}

	// 验证 pong 返回值
	if resp.GetPong() != "pong" {
		t.Errorf("Expected pong to be 'pong', got %q", resp.GetPong())
	}

	// 验证 config_revision 为 0（占位值）
	if resp.GetConfigRevision() != 0 {
		t.Errorf("Expected config_revision to be 0, got %d", resp.GetConfigRevision())
	}
}

func TestPingWithoutClientHint(t *testing.T) {
	client, cleanup := setupTestServer(t)
	defer cleanup()

	ctx := context.Background()
	req := &lobbyv1.PingRequest{} // 不使用 client_hint

	resp, err := client.Ping(ctx, req)
	if err != nil {
		t.Fatalf("Ping failed: %v", err)
	}

	if resp.GetPong() != "pong" {
		t.Errorf("Expected pong to be 'pong', got %q", resp.GetPong())
	}

	if resp.GetConfigRevision() != 0 {
		t.Errorf("Expected config_revision to be 0, got %d", resp.GetConfigRevision())
	}
}

func stringPtr(s string) *string {
	return &s
}
