#!/bin/bash
set -euo pipefail
ROOT="$(cd "$(dirname "$0")" && pwd)"
cd "$ROOT"

echo "根目录: $ROOT"

echo "构建 gateway"
# 构建 gateway
go -C "$ROOT/gateway" build -o "$ROOT/gateway.bin" ./cmd/gateway

echo "构建 user"
# 构建 user
go -C "$ROOT/services/user" build -o "$ROOT/user.bin" ./cmd/user

echo "构建 lobby"
# 构建 lobby
go -C "$ROOT/services/lobby" build -o "$ROOT/lobby.bin" ./cmd/lobby

echo "构建 seed-gateway-meta"
# 构建 seed-gateway-meta（部署时写入 Etcd 元数据）
go -C "$ROOT/gateway" build -o "$ROOT/seed-gateway-meta.bin" ./cmd/seed-gateway-meta

echo "构建 CLI"
# 构建 CLI
go -C "$ROOT/cmd/cli" build -o "$ROOT/kd48-cli" .