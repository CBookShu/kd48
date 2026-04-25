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
