package main

import (
	"context"

	gatewayv1 "github.com/CBookShu/kd48/api/proto/gateway/v1"
	userv1 "github.com/CBookShu/kd48/api/proto/user/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"
)

// userLoginRegister 便于 Ingress 与单测 mock 共用。
type userLoginRegister interface {
	Login(ctx context.Context, req *userv1.LoginRequest) (*userv1.LoginReply, error)
	Register(ctx context.Context, req *userv1.RegisterRequest) (*userv1.RegisterReply, error)
	VerifyToken(ctx context.Context, req *userv1.VerifyTokenRequest) (*userv1.VerifyTokenReply, error)
}

type ingressServer struct {
	gatewayv1.UnimplementedGatewayIngressServer
	inner userLoginRegister
}

func newIngressServer(inner userLoginRegister) *ingressServer {
	return &ingressServer{inner: inner}
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
		b, err := protojson.Marshal(out)
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
		b, err := protojson.Marshal(out)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "marshal reply: %v", err)
		}
		return &gatewayv1.IngressReply{JsonPayload: b}, nil

	case "/user.v1.UserService/VerifyToken":
		if s.inner == nil {
			return nil, status.Error(codes.Internal, "ingress inner not configured")
		}
		// No payload unmarshalling needed - user_id comes from context
		out, err := s.inner.VerifyToken(ctx, &userv1.VerifyTokenRequest{})
		if err != nil {
			return nil, err
		}
		b, err := protojson.Marshal(out)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "marshal reply: %v", err)
		}
		return &gatewayv1.IngressReply{JsonPayload: b}, nil

	default:
		return nil, status.Error(codes.InvalidArgument, "unknown ingress route")
	}
}
