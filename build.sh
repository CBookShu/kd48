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