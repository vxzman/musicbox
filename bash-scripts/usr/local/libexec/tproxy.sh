#!/usr/bin/env bash
set -euo pipefail

ACTION="${1:-}"
# PROXYCORE can be set via: CLI arg $2, env PROXYCORE, or systemctl wrapper
INSTANCE="${2:-${PROXYCORE:-}}"

# ==================== Requirements Check ====================
for cmd in ip nft; do
    command -v "$cmd" >/dev/null 2>&1 || {
        echo "FATAL: command not found: $cmd" >&2
        exit 1
    }
done

[ -n "$INSTANCE" ] || {
    echo "FATAL: PROXYCORE not set (pass as arg2 or set env PROXYCORE)" >&2
    echo "Usage: $0 {start|stop} [instance]" >&2
    exit 1
}

# 配置文件路径：匹配 tproxy@<INSTANCE> 读取 <INSTANCE>_tproxy.conf
CONFIG_FILE="/etc/sing-box/${INSTANCE}_tproxy.conf"

[ -f "$CONFIG_FILE" ] || {
    echo "FATAL: config file not found: $CONFIG_FILE" >&2
    echo "HINT: Please create it, e.g., /etc/sing-box/${INSTANCE}_tproxy.conf" >&2
    exit 1
}

# shellcheck source=/dev/null
source "$CONFIG_FILE"

# ==================== Variable Validation ====================
require_vars=(
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

# ==================== Bypass Rule Logic ====================
# 当两者都存在时，默认优先使用 EXCLUDE_GID
if [ -n "${EXCLUDE_GID:-}" ]; then
    BYPASS_RULE="meta skgid $EXCLUDE_GID return"
else
    BYPASS_RULE="meta mark $ROUTING_MARK return"
fi

# Reserved IPv4 Ranges for Bypassing
BYPASS_IPS=(
    "127.0.0.0/8"
    "10.0.0.0/8"
    "172.16.0.0/12"
    "192.168.0.0/16"
    "169.254.0.0/16"
    "224.0.0.0/4"
    "240.0.0.0/4"
)

# ==================== Routing Logic ====================
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

# ==================== nftables Orchestration ====================
setup_nftables() {
    nft delete table ip "$NFTABLES_TABLE" 2>/dev/null || true
    nft add table ip "$NFTABLES_TABLE"

    # Define bypass set with static reserved ranges
    nft add set ip "$NFTABLES_TABLE" BYPASS_LIST \
        '{ type ipv4_addr; flags interval; elements = { '"$(IFS=,; echo "${BYPASS_IPS[*]}")"' } }'

    # -------- TPROXY_FORWARD (LAN incoming) --------
    nft add chain ip "$NFTABLES_TABLE" TPROXY_FORWARD \
        '{ type filter hook prerouting priority -150; policy accept; }'

    # Exclude proxy self-traffic (Dynamic Rule)
    nft add rule ip "$NFTABLES_TABLE" TPROXY_FORWARD $BYPASS_RULE
    
    # Allow local DNS/Services on bypass list, except for specific overrides if needed
    nft add rule ip "$NFTABLES_TABLE" TPROXY_FORWARD ip daddr @BYPASS_LIST udp dport != 53 return
    nft add rule ip "$NFTABLES_TABLE" TPROXY_FORWARD ip daddr @BYPASS_LIST tcp dport != 53 return

    nft add rule ip "$NFTABLES_TABLE" TPROXY_FORWARD \
        ip protocol { tcp, udp } \
        meta mark set "$FWMARK" \
        tproxy to 127.0.0.1:"$TPROXY_PORT"

    # -------- TPROXY_LOCAL (本机发出) --------
    nft add chain ip "$NFTABLES_TABLE" TPROXY_LOCAL \
        '{ type route hook output priority -150; policy accept; }'

    # Exclude proxy self-traffic (Dynamic Rule)
    nft add rule ip "$NFTABLES_TABLE" TPROXY_LOCAL $BYPASS_RULE

    # Skip bypass list
    nft add rule ip "$NFTABLES_TABLE" TPROXY_LOCAL ip daddr @BYPASS_LIST udp dport != 53 return
    nft add rule ip "$NFTABLES_TABLE" TPROXY_LOCAL ip daddr @BYPASS_LIST tcp dport != 53 return

    # Mark local traffic to be routed via policy table
    nft add rule ip "$NFTABLES_TABLE" TPROXY_LOCAL ip protocol { tcp, udp } meta mark set "$FWMARK"
}

remove_nftables() {
    nft delete table ip "$NFTABLES_TABLE" 2>/dev/null || true
}

# ==================== Main ====================
case "$ACTION" in
    start)
        remove_nftables
        remove_routes
        setup_routes
        setup_nftables
        echo "TProxy configuration completed successfully for instance: $INSTANCE"
        ;;
    stop)
        remove_nftables
        remove_routes
        echo "TProxy configuration stopped for instance: $INSTANCE"
        ;;
    *)
        echo "Usage: $0 {start|stop} [instance]" >&2
        exit 1
        ;;
esac
