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
	lobbyv1 "github.com/CBookShu/kd48/api/proto/lobby/v1"
	"github.com/CBookShu/kd48/pkg/conf"
	"github.com/CBookShu/kd48/pkg/logzap"
	"github.com/CBookShu/kd48/pkg/otelkit"
	"github.com/CBookShu/kd48/pkg/registry"
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

	logPath := filepath.Join(c.Log.FilePath, "lobby-service.log")
	handler := logzap.New(c.Log.Level, logPath)
	slog.SetDefault(slog.New(handler))

	shutdown, err := otelkit.InitTracer(c.Server.Name + "-lobby-service")
	if err != nil {
		panic(err)
	}
	defer shutdown(context.Background())

	dsCfg := c.GetDataSourcesOrSynthesize().ToDSRouteConfig()

	// 初始化 MySQL 连接
	mysqlPools := make(map[string]*sql.DB)
	for name, spec := range dsCfg.MySQLPools {
		db, err := sql.Open("mysql", spec.DSN)
		if err != nil {
			panic(fmt.Errorf("failed to open mysql pool %q: %w", name, err))
		}

		if spec.MaxOpen > 0 {
			db.SetMaxOpenConns(spec.MaxOpen)
		}
		if spec.MaxIdle > 0 {
			db.SetMaxIdleConns(spec.MaxIdle)
		}
		if spec.ConnMaxLifetime > 0 {
			db.SetConnMaxLifetime(spec.ConnMaxLifetime)
		}
		if spec.ConnMaxIdleTime > 0 {
			db.SetConnMaxIdleTime(spec.ConnMaxIdleTime)
		}

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		if err := db.PingContext(ctx); err != nil {
			cancel()
			panic(fmt.Errorf("failed to ping mysql pool %q: %w", name, err))
		}
		cancel()

		mysqlPools[name] = db
		slog.Info("MySQL pool connected", "name", name)
	}
	defer func() {
		for _, db := range mysqlPools {
			db.Close()
		}
	}()

	// 初始化 Redis 连接
	redisPools := make(map[string]redis.UniversalClient)
	for name, spec := range dsCfg.RedisPools {
		opts := &redis.Options{
			Addr:         spec.Addr,
			Password:     spec.Password,
			DB:           spec.DB,
			PoolSize:     spec.PoolSize,
			MinIdleConns: spec.MinIdleConns,
		}
		if spec.DialTimeout > 0 {
			opts.DialTimeout = spec.DialTimeout
		}
		if spec.ReadTimeout > 0 {
			opts.ReadTimeout = spec.ReadTimeout
		}
		if spec.WriteTimeout > 0 {
			opts.WriteTimeout = spec.WriteTimeout
		}

		rdb := redis.NewClient(opts)

		if err := rdb.Ping(context.Background()).Err(); err != nil {
			panic(fmt.Errorf("failed to ping redis pool %q: %w", name, err))
		}

		redisPools[name] = rdb
		slog.Info("Redis pool connected", "name", name, "addr", spec.Addr)
	}
	defer func() {
		for _, rdb := range redisPools {
			rdb.Close()
		}
	}()

	// 创建 etcd 客户端
	etcdCli, err := registry.NewClient(c.Etcd)
	if err != nil {
		panic(err)
	}
	defer etcdCli.Close()

	// 启动 gRPC Server
	// Lobby 服务端口默认使用 9001 (User 服务使用 9000)
	port := 9001
	if c.UserService.Port != 0 && c.UserService.Port != 9000 {
		// 如果配置中有其他服务的端口参考，可以根据实际情况调整
		port = c.UserService.Port + 1
	}

	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		panic(err)
	}

	s := grpc.NewServer(
		grpc.StatsHandler(otelgrpc.NewServerHandler()),
	)

	lobbySvc := NewLobbyService(mysqlPools, redisPools)
	lobbyv1.RegisterLobbyServiceServer(s, lobbySvc)
	gatewayv1.RegisterGatewayIngressServer(s, newIngressServer(lobbySvc))

	go func() {
		slog.Info("Lobby Service gRPC server listening", "port", port)
		if err := s.Serve(lis); err != nil {
			panic(err)
		}
	}()

	// 注册到 Etcd
	localAddr := fmt.Sprintf("localhost:%d", port)
	serviceName := "kd48/lobby-service"

	if err := registry.RegisterService(etcdCli, serviceName, localAddr); err != nil {
		panic(err)
	}
	slog.Info("Lobby Service registered to Etcd", "name", serviceName, "addr", localAddr)

	// 阻塞等待退出
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	slog.Info("Shutting down Lobby Service...")
	s.GracefulStop()
}
