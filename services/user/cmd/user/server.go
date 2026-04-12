package main

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"time"

	userv1 "github.com/CBookShu/kd48/api/proto/user/v1"
	"github.com/CBookShu/kd48/services/user/internal/data/sqlc"
	"github.com/redis/go-redis/v9"
	"golang.org/x/crypto/bcrypt"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// userService 实现 proto 定义的接口
type userService struct {
	userv1.UnimplementedUserServiceServer
	qc       *sqlc.Queries
	rdb      *redis.Client
	tokenTTL time.Duration
}

func NewUserService(queries *sqlc.Queries, rdb *redis.Client, tokenTTL time.Duration) *userService {
	return &userService{
		qc:       queries,
		rdb:      rdb,
		tokenTTL: tokenTTL,
	}
}

func (s *userService) Login(ctx context.Context, req *userv1.LoginRequest) (*userv1.LoginReply, error) {
	slog.InfoContext(ctx, "Received Login request", "username", req.Username)

	user, err := s.qc.GetUserByUsername(ctx, req.Username)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			// 按照安全规范，不明确告诉前端是用户名错还是密码错
			return nil, status.Error(codes.Unauthenticated, "invalid username or password")
		}
		slog.ErrorContext(ctx, "GetUserByUsername failed", "error", err)
		return nil, status.Error(codes.Internal, "internal server error")
	}

	// 2. 密码校验 (安全比对)
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		// 密码不匹配
		return nil, status.Error(codes.Unauthenticated, "invalid username or password")
	}

	// 3. 生成极简 Token (32字节随机串)
	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		slog.ErrorContext(ctx, "Failed to generate token", "error", err)
		return nil, status.Error(codes.Internal, "internal server error")
	}
	token := hex.EncodeToString(tokenBytes)
	// 4. 存入 Redis (Key: token, Value: userId:username)
	sessionKey := fmt.Sprintf("user:session:%s", token)
	sessionValue := fmt.Sprintf("%d:%s", user.ID, user.Username)

	if err := s.rdb.Set(ctx, sessionKey, sessionValue, s.tokenTTL).Err(); err != nil {
		slog.ErrorContext(ctx, "Failed to save session to Redis", "error", err)
		return nil, status.Error(codes.Internal, "internal server error")
	}

	slog.InfoContext(ctx, "User logged in successfully", "username", user.Username)

	return &userv1.LoginReply{
		Success: true,
		Token:   token,
	}, nil
}
