#!/usr/bin/env bash
# musicbox 部署脚本
#   用法:
#     deploy.sh install   首次安装（构建、装二进制/单元、初始化配置）
#     deploy.sh copy      免编译安装（复制已构建二进制，目标机无需 Go）
#     deploy.sh upgrade   从旧 sing-box-web-panel / singbox-manager 布局升级（自动迁移）
#     deploy.sh remove    移除全部相关文件与服务（含规则清理）
set -euo pipefail

SELF_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "$SELF_DIR/.." && pwd)"

MANAGER_BIN=/usr/local/bin/musicbox
CONFIG_DIR=/opt/musicbox
ETC_SINGBOX=/etc/sing-box
SYSTEMD_DIR=/etc/systemd/system
MANAGER_YAML="$CONFIG_DIR/manager.yaml"

require_root() {
    if [ "$(id -u)" -ne 0 ]; then
        echo "FATAL: 需要 root 权限" >&2
        exit 1
    fi
}

# ---------------- 构建 ----------------

# 有已编译二进制则跳过构建（老机器无 Go 也能升级）
ensure_binary() {
    if [ -x "$ROOT_DIR/musicbox" ]; then
        echo "[1/5] 使用已编译二进制: $ROOT_DIR/musicbox"
        return 0
    fi
    build
}

build() {
    echo "[1/5] 构建 musicbox..."
    local version build_time git_commit
    version="$(git -C "$ROOT_DIR" describe --tags --exact-match 2>/dev/null || echo dev)"
    build_time="$(date -u '+%Y-%m-%dT%H:%M:%SZ')"
    git_commit="$(git -C "$ROOT_DIR" rev-parse --short HEAD 2>/dev/null || echo unknown)"
    (cd "$ROOT_DIR" && go build -ldflags="-s -w \
        -X main.version=$version \
        -X main.buildTime=$build_time \
        -X main.gitCommit=$git_commit" -o musicbox .)
    dist_files="$(find "$ROOT_DIR/web/dist" -type f 2>/dev/null | wc -l)"
    if [ "$dist_files" -le 1 ]; then
        echo "      [WARN] 前端未构建（当前为占位页）。构建方式: cd web && npm install && npm run build"
    fi
    echo "      [OK] 二进制: $ROOT_DIR/musicbox（$version @ $build_time, $git_commit）"
}

# ---------------- 安装公共部分 ----------------

# 把旧布局 /etc/singbox、/var/lib/singbox、singbox@.service 迁到官方 sing-box 命名。
migrate_singbox_layout() {
    local src dst
    for src in /etc/singbox /var/lib/singbox /var/log/singbox; do
        dst="${src/singbox/sing-box}"
        if [ -e "$src" ] && [ ! -e "$dst" ]; then
            mv "$src" "$dst"
            echo "      [OK] 已迁移 $src -> $dst"
        elif [ -d "$src" ] && [ -d "$dst" ]; then
            cp -an "$src"/. "$dst"/ 2>/dev/null || true
            rm -rf "$src"
            echo "      [OK] 已合并 $src -> $dst"
        fi
    done
    if [ -f "$MANAGER_YAML" ]; then
        sed -i \
            -e 's|/etc/singbox|/etc/sing-box|g' \
            -e 's|/var/lib/singbox|/var/lib/sing-box|g' \
            -e 's|unit: singbox@|unit: sing-box@|g' \
            "$MANAGER_YAML"
    fi
    if id sing-box >/dev/null 2>&1; then
        usermod -d /var/lib/sing-box sing-box 2>/dev/null || true
    fi
}

install_common() {
    echo "[2/5] 安装二进制与 systemd 单元..."
    migrate_singbox_layout
    for inst in tun tproxy redir-tproxy socks server ep ebpf; do
        systemctl stop "singbox@$inst" 2>/dev/null || true
    done
    install -m 0755 -o root -g root "$ROOT_DIR/musicbox" "$MANAGER_BIN"
    install -m 0644 -o root -g root "$SELF_DIR/sing-box@.service" "$SYSTEMD_DIR/sing-box@.service"
    install -m 0644 -o root -g root "$SELF_DIR/musicbox.service" "$SYSTEMD_DIR/musicbox.service"
    rm -f "$SYSTEMD_DIR/singbox@.service"
    echo "      [OK] 已安装 $MANAGER_BIN 与 systemd 单元"

    # 模板通用配置（仅首次；守护启动后自动生成各模式配置）
    install -d -m 0755 "$ETC_SINGBOX"
    if [ ! -f "$ETC_SINGBOX/config_generic.json" ]; then
        install -m 0644 -o root -g root "$SELF_DIR/etc-sing-box/config_generic.json" "$ETC_SINGBOX/config_generic.json"
        echo "      [OK] 已安装模板配置 config_generic.json"
    fi

    # manager.yaml（不存在时守护首启也会自动生成/迁移）
    install -d -m 0755 "$CONFIG_DIR"
}

ensure_user() {
    if ! id sing-box >/dev/null 2>&1; then
        useradd --system --shell /usr/sbin/nologin --home-dir /var/lib/sing-box --no-create-home sing-box
        echo "[3/5] 已创建系统用户 sing-box (gid=$(id -g sing-box))"
    else
        echo "[3/5] sing-box 用户已存在"
        usermod -d /var/lib/sing-box sing-box 2>/dev/null || true
    fi
    install -d -m 0750 -o sing-box -g sing-box /var/lib/sing-box
    install -d -m 0750 -o sing-box -g sing-box /var/log/sing-box
}

enable_and_start() {
    systemctl daemon-reload
    systemctl enable musicbox >/dev/null 2>&1 || true
    systemctl restart musicbox
    echo "[5/5] 已启动 musicbox。面板: http://<host>:8082"
}

# ---------------- install / copy ----------------

do_install() {
    require_root
    ensure_binary
    install_common
    ensure_user
    echo "[4/5] manager.yaml 由守护进程首次启动自动生成（或使用 MUSICBOX_CONFIG 指定）"
    enable_and_start
}

do_copy() {
    require_root
    ensure_binary
    install_common
    ensure_user
    echo "[4/5] manager.yaml 由守护进程首次启动自动生成（或使用 MUSICBOX_CONFIG 指定）"
    enable_and_start
}

# ---------------- upgrade（老布局迁移） ----------------

do_upgrade() {
    require_root
    ensure_binary

    echo "[1/6] 停止旧服务..."
    for unit in \
        musicbox singbox-manager sing-box-panel \
        tproxy@sing-box redir-tproxy@sing-box \
        singbox@tun singbox@tproxy singbox@redir-tproxy singbox@socks \
        singbox@server singbox@ep singbox@ebpf \
        sing-box@tun sing-box@tproxy sing-box@redir-tproxy sing-box@socks \
        sing-box@server sing-box@ep sing-box@ebpf; do
        systemctl disable --now "$unit" 2>/dev/null || true
    done

    # 先装新组件并启动守护：/etc/singbox → /etc/sing-box 与 manager.yaml 的迁移
    # 发生在 install_common / 守护首启
    install_common
    # 迁移旧 /opt/singbox-manager/manager.yaml
    if [ -f /opt/singbox-manager/manager.yaml ] && [ ! -f "$MANAGER_YAML" ]; then
        cp /opt/singbox-manager/manager.yaml "$MANAGER_YAML"
        echo "      [OK] 已迁移旧配置 /opt/singbox-manager/manager.yaml -> $MANAGER_YAML"
    fi
    ensure_user
    systemctl daemon-reload
    systemctl restart musicbox
    sleep 2

    echo "[5/6] 清理旧布局文件..."
    if [ -f "$MANAGER_YAML" ]; then
        echo "      [OK] manager.yaml 已就位: $MANAGER_YAML"
    else
        echo "      [WARN] manager.yaml 未生成，请检查 journalctl -u musicbox"
    fi
    rm -f "$SYSTEMD_DIR/singbox-manager.service" \
          "$SYSTEMD_DIR/sing-box-panel.service" \
          "$SYSTEMD_DIR/tproxy@.service" \
          "$SYSTEMD_DIR/redir-tproxy@.service"
    rm -f /usr/local/bin/singbox-manager \
          /usr/local/bin/sing-box-ctl
    rm -f /usr/local/libexec/tproxy.sh /usr/local/libexec/redir-tproxy.sh
    rmdir /usr/local/libexec 2>/dev/null || true
    rm -rf /opt/singbox-manager
    rm -rf /opt/sing-box-panel
    rm -rf /run/singbox-manager

    systemctl daemon-reload
    echo "[6/6] 升级完成。可用: musicbox status"
}

# ---------------- remove ----------------

cleanup_firewall() {
    # 清理各模式可能残留的策略路由与 nftables 规则
    local pref table
    for pref in 8999 9000 9001 9002 9010; do
        while ip rule del pref "$pref" 2>/dev/null; do :; done
    done
    for table in "inet sing-box" "ip sing-box_tproxy4" "ip sing-box_redir_tproxy4"; do
        # shellcheck disable=SC2086
        nft delete table $table 2>/dev/null || true
    done
    for table in 100 2022; do
        ip route flush table "$table" 2>/dev/null || true
    done
}

do_remove() {
    require_root

    echo "[1/4] 停止并禁用服务..."
    for unit in \
        musicbox singbox-manager \
        singbox@tun singbox@tproxy singbox@redir-tproxy singbox@socks \
        singbox@server singbox@ep singbox@ebpf \
        sing-box@tun sing-box@tproxy sing-box@redir-tproxy sing-box@socks \
        sing-box@server sing-box@ep sing-box@ebpf \
        tproxy@sing-box redir-tproxy@sing-box sing-box-panel; do
        systemctl disable --now "$unit" 2>/dev/null || true
    done

    echo "[2/4] 清理策略路由与 nftables 残留..."
    cleanup_firewall

    echo "[3/4] 删除文件..."
    rm -f "$MANAGER_BIN" /usr/local/bin/singbox-manager
    rm -f "$SYSTEMD_DIR/musicbox.service" \
          "$SYSTEMD_DIR/singbox-manager.service" \
          "$SYSTEMD_DIR/singbox@.service" \
          "$SYSTEMD_DIR/sing-box@.service" \
          "$SYSTEMD_DIR/tproxy@.service" \
          "$SYSTEMD_DIR/redir-tproxy@.service" \
          "$SYSTEMD_DIR/sing-box-panel.service"
    rm -rf "$ETC_SINGBOX" /etc/singbox /var/lib/sing-box /var/lib/singbox \
          /var/log/sing-box /var/log/singbox "$CONFIG_DIR" /opt/singbox-manager \
          /run/musicbox /run/singbox-manager
    rm -f /usr/local/bin/sing-box-ctl
    rm -f /usr/local/libexec/tproxy.sh /usr/local/libexec/redir-tproxy.sh
    rmdir /usr/local/libexec 2>/dev/null || true

    echo "[4/4] 删除 sing-box 用户..."
    userdel sing-box 2>/dev/null || true

    systemctl daemon-reload
    echo "[OK] 已移除全部 musicbox 相关文件与服务（未删除 /usr/local/bin/sing-box 内核二进制）"
}

# ---------------- Main ----------------

ACTION="${1:-}"
case "$ACTION" in
    install)  do_install ;;
    copy)     do_copy ;;
    upgrade)  do_upgrade ;;
    remove)   do_remove ;;
    *)
        echo "用法:"
        echo "  $0 install   首次安装（目标机编译）"
        echo "  $0 copy      免编译安装（复制已构建二进制，目标机无需 Go）"
        echo "  $0 upgrade   从旧布局升级（自动迁移 .conf → manager.yaml）"
        echo "  $0 remove    移除全部相关文件与服务"
        exit 1
        ;;
esac
