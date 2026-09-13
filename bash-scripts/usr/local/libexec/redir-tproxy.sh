#!/usr/bin/env bash
set -euo pipefail

ACTION="${1:-}"
# PROXYCORE can be set via: CLI arg $2, env PROXYCORE, or systemctl wrapper
INSTANCE="${2:-${PROXYCORE:-}}"

# ==================== 基础检查 ====================
for cmd in ip nft; do
    command -v "$cmd" >/dev/null 2>&1 || {
        echo "FATAL: command not found: $cmd" >&2
        exit 1
    }
done

[ -n "$INSTANCE" ] || {
    echo "FATAL: PROXYCORE not set (pass as arg2 or set env PROXYCORE)" >&2
    echo "Usage: $0 {start|stop|restart} [instance]" >&2
    exit 1
}

# 配置文件路径：匹配 redir-tproxy@<INSTANCE> 读取 <INSTANCE>_redir-tproxy.conf
CONFIG_FILE="/etc/sing-box/${INSTANCE}_redir-tproxy.conf"

[ -f "$CONFIG_FILE" ] || {
    echo "FATAL: config file not found: $CONFIG_FILE" >&2
    echo "HINT: Please create it, e.g., /etc/sing-box/${INSTANCE}_redir-tproxy.conf" >&2
    exit 1
}

# shellcheck source=/dev/null
source "$CONFIG_FILE"

# ==================== 必须变量检查 ====================
require_vars=(
    REDIRECT_PORT
    TPROXY_PORT
    FWMARK
    TABLE_ID
    NFTABLES_TABLE
)

for v in "${require_vars[@]}"; do
    if [ -z "${!v:-}" ]; then
        echo "FATAL: variable $v is not set in $CONFIG_FILE" >&2
        exit 1
    fi
done

# 检查 EXCLUDE_GID 或 ROUTING_MARK 至少存在一个
if [ -z "${EXCLUDE_GID:-}" ] && [ -z "${ROUTING_MARK:-}" ]; then
    echo "FATAL: Either EXCLUDE_GID or ROUTING_MARK must be set in $CONFIG_FILE" >&2
    exit 1
fi

# ==================== 规则生成逻辑 ====================
# 当两者都存在时，默认优先使用 EXCLUDE_GID
if [ -n "${EXCLUDE_GID:-}" ]; then
    BYPASS_RULE="meta skgid $EXCLUDE_GID return"
else
    BYPASS_RULE="meta mark $ROUTING_MARK return"
fi

# ==================== 路由 ====================
setup_routes() {
    ip rule del fwmark "$FWMARK" table "$TABLE_ID" 2>/dev/null || true
    ip rule add fwmark "$FWMARK" table "$TABLE_ID"

    ip route flush table "$TABLE_ID" 2>/dev/null || true
    ip route add local 0.0.0.0/0 dev lo table "$TABLE_ID"
}

remove_routes() {
    ip rule del fwmark "$FWMARK" table "$TABLE_ID" 2>/dev/null || true
    ip route flush table "$TABLE_ID" 2>/dev/null || true
}

# ==================== nftables ====================
setup_nftables() {
    nft delete table ip "$NFTABLES_TABLE" 2>/dev/null || true
    nft add table ip "$NFTABLES_TABLE"

    # 直接使用静态 bypass 列表
    nft add set ip "$NFTABLES_TABLE" BYPASS_LIST '{ type ipv4_addr; flags interval; elements = { 10.0.0.0/8, 127.0.0.0/8, 169.254.0.0/16, 172.16.0.0/12, 192.168.0.0/16, 224.0.0.0/4, 240.0.0.0/4 }; }'

    # ── TCP REDIRECT ── LAN incoming
    nft 'add chain ip '"$NFTABLES_TABLE"' RDR_LAN { type nat hook prerouting priority -100; policy accept; }'
    nft "add rule ip $NFTABLES_TABLE RDR_LAN ip protocol != tcp return"
    nft "add rule ip $NFTABLES_TABLE RDR_LAN $BYPASS_RULE"
    nft "add rule ip $NFTABLES_TABLE RDR_LAN ip daddr @BYPASS_LIST udp dport != 53 return"
    nft "add rule ip $NFTABLES_TABLE RDR_LAN ip daddr @BYPASS_LIST tcp dport != 53 return"
    nft "add rule ip $NFTABLES_TABLE RDR_LAN ip protocol tcp redirect to :$REDIRECT_PORT"

    # ── TCP REDIRECT ── 本机发出
    nft 'add chain ip '"$NFTABLES_TABLE"' RDR_SELF { type nat hook output priority -100; policy accept; }'
    nft "add rule ip $NFTABLES_TABLE RDR_SELF ip protocol != tcp return"
    nft "add rule ip $NFTABLES_TABLE RDR_SELF $BYPASS_RULE"
    nft "add rule ip $NFTABLES_TABLE RDR_SELF ip daddr @BYPASS_LIST udp dport != 53 return"
    nft "add rule ip $NFTABLES_TABLE RDR_SELF ip daddr @BYPASS_LIST tcp dport != 53 return"
    nft "add rule ip $NFTABLES_TABLE RDR_SELF ip protocol tcp redirect to :$REDIRECT_PORT"

    # ── UDP TPROXY ── LAN incoming
    nft 'add chain ip '"$NFTABLES_TABLE"' TPROXY_LAN { type filter hook prerouting priority -150; policy accept; }'
    nft "add rule ip $NFTABLES_TABLE TPROXY_LAN ip protocol != udp return"
    nft "add rule ip $NFTABLES_TABLE TPROXY_LAN $BYPASS_RULE"
    nft "add rule ip $NFTABLES_TABLE TPROXY_LAN ip daddr @BYPASS_LIST udp dport != 53 return"
    nft "add rule ip $NFTABLES_TABLE TPROXY_LAN ip protocol udp tproxy to 127.0.0.1:$TPROXY_PORT meta mark set $FWMARK"

    # ── UDP TPROXY ── 本机发出
    nft 'add chain ip '"$NFTABLES_TABLE"' TPROXY_SELF { type route hook output priority -150; policy accept; }'
    nft "add rule ip $NFTABLES_TABLE TPROXY_SELF ip protocol != udp return"
    nft "add rule ip $NFTABLES_TABLE TPROXY_SELF $BYPASS_RULE"
    nft "add rule ip $NFTABLES_TABLE TPROXY_SELF ip daddr @BYPASS_LIST udp dport != 53 return"
    nft "add rule ip $NFTABLES_TABLE TPROXY_SELF ip protocol udp meta mark set $FWMARK"
}

remove_nftables() {
    nft delete table ip "$NFTABLES_TABLE" 2>/dev/null || true
}

# ==================== Main ====================
case "$ACTION" in
    start|restart)
        remove_nftables
        remove_routes
        setup_routes
        setup_nftables
        echo "Redir-TProxy configuration completed successfully for instance: $INSTANCE"
        ;;
    stop)
        remove_nftables
        remove_routes
        echo "Redir-TProxy configuration stopped for instance: $INSTANCE"
        ;;
    *)
        echo "Usage: $0 {start|stop|restart} [instance]" >&2
        exit 1
        ;;
esac
