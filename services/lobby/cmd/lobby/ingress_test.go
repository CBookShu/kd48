// services/lobby/cmd/lobby/ingress_test.go
package main

import (
	"context"
	"testing"

	gatewayv1 "github.com/CBookShu/kd48/api/proto/gateway/v1"
	lobbyv1 "github.com/CBookShu/kd48/api/proto/lobby/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Mock implementations

type mockLobbyService struct {
	pingCalled bool
	pingErr    error
}

func (m *mockLobbyService) Ping(ctx context.Context, req *lobbyv1.PingRequest) (*lobbyv1.PingReply, error) {
	m.pingCalled = true
	if m.pingErr != nil {
		return nil, m.pingErr
	}
	return &lobbyv1.PingReply{ConfigRevision: 1}, nil
}

type mockCheckinService struct {
	checkinCalled   bool
	getStatusCalled bool
	checkinErr      error
	getStatusErr    error
	lastUserID      int64
}

func (m *mockCheckinService) Checkin(ctx context.Context, req *lobbyv1.CheckinRequest) (*lobbyv1.CheckinData, error) {
	m.checkinCalled = true
	if m.checkinErr != nil {
		return nil, m.checkinErr
	}
	// 提取 user_id 用于验证
	if userID, ok := ctx.Value("user_id").(int64); ok {
		m.lastUserID = userID
	}
	return &lobbyv1.CheckinData{
		ContinuousDays: 1,
		Rewards:        map[int32]int64{1001: 100},
	}, nil
}

func (m *mockCheckinService) GetStatus(ctx context.Context, req *lobbyv1.GetStatusRequest) (*lobbyv1.CheckinStatusData, error) {
	m.getStatusCalled = true
	if m.getStatusErr != nil {
		return nil, m.getStatusErr
	}
	if userID, ok := ctx.Value("user_id").(int64); ok {
		m.lastUserID = userID
	}
	return &lobbyv1.CheckinStatusData{
		PeriodId:     1,
		PeriodName:   "Test Period",
		TodayChecked: false,
	}, nil
}

type mockItemService struct {
	getMyItemsCalled bool
	getMyItemsErr    error
	lastUserID       int64
}

func (m *mockItemService) GetMyItems(ctx context.Context, req *lobbyv1.GetMyItemsRequest) (*lobbyv1.MyItemsData, error) {
	m.getMyItemsCalled = true
	if m.getMyItemsErr != nil {
		return nil, m.getMyItemsErr
	}
	if userID, ok := ctx.Value("user_id").(int64); ok {
		m.lastUserID = userID
	}
	return &lobbyv1.MyItemsData{
		Items: map[int32]int64{1001: 1000},
	}, nil
}

// Tests

func TestIngressServer_Call_Ping(t *testing.T) {
	mockLobby := &mockLobbyService{}
	ingress := newIngressServer(mockLobby, nil, nil)

	req := &gatewayv1.IngressRequest{
		Route:       "/lobby.v1.LobbyService/Ping",
		JsonPayload: []byte(`{}`),
	}

	reply, err := ingress.Call(context.Background(), req)
	require.NoError(t, err)
	assert.True(t, mockLobby.pingCalled)
	assert.NotNil(t, reply.JsonPayload)
}

func TestIngressServer_Call_Checkin(t *testing.T) {
	mockCheckin := &mockCheckinService{}
	ingress := newIngressServer(nil, mockCheckin, nil)

	ctx := context.WithValue(context.Background(), "user_id", int64(12345))
	req := &gatewayv1.IngressRequest{
		Route:       "/lobby.v1.CheckinService/Checkin",
		JsonPayload: []byte(`{}`),
	}

	reply, err := ingress.Call(ctx, req)
	require.NoError(t, err)
	assert.True(t, mockCheckin.checkinCalled)
	assert.Equal(t, int64(12345), mockCheckin.lastUserID)
	assert.NotNil(t, reply.JsonPayload)
}

func TestIngressServer_Call_GetStatus(t *testing.T) {
	mockCheckin := &mockCheckinService{}
	ingress := newIngressServer(nil, mockCheckin, nil)

	ctx := context.WithValue(context.Background(), "user_id", int64(12345))
	req := &gatewayv1.IngressRequest{
		Route:       "/lobby.v1.CheckinService/GetStatus",
		JsonPayload: []byte(`{}`),
	}

	reply, err := ingress.Call(ctx, req)
	require.NoError(t, err)
	assert.True(t, mockCheckin.getStatusCalled)
	assert.Equal(t, int64(12345), mockCheckin.lastUserID)
	assert.NotNil(t, reply.JsonPayload)
}

func TestIngressServer_Call_GetMyItems(t *testing.T) {
	mockItem := &mockItemService{}
	ingress := newIngressServer(nil, nil, mockItem)

	ctx := context.WithValue(context.Background(), "user_id", int64(12345))
	req := &gatewayv1.IngressRequest{
		Route:       "/lobby.v1.ItemService/GetMyItems",
		JsonPayload: []byte(`{}`),
	}

	reply, err := ingress.Call(ctx, req)
	require.NoError(t, err)
	assert.True(t, mockItem.getMyItemsCalled)
	assert.Equal(t, int64(12345), mockItem.lastUserID)
	assert.NotNil(t, reply.JsonPayload)
}

func TestIngressServer_Call_UnknownRoute(t *testing.T) {
	ingress := newIngressServer(nil, nil, nil)

	req := &gatewayv1.IngressRequest{
		Route:       "/unknown/Route",
		JsonPayload: []byte(`{}`),
	}

	_, err := ingress.Call(context.Background(), req)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown ingress route")
}

func TestIngressServer_Call_InvalidJSON(t *testing.T) {
	mockCheckin := &mockCheckinService{}
	ingress := newIngressServer(nil, mockCheckin, nil)

	req := &gatewayv1.IngressRequest{
		Route:       "/lobby.v1.CheckinService/Checkin",
		JsonPayload: []byte(`invalid json`),
	}

	_, err := ingress.Call(context.Background(), req)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid json")
}

func TestIngressServer_Call_ServiceNotConfigured(t *testing.T) {
	// 没有配置任何服务
	ingress := newIngressServer(nil, nil, nil)

	tests := []struct {
		name  string
		route string
	}{
		{"Lobby Ping", "/lobby.v1.LobbyService/Ping"},
		{"Checkin", "/lobby.v1.CheckinService/Checkin"},
		{"GetStatus", "/lobby.v1.CheckinService/GetStatus"},
		{"GetMyItems", "/lobby.v1.ItemService/GetMyItems"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := &gatewayv1.IngressRequest{
				Route:       tt.route,
				JsonPayload: []byte(`{}`),
			}
			_, err := ingress.Call(context.Background(), req)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "not configured")
		})
	}
}
