package ws

import (
	"context"
	"fmt"

	gatewayv1 "github.com/CBookShu/kd48/api/proto/gateway/v1"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

// WrapUnary 将标准的 gRPC 一元调用方法包装为 WsHandlerFunc（强类型业务 Client 用）。
// Req/Resp 须为实现 proto.Message 的指针类型。
func WrapUnary[Req proto.Message, Resp proto.Message](
	fn func(ctx context.Context, req Req, opts ...grpc.CallOption) (Resp, error),
) WsHandlerFunc {
	return func(ctx context.Context, payload []byte, meta *clientMeta) (*WsHandlerResult, error) {
		var zero Req
		req := zero.ProtoReflect().New().Interface().(Req)

		if err := protojson.Unmarshal(payload, req); err != nil {
			return nil, err
		}

		resp, err := fn(ctx, req)
		if err != nil {
			return nil, err
		}

		return &WsHandlerResult{Message: resp}, nil
	}
}

// WrapIngress 将 WS payload（UTF-8 JSON 字节）经 GatewayIngress/Call 转发至后端。
func WrapIngress(cli gatewayv1.GatewayIngressClient, route string) WsHandlerFunc {
	return func(ctx context.Context, payload []byte, meta *clientMeta) (*WsHandlerResult, error) {
		// 构建请求
		req := &gatewayv1.IngressRequest{
			Route:       route,
			JsonPayload: payload,
		}

		// 如果已认证，将 user_id 注入到 baggage
		if meta.userID > 0 {
			req.Baggage = map[string]string{
				"user_id": fmt.Sprintf("%d", meta.userID),
			}
		}

		reply, err := cli.Call(ctx, req)
		if err != nil {
			return nil, err
		}
		return &WsHandlerResult{JSON: reply.GetJsonPayload()}, nil
	}
}
