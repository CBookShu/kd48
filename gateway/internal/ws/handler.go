package ws

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gofiber/contrib/websocket"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// clientMeta 维护单个 WebSocket 连接的网关级状态
type clientMeta struct {
	connID          uint64
	conn            *websocket.Conn
	clientID        string // 客户端唯一标识（用于连接管理）
	isAuthenticated bool
	userID          int64  // 预留：后续强踢、消息路由使用
	token           string // 预留：后续强踢、会话恢复使用
}

// Handler 网关 WebSocket 处理器
type Handler struct {
	tracer         trace.Tracer
	router         *AtomicRouter        // Etcd 驱动的不可变路由快照
	clients        sync.Map              // map[connID]*clientMeta
	connCounter    atomic.Uint64
	connManager    *ConnectionManager   // 连接管理器（可选，为nil时不启用连接管理）
}

func NewHandler(tracer trace.Tracer, router *AtomicRouter, connManager *ConnectionManager) *Handler {
	return &Handler{
		tracer:      tracer,
		router:      router,
		connManager: connManager,
	}
}

// ServeWS 处理 WebSocket 循环读写
func (h *Handler) ServeWS(conn *websocket.Conn) {
	connID := h.connCounter.Add(1)
	// 生成客户端唯一标识（使用connID作为临时ID，后续认证后可替换为userID）
	clientID := fmt.Sprintf("conn-%d", connID)
	meta := &clientMeta{
		connID:   connID,
		conn:     conn,
		clientID: clientID,
	}
	h.clients.Store(connID, meta)

	// 注册连接到连接管理器（如果启用）
	if h.connManager != nil {
		h.connManager.RegisterConnection(clientID, conn)
	}

	// 【关键】脱离 fasthttp 的 context 复用池，创建长连接独享的 context
	ctx, span := h.tracer.Start(context.Background(), "WS.Session")
	defer span.End()

	defer func() {
		h.clients.Delete(connID)
		// 从连接管理器注销
		if h.connManager != nil {
			h.connManager.UnregisterConnection(clientID)
		}
		conn.Close()
		slog.InfoContext(ctx, "WebSocket connection closed and cleaned up", "conn_id", connID, "client_id", clientID)
	}()

	slog.InfoContext(ctx, "New Fiber WS connection established", "client_ip", conn.IP(), "client_id", clientID)

	handshakeTimeout := 10 * time.Second

	var msgType int
	var msg []byte
	var err error

	// 设置 Ping 处理器（如果连接管理器已启用）
	// 当服务器向客户端发送 Ping 时，客户端回复 Pong，此 handler 被调用
	// 此处记录客户端的响应（表示连接仍活跃）
	if h.connManager != nil {
		conn.SetPingHandler(func(appData string) error {
			// 收到客户端的 Ping，记录心跳
			h.connManager.RecordPing(clientID)
			// 回复 Pong
			return conn.WriteMessage(websocket.PongMessage, []byte(appData))
		})
	}

	for {
		if meta.isAuthenticated {
			conn.SetReadDeadline(time.Time{})
		} else {
			conn.SetReadDeadline(time.Now().Add(handshakeTimeout))
		}

		if msgType, msg, err = conn.ReadMessage(); err != nil {
			slog.ErrorContext(ctx, "WS read error, connection closed", "error", err)
			break
		}

		// 处理 Pong 消息（客户端响应服务器发起的 Ping）
		// 注意：Ping 消息由 SetPingHandler 自动处理（见上文），不会到达这里
		if msgType == websocket.PongMessage {
			if h.connManager != nil {
				h.connManager.RecordPong(clientID)
			}
			continue
		}

		if msgType != websocket.TextMessage {
			slog.WarnContext(ctx, "Non-text message received, closing connection", "msg_type", msgType)
			h.sendResp(ctx, conn, "", int32(codes.InvalidArgument), "non-text message received", nil)
			break
		}

		// 1. 解析统一信封 WsRequest (定义在 router.go 中)
		var req WsRequest
		if err := json.Unmarshal(msg, &req); err != nil {
			h.sendResp(ctx, conn, "", int32(codes.InvalidArgument), "invalid json format", nil)
			break
		}

		slog.InfoContext(ctx, "Received WS message", "method", req.Method, "payload", req.Payload)

		// 2. 路由快照：先解析 Etcd 路由（§11.9：无路由则 unknown）
		route, ok := h.router.Get(req.Method)
		if !ok {
			h.sendResp(ctx, conn, req.Method, int32(codes.NotFound), "unknown method", nil)
			break
		}

		// 3. 鉴权：非 public 且未登录则拒绝
		if !route.Public && !meta.isAuthenticated {
			slog.WarnContext(ctx, "Unauthenticated client blocked by gateway", "attempted_method", req.Method)
			h.sendResp(ctx, conn, req.Method, int32(codes.Unauthenticated), "unauthorized: please login first", nil)
			break
		}

		// 4. 执行业务逻辑
		resp, err := route.Handler(ctx, []byte(req.Payload), meta)
		if err != nil {
			// 5. 统一错误透传拦截
			sts, ok := status.FromError(err)
			if ok {
				h.sendResp(ctx, conn, req.Method, int32(sts.Code()), sts.Message(), nil)
			} else {
				// protojson 解析错误或其他底层灾难
				h.sendResp(ctx, conn, req.Method, int32(codes.Internal), err.Error(), nil)
			}
			if route.EstablishSession {
				break
			}
			continue
		}

		if route.EstablishSession {
			meta.isAuthenticated = true
		}

		// 6. 成功响应：与 response_data.go 单测对齐的转换逻辑
		data, convErr := DataFromWsHandlerResult(resp)
		if convErr != nil {
			h.sendResp(ctx, conn, req.Method, int32(codes.Internal), convErr.Error(), nil)
			if route.EstablishSession {
				break
			}
			continue
		}

		h.sendResp(ctx, conn, req.Method, int32(codes.OK), "success", data)

		// 记录客户端活动（用于空闲检测）
		if h.connManager != nil && meta.clientID != "" {
			h.connManager.RecordActivity(meta.clientID)
		}
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
