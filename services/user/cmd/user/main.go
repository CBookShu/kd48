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

		// 设置连接池参数
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

		// 连接健康检查
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		if err := db.PingContext(ctx); err != nil {
			cancel()
			panic(fmt.Errorf("failed to ping mysql pool %q: %w", name, err))
		}
		cancel()

		mysqlPools[name] = db
	}
	defer func() {
		for _, db := range mysqlPools {
			db.Close()
		}
	}()

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

	// 创建 etcd 客户端（用于路由加载和服务注册）
	etcdCli, err := registry.NewClient(c.Etcd)
	if err != nil {
		panic(err)
	}
	defer etcdCli.Close()

	// 从 etcd 加载路由配置
	routeLoader := dsroute.NewRouteLoader(etcdCli, "kd48/routing")
	routingCfg, err := routeLoader.Get(context.Background())
	if err != nil {
		slog.Error("failed to load routing config from etcd", "error", err)
		os.Exit(1)
	}

	// 创建 Router
	router, err := dsroute.NewRouter(
		mysqlPools,
		redisPools,
		routingCfg.MySQLRoutes,
		routingCfg.RedisRoutes,
	)
	if err != nil {
		panic(fmt.Errorf("failed to create router: %w", err))
	}

	// 启动路由配置监听
	go func() {
		if err := routeLoader.Watch(context.Background(), func(cfg *dsroute.RoutingConfig) {
			router.UpdateRoutes(cfg.MySQLRoutes, cfg.RedisRoutes)
		}); err != nil {
			slog.Error("route watcher stopped", "error", err)
		}
	}()

	// 1. 启动 gRPC Server
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", c.UserService.Port))
	if err != nil {
		panic(err)
	}

	s := grpc.NewServer(
		grpc.StatsHandler(otelgrpc.NewServerHandler()),
	)

	userSvc := NewUserService(router, time.Duration(c.Session.ExpireHours)*time.Hour)
	userv1.RegisterUserServiceServer(s, userSvc)

	// Get the default Redis client for ingress token verification
	var defaultRedis redis.UniversalClient
	for _, rdb := range redisPools {
		defaultRedis = rdb
		break
	}
	gatewayv1.RegisterGatewayIngressServer(s, newIngressServer(userSvc, defaultRedis))

	go func() {
		slog.Info("User Service gRPC server listening", "port", c.UserService.Port)
		if err := s.Serve(lis); err != nil {
			panic(err)
		}
	}()

	// 2. 注册到 Etcd
	// 广播地址从环境变量 ADVERTISE_ADDR 读取
	// - K8s: 通过 Downward API 注入 POD_IP
	// - Docker Compose: 使用服务名
	// - 本地开发: 留空则使用 localhost
	advertiseAddr := os.Getenv("ADVERTISE_ADDR")
	if advertiseAddr == "" {
		if c.Server.Env == "dev" {
			advertiseAddr = "localhost"
			slog.Warn("ADVERTISE_ADDR not set, falling back to localhost (dev mode)")
		} else {
			slog.Error("ADVERTISE_ADDR environment variable is required in non-dev environment", "env", c.Server.Env)
			os.Exit(1)
		}
	}
	localAddr := fmt.Sprintf("%s:%d", advertiseAddr, c.UserService.Port)
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
