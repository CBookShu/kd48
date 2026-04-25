// seed-gateway-meta 向 Etcd 写入网关 Bootstrap 所需的最小 ServiceType + GatewayRoute（开发用）。
// 同时初始化数据源路由配置（MySQL/Redis routing rules）。
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	gatewayv1 "github.com/CBookShu/kd48/api/proto/gateway/v1"
	"github.com/CBookShu/kd48/pkg/dsroute"
	clientv3 "go.etcd.io/etcd/client/v3"
	"go.uber.org/zap"
	"google.golang.org/protobuf/encoding/protojson"
)

func main() {
	endpoints := flag.String("endpoints", "127.0.0.1:2379", "comma-separated etcd endpoints")
	flag.Parse()

	ep := strings.Split(*endpoints, ",")
	for i := range ep {
		ep[i] = strings.TrimSpace(ep[i])
	}

	cli, err := clientv3.New(clientv3.Config{
		Endpoints:   ep,
		DialTimeout: 3 * time.Second,
		Logger:      zap.NewNop(),
	})
	if err != nil {
		slog.Error("etcd client", "error", err)
		os.Exit(1)
	}
	defer cli.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	marshal := protojson.MarshalOptions{Multiline: true, Indent: "  "}

	userType := &gatewayv1.ServiceTypeSpec{
		SchemaVersion: 1,
		TypeKey:       "user",
		DisplayName:   "User Service",
		RoutingMode:   gatewayv1.ServiceRoutingMode_SERVICE_ROUTING_MODE_STATELESS_LB,
		Discovery: &gatewayv1.ServiceTypeDiscovery{
			GrpcEtcdTarget: "etcd:///kd48/user-service",
			LoadBalancing:  "round_robin",
		},
		Ingress: &gatewayv1.ServiceTypeIngress{UseGatewayIngress: true},
	}
	typeJSON, err := marshal.Marshal(userType)
	if err != nil {
		slog.Error("marshal type", "error", err)
		os.Exit(1)
	}

	loginRoute := &gatewayv1.GatewayRouteSpec{
		SchemaVersion:      1,
		RouteId:            "login",
		WsMethod:           "/user.v1.UserService/Login",
		ServiceType:        "user",
		IngressRoute:       "/user.v1.UserService/Login",
		Public:             true,
		DisplayName:        "Login",
		EstablishesSession: true,
	}
	regRoute := &gatewayv1.GatewayRouteSpec{
		SchemaVersion:      1,
		RouteId:            "register",
		WsMethod:           "/user.v1.UserService/Register",
		ServiceType:        "user",
		IngressRoute:       "/user.v1.UserService/Register",
		Public:             true,
		DisplayName:        "Register",
		EstablishesSession: true,
	}
	verifyTokenRoute := &gatewayv1.GatewayRouteSpec{
		SchemaVersion:      1,
		RouteId:            "verify-token",
		WsMethod:           "/user.v1.UserService/VerifyToken",
		ServiceType:        "user",
		IngressRoute:       "/user.v1.UserService/VerifyToken",
		Public:             true, // Public API - validates token from payload
		DisplayName:        "Verify Token",
		EstablishesSession: true, // Restores session when token is valid
	}
	loginJSON, err := marshal.Marshal(loginRoute)
	if err != nil {
		slog.Error("marshal login route", "error", err)
		os.Exit(1)
	}
	regJSON, err := marshal.Marshal(regRoute)
	if err != nil {
		slog.Error("marshal register route", "error", err)
		os.Exit(1)
	}
	verifyTokenJSON, err := marshal.Marshal(verifyTokenRoute)
	if err != nil {
		slog.Error("marshal verify-token route", "error", err)
		os.Exit(1)
	}

	_, err = cli.Put(ctx, "kd48/meta/service-types/user", string(typeJSON))
	if err != nil {
		slog.Error("put service type", "error", err)
		os.Exit(1)
	}
	_, err = cli.Put(ctx, "kd48/meta/gateway-routes/login", string(loginJSON))
	if err != nil {
		slog.Error("put login route", "error", err)
		os.Exit(1)
	}
	_, err = cli.Put(ctx, "kd48/meta/gateway-routes/register", string(regJSON))
	if err != nil {
		slog.Error("put register route", "error", err)
		os.Exit(1)
	}
	_, err = cli.Put(ctx, "kd48/meta/gateway-routes/verify-token", string(verifyTokenJSON))
	if err != nil {
		slog.Error("put verify-token route", "error", err)
		os.Exit(1)
	}

	// Lobby Service Type
	lobbyType := &gatewayv1.ServiceTypeSpec{
		SchemaVersion: 1,
		TypeKey:       "lobby",
		DisplayName:   "Lobby Service",
		RoutingMode:   gatewayv1.ServiceRoutingMode_SERVICE_ROUTING_MODE_STATELESS_LB,
		Discovery: &gatewayv1.ServiceTypeDiscovery{
			GrpcEtcdTarget: "etcd:///kd48/lobby-service",
			LoadBalancing:  "round_robin",
		},
		Ingress: &gatewayv1.ServiceTypeIngress{UseGatewayIngress: true},
	}
	lobbyTypeJSON, err := marshal.Marshal(lobbyType)
	if err != nil {
		slog.Error("marshal lobby type", "error", err)
		os.Exit(1)
	}

	// Checkin routes
	checkinRoute := &gatewayv1.GatewayRouteSpec{
		SchemaVersion:      1,
		RouteId:            "checkin",
		WsMethod:           "/lobby.v1.CheckinService/Checkin",
		ServiceType:        "lobby",
		IngressRoute:       "/lobby.v1.CheckinService/Checkin",
		Public:             false,
		DisplayName:        "Checkin",
		EstablishesSession: false,
	}
	getStatusRoute := &gatewayv1.GatewayRouteSpec{
		SchemaVersion:      1,
		RouteId:            "get-checkin-status",
		WsMethod:           "/lobby.v1.CheckinService/GetStatus",
		ServiceType:        "lobby",
		IngressRoute:       "/lobby.v1.CheckinService/GetStatus",
		Public:             false,
		DisplayName:        "Get Checkin Status",
		EstablishesSession: false,
	}

	// Item routes
	getItemsRoute := &gatewayv1.GatewayRouteSpec{
		SchemaVersion:      1,
		RouteId:            "get-my-items",
		WsMethod:           "/lobby.v1.ItemService/GetMyItems",
		ServiceType:        "lobby",
		IngressRoute:       "/lobby.v1.ItemService/GetMyItems",
		Public:             false,
		DisplayName:        "Get My Items",
		EstablishesSession: false,
	}

	checkinJSON, err := marshal.Marshal(checkinRoute)
	if err != nil {
		slog.Error("marshal checkin route", "error", err)
		os.Exit(1)
	}
	getStatusJSON, err := marshal.Marshal(getStatusRoute)
	if err != nil {
		slog.Error("marshal get-status route", "error", err)
		os.Exit(1)
	}
	getItemsJSON, err := marshal.Marshal(getItemsRoute)
	if err != nil {
		slog.Error("marshal get-items route", "error", err)
		os.Exit(1)
	}

	_, err = cli.Put(ctx, "kd48/meta/service-types/lobby", string(lobbyTypeJSON))
	if err != nil {
		slog.Error("put lobby service type", "error", err)
		os.Exit(1)
	}
	_, err = cli.Put(ctx, "kd48/meta/gateway-routes/checkin", string(checkinJSON))
	if err != nil {
		slog.Error("put checkin route", "error", err)
		os.Exit(1)
	}
	_, err = cli.Put(ctx, "kd48/meta/gateway-routes/get-checkin-status", string(getStatusJSON))
	if err != nil {
		slog.Error("put get-status route", "error", err)
		os.Exit(1)
	}
	_, err = cli.Put(ctx, "kd48/meta/gateway-routes/get-my-items", string(getItemsJSON))
	if err != nil {
		slog.Error("put get-items route", "error", err)
		os.Exit(1)
	}

	// 初始化数据源路由配置（catch-all fallback to "default"）
	mysqlRoutes := []dsroute.RouteRule{
		{Prefix: "", Pool: "default"}, // catch-all: 未匹配的 routing key 使用 default pool
	}
	redisRoutes := []dsroute.RouteRule{
		{Prefix: "", Pool: "default"}, // catch-all: 未匹配的 routing key 使用 default pool
	}

	mysqlRoutesJSON, err := json.Marshal(mysqlRoutes)
	if err != nil {
		slog.Error("marshal mysql routes", "error", err)
		os.Exit(1)
	}
	redisRoutesJSON, err := json.Marshal(redisRoutes)
	if err != nil {
		slog.Error("marshal redis routes", "error", err)
		os.Exit(1)
	}

	_, err = cli.Put(ctx, "kd48/routing/mysql_routes", string(mysqlRoutesJSON))
	if err != nil {
		slog.Error("put mysql routes", "error", err)
		os.Exit(1)
	}
	_, err = cli.Put(ctx, "kd48/routing/redis_routes", string(redisRoutesJSON))
	if err != nil {
		slog.Error("put redis routes", "error", err)
		os.Exit(1)
	}

	fmt.Println("ok: service-types/user+lobby + gateway-routes + routing/mysql_routes+redis_routes")
}
