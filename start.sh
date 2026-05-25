#!/bin/bash

set -e

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
NC='\033[0m'

info()  { printf '%b\n' "${GREEN}[INFO]${NC}  $1"; }
warn()  { printf '%b\n' "${YELLOW}[WARN]${NC}  $1"; }
error() { printf '%b\n' "${RED}[ERROR]${NC} $1"; }

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
    local endpoint=$2
    shift 2
    local max=30
    local i=0
    info "等待 ${name} 启动 (${endpoint})..."
    while ! "$@" >/dev/null 2>&1; do
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
    docker compose -f docker-compose-dev.yml up -d
    wait_for "PostgreSQL" "localhost:54323" docker compose -f docker-compose-dev.yml exec -T postgres pg_isready -U postgres -d sso-server
    wait_for "Redis" "localhost:63783" docker compose -f docker-compose-dev.yml exec -T redis redis-cli -a 123456 ping
}

stop_deps() {
    info "停止依赖服务..."
    docker compose -f docker-compose-dev.yml down
}

run_migrate() {
    local command="${1:-up}"
    info "执行数据库迁移 (${command})..."
    ENV=local go run ./cmd/migrate "$@"
}

build_frontend() {
    if ! command -v npm &>/dev/null; then
        error "npm 未安装，请先安装 Node.js"
        exit 1
    fi

    info "构建前端资源..."
    (
        cd "$PROJECT_DIR/frontend"
        npm run build
    )

    info "发布前端资源到 web 目录..."
    mkdir -p "$PROJECT_DIR/web"
    rm -rf "$PROJECT_DIR/web/assets"
    cp -R "$PROJECT_DIR/frontend/dist/." "$PROJECT_DIR/web/"
    info "前端资源已就绪"
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
    migrate)
        shift
        run_migrate "$@"
        ;;
    frontend)
        build_frontend
        ;;
    server)
        run_migrate up
        start_server
        ;;
    stop)
        stop_deps
        ;;
    *)
        start_deps
        run_migrate up
        build_frontend
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
