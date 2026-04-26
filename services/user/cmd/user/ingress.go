package main

import (
	"context"
	"fmt"

	gatewayv1 "github.com/CBookShu/kd48/api/proto/gateway/v1"
	userv1 "github.com/CBookShu/kd48/api/proto/user/v1"
	"github.com/redis/go-redis/v9"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"
)

// userLoginRegister 便于 Ingress 与单测 mock 共用。
type userLoginRegister interface {
	Login(ctx context.Context, req *userv1.LoginRequest) (*userv1.LoginData, error)
	Register(ctx context.Context, req *userv1.RegisterRequest) (*userv1.RegisterData, error)
	VerifyToken(ctx context.Context, req *userv1.VerifyTokenRequest) (*userv1.VerifyTokenData, error)
}

type ingressServer struct {
	gatewayv1.UnimplementedGatewayIngressServer
	inner userLoginRegister
	rdb   redis.UniversalClient // Redis client for token verification
}

func newIngressServer(inner userLoginRegister, rdb redis.UniversalClient) *ingressServer {
	return &ingressServer{inner: inner, rdb: rdb}
}

// protojsonMarshaler 配置 protojson 输出 snake_case 字段名
var protojsonMarshaler = protojson.MarshalOptions{
	EmitUnpopulated: true,
	UseProtoNames:   true,
}

func (s *ingressServer) Call(ctx context.Context, req *gatewayv1.IngressRequest) (*gatewayv1.IngressReply, error) {
	switch req.GetRoute() {
	case "/user.v1.UserService/Login":
		if s.inner == nil {
			return nil, status.Error(codes.Internal, "ingress inner not configured")
		}
		var in userv1.LoginRequest
		if err := protojson.Unmarshal(req.GetJsonPayload(), &in); err != nil {
			return nil, status.Errorf(codes.InvalidArgument, "invalid json: %v", err)
		}
		out, err := s.inner.Login(ctx, &in)
		if err != nil {
			return nil, err
		}
		b, err := protojsonMarshaler.Marshal(out)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "marshal reply: %v", err)
		}
		return &gatewayv1.IngressReply{JsonPayload: b}, nil

	case "/user.v1.UserService/Register":
		if s.inner == nil {
			return nil, status.Error(codes.Internal, "ingress inner not configured")
		}
		var in userv1.RegisterRequest
		if err := protojson.Unmarshal(req.GetJsonPayload(), &in); err != nil {
			return nil, status.Errorf(codes.InvalidArgument, "invalid json: %v", err)
		}
		out, err := s.inner.Register(ctx, &in)
		if err != nil {
			return nil, err
		}
		b, err := protojsonMarshaler.Marshal(out)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "marshal reply: %v", err)
		}
		return &gatewayv1.IngressReply{JsonPayload: b}, nil

	case "/user.v1.UserService/VerifyToken":
		if s.inner == nil {
			return nil, status.Error(codes.Internal, "ingress inner not configured")
		}

		// Check if user_id is already in context (from gateway session)
		userIDVal := ctx.Value("user_id")
		if userIDVal != nil {
			// User already authenticated via gateway session
			out, err := s.inner.VerifyToken(ctx, &userv1.VerifyTokenRequest{})
			if err != nil {
				return nil, err
			}
			b, err := protojsonMarshaler.Marshal(out)
			if err != nil {
				return nil, status.Errorf(codes.Internal, "marshal reply: %v", err)
			}
			return &gatewayv1.IngressReply{JsonPayload: b}, nil
		}

		// No user_id in context - verify token from payload
		if s.rdb == nil {
			return nil, status.Error(codes.Internal, "redis not configured")
		}

		// Parse the VerifyTokenRequest to get token
		var in userv1.VerifyTokenRequest
		if err := protojson.Unmarshal(req.GetJsonPayload(), &in); err != nil {
			return nil, status.Errorf(codes.InvalidArgument, "invalid json: %v", err)
		}

		token := in.GetToken()
		if token == "" {
			return nil, status.Error(codes.Unauthenticated, "token required")
		}

		// Verify token in Redis
		sessionKey := fmt.Sprintf("user:session:%s", token)
		sessionValue, err := s.rdb.Get(ctx, sessionKey).Result()
		if err != nil {
			return nil, status.Error(codes.Unauthenticated, "invalid or expired token")
		}

		// Parse session value (format: "userID:username")
		var userID uint32
		var username string
		if _, err := fmt.Sscanf(sessionValue, "%d:%s", &userID, &username); err != nil {
			return nil, status.Error(codes.Internal, "invalid session format")
		}

		// Create context with user_id for the service
		ctxWithUser := context.WithValue(ctx, "user_id", userID)
		out, err := s.inner.VerifyToken(ctxWithUser, &userv1.VerifyTokenRequest{})
		if err != nil {
			return nil, err
		}
		b, err := protojsonMarshaler.Marshal(out)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "marshal reply: %v", err)
		}
		return &gatewayv1.IngressReply{JsonPayload: b}, nil

	default:
		return nil, status.Error(codes.InvalidArgument, "unknown ingress route")
	}
}
