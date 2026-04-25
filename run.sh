#!/bin/bash
set -euo pipefail

ROOT="$(cd "$(dirname "$0")" && pwd)"
cd "$ROOT"

PID_FILE="$ROOT/run.pid"
LOG_DIR="$ROOT/logs"

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# 确保日志目录存在
mkdir -p "$LOG_DIR"

# 日志函数
log_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# 检查依赖服务
check_dependencies() {
    log_info "检查依赖服务..."

    # 检查 Redis (支持 redis-cli 或 Docker)
    if redis-cli -h localhost -p 6379 ping >/dev/null 2>&1; then
        log_info "Redis ✓"
    elif docker exec kd48-redis-1 redis-cli ping >/dev/null 2>&1; then
        log_info "Redis ✓ (Docker)"
    else
        log_error "Redis 未运行 (localhost:6379)"
        return 1
    fi

    # 检查 MySQL (支持 mysqladmin 或 Docker)
    if mysqladmin -h localhost -u root -proot ping >/dev/null 2>&1; then
        log_info "MySQL ✓"
    elif docker exec kd48-mysql-1 mysqladmin ping -h 127.0.0.1 -u root -proot >/dev/null 2>&1; then
        log_info "MySQL ✓ (Docker)"
    else
        log_error "MySQL 未运行 (localhost:3306)"
        return 1
    fi

    # 检查 etcd
    if etcdctl --endpoints=localhost:2379 endpoint health >/dev/null 2>&1; then
        log_info "etcd ✓"
    else
        log_error "etcd 未运行 (localhost:2379)"
        return 1
    fi

    return 0
}

# 构建所有服务
build() {
    log_info "构建服务..."

    echo "构建 gateway"
    go -C "$ROOT/gateway" build -o "$ROOT/gateway.bin" ./cmd/gateway

    echo "构建 user"
    go -C "$ROOT/services/user" build -o "$ROOT/user.bin" ./cmd/user

    echo "构建 lobby"
    go -C "$ROOT/services/lobby" build -o "$ROOT/lobby.bin" ./cmd/lobby

    echo "构建 seed-gateway-meta"
    go -C "$ROOT/gateway" build -o "$ROOT/seed-gateway-meta.bin" ./cmd/seed-gateway-meta

    log_info "构建完成"
}

# 启动所有服务
start() {
    # 检查是否已在运行
    if [ -f "$PID_FILE" ]; then
        log_warn "服务已在运行，PID: $(cat $PID_FILE)"
        log_warn "如果需要重启，请先执行: $0 stop"
        exit 1
    fi

    # 检查依赖
    if ! check_dependencies; then
        log_error "请先启动依赖服务 (Redis, MySQL, etcd)"
        exit 1
    fi

    # 构建（如需要）
    if [ ! -f "$ROOT/gateway.bin" ] || [ ! -f "$ROOT/user.bin" ] || [ ! -f "$ROOT/lobby.bin" ]; then
        build
    fi

    log_info "启动服务..."

    # 种子网关元数据（只执行一次）
    log_info "种子网关元数据到 etcd..."
    if ./seed-gateway-meta.bin -endpoints=127.0.0.1:2379; then
        log_info "网关元数据已 seed"
    else
        log_warn "seed 失败，可能是重复执行（忽略）"
    fi

    # 启动服务（按依赖顺序）
    # 1. user service (gRPC)
    log_info "启动 user service (port 9000)..."
    ./user.bin > "$LOG_DIR/user.log" 2>&1 &
    USER_PID=$!
    echo "user: $USER_PID" >> "$PID_FILE"

    # 2. lobby service (gRPC)
    log_info "启动 lobby service (port 9001)..."
    ./lobby.bin > "$LOG_DIR/lobby.log" 2>&1 &
    LOBBY_PID=$!
    echo "lobby: $LOBBY_PID" >> "$PID_FILE"

    # 3. gateway (HTTP/WS)
    log_info "启动 gateway (port 8080)..."
    ./gateway.bin > "$LOG_DIR/gateway.log" 2>&1 &
    GATEWAY_PID=$!
    echo "gateway: $GATEWAY_PID" >> "$PID_FILE"

    # 等待服务启动
    sleep 2

    # 检查服务是否成功启动
    local running=true
    for pid in $USER_PID $LOBBY_PID $GATEWAY_PID; do
        if ! kill -0 "$pid" 2>/dev/null; then
            log_error "服务启动失败"
            running=false
        fi
    done

    if [ "$running" = true ]; then
        log_info "所有服务已启动"
        log_info "  - user service:  localhost:9000"
        log_info "  - lobby service: localhost:9001"
        log_info "  - gateway:       localhost:8080"
        log_info ""
        log_info "访问 http://localhost:8080"
    else
        log_error "部分服务启动失败，请查看日志"
        stop
        exit 1
    fi
}

# 停止所有服务
stop() {
    if [ ! -f "$PID_FILE" ]; then
        log_warn "没有正在运行的服务"
        return
    fi

    log_info "停止服务..."

    # 读取并杀掉所有进程
    while IFS=': ' read -r name pid; do
        if [ -n "$pid" ] && kill -0 "$pid" 2>/dev/null; then
            log_info "停止 $name (PID: $pid)..."
            kill "$pid" 2>/dev/null || true

            # 等待进程结束
            local count=0
            while kill -0 "$pid" 2>/dev/null && [ $count -lt 10 ]; do
                sleep 0.5
                count=$((count + 1))
            done

            # 强制杀掉如果还没结束
            if kill -0 "$pid" 2>/dev/null; then
                kill -9 "$pid" 2>/dev/null || true
            fi
        fi
    done < "$PID_FILE"

    # 清理 PID 文件
    rm -f "$PID_FILE"

    log_info "所有服务已停止"
}

# 查看服务状态
status() {
    if [ ! -f "$PID_FILE" ]; then
        log_warn "服务未运行"
        return
    fi

    log_info "服务状态:"
    while IFS=': ' read -r name pid; do
        if [ -n "$pid" ] && kill -0 "$pid" 2>/dev/null; then
            echo -e "  ${GREEN}$name${NC}: 运行中 (PID: $pid)"
        else
            echo -e "  ${RED}$name${NC}: 已停止"
        fi
    done < "$PID_FILE"

    # 检查端口
    echo ""
    log_info "端口监听:"
    if netstat -tln 2>/dev/null | grep -q ":8080 "; then
        echo -e "  ${GREEN}gateway:8080${NC}"
    fi
    if netstat -tln 2>/dev/null | grep -q ":9000 "; then
        echo -e "  ${GREEN}user:9000${NC}"
    fi
    if netstat -tln 2>/dev/null | grep -q ":9001 "; then
        echo -e "  ${GREEN}lobby:9001${NC}"
    fi
}

# 查看日志
logs() {
    local service="${1:-}"

    if [ -z "$service" ]; then
        log_info "用法: $0 logs <gateway|user|lobby>"
        exit 1
    fi

    case "$service" in
        gateway)
            tail -f "$LOG_DIR/gateway.log"
            ;;
        user)
            tail -f "$LOG_DIR/user.log"
            ;;
        lobby)
            tail -f "$LOG_DIR/lobby.log"
            ;;
        *)
            log_error "未知服务: $service"
            exit 1
            ;;
    esac
}

# 重新构建
rebuild() {
    log_info "重新构建..."
    rm -f "$ROOT/gateway.bin" "$ROOT/user.bin" "$ROOT/lobby.bin" "$ROOT/seed-gateway-meta.bin"
    build
}

# 显示帮助
help() {
    echo "Usage: $0 <command>"
    echo ""
    echo "Commands:"
    echo "  start     启动所有服务"
    echo "  stop      停止所有服务"
    echo "  status    查看服务状态"
    echo "  logs      查看服务日志 (需要指定服务名)"
    echo "  rebuild   重新构建"
    echo "  build     仅构建"
    echo ""
    echo "Examples:"
    echo "  $0 start"
    echo "  $0 stop"
    echo "  $0 status"
    echo "  $0 logs gateway"
    echo "  $0 rebuild"
}

# 主命令处理
case "${1:-help}" in
    start)
        start
        ;;
    stop)
        stop
        ;;
    status)
        status
        ;;
    logs)
        logs "${2:-}"
        ;;
    rebuild)
        rebuild
        ;;
    build)
        build
        ;;
    *)
        help
        ;;
esac
