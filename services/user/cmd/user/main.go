package main

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"syscall"

	userv1 "github.com/CBookShu/kd48/api/proto/user/v1"
	"github.com/CBookShu/kd48/pkg/conf"
	"github.com/CBookShu/kd48/pkg/logzap"
	"github.com/CBookShu/kd48/pkg/otelkit"
	"github.com/CBookShu/kd48/pkg/registry"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// userService 实现 proto 定义的接口
type userService struct {
	userv1.UnimplementedUserServiceServer
}

func (s *userService) Login(ctx context.Context, req *userv1.LoginRequest) (*userv1.LoginReply, error) {
	slog.InfoContext(ctx, "Received Login request", "username", req.Username)
	// Mock 业务逻辑
	if req.Username == "admin" && req.Password == "123456" {
		return &userv1.LoginReply{
			Success: true,
			Token:   "mock-jwt-token-12345",
		}, nil
	}

	return nil, status.Error(codes.Unauthenticated, "invalid username or password")
}

func main() {
	c, err := conf.Load("./config.yaml")
	if err != nil {
		panic(err)
	}

	handler := logzap.New(c.Log.Level)
	slog.SetDefault(slog.New(handler))

	shutdown, err := otelkit.InitTracer(c.Server.Name + "-user-service")
	if err != nil {
		panic(err)
	}
	defer shutdown(context.Background())

	// 1. 启动 gRPC Server
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", c.UserService.Port))
	if err != nil {
		panic(err)
	}

	s := grpc.NewServer(
		grpc.StatsHandler(otelgrpc.NewServerHandler()),
	)
	userv1.RegisterUserServiceServer(s, &userService{})

	go func() {
		slog.Info("User Service gRPC server listening", "port", c.UserService.Port)
		if err := s.Serve(lis); err != nil {
			panic(err)
		}
	}()

	// 2. 注册到 Etcd
	etcdCli, err := registry.NewClient(c.Etcd)
	if err != nil {
		panic(err)
	}
	defer etcdCli.Close()

	// 获取本机 IP (本地开发直接写 localhost)
	localAddr := fmt.Sprintf("localhost:%d", c.UserService.Port)
	serviceName := "kd48/user-service"

	if err := registry.RegisterService(etcdCli, serviceName, localAddr); err != nil {
		panic(err)
	}
	slog.Info("User Service registered to Etcd", "name", serviceName, "addr", localAddr)

	// 阻塞等待退出
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	slog.Info("Shutting down User Service...")
	s.GracefulStop()
}
