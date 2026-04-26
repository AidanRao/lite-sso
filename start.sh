#!/bin/bash

set -e

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
NC='\033[0m'

info()  { echo -e "${GREEN}[INFO]${NC}  $1"; }
warn()  { echo -e "${YELLOW}[WARN]${NC}  $1"; }
error() { echo -e "${RED}[ERROR]${NC} $1"; }

PROJECT_DIR="$(cd "$(dirname "$0")" && pwd)"
cd "$PROJECT_DIR"

check_docker() {
    if ! command -v docker &>/dev/null; then
        error "docker 未安装，请先安装 Docker"
        exit 1
    fi
    if ! docker info &>/dev/null; then
        error "Docker 未启动，请先启动 Docker"
        exit 1
    fi
}

wait_for() {
    local name=$1
    local host=$2
    local port=$3
    local max=30
    local i=0
    info "等待 ${name} 启动 (${host}:${port})..."
    while ! nc -z "$host" "$port" 2>/dev/null; do
        i=$((i + 1))
        if [ "$i" -ge "$max" ]; then
            error "${name} 启动超时"
            exit 1
        fi
        sleep 1
    done
    info "${name} 已就绪"
}

start_deps() {
    info "启动依赖服务 (PostgreSQL + Redis)..."
    docker compose up -d
    wait_for "PostgreSQL" "localhost" "54323"
    wait_for "Redis" "localhost" "63783"
}

stop_deps() {
    info "停止依赖服务..."
    docker compose down
}

run_seed() {
    info "执行数据库迁移和种子数据..."
    go run ./cmd/seed
}

start_server() {
    info "启动 SSO 服务器..."
    export ENV=local
    export DEV_ECHO_OTP=true
    go run ./cmd/server
}

case "${1:-}" in
    deps)
        start_deps
        ;;
    seed)
        run_seed
        ;;
    server)
        start_server
        ;;
    stop)
        stop_deps
        ;;
    *)
        start_deps
        echo ""
        info "========================================="
        info "  SSO 开发环境已就绪"
        info "  服务地址: http://localhost:8080"
        info "  PostgreSQL: localhost:54323"
        info "  Redis: localhost:63783"
        info "========================================="
        echo ""
        start_server
        ;;
esac
