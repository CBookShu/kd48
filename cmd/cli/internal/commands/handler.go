package commands

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/CBookShu/kd48/cli/internal/client"
	"github.com/CBookShu/kd48/cli/internal/state"
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

// checkinDo 每日签到 - stub
func (h *Handler) checkinDo() string {
	return "[TODO] checkinDo"
}

// checkinStatus 查看签到状态 - stub
func (h *Handler) checkinStatus() string {
	return "[TODO] checkinStatus"
}

// items 查看背包 - stub
func (h *Handler) items() string {
	return "[TODO] items"
}
