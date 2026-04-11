package ws

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"

	userv1 "github.com/CBookShu/kd48/api/proto/user/v1"
	"github.com/CBookShu/kd48/pkg/otelkit"
	"github.com/gofiber/contrib/websocket"
)

type WsMessage struct {
	Action string          `json:"action"`
	Data   json.RawMessage `json:"data"`
}

type WsResponse struct {
	Action string      `json:"action"`
	Data   interface{} `json:"data"`
}

type Handler struct {
	userClient userv1.UserServiceClient
}

func NewHandler(userClient userv1.UserServiceClient) *Handler {
	return &Handler{userClient: userClient}
}

// ServeWS 处理 WebSocket 循环读写
func (h *Handler) ServeWS(conn *websocket.Conn) {
	// 1. 从 Fiber Locals 中获取已经握手成功的连接对象

	// 2. 【关键】脱离 fasthttp 的 context 复用池，创建长连接独享的 context
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 3. 为这个长链接生成并注入独立的 TraceID
	ctx = otelkit.InjectTraceIDToCtx(ctx)
	traceID := "N/A"
	if tid, ok := ctx.Value("trace_id").(string); ok {
		traceID = tid
	}
	slog.InfoContext(ctx, "New Fiber WS connection established", "client_ip", conn.IP(), "trace_id", traceID)

	// 4. 读循环
	var msgType int
	var msg []byte
	var err error

	for {
		if msgType, msg, err = conn.ReadMessage(); err != nil {
			slog.ErrorContext(ctx, "WS read error, connection closed", "error", err)
			break // 退出循环，触发 defer 关闭连接
		}

		if msgType != websocket.TextMessage {
			continue
		}

		var req WsMessage
		if err := json.Unmarshal(msg, &req); err != nil {
			h.sendError(ctx, conn, "invalid json format")
			continue
		}

		slog.InfoContext(ctx, "Received WS message", "action", req.Action, "payload", string(msg))

		switch req.Action {
		case "login":
			h.handleLogin(ctx, conn, req.Data)
		default:
			h.sendError(ctx, conn, "unknown action: "+req.Action)
		}
	}

	return
}

func (h *Handler) handleLogin(ctx context.Context, conn *websocket.Conn, data json.RawMessage) {
	var req userv1.LoginRequest
	if err := json.Unmarshal(data, &req); err != nil {
		h.sendError(ctx, conn, "invalid login params")
		return
	}

	resp, err := h.userClient.Login(ctx, &req)
	if err != nil {
		slog.ErrorContext(ctx, "gRPC Login failed", "error", err)
		h.sendError(ctx, conn, "internal server error")
		return
	}

	h.sendSuccess(ctx, conn, "login_reply", map[string]interface{}{
		"success": resp.Success,
		"token":   resp.Token,
	})
}

func (h *Handler) sendSuccess(ctx context.Context, conn *websocket.Conn, action string, data interface{}) {
	resp := WsResponse{Action: action, Data: data}
	bytes, _ := json.Marshal(resp)
	// Fiber websocket WriteMessage 会忽略传入的 context，这里仅为了日志记录
	if err := conn.WriteMessage(websocket.TextMessage, bytes); err != nil {
		slog.ErrorContext(ctx, "WS write error", "error", err)
	}
}

func (h *Handler) sendError(ctx context.Context, conn *websocket.Conn, msg string) {
	h.sendSuccess(ctx, conn, "error_reply", map[string]string{"msg": msg})
}

// 注意：需要保留一个空引入，让编译器知道我们用到了标准库的 StatusCodes (如果后续需要的话)
var _ = http.StatusOK
