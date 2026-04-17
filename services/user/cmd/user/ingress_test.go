package main

import (
	"context"
	"encoding/json"
	"net"
	"testing"

	gatewayv1 "github.com/CBookShu/kd48/api/proto/gateway/v1"
	userv1 "github.com/CBookShu/kd48/api/proto/user/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
)

func TestIngress_Call_UnknownRoute(t *testing.T) {
	const bufSize = 1024 * 1024
	lis := bufconn.Listen(bufSize)
	s := grpc.NewServer()
	gatewayv1.RegisterGatewayIngressServer(s, newIngressServer(nil))
	go func() { _ = s.Serve(lis) }()
	t.Cleanup(func() { s.Stop() })

	ctx := context.Background()
	conn, err := grpc.DialContext(ctx, "bufnet",
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) { return lis.Dial() }),
		grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })

	cli := gatewayv1.NewGatewayIngressClient(conn)
	_, err = cli.Call(ctx, &gatewayv1.IngressRequest{
		Route:       "/unknown/Method",
		JsonPayload: []byte(`{}`),
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("expected InvalidArgument, got %v", err)
	}
}

type mockUserLR struct{}

func (mockUserLR) Login(ctx context.Context, req *userv1.LoginRequest) (*userv1.LoginReply, error) {
	_ = ctx
	_ = req
	return &userv1.LoginReply{Success: true, Token: "test-token"}, nil
}

func (mockUserLR) Register(ctx context.Context, req *userv1.RegisterRequest) (*userv1.RegisterReply, error) {
	_ = ctx
	_ = req
	return nil, status.Error(codes.Unimplemented, "not used")
}

func TestIngress_Call_Login_Mock(t *testing.T) {
	const bufSize = 1024 * 1024
	lis := bufconn.Listen(bufSize)
	s := grpc.NewServer()
	gatewayv1.RegisterGatewayIngressServer(s, newIngressServer(mockUserLR{}))
	go func() { _ = s.Serve(lis) }()
	t.Cleanup(func() { s.Stop() })

	ctx := context.Background()
	conn, err := grpc.DialContext(ctx, "bufnet",
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) { return lis.Dial() }),
		grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })

	cli := gatewayv1.NewGatewayIngressClient(conn)
	reply, err := cli.Call(ctx, &gatewayv1.IngressRequest{
		Route:       "/user.v1.UserService/Login",
		JsonPayload: []byte(`{"username":"u","password":"p"}`),
	})
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]interface{}
	if err := json.Unmarshal(reply.GetJsonPayload(), &got); err != nil {
		t.Fatal(err)
	}
	if got["success"] != true {
		t.Fatalf("expected success true, got %v", got["success"])
	}
	if got["token"] != "test-token" {
		t.Fatalf("expected token, got %v", got["token"])
	}
}
