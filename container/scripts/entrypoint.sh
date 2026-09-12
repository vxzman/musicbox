#!/bin/bash
set -euo pipefail

mkdir -p /opt/musicbox /etc/sing-box /var/lib/sing-box /run/musicbox

if [ ! -f /etc/sing-box/config_generic.json ]; then
    cp /usr/share/sing-box/config_generic.json /etc/sing-box/config_generic.json
fi

if [ ! -f /opt/musicbox/manager.yaml ]; then
    cp /usr/share/musicbox/manager.yaml /opt/musicbox/manager.yaml
fi

if [ "$#" -gt 0 ]; then
    exec "$@"
fi

exec /usr/local/bin/musicbox serve
