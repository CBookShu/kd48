package main

import (
	"context"
	"log/slog"

	userv1 "github.com/CBookShu/kd48/api/proto/user/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// userService 实现 proto 定义的接口
type userService struct {
	userv1.UnimplementedUserServiceServer
}

func (s *userService) Login(ctx context.Context, req *userv1.LoginRequest) (*userv1.LoginReply, error) {
	slog.InfoContext(ctx, "Received Login request", "username", req.Username)
	// Mock 业务逻辑
	if req.Username == "admin" && req.Password == "123456" {
		return &userv1.LoginReply{
			Success: true,
			Token:   "mock-jwt-token-12345",
		}, nil
	}

	return nil, status.Error(codes.Unauthenticated, "invalid username or password")
}
