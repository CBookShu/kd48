package main

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	gatewayv1 "github.com/CBookShu/kd48/api/proto/gateway/v1"
	userv1 "github.com/CBookShu/kd48/api/proto/user/v1"
	"github.com/CBookShu/kd48/pkg/conf"
	"github.com/CBookShu/kd48/pkg/logzap"
	"github.com/CBookShu/kd48/pkg/otelkit"
	"github.com/CBookShu/kd48/pkg/registry"
	"github.com/CBookShu/kd48/services/user/internal/data/sqlc"
	_ "github.com/go-sql-driver/mysql"
	"github.com/redis/go-redis/v9"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"google.golang.org/grpc"
)

func main() {
	c, err := conf.Load("./config.yaml")
	if err != nil {
		panic(err)
	}

	logPath := filepath.Join(c.Log.FilePath, "user-service.log")
	handler := logzap.New(c.Log.Level, logPath)
	slog.SetDefault(slog.New(handler))

	shutdown, err := otelkit.InitTracer(c.Server.Name + "-user-service")
	if err != nil {
		panic(err)
	}
	defer shutdown(context.Background())

	db, err := sql.Open("mysql", c.MySQL.DSN)
	if err != nil {
		panic(fmt.Errorf("failed to open mysql: %w", err))
	}
	defer db.Close()

	// 2. 显式初始化 Redis
	rdb := redis.NewClient(&redis.Options{
		Addr:     c.Redis.Addr,
		Password: c.Redis.Password,
		DB:       c.Redis.DB,
	})
	defer rdb.Close()
	if err := rdb.Ping(context.Background()).Err(); err != nil {
		panic(fmt.Errorf("failed to ping redis: %w", err))
	}
	slog.Info("Redis connected successfully")

	// 1. 启动 gRPC Server
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", c.UserService.Port))
	if err != nil {
		panic(err)
	}

	s := grpc.NewServer(
		grpc.StatsHandler(otelgrpc.NewServerHandler()),
	)

	queries := sqlc.New(db)
	userSvc := NewUserService(queries, rdb, time.Duration(c.Session.ExpireHours)*time.Hour)
	userv1.RegisterUserServiceServer(s, userSvc)
	gatewayv1.RegisterGatewayIngressServer(s, newIngressServer(userSvc))

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
