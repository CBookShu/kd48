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
	"github.com/CBookShu/kd48/pkg/dsroute"
	"github.com/CBookShu/kd48/services/user/internal/data/sqlc"
	"github.com/go-sql-driver/mysql"
	"golang.org/x/crypto/bcrypt"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type userService struct {
	userv1.UnimplementedUserServiceServer
	router   *dsroute.Router
	tokenTTL time.Duration
}

func NewUserService(router *dsroute.Router, tokenTTL time.Duration) *userService {
	return &userService{
		router:   router,
		tokenTTL: tokenTTL,
	}
}

const routingKeySession = "sys:session"

// loginSessionLua 原子化 Session 创建 Lua 脚本
// KEYS[1] = user:{userID}:session (userID → token 映射)
// KEYS[2] = user:session:{newToken} (token → session 数据)
// ARGV[1] = TTL (秒)
// ARGV[2] = {userID}:{username} (session 数据)
// ARGV[3] = newToken
const loginSessionLua = `
local oldToken = redis.call('GET', KEYS[1])
if oldToken and oldToken ~= ARGV[3] then
	redis.call('DEL', 'user:session:' .. oldToken)
end
redis.call('SETEX', KEYS[1], ARGV[1], ARGV[3])
redis.call('SETEX', KEYS[2], ARGV[1], ARGV[2])
return oldToken or ""
`

func (s *userService) getQueries(ctx context.Context, routingKey string) (*sqlc.Queries, error) {
	db, _, err := s.router.ResolveDB(ctx, routingKey)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "resolve db route: %v", err)
	}
	return sqlc.New(db), nil
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

	rdb, _, err := s.router.ResolveRedis(ctx, routingKeySession)
	if err != nil {
		slog.ErrorContext(ctx, "Failed to resolve redis for session", "error", err)
		return "", status.Error(codes.Internal, "internal server error")
	}

	if err := rdb.Set(ctx, sessionKey, sessionValue, s.tokenTTL).Err(); err != nil {
		slog.ErrorContext(ctx, "Failed to save session to Redis", "error", err)
		return "", status.Error(codes.Internal, "internal server error")
	}
	return token, nil
}

// issueSessionAtomic 原子化创建 Session（使用 Lua 脚本）
// 返回新 token 和是否有旧 token（用于发布 Pub/Sub）
func (s *userService) issueSessionAtomic(ctx context.Context, userID uint64, username string) (string, bool, error) {
	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		slog.ErrorContext(ctx, "Failed to generate token", "error", err)
		return "", false, status.Error(codes.Internal, "internal server error")
	}
	token := hex.EncodeToString(tokenBytes)

	userKey := fmt.Sprintf("user:%d:session", userID)
	sessionKey := fmt.Sprintf("user:session:%s", token)
	sessionValue := fmt.Sprintf("%d:%s", userID, username)

	rdb, _, err := s.router.ResolveRedis(ctx, routingKeySession)
	if err != nil {
		slog.ErrorContext(ctx, "Failed to resolve redis for session", "error", err)
		return "", false, status.Error(codes.Internal, "internal server error")
	}

	result, err := rdb.Eval(ctx, loginSessionLua,
		[]string{userKey, sessionKey},
		int64(s.tokenTTL.Seconds()), sessionValue, token).Result()
	if err != nil {
		slog.ErrorContext(ctx, "Failed to execute session Lua", "error", err)
		return "", false, status.Error(codes.Internal, "internal server error")
	}

	// 检查是否有旧 token
	oldToken, _ := result.(string)
	hasOldToken := oldToken != ""

	return token, hasOldToken, nil
}

// publishSessionInvalidate 发布 Session 失效通知
func (s *userService) publishSessionInvalidate(ctx context.Context, userID uint64) {
	rdb, _, err := s.router.ResolveRedis(ctx, routingKeySession)
	if err != nil {
		slog.WarnContext(ctx, "Failed to resolve redis for publish", "error", err)
		return
	}

	notifyData := fmt.Sprintf(`{"user_id":%d}`, userID)
	if err := rdb.Publish(ctx, "kd48:session:invalidate", notifyData).Err(); err != nil {
		slog.WarnContext(ctx, "Failed to publish session invalidate", "error", err)
	}
}

func (s *userService) Login(ctx context.Context, req *userv1.LoginRequest) (*userv1.LoginReply, error) {
	slog.InfoContext(ctx, "Received Login request", "username", req.Username)

	routingKey := "sys:user:" + req.Username

	queries, err := s.getQueries(ctx, routingKey)
	if err != nil {
		return nil, err
	}

	user, err := queries.GetUserByUsername(ctx, req.Username)
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

	token, hasOldToken, err := s.issueSessionAtomic(ctx, user.ID, user.Username)
	if err != nil {
		return nil, err
	}

	if hasOldToken {
		s.publishSessionInvalidate(ctx, user.ID)
	}

	slog.InfoContext(ctx, "User logged in successfully", "username", user.Username)

	return &userv1.LoginReply{
		Success: true,
		Token:   token,
		UserId:  user.ID,
	}, nil
}

func (s *userService) Register(ctx context.Context, req *userv1.RegisterRequest) (*userv1.RegisterReply, error) {
	slog.InfoContext(ctx, "Received Register request", "username", req.Username)

	username := strings.TrimSpace(req.Username)
	if username == "" || req.Password == "" {
		return nil, status.Error(codes.InvalidArgument, "username and password required")
	}

	routingKey := "sys:user:" + username

	queries, err := s.getQueries(ctx, routingKey)
	if err != nil {
		return nil, err
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		slog.ErrorContext(ctx, "bcrypt hash failed", "error", err)
		return nil, status.Error(codes.Internal, "internal server error")
	}

	err = queries.CreateUser(ctx, sqlc.CreateUserParams{
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

	user, err := queries.GetUserByUsername(ctx, username)
	if err != nil {
		slog.ErrorContext(ctx, "GetUserByUsername after register failed", "error", err)
		return nil, status.Error(codes.Internal, "internal server error")
	}

	token, _, err := s.issueSessionAtomic(ctx, user.ID, user.Username)
	if err != nil {
		return nil, err
	}

	slog.InfoContext(ctx, "User registered successfully", "username", user.Username)

	return &userv1.RegisterReply{
		Success: true,
		Token:   token,
		UserId:  user.ID,
	}, nil
}
