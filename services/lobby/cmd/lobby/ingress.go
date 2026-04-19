package main

import (
	"context"

	gatewayv1 "github.com/CBookShu/kd48/api/proto/gateway/v1"
	lobbyv1 "github.com/CBookShu/kd48/api/proto/lobby/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"
)

// lobbyPing 便于 Ingress 与单测 mock 共用
type lobbyPing interface {
	Ping(ctx context.Context, req *lobbyv1.PingRequest) (*lobbyv1.PingReply, error)
}

type ingressServer struct {
	gatewayv1.UnimplementedGatewayIngressServer
	inner lobbyPing
}

func newIngressServer(inner lobbyPing) *ingressServer {
	return &ingressServer{inner: inner}
}

func (s *ingressServer) Call(ctx context.Context, req *gatewayv1.IngressRequest) (*gatewayv1.IngressReply, error) {
	switch req.GetRoute() {
	case "/lobby.v1.LobbyService/Ping":
		if s.inner == nil {
			return nil, status.Error(codes.Internal, "ingress inner not configured")
		}
		var in lobbyv1.PingRequest
		if err := protojson.Unmarshal(req.GetJsonPayload(), &in); err != nil {
			return nil, status.Errorf(codes.InvalidArgument, "invalid json: %v", err)
		}
		out, err := s.inner.Ping(ctx, &in)
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
