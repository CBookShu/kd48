package ws

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"sync"
	"sync/atomic"

	userv1 "github.com/CBookShu/kd48/api/proto/user/v1"
	"github.com/gofiber/contrib/websocket"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc/status"
)

type WsMessage struct {
	Action string          `json:"action"`
	Data   json.RawMessage `json:"data"`
}

type WsResponse struct {
	Action string      `json:"action"`
	Data   interface{} `json:"data"`
}

// clientMeta 维护单个 WebSocket 连接的网关级状态
type clientMeta struct {
	connID          uint64
	conn            *websocket.Conn
	isAuthenticated bool
	userID          int64  // 预留：后续强踢、消息路由使用
	token           string // 预留：后续强踢、会话恢复使用
}

type Handler struct {
	userClient  userv1.UserServiceClient
	tracer      trace.Tracer
	clients     sync.Map // map[connID]*clientMeta
	connCounter atomic.Uint64
}

func NewHandler(userClient userv1.UserServiceClient, tracer trace.Tracer) *Handler {
	return &Handler{userClient: userClient, tracer: tracer}
}

// ServeWS 处理 WebSocket 循环读写
func (h *Handler) ServeWS(conn *websocket.Conn) {
	// 1. 从 Fiber Locals 中获取已经握手成功的连接对象
	connID := h.connCounter.Add(1)
	meta := &clientMeta{
		connID: connID,
		conn:   conn,
	}
	h.clients.Store(connID, meta)

	// 确保连接断开时清理内存档案
	defer func() {
		h.clients.Delete(connID)
		conn.Close()
		slog.Info("WebSocket connection closed and cleaned up", "conn_id", connID)
	}()

	// 2. 【关键】脱离 fasthttp 的 context 复用池，创建长连接独享的 context
	ctx, span := h.tracer.Start(context.Background(), "WS.Session")
	defer span.End()

	slog.InfoContext(ctx, "New Fiber WS connection established", "client_ip", conn.IP())

	// 3. 读循环
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

		// ==========================================
		// 🚨 新增：网关级未认证拦截守卫
		// ==========================================
		if req.Action != "login" && !meta.isAuthenticated {
			slog.WarnContext(ctx, "Unauthenticated client blocked by gateway", "attempted_action", req.Action)
			h.sendError(ctx, conn, "unauthorized: please login first")
			continue
		}
		// ==========================================

		switch req.Action {
		case "login":
			// 修改点：传入 meta 而不是 conn
			h.handleLogin(ctx, meta, req.Data)
		// case "ping": // 预留：心跳
		// case "chat": // 预留：聊天
		default:
			h.sendError(ctx, conn, "unknown action: "+req.Action)
		}
	}
}

// handleLogin 处理登录逻辑
// 修改点：参数 conn 变更为 meta
func (h *Handler) handleLogin(ctx context.Context, meta *clientMeta, data json.RawMessage) {
	var req userv1.LoginRequest
	if err := json.Unmarshal(data, &req); err != nil {
		h.sendError(ctx, meta.conn, "invalid login params")
		return
	}

	resp, err := h.userClient.Login(ctx, &req)
	if err != nil {
		slog.ErrorContext(ctx, "gRPC Login failed", "error", err)
		// 1. 尝试解包 gRPC 的标准错误
		sts, ok := status.FromError(err)
		if ok {
			// 解包成功：说明是后端微服务主动抛出的业务/逻辑错误
			// 直接把后端写好的真实 Message 原封不动丢给前端
			h.sendError(ctx, meta.conn, sts.Message())
		} else {
			// 解包失败：说明压根没走到后端（比如网关到后端的网络断了、DNS 解析失败等底层灾难）
			// 这种情况才给前端返回真正的 internal server error
			h.sendError(ctx, meta.conn, "gateway internal error: service unavailable")
		}
		return
	}

	// ==========================================
	// 🚨 新增：登录成功，在网关内存打上“已认证”烙印
	// ==========================================
	meta.isAuthenticated = true
	meta.token = resp.Token
	// ==========================================

	h.sendSuccess(ctx, meta.conn, "login_reply", map[string]interface{}{
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
