#!/bin/bash

# 启动前后端服务的脚本
# 使用方法: ./scripts/start.sh

set -e

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# 获取脚本所在目录的绝对路径
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
FRONTEND_DIR="$PROJECT_ROOT/../emomo-frontend"

# PID 文件
BACKEND_PID_FILE="/tmp/emomo-backend.pid"
FRONTEND_PID_FILE="/tmp/emomo-frontend.pid"

# 清理函数
cleanup() {
    echo -e "\n${YELLOW}正在关闭服务...${NC}"
    
    # 关闭后端
    if [ -f "$BACKEND_PID_FILE" ]; then
        BACKEND_PID=$(cat "$BACKEND_PID_FILE")
        if ps -p "$BACKEND_PID" > /dev/null 2>&1; then
            echo -e "${BLUE}关闭后端服务 (PID: $BACKEND_PID)...${NC}"
            kill "$BACKEND_PID" 2>/dev/null || true
            wait "$BACKEND_PID" 2>/dev/null || true
        fi
        rm -f "$BACKEND_PID_FILE"
    fi
    
    # 关闭前端
    if [ -f "$FRONTEND_PID_FILE" ]; then
        FRONTEND_PID=$(cat "$FRONTEND_PID_FILE")
        if ps -p "$FRONTEND_PID" > /dev/null 2>&1; then
            echo -e "${BLUE}关闭前端服务 (PID: $FRONTEND_PID)...${NC}"
            kill "$FRONTEND_PID" 2>/dev/null || true
            wait "$FRONTEND_PID" 2>/dev/null || true
        fi
        rm -f "$FRONTEND_PID_FILE"
    fi
    
    echo -e "${GREEN}所有服务已关闭${NC}"
    exit 0
}

# 注册清理函数
trap cleanup SIGINT SIGTERM

# 检查依赖
check_dependencies() {
    echo -e "${BLUE}检查依赖...${NC}"
    
    # 检查 Go
    if ! command -v go &> /dev/null; then
        echo -e "${RED}错误: 未找到 Go，请先安装 Go${NC}"
        exit 1
    fi
    echo -e "${GREEN}✓ Go 已安装: $(go version)${NC}"
    
    # 检查 Node.js
    if ! command -v node &> /dev/null; then
        echo -e "${RED}错误: 未找到 Node.js，请先安装 Node.js${NC}"
        exit 1
    fi
    echo -e "${GREEN}✓ Node.js 已安装: $(node --version)${NC}"
    
    # 检查 npm
    if ! command -v npm &> /dev/null; then
        echo -e "${RED}错误: 未找到 npm，请先安装 npm${NC}"
        exit 1
    fi
    echo -e "${GREEN}✓ npm 已安装: $(npm --version)${NC}"
}

# 检查前端目录
check_frontend() {
    if [ ! -d "$FRONTEND_DIR" ]; then
        echo -e "${RED}错误: 未找到前端目录: $FRONTEND_DIR${NC}"
        exit 1
    fi
    
    if [ ! -f "$FRONTEND_DIR/package.json" ]; then
        echo -e "${RED}错误: 前端目录中未找到 package.json${NC}"
        exit 1
    fi
}

# 启动后端服务
start_backend() {
    echo -e "\n${BLUE}启动后端服务...${NC}"
    cd "$PROJECT_ROOT"
    
    # 检查配置文件
    if [ ! -f "$PROJECT_ROOT/configs/config.yaml" ]; then
        echo -e "${RED}错误: 未找到配置文件 configs/config.yaml${NC}"
        exit 1
    fi
    
    # 启动后端（后台运行）
    echo -e "${GREEN}后端服务启动中... (端口: 8080)${NC}"
    go run cmd/api/main.go > /tmp/emomo-backend.log 2>&1 &
    BACKEND_PID=$!
    echo "$BACKEND_PID" > "$BACKEND_PID_FILE"
    
    # 等待后端启动
    echo -e "${YELLOW}等待后端服务启动...${NC}"
    for i in {1..30}; do
        if curl -s http://localhost:8080/health > /dev/null 2>&1; then
            echo -e "${GREEN}✓ 后端服务已启动 (PID: $BACKEND_PID)${NC}"
            return 0
        fi
        sleep 1
    done
    
    echo -e "${RED}错误: 后端服务启动超时${NC}"
    echo -e "${YELLOW}后端日志:${NC}"
    tail -20 /tmp/emomo-backend.log
    exit 1
}

# 启动前端服务
start_frontend() {
    echo -e "\n${BLUE}启动前端服务...${NC}"
    cd "$FRONTEND_DIR"
    
    # 检查 node_modules
    if [ ! -d "$FRONTEND_DIR/node_modules" ]; then
        echo -e "${YELLOW}未找到 node_modules，正在安装依赖...${NC}"
        npm install
    fi
    
    # 启动前端（后台运行）
    echo -e "${GREEN}前端服务启动中... (端口: 5173)${NC}"
    npm run dev > /tmp/emomo-frontend.log 2>&1 &
    FRONTEND_PID=$!
    echo "$FRONTEND_PID" > "$FRONTEND_PID_FILE"
    
    # 等待前端启动
    echo -e "${YELLOW}等待前端服务启动...${NC}"
    sleep 3
    
    if ps -p "$FRONTEND_PID" > /dev/null 2>&1; then
        echo -e "${GREEN}✓ 前端服务已启动 (PID: $FRONTEND_PID)${NC}"
    else
        echo -e "${RED}错误: 前端服务启动失败${NC}"
        echo -e "${YELLOW}前端日志:${NC}"
        tail -20 /tmp/emomo-frontend.log
        exit 1
    fi
}

# 显示服务信息
show_info() {
    echo -e "\n${GREEN}========================================${NC}"
    echo -e "${GREEN}  服务已启动！${NC}"
    echo -e "${GREEN}========================================${NC}"
    echo -e "${BLUE}后端 API:${NC}  http://localhost:8080"
    echo -e "${BLUE}前端应用:${NC}  http://localhost:5173"
    echo -e "${BLUE}健康检查:${NC}  http://localhost:8080/health"
    echo -e "${GREEN}========================================${NC}"
    echo -e "${YELLOW}按 Ctrl+C 停止所有服务${NC}\n"
}

# 显示日志
show_logs() {
    echo -e "\n${BLUE}实时日志 (按 Ctrl+C 停止):${NC}\n"
    
    # 使用 tail 同时显示两个日志文件
    tail -f /tmp/emomo-backend.log /tmp/emomo-frontend.log 2>/dev/null &
    TAIL_PID=$!
    
    # 等待用户中断
    wait $TAIL_PID 2>/dev/null || true
}

# 主函数
main() {
    echo -e "${GREEN}"
    echo "=========================================="
    echo "   Emomo 前后端启动脚本"
    echo "=========================================="
    echo -e "${NC}"
    
    check_dependencies
    check_frontend
    start_backend
    start_frontend
    show_info
    
    # 询问是否显示日志
    echo -e "${YELLOW}是否显示实时日志? (y/n, 默认: n)${NC}"
    read -t 3 -n 1 SHOW_LOGS || SHOW_LOGS="n"
    echo ""
    
    if [ "$SHOW_LOGS" = "y" ] || [ "$SHOW_LOGS" = "Y" ]; then
        show_logs
    else
        echo -e "${BLUE}日志文件位置:${NC}"
        echo -e "  后端: /tmp/emomo-backend.log"
        echo -e "  前端: /tmp/emomo-frontend.log"
        echo -e "\n${YELLOW}按 Ctrl+C 停止所有服务${NC}"
        # 等待信号
        wait
    fi
}

# 运行主函数
main

