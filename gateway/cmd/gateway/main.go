package main

import (
	"fmt"
	"log"

	"github.com/CBookShu/kd48/pkg/conf"
)

func main() {
	// 默认从当前工作目录(项目根目录)读取
	c, err := conf.Load("./config.yaml")
	if err != nil {
		log.Fatalf("load config failed: %v", err)
	}

	fmt.Printf("[Gateway] Booting... Server: %s, Env: %s, Listen Port: %d\n",
		c.Server.Name, c.Server.Env, c.Gateway.Port)

	// TODO: 注入 Wire
	// TODO: 启动 WS Server
}
