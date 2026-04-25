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
	line, err := r.line.Prompt(r.promptFn())
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

// PrintStatus 打印状态栏（不打印提示符，提示符由 ReadLine 处理）
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
}
