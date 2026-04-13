package bootstrap

import (
	"testing"

	gatewayv1 "github.com/CBookShu/kd48/api/proto/gateway/v1"
)

func TestKeySuffix(t *testing.T) {
	p := "kd48/meta/service-types/"
	if got := keySuffix("kd48/meta/service-types/user", p); got != "user" {
		t.Fatalf("got %q", got)
	}
	if got := keySuffix("kd48/meta/service-types//user//", p); got != "user" {
		t.Fatalf("got %q", got)
	}
}

func TestValidateServiceTypeSpec_OK(t *testing.T) {
	spec := &gatewayv1.ServiceTypeSpec{
		SchemaVersion: 1,
		TypeKey:       "user",
		RoutingMode:   gatewayv1.ServiceRoutingMode_SERVICE_ROUTING_MODE_STATELESS_LB,
		Discovery: &gatewayv1.ServiceTypeDiscovery{
			GrpcEtcdTarget: "etcd:///kd48/user-service",
			LoadBalancing:  "round_robin",
		},
		Ingress: &gatewayv1.ServiceTypeIngress{UseGatewayIngress: true},
	}
	if err := validateServiceTypeSpec(spec, "user"); err != nil {
		t.Fatal(err)
	}
}

func TestValidateGatewayRouteSpec_OK(t *testing.T) {
	spec := &gatewayv1.GatewayRouteSpec{
		SchemaVersion:       1,
		RouteId:             "login",
		WsMethod:            "/user.v1.UserService/Login",
		ServiceType:         "user",
		IngressRoute:        "/user.v1.UserService/Login",
		Public:              true,
		EstablishesSession:  true,
	}
	if err := validateGatewayRouteSpec(spec, "login"); err != nil {
		t.Fatal(err)
	}
}
