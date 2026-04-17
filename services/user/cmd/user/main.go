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
	"github.com/CBookShu/kd48/pkg/dsroute"
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

	dsCfg := c.GetDataSourcesOrSynthesize().ToDSRouteConfig()

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
		mysqlPools[name] = db
	}
	defer func() {
		for _, db := range mysqlPools {
			db.Close()
		}
	}()

	redisPools := make(map[string]redis.UniversalClient)
	for name, spec := range dsCfg.RedisPools {
		rdb := redis.NewClient(&redis.Options{
			Addr:     spec.Addr,
			Password: spec.Password,
			DB:       spec.DB,
			PoolSize: spec.PoolSize,
		})
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

	mysqlRoutes := make([]dsroute.RouteRule, len(dsCfg.MySQLRoutes))
	for i, r := range dsCfg.MySQLRoutes {
		mysqlRoutes[i] = dsroute.RouteRule{Prefix: r.Prefix, Pool: r.Pool}
	}
	redisRoutes := make([]dsroute.RouteRule, len(dsCfg.RedisRoutes))
	for i, r := range dsCfg.RedisRoutes {
		redisRoutes[i] = dsroute.RouteRule{Prefix: r.Prefix, Pool: r.Pool}
	}

	router, err := dsroute.NewRouter(mysqlPools, redisPools, mysqlRoutes, redisRoutes)
	if err != nil {
		panic(fmt.Errorf("failed to create dsroute router: %w", err))
	}

	// 1. 启动 gRPC Server
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", c.UserService.Port))
	if err != nil {
		panic(err)
	}

	s := grpc.NewServer(
		grpc.StatsHandler(otelgrpc.NewServerHandler()),
	)

	queries := sqlc.New(mysqlPools["default"])
	userSvc := NewUserService(queries, router, time.Duration(c.Session.ExpireHours)*time.Hour)
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
