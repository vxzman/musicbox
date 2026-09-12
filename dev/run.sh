#!/bin/bash
# 开发运行：守护进程使用 dev/ 下 fixture（不触碰 /opt/musicbox 与 /etc/sing-box）。
# 前端开发另开终端: cd web && npm run dev（vite 代理到 :8082）
set -e
cd "$(dirname "$0")/.."
export MUSICBOX_CONFIG="$PWD/dev/manager.yaml"
exec go run . serve
