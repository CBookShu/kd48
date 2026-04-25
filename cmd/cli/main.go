package main

import (
	"context"
	"fmt"

	"github.com/gorilla/websocket"
	"github.com/peterh/liner"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/encoding/protojson"
)

func main() {
	// WebSocket client
	wsConn, _, err := websocket.DefaultDialer.Dial("ws://localhost:8080/ws", nil)
	if err != nil {
		fmt.Println("WebSocket dial error:", err)
	}
	_ = wsConn
	_ = liner.NewLiner

	// gRPC connection
	conn, err := grpc.NewClient("localhost:8081", grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		fmt.Println("gRPC dial error:", err)
	}
	_ = conn
	_ = context.Background()
	_ = protojson.MarshalOptions{}
	_ = fmt.Sprintf
}
