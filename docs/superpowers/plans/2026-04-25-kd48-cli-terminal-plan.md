# KD48 CLI Terminal 实现计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 创建一个交互式命令行终端程序，通过 WebSocket 连接 Gateway，支持登录/注册/签到/查看背包

**Architecture:** CLI 通过 gorilla/websocket 连接 Gateway，使用 readline 处理 REPL 输入，按命名空间分组的命令通过统一的 WebSocket 客户端发送

**Tech Stack:** Go 1.21+, gorilla/websocket, readline

---

## 文件结构

```
cmd/cli/
├── main.go              # 入口，main 函数
├── go.mod
└── go.sum

internal/
├── client/
│   └── gateway.go       # WebSocket 客户端
├── repl/
│   └── repl.go          # REPL 循环
├── state/
│   └── state.go         # 状态管理
└── commands/
    ├── handler.go       # 命令分发
    ├── user.go          # user:login, user:register, user:logout, user:whoami
    ├── checkin.go       # checkin:do, checkin:status
    └── items.go         # items
```

---

## 路由列表（来自 seed-gateway-meta）

| 命令 | WebSocket Method | 登录前 | 登录后 |
|------|------------------|--------|--------|
| `user:login` | `/user.v1.UserService/Login` | ✓ | ✓ |
| `user:register` | `/user.v1.UserService/Register` | ✓ | ✓ |
| `checkin:do` | `/lobby.v1.CheckinService/Checkin` | ✗ | ✓ |
| `checkin:status` | `/lobby.v1.CheckinService/GetStatus` | ✗ | ✓ |
| `items` | `/lobby.v1.ItemService/GetMyItems` | ✗ | ✓ |

---

## WebSocket 协议

**请求格式:**
```json
{
  "method": "/user.v1.UserService/Login",
  "payload": "{\"username\":\"john\",\"password\":\"123456\"}"
}
```

**响应格式:**
```json
{
  "method": "/user.v1.UserService/Login",
  "code": 0,
  "msg": "success",
  "data": {
    "success": true,
    "token": "...",
    "userId": 123
  }
}
```

---

### Task 1: 创建 CLI 项目结构和基础依赖

**Files:**
- Create: `cmd/cli/go.mod`
- Create: `cmd/cli/go.sum`

- [ ] **Step 1: 创建 go.mod**

```bash
mkdir -p cmd/cli/internal/{client,repl,state,commands}
cat > cmd/cli/go.mod << 'EOF'
module github.com/CBookShu/kd48/cli

go 1.21

require (
	github.com/CBookShu/kd48/api/proto/gateway/v1 v0.0.0
	github.com/gorilla/websocket v1.5.3
	github.com/peterh/liner v1.3.0
	google.golang.org/protobuf v1.34.1
	google.golang.org/grpc v1.64.0
	google.golang.org/grpc/credentials/insecure v1.0.7
)
replace github.com/CBookShu/kd48/api/proto => ../../api/proto
EOF
```

- [ ] **Step 2: 运行 go mod tidy**

```bash
cd cmd/cli && go mod tidy
```

- [ ] **Step 3: 提交**

```bash
git add cmd/cli/go.mod cmd/cli/go.sum
git commit -m "feat(cli): add CLI project with go.mod dependencies

- gorilla/websocket for WebSocket client
- peterh/liner for REPL input
- proto generated code from gateway

Co-Authored-By: Claude Opus 4.7 <noreply@anthropic.com>"
```

---

### Task 2: 实现状态管理

**Files:**
- Create: `cmd/cli/internal/state/state.go`

- [ ] **Step 1: 创建 state.go**

```go
package state

// State 维护 CLI 会话状态
type State struct {
	IsLoggedIn     bool
	Username       string
	UserID         int64
	Token          string
	TodayChecked   bool
	ContinuousDays int
}

// New 创建新状态
func New() *State {
	return &State{}
}

// Reset 重置状态（登出时调用）
func (s *State) Reset() {
	s.IsLoggedIn = false
	s.Username = ""
	s.UserID = 0
	s.Token = ""
	s.TodayChecked = false
	s.ContinuousDays = 0
}

// SetUser 设置用户信息
func (s *State) SetUser(username string, userID int64, token string) {
	s.IsLoggedIn = true
	s.Username = username
	s.UserID = userID
	s.Token = token
}
```

- [ ] **Step 2: 提交**

```bash
git add cmd/cli/internal/state/state.go
git commit -m "feat(cli): add state management

Co-Authored-By: Claude Opus 4.7 <noreply@anthropic.com>"
```

---

### Task 3: 实现 WebSocket 客户端

**Files:**
- Create: `cmd/cli/internal/client/gateway.go`

- [ ] **Step 1: 创建 gateway.go**

```go
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
	if g.conn != nil {
		return g.conn.Close()
	}
	return nil
}
```

- [ ] **Step 2: 提交**

```bash
git add cmd/cli/internal/client/gateway.go
git commit -m "feat(cli): add WebSocket client for Gateway

Co-Authored-By: Claude Opus 4.7 <noreply@anthropic.com>"
```

---

### Task 4: 实现命令处理器基础框架

**Files:**
- Create: `cmd/cli/internal/commands/handler.go`

- [ ] **Step 1: 创建 handler.go**

```go
package commands

import (
	"fmt"
	"strings"

	"github.com/CBookShu/kd48/cli/internal/client"
	"github.com/CBookShu/kd48/cli/internal/state"
)

// Handler 命令处理器
type Handler struct {
	gateway *client.Gateway
	state   *state.State
}

// New 创建命令处理器
func New(gateway *client.Gateway, state *state.State) *Handler {
	return &Handler{
		gateway: gateway,
		state:   state,
	}
}

// Handle 处理命令输入
func (h *Handler) Handle(input string) string {
	input = strings.TrimSpace(input)
	if input == "" {
		return ""
	}

	parts := strings.Fields(input)
	if len(parts) == 0 {
		return ""
	}

	cmd := parts[0]
	args := parts[1:]

	// 系统命令
	switch cmd {
	case "quit", "exit":
		return "quit"
	case "help":
		return h.help()
	}

	// 登录前可用命令
	if !h.state.IsLoggedIn {
		switch cmd {
		case "user:login":
			return h.userLogin(args)
		case "user:register":
			return h.userRegister(args)
		default:
			return fmt.Sprintf("[错误] 未知命令 '%s'，输入 'help' 查看可用命令", cmd)
		}
	}

	// 登录后可用命令
	switch cmd {
	case "user:login":
		return h.userLogin(args)
	case "user:register":
		return h.userRegister(args)
	case "user:logout":
		return h.userLogout()
	case "user:whoami":
		return h.userWhoami()
	case "checkin:do":
		return h.checkinDo()
	case "checkin:status":
		return h.checkinStatus()
	case "items":
		return h.items()
	default:
		return fmt.Sprintf("[错误] 未知命令 '%s'，输入 'help' 查看可用命令", cmd)
	}
}

// help 返回帮助信息
func (h *Handler) help() string {
	if h.state.IsLoggedIn {
		return `  Available Commands:
    user:login <username> <password>    - Switch account
    user:register <username> <password> - Register new account
    user:logout                         - Logout
    user:whoami                         - Show current user
    checkin:do                          - Daily check-in
    checkin:status                      - View check-in status
    items                               - View your items
    help                                - Show all commands
    quit / exit                         - Exit`
	}
	return `  Available Commands:
    user:login <username> <password>    - Login
    user:register <username> <password> - Register new account
    help                                 - Show all commands
    quit / exit                          - Exit`
}
```

- [ ] **Step 2: 提交**

```bash
git add cmd/cli/internal/commands/handler.go
git commit -m "feat(cli): add command handler framework

Co-Authored-By: Claude Opus 4.7 <noreply@anthropic.com>"
```

---

### Task 5: 实现用户命令 (user:login, user:register, user:logout, user:whoami)

**Files:**
- Create: `cmd/cli/internal/commands/user.go`

- [ ] **Step 1: 创建 user.go**

```go
package commands

import (
	"encoding/json"
	"fmt"
)

// userLoginResp 登录响应
type userLoginResp struct {
	Success bool   `json:"success"`
	Token   string `json:"token"`
	UserID  int64  `json:"userId"`
}

// userRegisterResp 注册响应
type userRegisterResp struct {
	Success bool   `json:"success"`
	Token   string `json:"token"`
	UserID  int64  `json:"userId"`
}

// userLogin 处理登录
func (h *Handler) userLogin(args []string) string {
	if len(args) < 2 {
		return "[错误] 用法: user:login <username> <password>"
	}

	username := args[0]
	password := args[1]

	payload := map[string]string{
		"username": username,
		"password": password,
	}

	resp, err := h.gateway.Send(nil, "/user.v1.UserService/Login", payload)
	if err != nil {
		return fmt.Sprintf("[错误] %v", err)
	}

	if resp.Code != 0 {
		return fmt.Sprintf("[错误] %s", resp.Msg)
	}

	// 解析响应数据
	data, _ := json.Marshal(resp.Data)
	var loginResp userLoginResp
	if err := json.Unmarshal(data, &loginResp); err != nil {
		return fmt.Sprintf("[错误] 解析响应失败: %v", err)
	}

	if !loginResp.Success {
		return "[错误] 用户名或密码错误"
	}

	// 更新状态
	h.state.SetUser(username, loginResp.UserID, loginResp.Token)

	return fmt.Sprintf("[成功] 已登录，当前用户: %s", username)
}

// userRegister 处理注册
func (h *Handler) userRegister(args []string) string {
	if len(args) < 2 {
		return "[错误] 用法: user:register <username> <password>"
	}

	username := args[0]
	password := args[1]

	payload := map[string]string{
		"username": username,
		"password": password,
	}

	resp, err := h.gateway.Send(nil, "/user.v1.UserService/Register", payload)
	if err != nil {
		return fmt.Sprintf("[错误] %v", err)
	}

	if resp.Code != 0 {
		return fmt.Sprintf("[错误] %s", resp.Msg)
	}

	// 解析响应数据
	data, _ := json.Marshal(resp.Data)
	var regResp userRegisterResp
	if err := json.Unmarshal(data, &regResp); err != nil {
		return fmt.Sprintf("[错误] 解析响应失败: %v", err)
	}

	if !regResp.Success {
		return "[错误] 注册失败，用户名可能已存在"
	}

	// 更新状态
	h.state.SetUser(username, regResp.UserID, regResp.Token)

	return fmt.Sprintf("[成功] 注册并登录成功，当前用户: %s", username)
}

// userLogout 处理登出
func (h *Handler) userLogout() string {
	h.state.Reset()
	return "[成功] 已登出"
}

// userWhoami 查看当前用户
func (h *Handler) userWhoami() string {
	if !h.state.IsLoggedIn {
		return "[错误] 请先登录"
	}

	checkedStr := "否"
	if h.state.TodayChecked {
		checkedStr = "是"
	}

	return fmt.Sprintf(`┌─────────────────────────────────────────┐
│  Current User                          │
├─────────────────────────────────────────┤
│  Username:    %-25s│
│  User ID:     %-25d│
│  Checked in:  %-25s│
│  Streak:      %-25d│
└─────────────────────────────────────────┘`,
		h.state.Username,
		h.state.UserID,
		checkedStr,
		h.state.ContinuousDays)
}
```

- [ ] **Step 2: 提交**

```bash
git add cmd/cli/internal/commands/user.go
git commit -m "feat(cli): add user commands (login, register, logout, whoami)

Co-Authored-By: Claude Opus 4.7 <noreply@anthropic.com>"
```

---

### Task 6: 实现签到命令 (checkin:do, checkin:status)

**Files:**
- Create: `cmd/cli/internal/commands/checkin.go`

- [ ] **Step 1: 创建 checkin.go**

```go
package commands

import (
	"encoding/json"
	"fmt"
)

// checkinDoResp 签到响应
type checkinDoResp struct {
	Success bool `json:"success"`
}

// checkinStatusResp 签到状态响应
type checkinStatusResp struct {
	Success        bool     `json:"success"`
	PeriodID       int64    `json:"periodId"`
	PeriodName     string   `json:"periodName"`
	TodayChecked   bool     `json:"todayChecked"`
	ContinuousDays int32    `json:"continuousDays"`
	TotalDays      int32    `json:"totalDays"`
}

// checkinDo 执行签到
func (h *Handler) checkinDo() string {
	if !h.state.IsLoggedIn {
		return "[错误] 请先登录"
	}

	// 注意：Gateway 已经通过 WebSocket 连接认证，
	// user_id 会通过 context 传递给后端服务，无需在 payload 中发送 token
	payload := map[string]string{}

	resp, err := h.gateway.Send(nil, "/lobby.v1.CheckinService/Checkin", payload)
	if err != nil {
		return fmt.Sprintf("[错误] %v", err)
	}

	if resp.Code != 0 {
		return fmt.Sprintf("[错误] %s", resp.Msg)
	}

	// 解析响应
	data, _ := json.Marshal(resp.Data)
	var checkinResp checkinDoResp
	if err := json.Unmarshal(data, &checkinResp); err != nil {
		return fmt.Sprintf("[错误] 解析响应失败: %v", err)
	}

	if !checkinResp.Success {
		return "[错误] 签到失败"
	}

	// 更新状态
	h.state.TodayChecked = true
	h.state.ContinuousDays++

	return fmt.Sprintf("[成功] 签到成功！连续签到: %d 天", h.state.ContinuousDays)
}

// checkinStatus 查看签到状态
func (h *Handler) checkinStatus() string {
	if !h.state.IsLoggedIn {
		return "[错误] 请先登录"
	}

	// 注意：Gateway 已经通过 WebSocket 连接认证，
	// user_id 会通过 context 传递给后端服务，无需在 payload 中发送 token
	payload := map[string]string{}

	resp, err := h.gateway.Send(nil, "/lobby.v1.CheckinService/GetStatus", payload)
	if err != nil {
		return fmt.Sprintf("[错误] %v", err)
	}

	if resp.Code != 0 {
		return fmt.Sprintf("[错误] %s", resp.Msg)
	}

	// 解析响应
	data, _ := json.Marshal(resp.Data)
	var statusResp checkinStatusResp
	if err := json.Unmarshal(data, &statusResp); err != nil {
		return fmt.Sprintf("[错误] 解析响应失败: %v", err)
	}

	// 更新状态
	h.state.TodayChecked = statusResp.TodayChecked
	h.state.ContinuousDays = int(statusResp.ContinuousDays)

	checkStr := "✗ 未签到"
	if statusResp.TodayChecked {
		checkStr = "✓ 已签到"
	}

	return fmt.Sprintf(`┌─────────────────────────────────────────┐
│  Check-in Status                       │
├─────────────────────────────────────────┤
│  Period:     %-25s│
│  Today:      %-25s│
│  Streak:     %-25d days│
│  Total:      %-25d days│
└─────────────────────────────────────────┘`,
		statusResp.PeriodName,
		checkStr,
		statusResp.ContinuousDays,
		statusResp.TotalDays)
}
```

- [ ] **Step 2: 提交**

```bash
git add cmd/cli/internal/commands/checkin.go
git commit -m "feat(cli): add checkin commands (do, status)

Co-Authored-By: Claude Opus 4.7 <noreply@anthropic.com>"
```

---

### Task 7: 实现物品命令 (items)

**Files:**
- Create: `cmd/cli/internal/commands/items.go`

- [ ] **Step 1: 创建 items.go**

```go
package commands

import (
	"encoding/json"
	"fmt"
	"sort"
)

// itemsResp 物品响应
type itemsResp struct {
	Success bool   `json:"success"`
	Items   string `json:"items"` // JSON string of map[int32]int64
}

// items 查看背包
func (h *Handler) items() string {
	if !h.state.IsLoggedIn {
		return "[错误] 请先登录"
	}

	// 注意：Gateway 已经通过 WebSocket 连接认证，
	// user_id 会通过 context 传递给后端服务，无需在 payload 中发送 token
	payload := map[string]string{}

	resp, err := h.gateway.Send(nil, "/lobby.v1.ItemService/GetMyItems", payload)
	if err != nil {
		return fmt.Sprintf("[错误] %v", err)
	}

	if resp.Code != 0 {
		return fmt.Sprintf("[错误] %s", resp.Msg)
	}

	// 解析响应
	data, _ := json.Marshal(resp.Data)
	var itemsResp itemsResp
	if err := json.Unmarshal(data, &itemsResp); err != nil {
		return fmt.Sprintf("[错误] 解析响应失败: %v", err)
	}

	if !itemsResp.Success {
		return "[错误] 获取物品失败"
	}

	// 解析 items JSON
	var items map[int32]int64
	if err := json.Unmarshal([]byte(itemsResp.Items), &items); err != nil {
		return fmt.Sprintf("[错误] 解析物品数据失败: %v", err)
	}

	if len(items) == 0 {
		return "[信息] 背包为空"
	}

	// 排序并显示
	var ids []int32
	for id := range items {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })

	result := "[成功] 背包物品:\n"
	for _, id := range ids {
		result += fmt.Sprintf("  - %d: %d\n", id, items[id])
	}

	return result
}
```

- [ ] **Step 2: 提交**

```bash
git add cmd/cli/internal/commands/items.go
git commit -m "feat(cli): add items command

Co-Authored-By: Claude Opus 4.7 <noreply@anthropic.com>"
```

---

### Task 8: 实现 REPL 循环

**Files:**
- Create: `cmd/cli/internal/repl/repl.go`

- [ ] **Step 1: 创建 repl.go**

```go
package repl

import (
	"fmt"
	"strings"

	"github.com/peterh/liner"
)

// PromptFunc 生成提示符
type PromptFunc func() string

// REPL 交互式命令行
type REPL struct {
	line     *liner.State
	promptFn PromptFunc
}

// New 创建 REPL
func New(promptFn PromptFunc) *REPL {
	return &REPL{
		line:     liner.NewLiner(),
		promptFn: promptFn,
	}
}

// ReadLine 读取一行输入
func (r *REPL) ReadLine() (string, error) {
	line, err := r.line.Readline(r.promptFn())
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(line), nil
}

// Close 关闭 REPL
func (r *REPL) Close() {
	r.line.Close()
}

// SetHistory 设置历史记录
func (r *REPL) SetHistory(history []string) {
	for _, h := range history {
		r.line.AppendHistory(h)
	}
}

// AddHistory 添加历史记录
func (r *REPL) AddHistory(line string) {
	r.line.AppendHistory(line)
}

// PrintWelcome 打印欢迎信息
func PrintWelcome(isLoggedIn bool) {
	if isLoggedIn {
		fmt.Println(`══════════════════════════════════════════════════════════`)
		fmt.Println(`  Welcome to KD48 CLI Terminal`)
		fmt.Println(`══════════════════════════════════════════════════════════`)
		fmt.Println(`  Available Commands:`)
		fmt.Println(`    user:login <username> <password>    - Switch account`)
		fmt.Println(`    user:register <username> <password> - Register new account`)
		fmt.Println(`    user:logout                         - Logout`)
		fmt.Println(`    user:whoami                         - Show current user`)
		fmt.Println(`    checkin:do                          - Daily check-in`)
		fmt.Println(`    checkin:status                      - View check-in status`)
		fmt.Println(`    items                               - View your items`)
		fmt.Println(`    help                                - Show all commands`)
		fmt.Println(`    quit / exit                         - Exit`)
		fmt.Println(`══════════════════════════════════════════════════════════`)
	} else {
		fmt.Println(`══════════════════════════════════════════════════════════`)
		fmt.Println(`  Welcome to KD48 CLI Terminal`)
		fmt.Println(`══════════════════════════════════════════════════════════`)
		fmt.Println(`  Available Commands:`)
		fmt.Println(`    user:login <username> <password>    - Login`)
		fmt.Println(`    user:register <username> <password> - Register new account`)
		fmt.Println(`    help                                 - Show all commands`)
		fmt.Println(`    quit / exit                          - Exit`)
		fmt.Println(`══════════════════════════════════════════════════════════`)
	}
}

// PrintStatus 打印状态栏
func PrintStatus(username string, checked bool, streak int) {
	status := "Not logged in"
	if username != "" {
		checkedStr := "No"
		if checked {
			checkedStr = "Yes"
		}
		status = fmt.Sprintf("Logged in as [%s] | Checked: %s | Streak: %d days", username, checkedStr, streak)
	}
	fmt.Printf("  Status: %s\n", status)
	fmt.Println("────────────────────────────────────────────────────────────────")
	fmt.Print("kd48> ")
}
```

- [ ] **Step 2: 提交**

```bash
git add cmd/cli/internal/repl/repl.go
git commit -m "feat(cli): add REPL loop implementation

Co-Authored-By: Claude Opus 4.7 <noreply@anthropic.com>"
```

---

### Task 9: 实现 main 函数

**Files:**
- Create: `cmd/cli/main.go`

- [ ] **Step 1: 创建 main.go**

```go
package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/CBookShu/kd48/cli/internal/client"
	"github.com/CBookShu/kd48/cli/internal/commands"
	"github.com/CBookShu/kd48/cli/internal/repl"
	"github.com/CBookShu/kd48/cli/internal/state"
)

func main() {
	// 创建组件
	gw := client.Newateway()
	st := state.New()

	// 连接到 Gateway
	fmt.Println("[信息] 正在连接 Gateway...")
	if err := gw.Connect(context.Background()); err != nil {
		fmt.Printf("[错误] %v\n", err)
		fmt.Println("请确保 Gateway 服务已启动 (localhost:8080)")
		os.Exit(1)
	}
	fmt.Println("[信息] 已连接到 Gateway")

	// 创建命令处理器
	handler := commands.New(gw, st)

	// 创建 REPL
	r := repl.New(func() string { return "kd48> " })

	// 打印欢迎信息
	repl.PrintWelcome(false)
	repl.PrintStatus("", false, 0)

	// REPL 循环
	for {
		input, err := r.ReadLine()
		if err != nil {
			// Ctrl+C 或 Ctrl+D
			fmt.Println("\nGoodbye!")
			break
		}

		if input == "" {
			repl.PrintStatus(st.Username, st.TodayChecked, st.ContinuousDays)
			continue
		}

		// 处理命令
		result := handler.Handle(input)

		// 检查退出
		if result == "quit" {
			fmt.Println("Goodbye!")
			break
		}

		// 输出结果
		if result != "" {
			fmt.Println(result)
		}

		// 重新打印状态栏
		repl.PrintStatus(st.Username, st.TodayChecked, st.ContinuousDays)

		// 添加到历史
		r.AddHistory(input)
	}

	// 清理
	r.Close()
	gw.Close()
}
```

- [ ] **Step 2: 编译测试**

```bash
cd cmd/cli && go build -o kd48-cli .
```

如果编译失败，修复错误后重新编译。

- [ ] **Step 3: 提交**

```bash
git add cmd/cli/main.go
git commit -m "feat(cli): add main entry point

Co-Authored-By: Claude Opus 4.7 <noreply@anthropic.com>"
```

---

### Task 10: 手动测试完整流程

**前置条件:**
- 启动 MySQL
- 启动 Redis
- 启动 Gateway (localhost:8080)
- 启动 User 服务
- 启动 Lobby 服务

- [ ] **Step 1: 启动所有后端服务**

确保所有服务正常运行。

- [ ] **Step 2: 运行 CLI**

```bash
./cmd/cli/kd48-cli
```

- [ ] **Step 3: 测试注册**

```
kd48> user:register testuser 123456
```

预期: 显示 "[成功] 注册并登录成功，当前用户: testuser"

- [ ] **Step 4: 测试签到**

```
kd48> checkin:do
```

预期: 显示 "[成功] 签到成功！连续签到: 1 天"

- [ ] **Step 5: 测试查看状态**

```
kd48> checkin:status
```

预期: 显示签到状态表格

- [ ] **Step 6: 测试查看物品**

```
kd48> items
```

预期: 显示背包物品列表

- [ ] **Step 7: 测试查看用户**

```
kd48> user:whoami
```

预期: 显示当前用户信息

- [ ] **Step 8: 测试登出**

```
kd48> user:logout
```

预期: 显示 "[成功] 已登出"

- [ ] **Step 9: 提交**

```bash
git commit --allow-empty -m "test(cli): verify complete user flow

Co-Authored-By: Claude Opus 4.7 <noreply@anthropic.com>"
```

---
