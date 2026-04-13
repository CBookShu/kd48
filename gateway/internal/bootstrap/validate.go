package bootstrap

import (
	"fmt"

	gatewayv1 "github.com/CBookShu/kd48/api/proto/gateway/v1"
)

func keySuffix(etcdKey, prefix string) string {
	if len(etcdKey) < len(prefix) || etcdKey[:len(prefix)] != prefix {
		return ""
	}
	s := etcdKey[len(prefix):]
	for len(s) > 0 && s[0] == '/' {
		s = s[1:]
	}
	for len(s) > 0 && s[len(s)-1] == '/' {
		s = s[:len(s)-1]
	}
	return s
}

func validateServiceTypeSpec(spec *gatewayv1.ServiceTypeSpec, suffix string) error {
	if spec == nil {
		return fmt.Errorf("nil spec")
	}
	if spec.GetSchemaVersion() != 1 {
		return fmt.Errorf("schemaVersion want 1, got %d", spec.GetSchemaVersion())
	}
	if spec.GetTypeKey() == "" || spec.GetTypeKey() != suffix {
		return fmt.Errorf("typeKey %q must match etcd key suffix %q", spec.GetTypeKey(), suffix)
	}
	if spec.GetRoutingMode() != gatewayv1.ServiceRoutingMode_SERVICE_ROUTING_MODE_STATELESS_LB {
		return fmt.Errorf("routingMode must be STATELESS_LB, got %v", spec.GetRoutingMode())
	}
	d := spec.GetDiscovery()
	if d == nil || d.GetGrpcEtcdTarget() == "" {
		return fmt.Errorf("discovery.grpcEtcdTarget required")
	}
	ing := spec.GetIngress()
	if ing == nil || !ing.GetUseGatewayIngress() {
		return fmt.Errorf("ingress.useGatewayIngress must be true")
	}
	return nil
}

func validateGatewayRouteSpec(spec *gatewayv1.GatewayRouteSpec, suffix string) error {
	if spec == nil {
		return fmt.Errorf("nil spec")
	}
	if spec.GetSchemaVersion() != 1 {
		return fmt.Errorf("schemaVersion want 1, got %d", spec.GetSchemaVersion())
	}
	if spec.GetRouteId() == "" || spec.GetRouteId() != suffix {
		return fmt.Errorf("routeId %q must match etcd key suffix %q", spec.GetRouteId(), suffix)
	}
	if spec.GetWsMethod() == "" {
		return fmt.Errorf("wsMethod required")
	}
	if spec.GetIngressRoute() == "" {
		return fmt.Errorf("ingressRoute required")
	}
	if spec.GetServiceType() == "" {
		return fmt.Errorf("serviceType required")
	}
	return nil
}

func defaultServiceConfigJSON(loadBalancing string) string {
	if loadBalancing == "" || loadBalancing == "round_robin" {
		return `{"loadBalancingConfig": [{"round_robin":{}}]}`
	}
	// 预留：未来可接受完整 JSON 字符串
	return `{"loadBalancingConfig": [{"round_robin":{}}]}`
}
