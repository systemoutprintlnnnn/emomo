#!/bin/sh
# 数据导入脚本
# 从本地文件夹导入表情包到数据库和向量库
# 使用 go run 直接运行，无需预先编译

set -e

# 切换到项目根目录（脚本所在目录的上级）
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
cd "$PROJECT_ROOT"

# 加载 .env 文件（如果存在）
if [ -f .env ]; then
    export $(grep -v '^#' .env | grep -v '^$' | xargs)
fi

# 默认配置
DEFAULT_CONFIG="${CONFIG_PATH:-./configs/config.yaml}"
DEFAULT_LIMIT=100

# 颜色输出
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# 打印带颜色的消息
info() {
    echo "${BLUE}[INFO]${NC} $1"
}

success() {
    echo "${GREEN}[SUCCESS]${NC} $1"
}

warning() {
    echo "${YELLOW}[WARNING]${NC} $1"
}

error() {
    echo "${RED}[ERROR]${NC} $1"
}

# 显示使用说明
usage() {
    cat << EOF
用法: $0 [选项]

选项:
    -p, --path PATH             表情包文件夹路径（覆盖配置文件中的设置）
    -l, --limit LIMIT           导入数量限制（默认: ${DEFAULT_LIMIT}）
    -e, --embedding NAME        使用的 embedding 配置名称（如 jina, qwen3）
                                留空则使用默认配置
    -c, --config PATH           配置文件路径（默认: ${DEFAULT_CONFIG}）
    -f, --force                 强制重新处理，跳过重复检查
    -m, --auto-migrate          启用数据库 AutoMigrate（默认: 否）
    -r, --retry                 重试 pending 状态的数据
    -h, --help                  显示此帮助信息

示例:
    # 从配置文件中指定的路径导入 100 条数据
    $0 -l 100

    # 指定文件夹路径导入
    $0 -p /path/to/your/memes -l 100

    # 使用 jina embedding 导入
    $0 -e jina -l 100

    # 强制重新处理（跳过重复检查）
    $0 -f

    # 重试 pending 状态的数据
    $0 -r -l 50

注意:
    如果不指定 -p/--path，将使用 configs/config.yaml 中 sources.local.path 的配置。

EOF
}

# 检查 Go 环境
check_go() {
    # Probe common install paths if not on PATH
    if ! command -v go >/dev/null 2>&1; then
        for gobin in /usr/local/go/bin/go /opt/homebrew/bin/go "$HOME/go/bin/go"; do
            if [ -x "$gobin" ]; then
                export PATH="$(dirname "$gobin"):$PATH"
                break
            fi
        done
    fi
    
    if ! command -v go >/dev/null 2>&1; then
        error "未找到 Go 环境"
        error "请先安装 Go: https://go.dev/doc/install"
        exit 1
    fi
}

# 主函数
main() {
    local folder_path=""
    local limit="$DEFAULT_LIMIT"
    local embedding=""
    local config_path="$DEFAULT_CONFIG"
    local force=false
    local auto_migrate=false
    local retry=false

    # 解析命令行参数
    while [ $# -gt 0 ]; do
        case "$1" in
            -p|--path)
                folder_path="$2"
                shift 2
                ;;
            -l|--limit)
                limit="$2"
                shift 2
                ;;
            -e|--embedding)
                embedding="$2"
                shift 2
                ;;
            -c|--config)
                config_path="$2"
                shift 2
                ;;
            -f|--force)
                force=true
                shift
                ;;
            -m|--auto-migrate)
                auto_migrate=true
                shift
                ;;
            -r|--retry)
                retry=true
                shift
                ;;
            -h|--help)
                usage
                exit 0
                ;;
            *)
                error "未知选项: $1"
                usage
                exit 1
                ;;
        esac
    done

    # 检查配置文件
    if [ ! -f "$config_path" ]; then
        error "配置文件不存在: $config_path"
        exit 1
    fi

    # 检查 Go 环境
    check_go

    # 如果指定了文件夹路径，临时设置环境变量
    if [ -n "$folder_path" ]; then
        if [ ! -d "$folder_path" ]; then
            error "文件夹不存在: $folder_path"
            exit 1
        fi
        export SOURCES_LOCAL_PATH="$folder_path"
    else
        # 未指定 path 时，做一个轻量提示：默认会使用 config 里的 sources.local.path
        warning "未指定 --path，将使用配置文件中的 sources.local.path（可用 -p 覆盖）"
    fi

    # 前置检查：如果指定了路径，尽量判断是否有图片文件
    if [ -n "$folder_path" ]; then
        img_count=$(find "$folder_path" -type f \( -iname "*.jpg" -o -iname "*.jpeg" -o -iname "*.png" -o -iname "*.gif" -o -iname "*.webp" -o -iname "*.bmp" \) 2>/dev/null | head -n 1 | wc -l | tr -d ' ')
        if [ "$img_count" = "0" ]; then
            warning "在指定目录下未检测到常见图片格式文件（jpg/jpeg/png/gif/webp/bmp），如果是空目录导入会没有数据"
        fi
    fi

    # 前置检查：embedding key（尽量提示，不强制）
    # 默认 embedding 在 config.yaml 里是 qwen3 (MODELSCOPE_API_KEY)
    if [ -z "$embedding" ]; then
        if [ -z "$MODELSCOPE_API_KEY" ] && [ -z "$JINA_API_KEY" ]; then
            warning "未检测到 MODELSCOPE_API_KEY / JINA_API_KEY。若导入时报 embedding 相关错误，请先在 .env 中配置对应 key"
        fi
    fi

    # 显示导入信息
    echo ""
    info "=========================================="
    info "开始数据导入"
    info "=========================================="
    if [ -n "$folder_path" ]; then
        info "数据源: 本地文件夹 ($folder_path)"
    else
        info "数据源: 本地文件夹 (使用配置文件路径)"
    fi
    info "限制数量: $limit"
    info "配置文件: $config_path"
    if [ -n "$embedding" ]; then
        info "Embedding 配置: $embedding"
    else
        info "Embedding 配置: 默认"
    fi
    if [ "$force" = true ]; then
        info "强制模式: 是（跳过重复检查）"
    fi
    if [ "$auto_migrate" = true ]; then
        info "AutoMigrate: 启用"
    fi
    info "=========================================="
    echo ""

    # 构建参数
    local args="--source=local --limit=$limit --config=$config_path"

    if [ -n "$embedding" ]; then
        args="$args --embedding=$embedding"
    fi
    if [ "$auto_migrate" = true ]; then
        args="$args --auto-migrate"
    fi
    if [ "$force" = true ]; then
        args="$args --force"
    fi
    if [ "$retry" = true ]; then
        args="$args --retry"
    fi

    info "执行命令: go run ./cmd/ingest $args"
    echo ""

    # 执行导入
    go run ./cmd/ingest $args
}

# 运行主函数
main "$@"
