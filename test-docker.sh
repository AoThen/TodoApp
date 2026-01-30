#!/bin/bash

# TodoApp Docker 构建测试脚本
# 用于验证 Dockerfile 和 docker-compose.yml 配置是否正确

set -e

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# 打印函数
print_success() {
    echo -e "${GREEN}✅ $1${NC}"
}

print_error() {
    echo -e "${RED}❌ $1${NC}"
}

print_info() {
    echo -e "${YELLOW}ℹ️  $1${NC}"
}

print_header() {
    echo ""
    echo -e "${YELLOW}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    echo -e "${YELLOW}  $1${NC}"
    echo -e "${YELLOW}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
}

# 检查 Docker 是否安装
check_docker() {
    print_header "检查 Docker 安装"

    if ! command -v docker &> /dev/null; then
        print_error "Docker 未安装，请先安装 Docker"
        exit 1
    fi

    DOCKER_VERSION=$(docker --version | awk '{print $3}' | sed 's/,//')
    print_success "Docker 已安装 (版本: $DOCKER_VERSION)"
}

# 检查 Docker Compose 是否安装
check_docker_compose() {
    print_header "检查 Docker Compose 安装"

    if ! command -v docker-compose &> /dev/null; then
        # 尝试使用 docker compose (v2)
        if docker compose version &> /dev/null; then
            print_success "Docker Compose v2 已安装"
            return 0
        fi
        print_error "Docker Compose 未安装，请先安装 Docker Compose"
        exit 1
    fi

    COMPOSE_VERSION=$(docker-compose --version | awk '{print $4}' | sed 's/,//')
    print_success "Docker Compose 已安装 (版本: $COMPOSE_VERSION)"
}

# 检查必需文件
check_required_files() {
    print_header "检查必需文件"

    required_files=(
        "Dockerfile"
        "docker-compose.yml"
        ".env.docker"
        "go.mod"
        "go.sum"
        "web/package.json"
    )

    missing_files=0
    for file in "${required_files[@]}"; do
        if [ ! -f "$file" ]; then
            print_error "缺少文件: $file"
            ((missing_files++))
        else
            print_success "文件存在: $file"
        fi
    done

    if [ $missing_files -gt 0 ]; then
        print_error "缺少 $missing_files 个必需文件，请检查项目结构"
        exit 1
    fi
}

# 设置环境变量
setup_env() {
    print_header "设置环境变量"

    if [ ! -f ".env" ]; then
        print_info "创建 .env 文件..."
        cp .env.docker .env

        # 生成随机的 JWT_SECRET
        JWT_SECRET=$(openssl rand -hex 32)
        sed -i "s/your-secure-jwt-secret-key-min-32-characters-long/$JWT_SECRET/" .env

        print_success "环境变量已配置"
    else
        print_info ".env 文件已存在"
    fi
}

# 构建 Docker 镜像
build_image() {
    print_header "构建 Docker 镜像"

    print_info "开始构建镜像..."
    if docker build -t todoapp:latest .; then
        print_success "镜像构建成功"

        # 显示镜像大小
        IMAGE_SIZE=$(docker images todoapp:latest --format "{{.Size}}")
        print_info "镜像大小: $IMAGE_SIZE"
    else
        print_error "镜像构建失败"
        exit 1
    fi
}

# 启动容器
start_container() {
    print_header "启动容器"

    # 停止现有容器（如果有）
    docker-compose down 2>/dev/null || true

    # 启动新容器
    print_info "启动容器..."
    if docker-compose up -d; then
        print_success "容器启动成功"
    else
        print_error "容器启动失败"
        exit 1
    fi

    # 等待容器就绪
    print_info "等待容器就绪..."
    sleep 3
}

# 检查容器状态
check_container_status() {
    print_header "检查容器状态"

    STATUS=$(docker-compose ps -q todoapp | xargs docker inspect --format='{{.State.Status}}' 2>/dev/null)

    if [ "$STATUS" = "running" ]; then
        print_success "容器运行正常"
    else
        print_error "容器状态异常: $STATUS"
        docker-compose logs todoapp
        exit 1
    fi
}

# 测试健康检查
test_health_check() {
    print_header "测试健康检查"

    print_info "等待健康检查..."
    sleep 2

    MAX_ATTEMPTS=5
    ATTEMPT=1

    while [ $ATTEMPT -le $MAX_ATTEMPTS ]; do
        print_info "尝试 $ATTEMPT/$MAX_ATTEMPTS..."

        if curl -s http://localhost:8080/api/v1/health > /dev/null 2>&1; then
            print_success "健康检查通过"

            # 显示健康检查响应
            HEALTH_RESPONSE=$(curl -s http://localhost:8080/api/v1/health)
            print_info "响应: $HEALTH_RESPONSE"
            return 0
        fi

        sleep 2
        ((ATTEMPT++))
    done

    print_error "健康检查失败"
    docker-compose logs todoapp
    exit 1
}

# 测试 API 端点
test_api_endpoints() {
    print_header "测试 API 端点"

    # 测试登录端点（应该失败，因为没有用户）
    print_info "测试登录端点..."
    LOGIN_RESPONSE=$(curl -s -X POST http://localhost:8080/api/v1/auth/login \
        -H "Content-Type: application/json" \
        -d '{"email":"test@test.com","password":"test"}')

    if echo "$LOGIN_RESPONSE" | grep -q "error"; then
        print_success "登录端点正常（预期失败）"
    else
        print_error "登录端点异常"
        echo "响应: $LOGIN_RESPONSE"
    fi
}

# 显示容器日志
show_logs() {
    print_header "容器日志（最近 20 行）"

    docker-compose logs --tail=20 todoapp
}

# 清理
cleanup() {
    print_header "清理测试环境"

    print_info "停止容器..."
    docker-compose down

    print_success "清理完成"
}

# 主函数
main() {
    echo ""
    echo -e "${YELLOW}"
    echo "╔════════════════════════════════════════════════════════════════╗"
    echo "║                                                                ║"
    echo "║          TodoApp Docker 构建测试脚本                          ║"
    echo "║                                                                ║"
    echo "╚════════════════════════════════════════════════════════════════╝"
    echo -e "${NC}"

    # 运行测试
    check_docker
    check_docker_compose
    check_required_files
    setup_env
    build_image
    start_container
    check_container_status
    test_health_check
    test_api_endpoints
    show_logs

    print_header "测试完成"

    print_success "所有测试通过！✨"
    print_info "应用已启动并运行在 http://localhost:8080"
    print_info ""
    print_info "使用以下命令管理容器:"
    print_info "  查看日志: docker-compose logs -f"
    print_info "  停止容器: docker-compose down"
    print_info "  重启容器: docker-compose restart"
    print_info ""
    print_info "清理测试环境? [y/N]"

    read -r response
    if [[ "$response" =~ ^([yY][eE][sS]|[yY])$ ]]; then
        cleanup
    fi

    print_success "脚本执行完成！"
}

# 运行主函数
main "$@"
