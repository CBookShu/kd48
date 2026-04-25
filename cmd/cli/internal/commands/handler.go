package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/CBookShu/kd48/cli/internal/client"
	"github.com/CBookShu/kd48/cli/internal/state"
)

// int64String 可以解析 JSON 中的数字或字符串形式的 int64
type int64String int64

func (i *int64String) UnmarshalJSON(data []byte) error {
	// 去掉首尾空格
	s := strings.TrimSpace(string(data))
	// 去掉引号（如果是字符串）
	s = strings.Trim(s, `"`)
	// 解析
	n, err := fmt.Sscanf(s, "%d", (*int64)(i))
	if err != nil {
		return err
	}
	if n != 1 {
		return fmt.Errorf("cannot parse %s as int64", data)
	}
	return nil
}

// userLoginResp 登录响应
type userLoginResp struct {
	Success bool       `json:"success"`
	Token   string     `json:"token"`
	UserID  int64String `json:"userId"`
}

// userRegisterResp 注册响应
type userRegisterResp struct {
	Success bool       `json:"success"`
	Token   string     `json:"token"`
	UserID  int64String `json:"userId"`
}

// checkinDoResp 签到响应（空结构，响应通过 Code 判断）
type checkinDoResp struct{}

// checkinStatusResp 签到状态响应
type checkinStatusResp struct {
	Success        bool   `json:"success"`
	PeriodID       int64  `json:"periodId"`
	PeriodName     string `json:"periodName"`
	TodayChecked   bool   `json:"todayChecked"`
	ContinuousDays int32  `json:"continuousDays"`
	TotalDays      int32  `json:"totalDays"`
}

// itemsResp 物品响应
type itemsResp struct {
	Items string `json:"items"` // JSON string of map[int32]int64
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
			// 检查是否是需要登录的命令
			if h.isLoginRequiredCommand(cmd) {
				return "[错误] 请先登录"
			}
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
	case "item:list":
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
    item:list                           - View your items
    help                                - Show all commands
    quit / exit                         - Exit`
	}
	return `  Available Commands:
    user:login <username> <password>    - Login
    user:register <username> <password> - Register new account
    help                                 - Show all commands
    quit / exit                          - Exit`
}

// isLoginRequiredCommand 检查命令是否需要登录
func (h *Handler) isLoginRequiredCommand(cmd string) bool {
	switch cmd {
	case "user:logout", "user:whoami", "checkin:do", "checkin:status", "item:list":
		return true
	}
	return false
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

	resp, err := h.gateway.Send(context.Background(), "/user.v1.UserService/Login", payload)
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
	h.state.SetUser(username, int64(loginResp.UserID), loginResp.Token)

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

	resp, err := h.gateway.Send(context.Background(), "/user.v1.UserService/Register", payload)
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
	h.state.SetUser(username, int64(regResp.UserID), regResp.Token)

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

// checkinDo 执行签到
func (h *Handler) checkinDo() string {
	if !h.state.IsLoggedIn {
		return "[错误] 请先登录"
	}

	// user_id 由 Gateway 通过 WebSocket 连接认证
	resp, err := h.gateway.Send(context.Background(), "/lobby.v1.CheckinService/Checkin", nil)
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

	// user_id 由 Gateway 通过 WebSocket 连接认证
	resp, err := h.gateway.Send(context.Background(), "/lobby.v1.CheckinService/GetStatus", nil)
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

// items 查看背包
func (h *Handler) items() string {
	if !h.state.IsLoggedIn {
		return "[错误] 请先登录"
	}

	// user_id 由 Gateway 通过 WebSocket 连接认证
	resp, err := h.gateway.Send(context.Background(), "/lobby.v1.ItemService/GetMyItems", nil)
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

	// 解析 items JSON (空字符串视为空 map)
	var items map[int32]int64
	if itemsResp.Items == "" {
		items = make(map[int32]int64)
	} else if err := json.Unmarshal([]byte(itemsResp.Items), &items); err != nil {
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
