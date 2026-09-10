#!/bin/bash

# sb-mmw — 远程 sing-box 服务器端管理菜单
# 安装在 sing-box 服务器上，通过妙妙屋面板 API 回连管理
# 用法: sb-mmw [命令] 或直接运行进入交互菜单

set -euo pipefail

# ========== 配置 ==========
CONFIG_FILE="/etc/sing-box/.sb-mmw.conf"
AUTH_HEADER="MM-Authorization"

# 颜色
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
NC='\033[0m'

info()    { echo -e "${GREEN}[INFO]${NC} $1"; }
warn()    { echo -e "${YELLOW}[WARN]${NC} $1"; }
error()   { echo -e "${RED}[ERROR]${NC} $1"; }
success() { echo -e "${GREEN}[OK]${NC} $1"; }

# 加载配置
load_config() {
    if [ -f "$CONFIG_FILE" ]; then
        source "$CONFIG_FILE"
    fi
}

# 检查依赖
check_deps() {
    for cmd in curl jq; do
        if ! command -v "$cmd" &>/dev/null; then
            error "缺少依赖: $cmd"
            echo "安装: apt-get install -y $cmd"
            exit 1
        fi
    done
}

# 登录获取 token
get_token() {
    if [ -n "${MMW_API_TOKEN:-}" ]; then
        echo "$MMW_API_TOKEN"
        return
    fi
    if [ -z "${MMW_API_URL:-}" ] || [ -z "${MMW_API_USER:-}" ] || [ -z "${MMW_API_PASS:-}" ]; then
        error "未配置面板连接，请先运行: sb-mmw config"
        exit 1
    fi
    local resp token
    resp=$(curl -s -X POST "${MMW_API_URL}/api/login" \
        -H "Content-Type: application/json" \
        -d "{\"username\":\"${MMW_API_USER}\",\"password\":\"${MMW_API_PASS}\"}" 2>/dev/null) || true
    token=$(echo "$resp" | jq -r '.token // empty' 2>/dev/null) || true
    if [ -z "$token" ]; then
        error "登录失败: $resp"
        exit 1
    fi
    MMW_API_TOKEN="$token"
    echo "$token"
}

# API 调用
api_call() {
    local method="$1" path="$2" data="${3:-}"
    local token
    token=$(get_token)

    local resp code body
    if [ -n "$data" ]; then
        resp=$(curl -s -w "\n%{http_code}" -X "$method" "${MMW_API_URL}${path}" \
            -H "Content-Type: application/json" \
            -H "${AUTH_HEADER}: ${token}" \
            -d "$data" 2>/dev/null)
    else
        resp=$(curl -s -w "\n%{http_code}" -X "$method" "${MMW_API_URL}${path}" \
            -H "Content-Type: application/json" \
            -H "${AUTH_HEADER}: ${token}" 2>/dev/null)
    fi
    code=$(echo "$resp" | tail -1)
    body=$(echo "$resp" | sed '$d')

    if [ "$code" = "401" ]; then
        MMW_API_TOKEN=""
        token=$(get_token)
        if [ -n "$data" ]; then
            resp=$(curl -s -w "\n%{http_code}" -X "$method" "${MMW_API_URL}${path}" \
                -H "Content-Type: application/json" \
                -H "${AUTH_HEADER}: ${token}" \
                -d "$data" 2>/dev/null)
        else
            resp=$(curl -s -w "\n%{http_code}" -X "$method" "${MMW_API_URL}${path}" \
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
        return 1
    fi
}

# 找到本机在面板中对应的 sing-box 服务器 ID
find_server_id() {
    local my_ip
    my_ip=$(hostname -I 2>/dev/null | awk '{print $1}')
    if [ -z "$my_ip" ]; then
        my_ip=$(curl -s ifconfig.me 2>/dev/null || echo "")
    fi

    local resp
    resp=$(api_call GET "/api/admin/singbox-servers") || return 1

    # 尝试按 IP 匹配
    local id
    id=$(echo "$resp" | jq -r --arg ip "$my_ip" '.[] | select(.host == $ip) | .id' 2>/dev/null | head -1)
    if [ -n "$id" ] && [ "$id" != "null" ]; then
        echo "$id"
        return
    fi

    # 没找到，交互选择
    warn "未自动匹配到本机 IP ($my_ip)，请手动选择:"
    echo "$resp" | jq -r '.[] | "\(.id) \(.name) \(.host):\(.port)"' 2>/dev/null
    read -p "输入服务器 ID: " id
    if [ -z "$id" ]; then
        error "未选择服务器"
        return 1
    fi
    echo "$id"
}

# ========== 命令实现 ==========

cmd_config() {
    echo -e "${CYAN}═══ 配置妙妙屋面板连接 ═══${NC}"
    mkdir -p "$(dirname "$CONFIG_FILE")"

    read -p "面板地址 (如 http://1.2.3.4:8080): " url
    [ -z "$url" ] && { error "地址不能为空"; exit 1; }
    url="${url%/}"

    read -p "管理员用户名: " user
    [ -z "$user" ] && { error "用户名不能为空"; exit 1; }

    read -s -p "管理员密码: " pass
    echo ""

    cat > "$CONFIG_FILE" <<EOF
MMW_API_URL="$url"
MMW_API_USER="$user"
MMW_API_PASS="$pass"
EOF
    chmod 600 "$CONFIG_FILE"
    success "配置已保存到 $CONFIG_FILE"
}

cmd_status() {
    echo -e "${CYAN}═══ 本机 sing-box 状态 ═══${NC}"
    echo ""

    # 本机状态
    if systemctl is-active --quiet sing-box 2>/dev/null; then
        success "systemd 服务: 运行中"
    elif pgrep -x sing-box >/dev/null 2>&1; then
        warn "进程运行中（非 systemd）"
    else
        error "sing-box: 未运行"
    fi

    # 版本
    if command -v sing-box &>/dev/null; then
        echo "版本: $(sing-box version 2>&1 | head -1)"
    fi

    # 配置文件
    local cfg="/etc/sing-box/config.json"
    if [ -f "$cfg" ]; then
        info "配置文件: $cfg ($(du -sh "$cfg" | awk '{print $1}'))"
    else
        warn "配置文件不存在: $cfg"
    fi

    # 监听端口
    echo ""
    echo "监听端口:"
    ss -tlnp 2>/dev/null | grep -i sing-box || ss -tlnp 2>/dev/null | grep -E ':(7890|9090)\b' || echo "  (未检测到)"

    # 面板端状态
    if [ -n "${MMW_API_URL:-}" ]; then
        echo ""
        echo -e "${CYAN}═══ 面板端状态 ═══${NC}"
        local id
        id=$(find_server_id 2>/dev/null) || true
        if [ -n "$id" ]; then
            local resp
            resp=$(api_call POST "/api/admin/singbox-servers/${id}/status") || true
            if [ -n "$resp" ]; then
                echo "$resp" | jq -r '
                    "SSH 连接: \(.ssh_ok // false | if . then "✓" else "✗" end)",
                    "运行状态: \(.running // false | if . then "运行中" else "已停止" end)",
                    "版本: \(.version // "未知")"
                ' 2>/dev/null || true
            fi
        fi
    fi
    echo ""
}

cmd_deploy() {
    local id
    id=$(find_server_id) || { error "无法确定服务器 ID"; exit 1; }
    info "从面板推送配置到本机 (服务器 #$id)..."
    api_call POST "/api/admin/singbox-servers/${id}/deploy" >/dev/null
    success "配置已部署并重载"
}

cmd_restart() {
    info "重启本机 sing-box..."
    if systemctl restart sing-box 2>/dev/null; then
        sleep 1
        systemctl is-active --quiet sing-box && success "sing-box 已重启" || { error "重启失败"; exit 1; }
    elif command -v sing-box &>/dev/null; then
        pkill -x sing-box 2>/dev/null || true
        sleep 1
        nohup sing-box run -c /etc/sing-box/config.json >/var/log/sing-box.log 2>&1 &
        sleep 2
        pgrep -x sing-box >/dev/null && success "sing-box 已重启" || { error "重启失败"; exit 1; }
    else
        error "sing-box 未安装"
        exit 1
    fi
}

cmd_sync() {
    local id
    id=$(find_server_id) || { error "无法确定服务器 ID"; exit 1; }
    info "同步本机节点到面板 nodes 表 (服务器 #$id)..."
    local resp
    resp=$(api_call POST "/api/admin/singbox-servers/${id}/sync-node")
    local node_id node_name
    node_id=$(echo "$resp" | jq -r '.node_id // "-"')
    node_name=$(echo "$resp" | jq -r '.node_name // "-"')
    success "节点已同步: ID=$node_id 名称=$node_name"
}

cmd_install_singbox() {
    info "安装 sing-box..."
    if command -v sing-box &>/dev/null; then
        warn "sing-box 已安装: $(sing-box version 2>&1 | head -1)"
        read -p "是否重新安装？(y/N): " confirm
        [ "$confirm" != "y" ] && [ "$confirm" != "Y" ] && return
    fi
    bash <(curl -fsSL https://sing-box.app/install.sh) || {
        # 备用方案: 手动下载
        local arch
        arch=$(uname -m)
        case "$arch" in
            x86_64|amd64) arch="amd64" ;;
            aarch64|arm64) arch="arm64" ;;
            *) error "不支持的架构: $arch"; exit 1 ;;
        esac
        local version
        version=$(curl -s https://api.github.com/repos/SagerNet/sing-box/releases/latest | jq -r '.tag_name' | sed 's/v//')
        info "下载 sing-box v$version ($arch)..."
        curl -sL "https://github.com/SagerNet/sing-box/releases/download/v${version}/sing-box-${version}-linux-${arch}.tar.gz" | tar xz -C /tmp
        cp "/tmp/sing-box-${version}-linux-${arch}/sing-box" /usr/local/bin/
        chmod +x /usr/local/bin/sing-box
        mkdir -p /etc/sing-box
    }
    success "sing-box 安装完成: $(sing-box version 2>&1 | head -1)"
}

cmd_uninstall_singbox() {
    read -p "确认卸载本机 sing-box？(y/N): " confirm
    [ "$confirm" != "y" ] && [ "$confirm" != "Y" ] && { warn "已取消"; return; }

    info "卸载 sing-box..."
    systemctl stop sing-box 2>/dev/null || true
    systemctl disable sing-box 2>/dev/null || true
    rm -f /etc/systemd/system/sing-box.service
    systemctl daemon-reload 2>/dev/null || true

    rm -f /usr/local/bin/sing-box
    rm -rf /etc/sing-box
    rm -f /tmp/sing-box-*

    pkill -x sing-box 2>/dev/null || true

    success "sing-box 已卸载"

    read -p "是否同时删除 sb-mmw 配置？(y/N): " del_cfg
    [ "$del_cfg" = "y" ] && rm -f "$CONFIG_FILE" && info "配置已删除"
}

# ========== 交互菜单 ==========

show_menu() {
    echo ""
    echo -e "${CYAN}═══════════════════════════════════════${NC}"
    echo -e "${CYAN}       sb-mmw sing-box 管理菜单       ${NC}"
    echo -e "${CYAN}═══════════════════════════════════════${NC}"
    echo ""
    echo "  1) 查看状态          (status)"
    echo "  2) 部署配置（从面板） (deploy)"
    echo "  3) 重启 sing-box      (restart)"
    echo "  4) 同步节点到面板    (sync)"
    echo "  5) 安装 sing-box      (install)"
    echo "  6) 卸载 sing-box      (uninstall)"
    echo "  7) 配置面板连接      (config)"
    echo "  0) 退出"
    echo ""
    read -p "请选择 [0-7]: " choice

    case "$choice" in
        1) cmd_status ;;
        2) cmd_deploy ;;
        3) cmd_restart ;;
        4) cmd_sync ;;
        5) cmd_install_singbox ;;
        6) cmd_uninstall_singbox ;;
        7) cmd_config ;;
        0) exit 0 ;;
        *) error "无效选择" ;;
    esac
}

# ========== 主入口 ==========

main() {
    load_config
    check_deps

    if [ $# -gt 0 ]; then
        case "$1" in
            config)    cmd_config ;;
            status)    cmd_status ;;
            deploy)    cmd_deploy ;;
            restart)   cmd_restart ;;
            sync)      cmd_sync ;;
            install)   cmd_install_singbox ;;
            uninstall) cmd_uninstall_singbox ;;
            help|-h|--help)
                echo "用法: sb-mmw [命令]"
                echo ""
                echo "命令:"
                echo "  config     配置面板连接"
                echo "  status     查看状态"
                echo "  deploy     从面板部署配置"
                echo "  restart    重启 sing-box"
                echo "  sync       同步节点到面板"
                echo "  install    安装 sing-box"
                echo "  uninstall  卸载 sing-box"
                echo "  (无参数)   进入交互菜单"
                ;;
            *)
                error "未知命令: $1"
                echo "运行 sb-mmw help 查看帮助"
                exit 1
                ;;
        esac
    else
        # 交互菜单循环
        while true; do
            show_menu
            echo ""
            read -p "按回车继续，或输入 q 退出: " cont
            [ "$cont" = "q" ] && exit 0
        done
    fi
}

main "$@"
