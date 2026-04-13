package ws

import (
	"context"

	"google.golang.org/protobuf/proto"
)

// WsRequest 前端统一请求信封
// method 对应 gRPC 的全路径方法名，payload 是对应 proto 结构体的 JSON 字符串
type WsRequest struct {
	Method  string `json:"method"`
	Payload string `json:"payload"`
}

// WsResponse 网关统一响应信封
type WsResponse struct {
	Method string      `json:"method"`
	Code   int32       `json:"code"` // 0 为成功，非 0 为 gRPC 标准错误码
	Msg    string      `json:"msg"`
	Data   interface{} `json:"data"`
}

// WsHandlerResult 成功响应：Message 与 JSON 二选一（互斥）。
type WsHandlerResult struct {
	Message proto.Message // 非 nil 时走 protojson → Data
	JSON    []byte        // Message==nil 且 len(JSON)>0 时直接 json.Unmarshal 到 Data
}

// WsHandlerFunc 网关业务处理器的统一签名
type WsHandlerFunc func(ctx context.Context, payload []byte, meta *clientMeta) (*WsHandlerResult, error)

// WsRouter 动态路由表
type WsRouter struct {
	handlers map[string]WsHandlerFunc
}

func NewWsRouter() *WsRouter {
	return &WsRouter{
		handlers: make(map[string]WsHandlerFunc),
	}
}

// Register 显式注册路由
func (r *WsRouter) Register(method string, handler WsHandlerFunc) {
	r.handlers[method] = handler
}

// GetHandler 获取处理器
func (r *WsRouter) GetHandler(method string) (WsHandlerFunc, bool) {
	h, ok := r.handlers[method]
	return h, ok
}
