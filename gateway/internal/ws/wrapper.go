package ws

import (
	"context"

	"google.golang.org/grpc"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

// WrapUnary 将标准的 gRPC 一元调用方法包装为 WsHandlerFunc
// Req 必须是实现 proto.Message 的指针类型 (如 *userv1.LoginRequest)
// Resp 必须是实现 proto.Message 的指针类型 (如 *userv1.LoginReply)
func WrapUnary[Req proto.Message, Resp proto.Message](
	fn func(ctx context.Context, req Req, opts ...grpc.CallOption) (Resp, error),
) WsHandlerFunc {
	return func(ctx context.Context, payload []byte, meta *clientMeta) (proto.Message, error) {
		// 1. 实例化空的 Request 指针
		var zero Req
		req := zero.ProtoReflect().New().Interface().(Req)

		// 2. 神奇的 protojson 反序列化：
		// 即使 req 是 nil 指针，protojson 也能通过反射自动 new 出底层对象并赋值
		if err := protojson.Unmarshal(payload, req); err != nil {
			return nil, err
		}

		// 3. 执行真实的 gRPC 调用
		resp, err := fn(ctx, req)
		if err != nil {
			return nil, err
		}

		// 4. 返回 proto.Message 接口
		return resp, nil
	}
}
