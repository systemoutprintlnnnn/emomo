#!/bin/sh
# 数据导入脚本
# 用于从 staging 目录或 chinesebqb 导入数据到数据库和向量库

set -e

# 加载 .env 文件（如果存在）
if [ -f .env ]; then
    export $(grep -v '^#' .env | grep -v '^$' | xargs)
fi

# 默认配置
DEFAULT_CONFIG="${CONFIG_PATH:-./configs/config.yaml}"
DEFAULT_STAGING_PATH="${STAGING_PATH:-./data/staging}"
DEFAULT_LIMIT=100
DEFAULT_WORKERS=5
DEFAULT_BATCH_SIZE=10

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
    -s, --source SOURCE         数据源类型
                                - chinesebqb: 从 ChineseBQB 仓库导入
                                - staging:SOURCE_ID: 从 staging 目录导入（如 staging:fabiaoqing）
                                - staging: 交互式选择 staging 源
    -l, --limit LIMIT           导入数量限制（默认: ${DEFAULT_LIMIT}）
    -e, --embedding NAME        使用的 embedding 配置名称（如 jina, qwen3）
                                留空则使用默认配置
    -c, --config PATH           配置文件路径（默认: ${DEFAULT_CONFIG}）
    -f, --force                 强制重新处理，跳过重复检查
    -r, --retry                 重试 pending 状态的数据
    -h, --help                  显示此帮助信息

示例:
    # 从 staging:fabiaoqing 导入 50 条数据
    $0 -s staging:fabiaoqing -l 50

    # 使用 jina embedding 导入
    $0 -s staging:fabiaoqing -e jina -l 100

    # 强制重新处理（跳过重复检查）
    $0 -s staging:fabiaoqing -f

    # 重试 pending 状态的数据
    $0 -r -l 50

    # 交互式选择 staging 源
    $0 -s staging

EOF
}

# 列出可用的 staging 源
list_staging_sources() {
    local staging_path="$1"
    if [ ! -d "$staging_path" ]; then
        warning "Staging 目录不存在: $staging_path"
        return 1
    fi

    info "扫描 staging 目录: $staging_path"
    local sources=""
    for dir in "$staging_path"/*; do
        if [ -d "$dir" ] && [ -f "$dir/manifest.jsonl" ]; then
            local source_id=$(basename "$dir")
            if [ -z "$sources" ]; then
                sources="$source_id"
            else
                sources="$sources $source_id"
            fi
        fi
    done

    if [ -z "$sources" ]; then
        warning "未找到可用的 staging 源"
        return 1
    fi

    echo "$sources"
}

# 交互式选择 staging 源
interactive_select_staging() {
    local staging_path="$1"
    local sources=$(list_staging_sources "$staging_path")
    
    if [ $? -ne 0 ] || [ -z "$sources" ]; then
        error "没有可用的 staging 源"
        exit 1
    fi

    echo ""
    info "可用的 staging 源:"
    local index=1
    local source_array=""
    for source in $sources; do
        echo "  $index) $source"
        if [ -z "$source_array" ]; then
            source_array="$source"
        else
            source_array="$source_array $source"
        fi
        index=$((index + 1))
    done
    echo ""

    while true; do
        printf "请选择源 (1-$((index - 1))): "
        read choice
        if [ -z "$choice" ]; then
            error "输入不能为空"
            continue
        fi
        
        local selected_index=1
        for source in $source_array; do
            if [ "$selected_index" -eq "$choice" ] 2>/dev/null; then
                echo "$source"
                return 0
            fi
            selected_index=$((selected_index + 1))
        done
        
        error "无效的选择，请重新输入"
    done
}

# 检查 ingest 二进制文件是否存在
check_ingest_binary() {
    if [ -f "./ingest" ]; then
        echo "./ingest"
    elif command -v ingest >/dev/null 2>&1; then
        echo "ingest"
    else
        error "未找到 ingest 二进制文件"
        error "请先构建: go build -o ingest ./cmd/ingest"
        exit 1
    fi
}

# 主函数
main() {
    local source_type=""
    local limit="$DEFAULT_LIMIT"
    local embedding=""
    local config_path="$DEFAULT_CONFIG"
    local force=false
    local retry=false

    # 解析命令行参数
    while [ $# -gt 0 ]; do
        case "$1" in
            -s|--source)
                source_type="$2"
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

    # 检查 ingest 二进制文件
    local ingest_binary=$(check_ingest_binary)

    # 如果是重试模式
    if [ "$retry" = true ]; then
        info "重试 pending 状态的数据..."
        info "配置文件: $config_path"
        info "限制数量: $limit"
        
        local cmd="$ingest_binary --retry --limit=$limit --config=$config_path"
        if [ -n "$embedding" ]; then
            cmd="$cmd --embedding=$embedding"
        fi
        
        info "执行命令: $cmd"
        exec $cmd
        exit 0
    fi

    # 检查数据源
    if [ -z "$source_type" ]; then
        error "请指定数据源类型 (-s/--source)"
        usage
        exit 1
    fi

    # 处理 staging 源
    if [ "$source_type" = "staging" ]; then
        # 交互式选择
        local selected_source=$(interactive_select_staging "$DEFAULT_STAGING_PATH")
        source_type="staging:$selected_source"
    elif echo "$source_type" | grep -q "^staging:"; then
        # 已经指定了 staging:SOURCE_ID
        local source_id=$(echo "$source_type" | cut -d: -f2)
        if [ -z "$source_id" ]; then
            error "staging 源格式错误，应为 staging:SOURCE_ID"
            exit 1
        fi
    elif [ "$source_type" != "chinesebqb" ]; then
        error "不支持的数据源类型: $source_type"
        error "支持的类型: chinesebqb, staging, staging:SOURCE_ID"
        exit 1
    fi

    # 显示导入信息
    echo ""
    info "=========================================="
    info "开始数据导入"
    info "=========================================="
    info "数据源: $source_type"
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
    info "=========================================="
    echo ""

    # 构建命令
    local cmd="$ingest_binary --source=$source_type --limit=$limit --config=$config_path"
    
    if [ -n "$embedding" ]; then
        cmd="$cmd --embedding=$embedding"
    fi
    
    if [ "$force" = true ]; then
        cmd="$cmd --force"
    fi

    info "执行命令: $cmd"
    echo ""

    # 执行导入
    exec $cmd
}

# 运行主函数
main "$@"

