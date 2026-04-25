package main

import (
	"context"
	"fmt"
	"os"

	"github.com/CBookShu/kd48/cli/internal/client"
	"github.com/CBookShu/kd48/cli/internal/commands"
	"github.com/CBookShu/kd48/cli/internal/repl"
	"github.com/CBookShu/kd48/cli/internal/state"
)

func main() {
	// 创建组件
	gw := client.New()
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
