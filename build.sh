#!/bin/bash
set -euo pipefail
ROOT="$(cd "$(dirname "$0")" && pwd)"
cd "$ROOT"

echo "根目录: $ROOT"

echo "构建 gateway"
# 构建 gateway
go -C "$ROOT/gateway" build -o "$ROOT/bin/gateway" ./cmd/gateway

echo "构建 user"
# 构建 user
go -C "$ROOT/services/user" build -o "$ROOT/bin/user" ./cmd/user