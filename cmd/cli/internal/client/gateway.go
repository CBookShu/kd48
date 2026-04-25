package client

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

const (
	// GatewayURL WebSocket Gateway 地址
	GatewayURL = "ws://localhost:8080/ws"
	// WriteTimeout WebSocket 写超时
	WriteTimeout = 10 * time.Second
	// ReadTimeout WebSocket 读超时
	ReadTimeout = 30 * time.Second
	// PongWait Pong 等待时间
	PongWait = 60 * time.Second
)

// WsRequest WebSocket 请求
type WsRequest struct {
	Method  string `json:"method"`
	Payload string `json:"payload"`
}

// WsResponse WebSocket 响应
type WsResponse struct {
	Method string      `json:"method"`
	Code   int32       `json:"code"`
	Msg    string      `json:"msg"`
	Data   interface{} `json:"data"`
}

// Gateway WebSocket 客户端
type Gateway struct {
	conn *websocket.Conn
	mu   sync.Mutex
}

// New 创建 Gateway 客户端
func New() *Gateway {
	return &Gateway{}
}

// Connect 连接到 Gateway
func (g *Gateway) Connect(ctx context.Context) error {
	conn, _, err := websocket.DefaultDialer.DialContext(ctx, GatewayURL, nil)
	if err != nil {
		return fmt.Errorf("连接 Gateway 失败: %v，请确保服务已启动", err)
	}

	// 设置读取 DeadLine（认证前短，认证后长）
	conn.SetReadDeadline(time.Now().Add(PongWait))
	conn.EnableWriteCompression(true)

	g.conn = conn
	return nil
}

// Send 发送请求并等待响应
func (g *Gateway) Send(ctx context.Context, method string, payload interface{}) (*WsResponse, error) {
	g.mu.Lock()
	defer g.mu.Unlock()

	if g.conn == nil {
		return nil, fmt.Errorf("未连接 Gateway")
	}

	// 序列化 payload
	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("序列化 payload 失败: %v", err)
	}

	// 构造请求
	req := WsRequest{
		Method:  method,
		Payload: string(payloadJSON),
	}

	// 发送
	g.conn.SetWriteDeadline(time.Now().Add(WriteTimeout))
	if err := g.conn.WriteJSON(req); err != nil {
		return nil, fmt.Errorf("发送请求失败: %v", err)
	}

	// 读取响应
	var resp WsResponse
	if err := g.conn.ReadJSON(&resp); err != nil {
		return nil, fmt.Errorf("读取响应失败: %v", err)
	}

	return &resp, nil
}

// Close 关闭连接
func (g *Gateway) Close() error {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.conn != nil {
		return g.conn.Close()
	}
	return nil
}
