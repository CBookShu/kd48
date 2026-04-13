package bootstrap

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	gatewayv1 "github.com/CBookShu/kd48/api/proto/gateway/v1"
	"github.com/CBookShu/kd48/gateway/internal/ws"

	clientv3 	"go.etcd.io/etcd/client/v3"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

var protoUnmarshal = protojson.UnmarshalOptions{DiscardUnknown: true}

// Build 从 Etcd 全量加载类型与路由，构建不可变路由快照与连接池。
// 返回 nextWatchRev = max(两 Range Header.Revision) + 1，供 Watch 使用。
func Build(ctx context.Context, cli *clientv3.Client, typesPrefix, routesPrefix string, dialOpts []grpc.DialOption) (*ws.AtomicRoutes, []*grpc.ClientConn, int64, error) {
	tResp, err := cli.Get(ctx, typesPrefix, clientv3.WithPrefix())
	if err != nil {
		return nil, nil, 0, fmt.Errorf("range service-types: %w", err)
	}
	if len(tResp.Kvs) == 0 {
		return nil, nil, 0, ErrEmptyServiceTypes
	}

	rResp, err := cli.Get(ctx, routesPrefix, clientv3.WithPrefix())
	if err != nil {
		return nil, nil, 0, fmt.Errorf("range gateway-routes: %w", err)
	}
	if len(rResp.Kvs) == 0 {
		return nil, nil, 0, ErrEmptyGatewayRoutes
	}

	rev := tResp.Header.Revision
	if rResp.Header.Revision > rev {
		rev = rResp.Header.Revision
	}
	nextWatchRev := rev + 1

	typeConns := make(map[string]*grpc.ClientConn)

	for _, kv := range tResp.Kvs {
		key := string(kv.Key)
		sfx := keySuffix(key, typesPrefix)
		if sfx == "" {
			slog.Warn("bootstrap: skip type key with empty suffix", "key", key)
			continue
		}
		var spec gatewayv1.ServiceTypeSpec
		if err := protoUnmarshal.Unmarshal(kv.Value, &spec); err != nil {
			slog.Warn("bootstrap: skip invalid service type json", "key", key, "error", err)
			continue
		}
		if err := validateServiceTypeSpec(&spec, sfx); err != nil {
			slog.Warn("bootstrap: skip invalid service type spec", "key", key, "error", err)
			continue
		}
		tk := spec.GetTypeKey()
		cfgJSON := defaultServiceConfigJSON(spec.GetDiscovery().GetLoadBalancing())
		dialCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
		conn, err := grpc.DialContext(dialCtx, spec.GetDiscovery().GetGrpcEtcdTarget(),
			append(append([]grpc.DialOption{}, dialOpts...),
				grpc.WithDefaultServiceConfig(cfgJSON))...)
		cancel()
		if err != nil {
			slog.Warn("bootstrap: skip type dial failed", "type_key", tk, "error", err)
			continue
		}
		if old, dup := typeConns[tk]; dup {
			slog.Warn("bootstrap: duplicate type_key, replacing connection", "type_key", tk)
			_ = old.Close()
		}
		typeConns[tk] = conn
	}

	if len(typeConns) == 0 {
		return nil, nil, 0, fmt.Errorf("bootstrap: no valid service types after validation")
	}

	winners := make(map[string]*gatewayv1.GatewayRouteSpec)
	for _, kv := range rResp.Kvs {
		key := string(kv.Key)
		sfx := keySuffix(key, routesPrefix)
		if sfx == "" {
			slog.Warn("bootstrap: skip route key with empty suffix", "key", key)
			continue
		}
		var spec gatewayv1.GatewayRouteSpec
		if err := protoUnmarshal.Unmarshal(kv.Value, &spec); err != nil {
			slog.Warn("bootstrap: skip invalid gateway route json", "key", key, "error", err)
			continue
		}
		if err := validateGatewayRouteSpec(&spec, sfx); err != nil {
			slog.Warn("bootstrap: skip invalid gateway route spec", "key", key, "error", err)
			continue
		}
		st := spec.GetServiceType()
		if _, ok := typeConns[st]; !ok {
			slog.Warn("bootstrap: skip route, unknown service_type", "route_id", spec.GetRouteId(), "service_type", st)
			continue
		}
		wm := spec.GetWsMethod()
		if old, ok := winners[wm]; !ok || spec.GetRouteId() > old.GetRouteId() {
			winners[wm] = proto.Clone(&spec).(*gatewayv1.GatewayRouteSpec)
		}
	}

	bindings := make(map[string]ws.RouteBinding)
	for _, spec := range winners {
		conn := typeConns[spec.GetServiceType()]
		cliIngress := gatewayv1.NewGatewayIngressClient(conn)
		h := ws.WrapIngress(cliIngress, spec.GetIngressRoute())
		bindings[spec.GetWsMethod()] = ws.RouteBinding{
			Handler:          h,
			Public:           spec.GetPublic(),
			EstablishSession: spec.GetEstablishesSession(),
		}
	}

	var allConns []*grpc.ClientConn
	for _, c := range typeConns {
		allConns = append(allConns, c)
	}

	return ws.NewAtomicRoutes(bindings), allConns, nextWatchRev, nil
}
