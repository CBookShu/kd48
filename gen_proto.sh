#!/bin/bash
set -euo pipefail
ROOT="$(cd "$(dirname "$0")" && pwd)"
cd "$ROOT"

protoc -I api/proto \
	--go_out=api/proto --go_opt=module=github.com/CBookShu/kd48/api/proto \
	--go-grpc_out=api/proto --go-grpc_opt=module=github.com/CBookShu/kd48/api/proto \
	user/v1/user.proto \
	gateway/v1/gateway.proto \
	gateway/v1/service_type.proto \
	gateway/v1/gateway_route.proto
