#!/bin/bash

# mmw-cli — 妙妙屋快捷管理工具
# 功能：面板管理（status/restart/stop/start/update/backup/log/uninstall）
#       远程 sing-box 管理（sb list/test/status/deploy/restart/sync/add/del）
# 适用于 Debian/Ubuntu Linux 系统（需 curl + jq）

set -euo pipefail

# ========== 配置 ==========
SERVICE_NAME="mmw"
INSTALL_DIR="/usr/local/bin"
DATA_DIR="/etc/mmw"
GITHUB_REPO="iluobei/miaomiaowu"
CONFIG_FILE="${DATA_DIR}/.mmw-cli.conf"

# API 配置（从配置文件或环境变量读取）
MMW_API_URL="${MMW_API_URL:-}"
MMW_API_USER="${MMW_API_USER:-}"
MMW_API_PASS="${MMW_API_PASS:-}"
MMW_API_TOKEN="${MMW_API_TOKEN:-}"
AUTH_HEADER="MM-Authorization"

# ========== 颜色 ==========
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
NC='\033[0m'

info()    { echo -e "${GREEN}[INFO]${NC} $1"; }
warn()    { echo -e "${YELLOW}[WARN]${NC} $1"; }
error()   { echo -e "${RED}[ERROR]${NC} $1"; }
success() { echo -e "${GREEN}[OK]${NC} $1"; }

# ========== 工具函数 ==========

check_cmd() {
    for cmd in "$@"; do
        if ! command -v "$cmd" &>/dev/null; then
            error "缺少依赖: $cmd，请先安装"
            exit 1
        fi
    done
}

# 读取配置文件
load_config() {
    if [ -f "$CONFIG_FILE" ]; then
        # shellcheck source=/dev/null
        source "$CONFIG_FILE"
    fi
}

# 保存配置
save_config() {
    cat > "$CONFIG_FILE" <<EOF
MMW_API_URL="$MMW_API_URL"
MMW_API_USER="$MMW_API_USER"
MMW_API_TOKEN="$MMW_API_TOKEN"
EOF
    chmod 600 "$CONFIG_FILE"
}

# 获取面板端口
get_panel_port() {
    local port=8080
    if [ -f "/etc/systemd/system/${SERVICE_NAME}.service" ]; then
        port=$(grep "Environment=\"PORT=" /etc/systemd/system/${SERVICE_NAME}.service 2>/dev/null | sed 's/.*PORT=\([0-9]*\).*/\1/' || true)
        port=${port:-8080}
    fi
    echo "$port"
}

# 获取面板地址
get_api_url() {
    if [ -n "$MMW_API_URL" ]; then
        echo "$MMW_API_URL"
        return
    fi
    local port
    port=$(get_panel_port)
    echo "http://127.0.0.1:${port}"
}

# 登录获取 token
do_login() {
    local api_url user pass
    api_url=$(get_api_url)
    user="$MMW_API_USER"
    pass="$MMW_API_PASS"

    if [ -n "$MMW_API_TOKEN" ]; then
        echo "$MMW_API_TOKEN"
        return
    fi

    if [ -z "$user" ] || [ -z "$pass" ]; then
        error "未配置 API 账号。请运行: mmw sb config"
        exit 1
    fi

    local resp
    resp=$(curl -s -X POST "${api_url}/api/login" \
        -H "Content-Type: application/json" \
        -d "{\"username\":\"${user}\",\"password\":\"${pass}\"}" 2>/dev/null) || true

    local token
    token=$(echo "$resp" | jq -r '.token // empty' 2>/dev/null) || true

    if [ -z "$token" ]; then
        error "登录失败，请检查账号密码: $resp"
        exit 1
    fi

    # 缓存 token
    MMW_API_TOKEN="$token"
    save_config
    echo "$token"
}

# 带 auth 调用 API
api_call() {
    local method="$1" path="$2" data="${3:-}"
    local api_url token
    api_url=$(get_api_url)
    token=$(do_login)

    local resp code
    if [ -n "$data" ]; then
        resp=$(curl -s -w "\n%{http_code}" -X "$method" "${api_url}${path}" \
            -H "Content-Type: application/json" \
            -H "${AUTH_HEADER}: ${token}" \
            -d "$data" 2>/dev/null)
    else
        resp=$(curl -s -w "\n%{http_code}" -X "$method" "${api_url}${path}" \
            -H "Content-Type: application/json" \
            -H "${AUTH_HEADER}: ${token}" 2>/dev/null)
    fi

    code=$(echo "$resp" | tail -1)
    body=$(echo "$resp" | sed '$d')

    if [ "$code" = "401" ]; then
        warn "Token 已过期，重新登录..."
        MMW_API_TOKEN=""
        save_config
        token=$(do_login)
        if [ -n "$data" ]; then
            resp=$(curl -s -w "\n%{http_code}" -X "$method" "${api_url}${path}" \
                -H "Content-Type: application/json" \
                -H "${AUTH_HEADER}: ${token}" \
                -d "$data" 2>/dev/null)
        else
            resp=$(curl -s -w "\n%{http_code}" -X "$method" "${api_url}${path}" \
                -H "Content-Type: application/json" \
                -H "${AUTH_HEADER}: ${token}" 2>/dev/null)
        fi
        code=$(echo "$resp" | tail -1)
        body=$(echo "$resp" | sed '$d')
    fi

    if [ "$code" = "200" ] || [ "$code" = "201" ]; then
        echo "$body"
    else
        error "API 调用失败 ($code): $body"
        exit 1
    fi
}

# ========== 面板管理命令 ==========

cmd_status() {
    echo -e "${CYAN}═══ 妙妙屋面板状态 ═══${NC}"
    echo ""

    # systemd 状态
    if systemctl is-active --quiet "$SERVICE_NAME" 2>/dev/null; then
        success "服务状态: 运行中"
    elif systemctl is-active --quiet "${SERVICE_NAME}.service" 2>/dev/null; then
        success "服务状态: 运行中"
    elif pgrep -x "$SERVICE_NAME" >/dev/null 2>&1; then
        warn "服务状态: 运行中（非 systemd）"
    else
        error "服务状态: 未运行"
    fi

    # 版本
    if [ -f "$DATA_DIR/.version" ]; then
        echo "当前版本: $(cat "$DATA_DIR/.version")"
    fi

    # 端口
    local port
    port=$(get_panel_port)
    echo "监听端口: $port"

    # 进程信息
    if pgrep -x "$SERVICE_NAME" >/dev/null 2>&1; then
        echo "进程 PID: $(pgrep -x "$SERVICE_NAME" | head -1)"
        echo "内存占用: $(ps -o rss= -p "$(pgrep -x "$SERVICE_NAME" | head -1)" 2>/dev/null | awk '{printf "%.1f MB\n", $1/1024}' || echo "N/A")"
    fi

    # 数据目录
    if [ -d "$DATA_DIR" ]; then
        local db_size
        db_size=$(du -sh "$DATA_DIR" 2>/dev/null | awk '{print $1}')
        echo "数据目录: $DATA_DIR ($db_size)"
    fi

    # API 健康检查
    local api_url
    api_url=$(get_api_url)
    if curl -s -o /dev/null -w "%{http_code}" "$api_url/" 2>/dev/null | grep -q "200\|301\|302\|304"; then
        success "API 可达: $api_url"
    else
        warn "API 不可达: $api_url"
    fi
    echo ""
}

cmd_start() {
    info "启动妙妙屋服务..."
    if [ -f "/etc/systemd/system/${SERVICE_NAME}.service" ]; then
        systemctl start "$SERVICE_NAME" || { error "启动失败"; exit 1; }
        sleep 1
        systemctl is-active --quiet "$SERVICE_NAME" && success "服务已启动" || { error "启动失败"; exit 1; }
    else
        error "未检测到 systemd 服务，请先安装: bash install.sh"
        exit 1
    fi
}

cmd_stop() {
    info "停止妙妙屋服务..."
    if [ -f "/etc/systemd/system/${SERVICE_NAME}.service" ]; then
        systemctl stop "$SERVICE_NAME" 2>/dev/null || true
        success "服务已停止"
    elif pgrep -x "$SERVICE_NAME" >/dev/null 2>&1; then
        pkill -x "$SERVICE_NAME" 2>/dev/null || true
        success "进程已终止"
    else
        warn "服务未运行"
    fi
}

cmd_restart() {
    info "重启妙妙屋服务..."
    if [ -f "/etc/systemd/system/${SERVICE_NAME}.service" ]; then
        systemctl restart "$SERVICE_NAME" || { error "重启失败"; exit 1; }
        sleep 2
        systemctl is-active --quiet "$SERVICE_NAME" && success "服务已重启" || { error "重启失败"; exit 1; }
    elif pgrep -x "$SERVICE_NAME" >/dev/null 2>&1; then
        pkill -x "$SERVICE_NAME" 2>/dev/null || true
        sleep 1
        cd "$DATA_DIR" && nohup "$INSTALL_DIR/$SERVICE_NAME" >/dev/null 2>&1 &
        sleep 2
        pgrep -x "$SERVICE_NAME" >/dev/null 2>&1 && success "服务已重启" || { error "重启失败"; exit 1; }
    else
        warn "服务未运行，尝试启动..."
        cmd_start
    fi
}

cmd_update() {
    info "检查并更新妙妙屋..."
    if [ -f "./install.sh" ]; then
        bash ./install.sh update
    elif [ -f "$INSTALL_DIR/$SERVICE_NAME" ]; then
        curl -sL "https://raw.githubusercontent.com/${GITHUB_REPO}/main/install.sh" | bash -s update
    else
        error "未检测到已安装的服务，请先安装: bash install.sh"
        exit 1
    fi
}

cmd_backup() {
    local backup_dir="${1:-${DATA_DIR}/backups}"
    local ts
    ts=$(date +%Y%m%d_%H%M%S)
    local target="${backup_dir}/mmw_backup_${ts}"

    info "备份妙妙屋数据到: $target"
    mkdir -p "$target"

    # 备份数据库
    if [ -f "$DATA_DIR/traffic.db" ]; then
        cp "$DATA_DIR/traffic.db" "$target/"
        info "已备份数据库: traffic.db"
    fi

    # 备份订阅文件
    if [ -d "$DATA_DIR/subscribes" ]; then
        cp -r "$DATA_DIR/subscribes" "$target/"
        info "已备份订阅文件"
    fi

    # 备份规则模板
    if [ -d "$DATA_DIR/rule_templates" ]; then
        cp -r "$DATA_DIR/rule_templates" "$target/"
        info "已备份规则模板"
    fi

    # 备份版本信息
    if [ -f "$DATA_DIR/.version" ]; then
        cp "$DATA_DIR/.version" "$target/"
    fi

    # 打包
    tar czf "${target}.tar.gz" -C "$backup_dir" "mmw_backup_${ts}"
    rm -rf "$target"

    success "备份完成: ${target}.tar.gz ($(du -sh "${target}.tar.gz" | awk '{print $1}'))"
}

cmd_log() {
    local lines="${1:-50}"
    if [ -f "/etc/systemd/system/${SERVICE_NAME}.service" ]; then
        journalctl -u "$SERVICE_NAME" -n "$lines" --no-pager
    elif [ -f "$DATA_DIR/mmw.log" ]; then
        tail -n "$lines" "$DATA_DIR/mmw.log"
    else
        error "未找到日志文件"
        exit 1
    fi
}

cmd_uninstall() {
    if [ -f "./install.sh" ]; then
        bash ./install.sh uninstall
    else
        curl -sL "https://raw.githubusercontent.com/${GITHUB_REPO}/main/install.sh" | bash -s uninstall
    fi
}

# ========== 远程 sing-box 管理命令 ==========

sb_config() {
    echo -e "${CYAN}═══ 配置妙妙屋 API 连接 ═══${NC}"
    local api_url user pass

    # API 地址
    local default_url
    default_url=$(get_api_url)
    read -p "面板地址（默认 $default_url）: " api_url
    api_url="${api_url:-$default_url}"
    # 去掉末尾斜杠
    api_url="${api_url%/}"

    read -p "管理员用户名: " user
    if [ -z "$user" ]; then
        error "用户名不能为空"
        exit 1
    fi

    read -s -p "管理员密码: " pass
    echo ""
    if [ -z "$pass" ]; then
        error "密码不能为空"
        exit 1
    fi

    MMW_API_URL="$api_url"
    MMW_API_USER="$user"
    MMW_API_PASS="$pass"
    MMW_API_TOKEN=""

    # 测试登录
    local token
    token=$(do_login) || { error "登录失败，请检查配置"; exit 1; }
    success "配置已保存，登录成功"
}

sb_list() {
    echo -e "${CYAN}═══ sing-box 服务器列表 ═══${NC}"
    echo ""
    local resp
    resp=$(api_call GET "/api/admin/singbox-servers")

    local count
    count=$(echo "$resp" | jq 'length')
    if [ "$count" = "0" ] || [ -z "$count" ]; then
        warn "暂无 sing-box 服务器"
        return
    fi

    printf "%-4s %-16s %-22s %-8s %-8s %-10s\n" "ID" "名称" "主机:端口" "入站" "状态" "同步"
    printf "%-4s %-16s %-22s %-8s %-8s %-10s\n" "--" "----" "--------" "----" "----" "----"
    echo "$resp" | jq -r '.[] | "\(.id) \(.name) \(.host):\(.port) \(.singbox_port) \(.has_status // "-") \(.enabled)"' | while read -r id name hostport inbound status enabled; do
        printf "%-4s %-16s %-22s %-8s %-8s %-10s\n" "$id" "$name" "$hostport" "$inbound" "${status:- -}" "$enabled"
    done
    echo ""
    info "共 $count 台服务器"
}

sb_test() {
    local id="${1:-}"
    if [ -z "$id" ]; then
        error "用法: mmw sb test <ID>"
        exit 1
    fi
    info "测试服务器 #$id SSH 连接..."
    api_call POST "/api/admin/singbox-servers/${id}/test" >/dev/null
    success "SSH 连接正常"
}

sb_status() {
    local id="${1:-}"
    if [ -z "$id" ]; then
        error "用法: mmw sb status <ID>"
        exit 1
    fi
    echo -e "${CYAN}═══ 服务器 #$id 状态 ═══${NC}"
    echo ""
    local resp
    resp=$(api_call POST "/api/admin/singbox-servers/${id}/status")
    echo "$resp" | jq -r '
        "SSH 连接: \(.ssh_ok // false | if . then "✓ 正常" else "✗ 失败" end)",
        "运行状态: \(.running // false | if . then "运行中" else "已停止" end)",
        "版本: \(.version // "未知")",
        "配置文件: \(.config_exists // false | if . then "存在" else "不存在" end)",
        "消息: \(.message // "-")"
    '
}

sb_deploy() {
    local id="${1:-}"
    if [ -z "$id" ]; then
        error "用法: mmw sb deploy <ID>"
        exit 1
    fi
    info "部署配置到服务器 #$id..."
    api_call POST "/api/admin/singbox-servers/${id}/deploy" >/dev/null
    success "配置已部署并重载"
}

sb_restart() {
    local id="${1:-}"
    if [ -z "$id" ]; then
        error "用法: mmw sb restart <ID>"
        exit 1
    fi
    info "重启服务器 #$id 的 sing-box..."
    api_call POST "/api/admin/singbox-servers/${id}/restart" >/dev/null
    success "sing-box 已重启"
}

sb_sync() {
    local id="${1:-}"
    if [ -z "$id" ]; then
        error "用法: mmw sb sync <ID>"
        exit 1
    fi
    info "同步服务器 #$id 节点到 nodes 表..."
    local resp
    resp=$(api_call POST "/api/admin/singbox-servers/${id}/sync-node")
    local node_id node_name
    node_id=$(echo "$resp" | jq -r '.node_id // "-"')
    node_name=$(echo "$resp" | jq -r '.node_name // "-"')
    success "节点已同步: ID=$node_id 名称=$node_name"
}

sb_add() {
    echo -e "${CYAN}═══ 添加 sing-box 服务器 ═══${NC}"
    local name host port ssh_user auth_type auth_data singbox_port api_port config_path

    read -p "名称: " name
    [ -z "$name" ] && { error "名称不能为空"; exit 1; }

    read -p "主机 (IP/域名): " host
    [ -z "$host" ] && { error "主机不能为空"; exit 1; }

    read -p "SSH 端口 (默认 22): " port
    port="${port:-22}"

    read -p "SSH 用户 (默认 root): " ssh_user
    ssh_user="${ssh_user:-root}"

    echo "认证方式:"
    echo "  1) 密码"
    echo "  2) 私钥"
    read -p "选择 (1/2，默认 1): " auth_choice
    if [ "$auth_choice" = "2" ]; then
        auth_type="private_key"
        echo "请粘贴私钥内容（输入完成后按 Ctrl+D 结束）:"
        auth_data=$(cat)
    else
        auth_type="password"
        read -s -p "密码: " auth_data
        echo ""
    fi

    [ -z "$auth_data" ] && { error "认证数据不能为空"; exit 1; }

    read -p "入站端口 (默认 7890): " singbox_port
    singbox_port="${singbox_port:-7890}"

    read -p "API 端口 (默认 9090): " api_port
    api_port="${api_port:-9090}"

    read -p "配置路径 (默认 /etc/sing-box/config.json): " config_path
    config_path="${config_path:-/etc/sing-box/config.json}"

    # JSON 构造（用 jq 安全转义）
    local data
    data=$(jq -n \
        --arg name "$name" \
        --arg host "$host" \
        --argjson port "$port" \
        --arg ssh_user "$ssh_user" \
        --arg auth_type "$auth_type" \
        --arg auth_data "$auth_data" \
        --argjson singbox_port "$singbox_port" \
        --argjson api_port "$api_port" \
        --arg config_path "$config_path" \
        --argjson enabled true \
        '{name:$name, host:$host, port:$port, ssh_user:$ssh_user, auth_type:$auth_type, auth_data:$auth_data, singbox_port:$singbox_port, api_port:$api_port, config_path:$config_path, enabled:$enabled}')

    info "创建服务器..."
    local resp
    resp=$(api_call POST "/api/admin/singbox-servers" "$data")
    local new_id
    new_id=$(echo "$resp" | jq -r '.id')
    success "服务器已创建: ID=$new_id"
}

sb_del() {
    local id="${1:-}"
    if [ -z "$id" ]; then
        error "用法: mmw sb del <ID>"
        exit 1
    fi
    read -p "确认删除服务器 #$id？(y/N): " confirm
    if [ "$confirm" != "y" ] && [ "$confirm" != "Y" ]; then
        warn "已取消"
        return
    fi
    info "删除服务器 #$id..."
    api_call DELETE "/api/admin/singbox-servers/${id}" >/dev/null
    success "服务器已删除"
}

sb_help() {
    echo -e "${CYAN}═══ sing-box 服务器管理 ═══${NC}"
    echo ""
    echo "用法: mmw sb <命令> [参数]"
    echo ""
    echo "命令:"
    echo "  config        配置 API 连接（地址/用户名/密码）"
    echo "  list          列出所有服务器"
    echo "  test <ID>     测试 SSH 连接"
    echo "  status <ID>   查询远端 sing-box 运行状态"
    echo "  deploy <ID>   生成并推送配置 + 重载"
    echo "  restart <ID>  重启远端 sing-box"
    echo "  sync <ID>     同步节点到 nodes 表（供订阅输出）"
    echo "  add           交互式添加服务器"
    echo "  del <ID>      删除服务器"
    echo "  help          显示此帮助"
    echo ""
}

# ========== 帮助 ==========

show_help() {
    echo -e "${CYAN}═══ mmw-cli 妙妙屋快捷管理工具 ═══${NC}"
    echo ""
    echo "用法: mmw <命令> [参数]"
    echo ""
    echo "面板管理:"
    echo "  status        查看面板运行状态"
    echo "  start         启动面板"
    echo "  stop          停止面板"
    echo "  restart       重启面板"
    echo "  update        更新到最新版本"
    echo "  backup [目录]  备份数据库+订阅文件（默认备份到 ${DATA_DIR}/backups）"
    echo "  log [行数]    查看最近日志（默认 50 行）"
    echo "  uninstall     卸载面板"
    echo ""
    echo "sing-box 服务器管理:"
    echo "  sb config     配置 API 连接"
    echo "  sb list       列出服务器"
    echo "  sb test <ID>  测试 SSH 连接"
    echo "  sb status <ID> 查询运行状态"
    echo "  sb deploy <ID> 部署配置"
    echo "  sb restart <ID> 重启 sing-box"
    echo "  sb sync <ID>  同步节点"
    echo "  sb add        添加服务器"
    echo "  sb del <ID>   删除服务器"
    echo "  sb help       sing-box 管理帮助"
    echo ""
    echo "环境变量:"
    echo "  MMW_API_URL   面板地址（默认 http://127.0.0.1:8080）"
    echo "  MMW_API_USER  管理员用户名"
    echo "  MMW_API_PASS  管理员密码"
    echo ""
}

# ========== 主入口 ==========

main() {
    load_config

    local cmd="${1:-help}"
    shift 2>/dev/null || true

    case "$cmd" in
        # 面板管理
        status)    cmd_status "$@" ;;
        start)     cmd_start "$@" ;;
        stop)      cmd_stop "$@" ;;
        restart)   cmd_restart "$@" ;;
        update)    cmd_update "$@" ;;
        backup)    cmd_backup "$@" ;;
        log)       cmd_log "$@" ;;
        uninstall) cmd_uninstall "$@" ;;
        # sing-box 管理
        sb)
            local subcmd="${1:-help}"
            shift 2>/dev/null || true
            case "$subcmd" in
                config)  sb_config "$@" ;;
                list|ls) sb_list "$@" ;;
                test)    sb_test "$@" ;;
                status)  sb_status "$@" ;;
                deploy)  sb_deploy "$@" ;;
                restart) sb_restart "$@" ;;
                sync)    sb_sync "$@" ;;
                add)     sb_add "$@" ;;
                del|delete|rm) sb_del "$@" ;;
                help|-h|--help) sb_help ;;
                *) error "未知命令: mmw sb $subcmd"; sb_help; exit 1 ;;
            esac
            ;;
        help|-h|--help) show_help ;;
        *)
            error "未知命令: $cmd"
            echo ""
            show_help
            exit 1
            ;;
    esac
}

main "$@"
