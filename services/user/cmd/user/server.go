package main

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	userv1 "github.com/CBookShu/kd48/api/proto/user/v1"
	"github.com/CBookShu/kd48/services/user/internal/data/sqlc"
	"github.com/go-sql-driver/mysql"
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

func (s *userService) issueSession(ctx context.Context, userID uint64, username string) (string, error) {
	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		slog.ErrorContext(ctx, "Failed to generate token", "error", err)
		return "", status.Error(codes.Internal, "internal server error")
	}
	token := hex.EncodeToString(tokenBytes)
	sessionKey := fmt.Sprintf("user:session:%s", token)
	sessionValue := fmt.Sprintf("%d:%s", userID, username)

	if err := s.rdb.Set(ctx, sessionKey, sessionValue, s.tokenTTL).Err(); err != nil {
		slog.ErrorContext(ctx, "Failed to save session to Redis", "error", err)
		return "", status.Error(codes.Internal, "internal server error")
	}
	return token, nil
}

func (s *userService) Login(ctx context.Context, req *userv1.LoginRequest) (*userv1.LoginReply, error) {
	slog.InfoContext(ctx, "Received Login request", "username", req.Username)

	user, err := s.qc.GetUserByUsername(ctx, req.Username)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, status.Error(codes.Unauthenticated, "invalid username or password")
		}
		slog.ErrorContext(ctx, "GetUserByUsername failed", "error", err)
		return nil, status.Error(codes.Internal, "internal server error")
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		return nil, status.Error(codes.Unauthenticated, "invalid username or password")
	}

	token, err := s.issueSession(ctx, user.ID, user.Username)
	if err != nil {
		return nil, err
	}

	slog.InfoContext(ctx, "User logged in successfully", "username", user.Username)

	return &userv1.LoginReply{
		Success: true,
		Token:   token,
	}, nil
}

func (s *userService) Register(ctx context.Context, req *userv1.RegisterRequest) (*userv1.RegisterReply, error) {
	slog.InfoContext(ctx, "Received Register request", "username", req.Username)

	username := strings.TrimSpace(req.Username)
	if username == "" || req.Password == "" {
		return nil, status.Error(codes.InvalidArgument, "username and password required")
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		slog.ErrorContext(ctx, "bcrypt hash failed", "error", err)
		return nil, status.Error(codes.Internal, "internal server error")
	}

	err = s.qc.CreateUser(ctx, sqlc.CreateUserParams{
		Username:     username,
		PasswordHash: string(hash),
	})
	if err != nil {
		var mysqlErr *mysql.MySQLError
		if errors.As(err, &mysqlErr) && mysqlErr.Number == 1062 {
			return nil, status.Error(codes.AlreadyExists, "username already exists")
		}
		slog.ErrorContext(ctx, "CreateUser failed", "error", err)
		return nil, status.Error(codes.Internal, "internal server error")
	}

	user, err := s.qc.GetUserByUsername(ctx, username)
	if err != nil {
		slog.ErrorContext(ctx, "GetUserByUsername after register failed", "error", err)
		return nil, status.Error(codes.Internal, "internal server error")
	}

	token, err := s.issueSession(ctx, user.ID, user.Username)
	if err != nil {
		return nil, err
	}

	slog.InfoContext(ctx, "User registered successfully", "username", user.Username)

	return &userv1.RegisterReply{
		Success: true,
		Token:   token,
	}, nil
}
