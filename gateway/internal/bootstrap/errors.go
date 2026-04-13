package bootstrap

import "errors"

var (
	// ErrEmptyServiceTypes Bootstrap 时 service-types 前缀下物理 key 数为 0。
	ErrEmptyServiceTypes = errors.New("gateway meta: service-types prefix has no keys")
	// ErrEmptyGatewayRoutes Bootstrap 时 gateway-routes 前缀下物理 key 数为 0。
	ErrEmptyGatewayRoutes = errors.New("gateway meta: gateway-routes prefix has no keys")
)
