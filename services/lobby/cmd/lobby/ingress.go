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

// checkinService 签到服务接口
type checkinService interface {
	Checkin(ctx context.Context, req *lobbyv1.CheckinRequest) (*lobbyv1.CheckinData, error)
	GetStatus(ctx context.Context, req *lobbyv1.GetStatusRequest) (*lobbyv1.CheckinStatusData, error)
}

// itemService 物品服务接口
type itemService interface {
	GetMyItems(ctx context.Context, req *lobbyv1.GetMyItemsRequest) (*lobbyv1.MyItemsData, error)
}

type ingressServer struct {
	gatewayv1.UnimplementedGatewayIngressServer
	lobbySvc   lobbyPing
	checkinSvc checkinService
	itemSvc    itemService
}

func newIngressServer(lobbySvc lobbyPing, checkinSvc checkinService, itemSvc itemService) *ingressServer {
	return &ingressServer{
		lobbySvc:   lobbySvc,
		checkinSvc: checkinSvc,
		itemSvc:    itemSvc,
	}
}

func (s *ingressServer) Call(ctx context.Context, req *gatewayv1.IngressRequest) (*gatewayv1.IngressReply, error) {
	switch req.GetRoute() {
	case "/lobby.v1.LobbyService/Ping":
		if s.lobbySvc == nil {
			return nil, status.Error(codes.Internal, "ingress lobby service not configured")
		}
		var in lobbyv1.PingRequest
		if err := protojson.Unmarshal(req.GetJsonPayload(), &in); err != nil {
			return nil, status.Errorf(codes.InvalidArgument, "invalid json: %v", err)
		}
		out, err := s.lobbySvc.Ping(ctx, &in)
		if err != nil {
			return nil, err
		}
		b, err := protojson.Marshal(out)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "marshal reply: %v", err)
		}
		return &gatewayv1.IngressReply{JsonPayload: b}, nil

	case "/lobby.v1.CheckinService/Checkin":
		if s.checkinSvc == nil {
			return nil, status.Error(codes.Internal, "ingress checkin service not configured")
		}
		var in lobbyv1.CheckinRequest
		if err := protojson.Unmarshal(req.GetJsonPayload(), &in); err != nil {
			return nil, status.Errorf(codes.InvalidArgument, "invalid json: %v", err)
		}
		out, err := s.checkinSvc.Checkin(ctx, &in)
		if err != nil {
			return nil, err
		}
		b, err := protojson.Marshal(out)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "marshal reply: %v", err)
		}
		return &gatewayv1.IngressReply{JsonPayload: b}, nil

	case "/lobby.v1.CheckinService/GetStatus":
		if s.checkinSvc == nil {
			return nil, status.Error(codes.Internal, "ingress checkin service not configured")
		}
		var in lobbyv1.GetStatusRequest
		if err := protojson.Unmarshal(req.GetJsonPayload(), &in); err != nil {
			return nil, status.Errorf(codes.InvalidArgument, "invalid json: %v", err)
		}
		out, err := s.checkinSvc.GetStatus(ctx, &in)
		if err != nil {
			return nil, err
		}
		b, err := protojson.Marshal(out)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "marshal reply: %v", err)
		}
		return &gatewayv1.IngressReply{JsonPayload: b}, nil

	case "/lobby.v1.ItemService/GetMyItems":
		if s.itemSvc == nil {
			return nil, status.Error(codes.Internal, "ingress item service not configured")
		}
		var in lobbyv1.GetMyItemsRequest
		if err := protojson.Unmarshal(req.GetJsonPayload(), &in); err != nil {
			return nil, status.Errorf(codes.InvalidArgument, "invalid json: %v", err)
		}
		out, err := s.itemSvc.GetMyItems(ctx, &in)
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
