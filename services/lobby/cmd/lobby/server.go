package main

import (
	"context"
	"log/slog"

	lobbyv1 "github.com/CBookShu/kd48/api/proto/lobby/v1"
	"github.com/CBookShu/kd48/pkg/dsroute"
)

// lobbyService 大厅服务实现
type lobbyService struct {
	lobbyv1.UnimplementedLobbyServiceServer
	router *dsroute.Router
}

// NewLobbyService 创建大厅服务实例
func NewLobbyService(router *dsroute.Router) *lobbyService {
	return &lobbyService{
		router: router,
	}
}

// Ping 健康检查/心跳接口
func (s *lobbyService) Ping(ctx context.Context, req *lobbyv1.PingRequest) (*lobbyv1.PingReply, error) {
	slog.InfoContext(ctx, "Received Ping request", "client_hint", req.GetClientHint())

	// 当前返回占位值，后续 Task 实现配置加载后返回实际的 config_revision
	return &lobbyv1.PingReply{
		Pong:           "pong",
		ConfigRevision: 0,
	}, nil
}
