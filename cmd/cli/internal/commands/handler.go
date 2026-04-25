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

// userLogin 用户登录 - stub
func (h *Handler) userLogin(args []string) string {
	return "[TODO] userLogin"
}

// userRegister 用户注册 - stub
func (h *Handler) userRegister(args []string) string {
	return "[TODO] userRegister"
}

// userLogout 用户登出 - stub
func (h *Handler) userLogout() string {
	return "[TODO] userLogout"
}

// userWhoami 查看当前用户 - stub
func (h *Handler) userWhoami() string {
	return "[TODO] userWhoami"
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
