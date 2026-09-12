#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
BUILD_DIR="${BUILD_DIR:-$ROOT_DIR/build}"
TARGET_ARCH="${TARGET_ARCH:-amd64}"
IMAGE_NAME="${IMAGE_NAME:-localhost/musicbox}"
SINGBOX_BIN="${SINGBOX_BIN:-$BUILD_DIR/sing-box}"
VERSION="${VERSION:-$("$SINGBOX_BIN" version 2>/dev/null | awk '{print $3; exit}' || true)}"
VERSION="${VERSION:-latest}"

usage() {
    cat <<EOF
Usage: TARGET_ARCH=amd64|arm64 $0 [binary|container|all]

  binary     Build both binaries: build/musicbox and build/musicbox-container
  container  Build both binaries, Rocky Linux image, and tar.gz archive
  all        Same as container (default)

Environment:
  TARGET_ARCH  Target architecture for builds (default: amd64)
  IMAGE_NAME   Container image name (default: localhost/musicbox)
  VERSION      Image/archive version (default: detected sing-box version)
  SINGBOX_BIN  sing-box runtime binary used in the image (default: build/sing-box)
EOF
}

log() { printf '[INFO] %s\n' "$*"; }
die() { printf '[ERROR] %s\n' "$*" >&2; exit 1; }
need_cmd() { command -v "$1" >/dev/null 2>&1 || die "$1 is required"; }

check_arch() {
    case "$TARGET_ARCH" in
        amd64|arm64) ;;
        *) die "TARGET_ARCH must be amd64 or arm64" ;;
    esac
}

git_value() {
    git -C "$ROOT_DIR" "$@" 2>/dev/null || printf 'unknown'
}

build_time="$(date -u '+%Y-%m-%dT%H:%M:%SZ')"
commit="$(git_value rev-parse --short HEAD)"

ensure_frontend() {
    if [ ! -f "$ROOT_DIR/web/dist/index.html" ] || [ ! -d "$ROOT_DIR/web/dist/assets" ]; then
        if command -v npm >/dev/null 2>&1 && [ -f "$ROOT_DIR/web/package.json" ]; then
            log "Building web frontend..."
            (
                cd "$ROOT_DIR/web"
                [ -d node_modules ] || npm install
                npm run build
            )
        else
            log "Warning: web frontend assets not found and npm not available; building with fallback placeholder"
        fi
    fi
}

ldflags() {
    printf -- '-s -w -X main.version=%s -X main.buildTime=%s -X main.gitCommit=%s' \
        "$VERSION" "$build_time" "$commit"
}

build_binaries() {
    need_cmd go
    check_arch
    ensure_frontend
    mkdir -p "$BUILD_DIR"
    local flags
    flags="$(ldflags)"

    log "Building MusicBox binary for linux/$TARGET_ARCH"
    (
        cd "$ROOT_DIR"
        CGO_ENABLED=0 GOOS=linux GOARCH="$TARGET_ARCH" \
            go build -trimpath \
            -ldflags "$flags" \
            -o "$BUILD_DIR/musicbox" .
    )
    chmod 0755 "$BUILD_DIR/musicbox"

    log "Building container MusicBox binary for linux/$TARGET_ARCH"
    (
        cd "$ROOT_DIR"
        CGO_ENABLED=0 GOOS=linux GOARCH="$TARGET_ARCH" \
            go build -tags container -trimpath \
            -ldflags "$flags" \
            -o "$BUILD_DIR/musicbox-container" .
    )
    chmod 0755 "$BUILD_DIR/musicbox-container"

    "$BUILD_DIR/musicbox" info
    log "Binaries: $BUILD_DIR/musicbox  $BUILD_DIR/musicbox-container"
}

check_container_inputs() {
    need_cmd docker
    check_arch
    [ -f "$SINGBOX_BIN" ] || die "sing-box binary not found: $SINGBOX_BIN"
    if [ "$SINGBOX_BIN" != "$BUILD_DIR/sing-box" ]; then
        mkdir -p "$BUILD_DIR"
        cp "$SINGBOX_BIN" "$BUILD_DIR/sing-box"
        chmod 0755 "$BUILD_DIR/sing-box"
    fi
    build_binaries
}

build_container() {
    check_container_inputs
    local tag="${IMAGE_NAME}:${VERSION}"
    local alt_image="localhost/musicbox-container"
    if [ "$IMAGE_NAME" = "localhost/musicbox-container" ]; then
        alt_image="localhost/musicbox"
    fi

    log "Building Rocky Linux container image $tag for linux/$TARGET_ARCH"
    local build_args=(
        --platform "linux/$TARGET_ARCH"
        --tag "$tag"
        --tag "${IMAGE_NAME}:latest"
        --tag "${alt_image}:${VERSION}"
        --tag "${alt_image}:latest"
    )
    docker build "${build_args[@]}" "$ROOT_DIR"

    local stamp out tmp
    stamp="$(date +%Y%m%d_%H%M%S)"
    out="$BUILD_DIR/musicbox-${VERSION}-${TARGET_ARCH}-${stamp}.tar.gz"
    tmp="${out}.partial"
    local tags=("$tag" "${IMAGE_NAME}:latest" "${alt_image}:${VERSION}" "${alt_image}:latest")
    log "Saving ${tags[*]} -> $out"
    docker save "${tags[@]}" | gzip -c >"$tmp"
    mv "$tmp" "$out"
    log "Container archive: $out ($(du -h "$out" | cut -f1))"
}

case "${1:-all}" in
    binary)
        build_binaries
        ;;
    container)
        build_container
        ;;
    all)
        build_container
        ;;
    -h|--help|help)
        usage
        ;;
    *)
        usage >&2
        exit 2
        ;;
esac
