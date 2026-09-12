FROM rockylinux:9-minimal

LABEL maintainer="musicbox"
LABEL description="sing-box manager container without systemd (Rocky Linux)"

RUN microdnf -y install --setopt=install_weak_deps=0 \
        nftables \
        iproute \
        ca-certificates \
        procps-ng \
        bash \
        curl \
    && microdnf clean all \
    && rm -rf /var/cache/dnf

COPY build/sing-box /usr/local/bin/sing-box
COPY build/musicbox-container /usr/local/bin/musicbox
COPY container/defaults/config_generic.json /usr/share/sing-box/config_generic.json
COPY container/defaults/manager.yaml /usr/share/musicbox/manager.yaml
COPY container/scripts/entrypoint.sh /entrypoint.sh
COPY container/scripts/systemctl /usr/local/bin/systemctl

RUN chmod 0755 /usr/local/bin/sing-box /usr/local/bin/musicbox /usr/local/bin/systemctl /entrypoint.sh \
    && mkdir -p /opt/musicbox /etc/sing-box /var/lib/sing-box /run/musicbox

VOLUME ["/etc/sing-box", "/var/lib/sing-box", "/opt/musicbox"]

EXPOSE 8082 20080

HEALTHCHECK --interval=30s --timeout=5s --retries=3 \
    CMD curl -sf http://127.0.0.1:8082/api/status || exit 1

ENTRYPOINT ["/entrypoint.sh"]
