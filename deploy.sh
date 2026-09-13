#!/usr/bin/env bash
# ==============================================================================
# MusicBox 部署脚本
#
# 用法:
#   ./deploy.sh --pack [输出路径]      在开发机打包所有需要的文件（自动带时间戳）
#   ./deploy.sh --install [部署包]     在目标服务器上部署安装（需 root）
#   ./deploy.sh --remove [选项]        在目标服务器上卸载并移除文件（需 root）
# ==============================================================================
set -euo pipefail

# ---------------- 常量与路径探测 ----------------

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# 自动判断根目录（兼容根目录执行或 deploy/ 子目录执行）
if [ -f "$SCRIPT_DIR/main.go" ]; then
    ROOT_DIR="$SCRIPT_DIR"
elif [ -f "$SCRIPT_DIR/../main.go" ]; then
    ROOT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
else
    ROOT_DIR="$SCRIPT_DIR"
fi

DESTDIR="${DESTDIR:-}"
SYSTEM_BIN_DIR="${DESTDIR}/usr/local/bin"
SYSTEM_SYSTEMD_DIR="${DESTDIR}/etc/systemd/system"
ETC_SINGBOX_DIR="${DESTDIR}/etc/sing-box"
DATA_SINGBOX_DIR="${DESTDIR}/var/lib/sing-box"
LOG_SINGBOX_DIR="${DESTDIR}/var/log/sing-box"
OPT_MUSICBOX_DIR="${DESTDIR}/opt/musicbox"
RUN_MUSICBOX_DIR="${DESTDIR}/run/musicbox"

# ---------------- 日志与提示 ----------------

log_info()    { printf '\033[34m[INFO]\033[0m %s\n' "$*"; }
log_success() { printf '\033[32m[OK]\033[0m %s\n' "$*"; }
log_warn()    { printf '\033[33m[WARN]\033[0m %s\n' "$*"; }
log_error()   { printf '\033[31m[ERROR]\033[0m %s\n' "$*" >&2; }
die()         { log_error "$*"; exit 1; }

require_root() {
    if [ -n "$DESTDIR" ]; then
        return 0
    fi
    if [ "$(id -u)" -ne 0 ]; then
        die "此操作需要 root 权限，请使用 sudo 执行: sudo $0 ${ACTION:-}"
    fi
}

usage() {
    cat <<EOF
MusicBox 部署与维护工具

用法:
  $0 --pack [--file 输出路径.tar.gz]
      在本地开发机器上打包所有需要的文件（二进制、systemd 单元、配置模板与本脚本），
      默认文件名自动附加时间戳。

  sudo $0 --install [--file 部署包.tar.gz]
      在目标服务器上部署运行环境：
      - 创建 sing-box 系统用户与用户组
      - 创建并赋权必要目录 (/etc/sing-box, /var/lib/sing-box, /var/log/sing-box, /opt/musicbox)
      - 安装 musicbox 与 sing-box 二进制到 /usr/local/bin
      - 安装并初始化模板配置 /etc/sing-box/config_generic.json（保留既有配置）
      - 安装 systemd 单元并开机自启 musicbox.service

  sudo $0 --remove [--keep-singbox] [--keep-config]
      在目标服务器上完全移除 MusicBox 服务与相关文件：
      - 停止并禁用所有相关 systemd 单元
      - 清理残留策略路由 (ip rule/route) 与 nftables 表
      - 移除服务单元、二进制文件与配置/数据目录
      - 移除 sing-box 系统用户与组
      选项:
        --keep-singbox  保留 /usr/local/bin/sing-box 内核二进制
        --keep-config   保留 /etc/sing-box 配置目录

选项别名:
  -p, --pack       打包模式（支持 --file / -f 指定输出）
  -i, --install    安装部署模式（支持 --file / -f 指定安装包）
  -r, --remove     卸载移除模式
  -h, --help       显示帮助信息
EOF
}

# ---------------- 辅助函数 ----------------

get_timestamp() {
    date '+%Y%m%d_%H%M%S'
}

cleanup_firewall() {
    log_info "清理策略路由与 nftables 残留..."
    local pref table

    # 清除策略路由规则
    for pref in 8999 9000 9001 9002 9010; do
        while ip rule del pref "$pref" 2>/dev/null; do
            :
        done
    done

    # 清除 nftables 表
    for table in "inet sing-box" "ip sing-box_tproxy4" "ip sing-box_redir_tproxy4"; do
        # shellcheck disable=SC2086
        nft delete table $table 2>/dev/null || true
    done

    # 清除专用路由表
    for table in 100 2022; do
        ip route flush table "$table" 2>/dev/null || true
    done
}

# 迁移旧版本布局 (/etc/singbox -> /etc/sing-box, /opt/singbox-manager -> /opt/musicbox)
migrate_legacy_layout() {
    local src dst
    for src in /etc/singbox /var/lib/singbox /var/log/singbox; do
        dst="${src/singbox/sing-box}"
        if [ -e "$src" ] && [ ! -e "$dst" ]; then
            mv "$src" "$dst"
            log_info "已迁移旧目录: $src -> $dst"
        elif [ -d "$src" ] && [ -d "$dst" ]; then
            cp -an "$src"/. "$dst"/ 2>/dev/null || true
            rm -rf "$src"
            log_info "已合并旧目录: $src -> $dst"
        fi
    done

    if [ -f /opt/singbox-manager/manager.yaml ] && [ ! -f "$OPT_MUSICBOX_DIR/manager.yaml" ]; then
        cp /opt/singbox-manager/manager.yaml "$OPT_MUSICBOX_DIR/manager.yaml"
        log_info "已迁移旧配置: /opt/singbox-manager/manager.yaml -> $OPT_MUSICBOX_DIR/manager.yaml"
    fi
}

# ---------------- 打包 (--pack) ----------------

do_pack() {
    local target_out=""
    while [ $# -gt 0 ]; do
        case "$1" in
            --file|-f)
                shift
                [ $# -gt 0 ] || die "--file 缺少输出路径参数"
                target_out="$1"
                ;;
            --file=*)
                target_out="${1#*=}"
                ;;
            -*)
                log_warn "忽略未知选项: $1"
                ;;
            *)
                if [ -z "$target_out" ]; then
                    target_out="$1"
                fi
                ;;
        esac
        shift || true
    done

    local timestamp
    timestamp="$(get_timestamp)"

    log_info "开始为项目准备打包..."

    # 1. 查找或构建 musicbox 二进制（严格存放于 build/ 目录）
    local musicbox_bin=""
    if [ -f "$ROOT_DIR/build/musicbox" ] && [ -x "$ROOT_DIR/build/musicbox" ]; then
        musicbox_bin="$ROOT_DIR/build/musicbox"
    fi

    if [ -z "$musicbox_bin" ]; then
        log_info "未检测到 build/musicbox 二进制，尝试自动构建..."
        if command -v go >/dev/null 2>&1; then
            mkdir -p "$ROOT_DIR/build"
            if [ -x "$ROOT_DIR/build.sh" ]; then
                "$ROOT_DIR/build.sh" binary
                musicbox_bin="$ROOT_DIR/build/musicbox"
            else
                local ver
                ver="$(git -C "$ROOT_DIR" describe --tags --exact-match 2>/dev/null || echo dev)"
                (cd "$ROOT_DIR" && go build -ldflags="-s -w -X main.version=$ver" -o "$ROOT_DIR/build/musicbox" .)
                musicbox_bin="$ROOT_DIR/build/musicbox"
            fi
        else
            die "未找到 build/musicbox 且当前环境无 Go 编译器，无法打包！请先编译出 musicbox 二进制。"
        fi
    fi

    # 检测版本号辅助命名
    local version=""
    if [ -x "$musicbox_bin" ]; then
        version="$("$musicbox_bin" info 2>/dev/null | awk '/^musicbox/{print $2; exit}' || true)"
    fi
    if [ -z "$version" ]; then
        version="$(git -C "$ROOT_DIR" describe --tags --exact-match 2>/dev/null || true)"
    fi

    # 默认输出包名（严格统一存放于 build/ 目录，不污染项目根目录）
    local out_file="$target_out"
    if [ -z "$out_file" ]; then
        mkdir -p "$ROOT_DIR/build"
        if [ -n "$version" ]; then
            out_file="$ROOT_DIR/build/musicbox-deploy-${version}-${timestamp}.tar.gz"
        else
            out_file="$ROOT_DIR/build/musicbox-deploy-${timestamp}.tar.gz"
        fi
    fi

    # 2. 查找 sing-box 内核二进制
    local singbox_bin=""
    for candidate in "$ROOT_DIR/build/sing-box" "$ROOT_DIR/bin/sing-box"; do
        if [ -f "$candidate" ] && [ -x "$candidate" ]; then
            singbox_bin="$candidate"
            break
        fi
    done
    if [ -z "$singbox_bin" ]; then
        if command -v sing-box >/dev/null 2>&1; then
            singbox_bin="$(command -v sing-box)"
        fi
    fi

    # 3. 查找服务单元与默认配置
    local mb_service="$ROOT_DIR/deploy/musicbox.service"
    local sb_service="$ROOT_DIR/deploy/sing-box@.service"
    local manager_yaml="$ROOT_DIR/deploy/manager.yaml"
    local config_generic="$ROOT_DIR/deploy/etc-sing-box/config_generic.json"
    local etc_singbox_dir="$ROOT_DIR/deploy/etc-sing-box"
    local kernel_config="$ROOT_DIR/deploy/kernel-proxy.config"

    [ -f "$mb_service" ] || die "缺少必要文件: $mb_service"
    [ -f "$sb_service" ] || die "缺少必要文件: $sb_service"
    [ -f "$config_generic" ] || die "缺少必要文件: $config_generic"

    # 4. 创建暂存目录组装
    local stage_dir
    stage_dir="$(mktemp -d)"
    trap 'rm -rf "${stage_dir:-}"' EXIT

    log_info "组装打包文件到临时目录..."

    # 复制 musicbox 二进制
    cp "$musicbox_bin" "$stage_dir/musicbox"
    chmod 0755 "$stage_dir/musicbox"
    log_info "  + 二进制: musicbox ($(du -h "$musicbox_bin" | awk '{print $1}'))"

    # 复制 sing-box 二进制（若存在）
    if [ -n "$singbox_bin" ]; then
        cp "$singbox_bin" "$stage_dir/sing-box"
        chmod 0755 "$stage_dir/sing-box"
        log_info "  + 内核:   sing-box ($(du -h "$singbox_bin" | awk '{print $1}'))"
    else
        log_warn "  ! 未在 build/ 或系统找到 sing-box 内核二进制，打包中将不包含 sing-box。"
        log_warn "    在目标服务器部署时需自行提供 /usr/local/bin/sing-box。"
    fi

    # 复制服务单元与设置
    cp "$mb_service" "$stage_dir/musicbox.service"
    chmod 0644 "$stage_dir/musicbox.service"
    log_info "  + 单元:   musicbox.service"

    cp "$sb_service" "$stage_dir/sing-box@.service"
    chmod 0644 "$stage_dir/sing-box@.service"
    log_info "  + 单元:   sing-box@.service"

    if [ -f "$manager_yaml" ]; then
        cp "$manager_yaml" "$stage_dir/manager.yaml"
        chmod 0644 "$stage_dir/manager.yaml"
        log_info "  + 设置:   manager.yaml"
    fi

    cp "$config_generic" "$stage_dir/config_generic.json"
    chmod 0644 "$stage_dir/config_generic.json"
    log_info "  + 配置:   config_generic.json"

    if [ -d "$etc_singbox_dir" ]; then
        mkdir -p "$stage_dir/etc-sing-box"
        cp -r "$etc_singbox_dir"/* "$stage_dir/etc-sing-box/"
        chmod -R 0644 "$stage_dir/etc-sing-box"/*
        log_info "  + 模式配置目录: etc-sing-box/"
    fi

    if [ -f "$kernel_config" ]; then
        cp "$kernel_config" "$stage_dir/kernel-proxy.config"
        log_info "  + 参考:   kernel-proxy.config"
    fi

    # 复制本脚本自身（用于解压后一键部署与卸载）
    cp "$ROOT_DIR/deploy.sh" "$stage_dir/deploy.sh"
    chmod 0755 "$stage_dir/deploy.sh"
    log_info "  + 脚本:   deploy.sh"

    # 5. 打包归档
    mkdir -p "$(dirname "$out_file")"
    tar -czf "$out_file" -C "$stage_dir" .

    local file_size
    file_size="$(du -h "$out_file" | awk '{print $1}')"
    rm -rf "$stage_dir"
    trap - EXIT

    log_success "打包完成！产物路径: $out_file (大小: $file_size)"
    echo ""
    log_info "部署到远程服务器示例:"
    log_info "  1. 上传安装包:"
    log_info "     scp \"$out_file\" root@<server_ip>:/tmp/"
    log_info "  2. 在服务器上使用 --file 直接部署 (推荐):"
    log_info "     ssh root@<server_ip> \"./deploy.sh --install --file /tmp/$(basename "$out_file")\""
    log_info "  或者解压后进入目录部署:"
    log_info "     ssh root@<server_ip> \"mkdir -p /tmp/mb-deploy && tar -xzf /tmp/$(basename "$out_file") -C /tmp/mb-deploy && cd /tmp/mb-deploy && ./deploy.sh --install\""
}

# ---------------- 部署安装 (--install) ----------------

do_install() {
    require_root "$@"

    local pkg_arg=""
    while [ $# -gt 0 ]; do
        case "$1" in
            --file|-f)
                shift
                [ $# -gt 0 ] || die "--file 缺少部署包路径参数"
                pkg_arg="$1"
                ;;
            --file=*)
                pkg_arg="${1#*=}"
                ;;
            -*)
                log_warn "忽略未知选项: $1"
                ;;
            *)
                if [ -z "$pkg_arg" ]; then
                    pkg_arg="$1"
                fi
                ;;
        esac
        shift || true
    done

    local work_dir="$SCRIPT_DIR"
    local cleanup_work_dir=false

    # 如果传了 tar.gz 包路径，先解压到临时目录
    if [ -n "$pkg_arg" ]; then
        if [ -f "$pkg_arg" ]; then
            work_dir="$(mktemp -d)"
            cleanup_work_dir=true
            log_info "解压部署归档包: $pkg_arg ..."
            tar -xzf "$pkg_arg" -C "$work_dir"
        else
            die "指定的部署包不存在: $pkg_arg"
        fi
    fi

    if [ "$cleanup_work_dir" = true ]; then
        trap 'rm -rf "${work_dir:-}"' EXIT
    fi

    # 定位各物料文件
    local musicbox_src=""
    for f in "$work_dir/musicbox" "$work_dir/build/musicbox" "$ROOT_DIR/musicbox" "$ROOT_DIR/build/musicbox"; do
        if [ -f "$f" ] && [ -x "$f" ]; then
            musicbox_src="$f"
            break
        fi
    done
    [ -n "$musicbox_src" ] || die "未找到 musicbox 二进制文件，无法进行安装！"

    local singbox_src=""
    for f in "$work_dir/sing-box" "$work_dir/build/sing-box" "$work_dir/bin/sing-box" "$ROOT_DIR/build/sing-box" "$ROOT_DIR/bin/sing-box"; do
        if [ -f "$f" ] && [ -x "$f" ]; then
            singbox_src="$f"
            break
        fi
    done

    local mb_service_src=""
    for f in "$work_dir/musicbox.service" "$work_dir/deploy/musicbox.service" "$ROOT_DIR/deploy/musicbox.service"; do
        if [ -f "$f" ]; then
            mb_service_src="$f"
            break
        fi
    done
    [ -n "$mb_service_src" ] || die "未找到 musicbox.service 服务单元文件！"

    local sb_service_src=""
    for f in "$work_dir/sing-box@.service" "$work_dir/deploy/sing-box@.service" "$ROOT_DIR/deploy/sing-box@.service"; do
        if [ -f "$f" ]; then
            sb_service_src="$f"
            break
        fi
    done
    [ -n "$sb_service_src" ] || die "未找到 sing-box@.service 服务单元文件！"

    local cfg_generic_src=""
    for f in "$work_dir/config_generic.json" "$work_dir/deploy/etc-sing-box/config_generic.json" "$ROOT_DIR/deploy/etc-sing-box/config_generic.json"; do
        if [ -f "$f" ]; then
            cfg_generic_src="$f"
            break
        fi
    done
    [ -n "$cfg_generic_src" ] || die "未找到 config_generic.json 模板配置文件！"

    local manager_yaml_src=""
    for f in "$work_dir/manager.yaml" "$work_dir/deploy/manager.yaml" "$ROOT_DIR/deploy/manager.yaml"; do
        if [ -f "$f" ]; then
            manager_yaml_src="$f"
            break
        fi
    done

    # 开始安装流程
    log_info "=================================================="
    log_info "开始在当前系统部署 MusicBox"
    log_info "=================================================="

    # 1. 用户创建
    log_info "[1/6] 创建/检查 sing-box 系统用户与用户组..."
    if [ -z "$DESTDIR" ]; then
        if ! id sing-box >/dev/null 2>&1; then
            useradd --system --shell /usr/sbin/nologin --home-dir "$DATA_SINGBOX_DIR" --no-create-home sing-box
            log_success "已创建系统用户 sing-box (UID=$(id -u sing-box), GID=$(id -g sing-box))"
        else
            log_info "系统用户 sing-box 已存在 (UID=$(id -u sing-box), GID=$(id -g sing-box))"
            usermod -d "$DATA_SINGBOX_DIR" sing-box 2>/dev/null || true
        fi
    else
        log_info "[DESTDIR 模式] 跳过真实系统用户创建"
    fi

    # 2. 文件夹创建与权限
    log_info "[2/6] 创建对应文件夹并设置权限..."
    if [ -z "$DESTDIR" ]; then
        # 配置目录: 0755 root:root
        install -d -m 0755 -o root -g root "$ETC_SINGBOX_DIR"
        # 管理器主目录: 0755 root:root
        install -d -m 0755 -o root -g root "$OPT_MUSICBOX_DIR"
        # 数据运行目录: 0750 sing-box:sing-box
        install -d -m 0750 -o sing-box -g sing-box "$DATA_SINGBOX_DIR"
        chown -R sing-box:sing-box "$DATA_SINGBOX_DIR"
        chmod 0750 "$DATA_SINGBOX_DIR"
        # 日志目录: 0750 sing-box:sing-box
        install -d -m 0750 -o sing-box -g sing-box "$LOG_SINGBOX_DIR"
        chown -R sing-box:sing-box "$LOG_SINGBOX_DIR"
        chmod 0750 "$LOG_SINGBOX_DIR"
        # 运行时目录: 0750 root:root
        install -d -m 0750 -o root -g root "$RUN_MUSICBOX_DIR"
    else
        install -d -m 0755 "$ETC_SINGBOX_DIR"
        install -d -m 0755 "$OPT_MUSICBOX_DIR"
        install -d -m 0750 "$DATA_SINGBOX_DIR"
        install -d -m 0750 "$LOG_SINGBOX_DIR"
        install -d -m 0750 "$RUN_MUSICBOX_DIR"
    fi
    log_success "配置目录:     $ETC_SINGBOX_DIR (0755)"
    log_success "管理器目录:   $OPT_MUSICBOX_DIR (0755)"
    log_success "数据运行目录: $DATA_SINGBOX_DIR (0750 sing-box:sing-box)"
    log_success "日志目录:     $LOG_SINGBOX_DIR (0750 sing-box:sing-box)"

    # 3. 安装二进制文件
    log_info "[3/6] 安装二进制文件到 $SYSTEM_BIN_DIR ..."
    install -d "$SYSTEM_BIN_DIR"
    if [ -z "$DESTDIR" ]; then
        install -m 0755 -o root -g root "$musicbox_src" "$SYSTEM_BIN_DIR/musicbox"
    else
        install -m 0755 "$musicbox_src" "$SYSTEM_BIN_DIR/musicbox"
    fi
    log_success "已安装: $SYSTEM_BIN_DIR/musicbox"

    if [ -n "$singbox_src" ]; then
        if [ -z "$DESTDIR" ]; then
            install -m 0755 -o root -g root "$singbox_src" "$SYSTEM_BIN_DIR/sing-box"
        else
            install -m 0755 "$singbox_src" "$SYSTEM_BIN_DIR/sing-box"
        fi
        log_success "已安装: $SYSTEM_BIN_DIR/sing-box"
    elif [ -x "$SYSTEM_BIN_DIR/sing-box" ]; then
        log_info "系统中已存在 $SYSTEM_BIN_DIR/sing-box，保留当前版本"
    else
        log_warn "未提供 sing-box 内核且 $SYSTEM_BIN_DIR/sing-box 不存在！"
        log_warn "sing-box 服务启动需要该内核。请自行下载 sing-box 并拷贝到 $SYSTEM_BIN_DIR/sing-box"
    fi

    # 赋予内核必要 capability（如系统支持 setcap）
    if [ -z "$DESTDIR" ] && command -v setcap >/dev/null 2>&1 && [ -x "$SYSTEM_BIN_DIR/sing-box" ]; then
        setcap 'cap_net_bind_service,cap_net_admin,cap_net_raw+ep' "$SYSTEM_BIN_DIR/sing-box" 2>/dev/null || true
    fi

    # 4. 配置初始化与迁移
    log_info "[4/6] 检查配置模板与兼容迁移..."
    if [ -z "$DESTDIR" ]; then
        migrate_legacy_layout
    fi

    if [ ! -f "$ETC_SINGBOX_DIR/config_generic.json" ]; then
        if [ -z "$DESTDIR" ]; then
            install -m 0644 -o root -g root "$cfg_generic_src" "$ETC_SINGBOX_DIR/config_generic.json"
        else
            install -m 0644 "$cfg_generic_src" "$ETC_SINGBOX_DIR/config_generic.json"
        fi
        log_success "已初始化模板配置: $ETC_SINGBOX_DIR/config_generic.json"
    else
        log_info "检测到已有 $ETC_SINGBOX_DIR/config_generic.json，保留现有配置不覆盖"
    fi

    # 复制各模式初始参考配置（如不存在）
    for cfg_dir in "$work_dir/etc-sing-box" "$work_dir/deploy/etc-sing-box" "$ROOT_DIR/deploy/etc-sing-box"; do
        if [ -d "$cfg_dir" ]; then
            for mode_cfg in "$cfg_dir"/config_*.json; do
                if [ -f "$mode_cfg" ]; then
                    local fname
                    fname="$(basename "$mode_cfg")"
                    if [ ! -f "$ETC_SINGBOX_DIR/$fname" ]; then
                        if [ -z "$DESTDIR" ]; then
                            install -m 0644 -o root -g root "$mode_cfg" "$ETC_SINGBOX_DIR/$fname"
                        else
                            install -m 0644 "$mode_cfg" "$ETC_SINGBOX_DIR/$fname"
                        fi
                        log_info "  + 初始化模式配置: $fname"
                    fi
                fi
            done
            break
        fi
    done

    # 初始化 manager.yaml（若不存在）
    if [ -n "$manager_yaml_src" ] && [ ! -f "$OPT_MUSICBOX_DIR/manager.yaml" ]; then
        if [ -z "$DESTDIR" ]; then
            install -m 0644 -o root -g root "$manager_yaml_src" "$OPT_MUSICBOX_DIR/manager.yaml"
            if id sing-box >/dev/null 2>&1; then
                local sbgid
                sbgid="$(id -g sing-box)"
                sed -i "s/exclude_gid: [0-9]\+/exclude_gid: $sbgid/g" "$OPT_MUSICBOX_DIR/manager.yaml"
            fi
        else
            install -m 0644 "$manager_yaml_src" "$OPT_MUSICBOX_DIR/manager.yaml"
        fi
        log_success "已初始化系统设置: $OPT_MUSICBOX_DIR/manager.yaml"
    elif [ -f "$OPT_MUSICBOX_DIR/manager.yaml" ]; then
        log_info "已有系统设置 $OPT_MUSICBOX_DIR/manager.yaml，保留现有配置"
    fi

    # 5. 安装 systemd 单元
    log_info "[5/6] 安装 systemd 单元文件..."
    install -d "$SYSTEM_SYSTEMD_DIR"
    if [ -z "$DESTDIR" ]; then
        # 停止历史遗留可能冲突的旧服务
        for inst in tun tproxy redir-tproxy socks server ep ebpf; do
            systemctl stop "singbox@$inst" 2>/dev/null || true
        done
        systemctl stop singbox-manager 2>/dev/null || true
        systemctl stop sing-box-panel 2>/dev/null || true

        install -m 0644 -o root -g root "$mb_service_src" "$SYSTEM_SYSTEMD_DIR/musicbox.service"
        install -m 0644 -o root -g root "$sb_service_src" "$SYSTEM_SYSTEMD_DIR/sing-box@.service"

        # 清除旧单例单元
        rm -f "$SYSTEM_SYSTEMD_DIR/singbox@.service" \
              "$SYSTEM_SYSTEMD_DIR/singbox-manager.service" \
              "$SYSTEM_SYSTEMD_DIR/sing-box-panel.service"
    else
        install -m 0644 "$mb_service_src" "$SYSTEM_SYSTEMD_DIR/musicbox.service"
        install -m 0644 "$sb_service_src" "$SYSTEM_SYSTEMD_DIR/sing-box@.service"
    fi

    log_success "已安装: $SYSTEM_SYSTEMD_DIR/musicbox.service"
    log_success "已安装: $SYSTEM_SYSTEMD_DIR/sing-box@.service"

    # 6. 重新加载并启动
    if [ -z "$DESTDIR" ]; then
        log_info "[6/6] 重载 systemd 并启动服务..."
        systemctl daemon-reload
        systemctl enable musicbox.service >/dev/null 2>&1 || true
        systemctl restart musicbox.service

        sleep 2

        if systemctl is-active --quiet musicbox.service; then
            local host_ip
            host_ip="$(ip route get 1.1.1.1 2>/dev/null | awk '{print $7; exit}' || true)"
            [ -n "$host_ip" ] || host_ip="127.0.0.1"

            echo ""
            log_success "=================================================="
            log_success "MusicBox 部署完成并已成功启动！"
            log_success "  - Web 控制面板:  http://${host_ip}:8082"
            log_success "  - 服务状态查询:  systemctl status musicbox"
            log_success "  - CLI 命令行工具: musicbox status / musicbox info"
            log_success "=================================================="
        else
            log_warn "musicbox.service 状态非 active，请查看日志排查:"
            journalctl -u musicbox.service -n 25 --no-pager || true
        fi
    else
        log_success "[6/6] [DESTDIR 模式] 文件部署完成，跳过 systemd 守护进程启停"
    fi

    if [ "$cleanup_work_dir" = true ]; then
        rm -rf "$work_dir"
        trap - EXIT
    fi
}

# ---------------- 移除卸载 (--remove) ----------------

do_remove() {
    require_root "$@"

    local keep_singbox=false
    local keep_config=false

    for arg in "$@"; do
        case "$arg" in
            --keep-singbox) keep_singbox=true ;;
            --keep-config)  keep_config=true ;;
        esac
    done

    log_info "=================================================="
    log_info "开始在当前系统移除 MusicBox 相关文件与服务"
    log_info "=================================================="

    # 1. 停止并禁用所有相关服务
    if [ -z "$DESTDIR" ]; then
        log_info "[1/4] 停止并禁用 systemd 服务..."
        local units=(
            musicbox
            singbox-manager
            sing-box-panel
            tproxy@sing-box
            redir-tproxy@sing-box
        )
        for inst in tun tproxy redir-tproxy socks server ep ebpf; do
            units+=("sing-box@$inst" "singbox@$inst")
        done

        for u in "${units[@]}"; do
            systemctl disable --now "$u" 2>/dev/null || true
        done
    else
        log_info "[1/4] [DESTDIR 模式] 跳过 systemd 服务停止"
    fi

    # 2. 清理策略路由与防火墙表残留
    if [ -z "$DESTDIR" ]; then
        log_info "[2/4] 清理策略路由与网络规则残留..."
        cleanup_firewall
    else
        log_info "[2/4] [DESTDIR 模式] 跳过策略路由与防火墙清理"
    fi

    # 3. 移除服务单元、二进制与目录
    log_info "[3/4] 移除服务单元与文件..."
    rm -f "$SYSTEM_SYSTEMD_DIR/musicbox.service" \
          "$SYSTEM_SYSTEMD_DIR/sing-box@.service" \
          "$SYSTEM_SYSTEMD_DIR/singbox@.service" \
          "$SYSTEM_SYSTEMD_DIR/singbox-manager.service" \
          "$SYSTEM_SYSTEMD_DIR/sing-box-panel.service" \
          "$SYSTEM_SYSTEMD_DIR/tproxy@.service" \
          "$SYSTEM_SYSTEMD_DIR/redir-tproxy@.service"

    rm -f "$SYSTEM_BIN_DIR/musicbox" \
          "$SYSTEM_BIN_DIR/singbox-manager" \
          "$SYSTEM_BIN_DIR/sing-box-ctl" \
          /usr/local/libexec/tproxy.sh \
          /usr/local/libexec/redir-tproxy.sh
    rmdir /usr/local/libexec 2>/dev/null || true

    if [ "$keep_singbox" = true ]; then
        log_info "已保留内核二进制: $SYSTEM_BIN_DIR/sing-box"
    else
        rm -f "$SYSTEM_BIN_DIR/sing-box"
        log_info "已删除内核二进制: $SYSTEM_BIN_DIR/sing-box"
    fi

    # 目录清理
    rm -rf "$OPT_MUSICBOX_DIR" /opt/singbox-manager /opt/sing-box-panel \
           "$DATA_SINGBOX_DIR" /var/lib/singbox \
           "$LOG_SINGBOX_DIR" /var/log/singbox \
           "$RUN_MUSICBOX_DIR" /run/singbox-manager

    if [ "$keep_config" = true ]; then
        log_info "已保留配置目录: $ETC_SINGBOX_DIR"
    else
        rm -rf "$ETC_SINGBOX_DIR" /etc/singbox
        log_info "已删除配置目录: $ETC_SINGBOX_DIR"
    fi

    # 4. 删除用户
    if [ -z "$DESTDIR" ]; then
        log_info "[4/4] 移除 sing-box 系统用户与用户组..."
        pkill -u sing-box 2>/dev/null || true
        userdel sing-box 2>/dev/null || true
        groupdel sing-box 2>/dev/null || true

        systemctl daemon-reload
        systemctl reset-failed 2>/dev/null || true
    else
        log_info "[4/4] [DESTDIR 模式] 跳过系统用户删除与 systemctl 重载"
    fi

    log_success "=================================================="
    log_success "MusicBox 相关文件与服务已全部清理移除完成。"
    log_success "=================================================="
}

# ---------------- 主入口分发 ----------------

ACTION="${1:-}"
case "$ACTION" in
    --pack|-p|pack)
        shift || true
        do_pack "$@"
        ;;
    --install|-i|install|copy|upgrade)
        shift || true
        do_install "$@"
        ;;
    --remove|-r|remove)
        shift || true
        do_remove "$@"
        ;;
    --help|-h|help|"")
        usage
        ;;
    *)
        log_error "未知指令: $ACTION"
        usage >&2
        exit 1
        ;;
esac
