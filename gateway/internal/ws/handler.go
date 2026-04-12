package ws

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gofiber/contrib/websocket"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"
)

// clientMeta 维护单个 WebSocket 连接的网关级状态
type clientMeta struct {
	connID          uint64
	conn            *websocket.Conn
	isAuthenticated bool
	userID          int64  // 预留：后续强踢、消息路由使用
	token           string // 预留：后续强踢、会话恢复使用
}

// Handler 网关 WebSocket 处理器
type Handler struct {
	tracer      trace.Tracer
	router      *WsRouter // 注入动态路由表
	clients     sync.Map  // map[connID]*clientMeta
	connCounter atomic.Uint64
}

// 修改点：构造函数不再依赖具体的 userClient，而是依赖通用的 router
func NewHandler(tracer trace.Tracer, router *WsRouter) *Handler {
	return &Handler{
		tracer: tracer,
		router: router,
	}
}

// ServeWS 处理 WebSocket 循环读写
func (h *Handler) ServeWS(conn *websocket.Conn) {
	connID := h.connCounter.Add(1)
	meta := &clientMeta{
		connID: connID,
		conn:   conn,
	}
	h.clients.Store(connID, meta)

	// 【关键】脱离 fasthttp 的 context 复用池，创建长连接独享的 context
	ctx, span := h.tracer.Start(context.Background(), "WS.Session")
	defer span.End()

	defer func() {
		h.clients.Delete(connID)
		conn.Close()
		slog.InfoContext(ctx, "WebSocket connection closed and cleaned up", "conn_id", connID)
	}()

	slog.InfoContext(ctx, "New Fiber WS connection established", "client_ip", conn.IP())

	handshakeTimeout := 10 * time.Second
	conn.SetReadDeadline(time.Now().Add(handshakeTimeout))

	var msgType int
	var msg []byte
	var err error

	for {
		if msgType, msg, err = conn.ReadMessage(); err != nil {
			slog.ErrorContext(ctx, "WS read error, connection closed", "error", err)
			break
		}

		if msgType != websocket.TextMessage {
			break
		}

		// 1. 解析统一信封 WsRequest (定义在 router.go 中)
		var req WsRequest
		if err := json.Unmarshal(msg, &req); err != nil {
			h.sendResp(ctx, conn, "", int32(codes.InvalidArgument), "invalid json format", nil)
			break
		}

		slog.InfoContext(ctx, "Received WS message", "method", req.Method, "payload", req.Payload)

		// 2. 🚨 网关级未认证拦截守卫 (基于方法名后缀白名单，彻底解耦业务 proto)
		allowList := []string{"/Login", "/Register"} // 放行后缀
		isAuthRoute := false
		for _, suffix := range allowList {
			if strings.HasSuffix(req.Method, suffix) {
				isAuthRoute = true
				break
			}
		}
		if !isAuthRoute && !meta.isAuthenticated {
			slog.WarnContext(ctx, "Unauthenticated client blocked by gateway", "attempted_method", req.Method)
			h.sendResp(ctx, conn, req.Method, int32(codes.Unauthenticated), "unauthorized: please login first", nil)
			break
		}

		// 3. 🚨 动态路由查找
		handler, ok := h.router.GetHandler(req.Method)
		if !ok {
			h.sendResp(ctx, conn, req.Method, int32(codes.NotFound), "unknown method", nil)
			break
		}

		// 4. 执行业务逻辑 (由 WrapUnary 包装的泛型函数处理)
		resp, err := handler(ctx, []byte(req.Payload), meta)
		if err != nil {
			// 5. 统一错误透传拦截
			sts, ok := status.FromError(err)
			if ok {
				h.sendResp(ctx, conn, req.Method, int32(sts.Code()), sts.Message(), nil)
			} else {
				// protojson 解析错误或其他底层灾难
				h.sendResp(ctx, conn, req.Method, int32(codes.Internal), err.Error(), nil)
			}
			// 登录/注册失败（业务错误），直接踢
			if isAuthRoute {
				break
			}
			continue
		}

		if isAuthRoute {
			meta.isAuthenticated = true
			conn.SetReadDeadline(time.Time{})
		}

		// 6. 成功响应：Proto 转 标准 JSON Map (防止前端拿到 proto 的特殊字段如 @type)
		var data interface{}
		if resp != nil {
			marshaler := protojson.MarshalOptions{EmitUnpopulated: true}
			jsonBytes, _ := marshaler.Marshal(resp)
			json.Unmarshal(jsonBytes, &data)
		}

		h.sendResp(ctx, conn, req.Method, int32(codes.OK), "success", data)
	}
}

// sendResp 统一收口响应方法，引用 router.go 中的 WsResponse
func (h *Handler) sendResp(ctx context.Context, conn *websocket.Conn, method string, code int32, msg string, data interface{}) {
	resp := WsResponse{
		Method: method,
		Code:   code,
		Msg:    msg,
		Data:   data,
	}
	bytes, _ := json.Marshal(resp)
	if err := conn.WriteMessage(websocket.TextMessage, bytes); err != nil {
		slog.ErrorContext(ctx, "WS write error", "error", err)
	}
}

// 注意：需要保留一个空引入，让编译器知道我们用到了标准库的 StatusCodes
var _ = http.StatusOK
