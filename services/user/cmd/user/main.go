package main

import (
	"fmt"
	"log"

	"github.com/CBookShu/kd48/pkg/conf"
)

func main() {
	c, err := conf.Load("./config.yaml")
	if err != nil {
		log.Fatalf("load config failed: %v", err)
	}

	fmt.Printf("[User Service] Booting... Server: %s, Env: %s, gRPC Port: %d\n",
		c.Server.Name, c.Server.Env, c.UserService.Port)

	// TODO: 注入 Wire
	// TODO: 启动 gRPC Server
}
