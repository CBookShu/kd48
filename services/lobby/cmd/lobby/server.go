package main

import (
	"context"
	"database/sql"
	"log/slog"

	lobbyv1 "github.com/CBookShu/kd48/api/proto/lobby/v1"
	"github.com/redis/go-redis/v9"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// lobbyService 大厅服务实现
type lobbyService struct {
	lobbyv1.UnimplementedLobbyServiceServer
	mysqlPools map[string]*sql.DB
	redisPools map[string]redis.UniversalClient
}

// NewLobbyService 创建大厅服务实例
func NewLobbyService(
	mysqlPools map[string]*sql.DB,
	redisPools map[string]redis.UniversalClient,
) *lobbyService {
	return &lobbyService{
		mysqlPools: mysqlPools,
		redisPools: redisPools,
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

// getMySQLDB 获取指定名称的 MySQL 连接池
func (s *lobbyService) getMySQLDB(name string) (*sql.DB, error) {
	db, ok := s.mysqlPools[name]
	if !ok {
		return nil, status.Errorf(codes.Internal, "mysql pool %q not found", name)
	}
	return db, nil
}

// getRedis 获取指定名称的 Redis 客户端
func (s *lobbyService) getRedis(name string) (redis.UniversalClient, error) {
	rdb, ok := s.redisPools[name]
	if !ok {
		return nil, status.Errorf(codes.Internal, "redis pool %q not found", name)
	}
	return rdb, nil
}
