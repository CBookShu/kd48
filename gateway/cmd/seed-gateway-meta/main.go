// seed-gateway-meta 向 Etcd 写入网关 Bootstrap 所需的最小 ServiceType + GatewayRoute（开发用）。
package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	gatewayv1 "github.com/CBookShu/kd48/api/proto/gateway/v1"
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

	fmt.Println("ok: kd48/meta/service-types/user + gateway-routes/login + register")
}
