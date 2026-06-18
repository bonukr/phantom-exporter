#!/usr/bin/env bash
# Build phantom-exporter and assemble ./dist/ for console deployment.
#
# Usage:
#   ./dist.sh            # native binary -> dist/phantom-exporter
#   ./dist.sh --amd64    # native + linux/amd64 -> dist/phantom-exporter-amd64
set -euo pipefail

ROOT="$(cd "$(dirname "$0")" && pwd)"
cd "$ROOT"

DIST="$ROOT/dist"
BUILD_AMD64=0

usage() {
    cat <<'EOF'
Usage: ./dist.sh [OPTIONS]

Options:
  --amd64    Also build linux/amd64 binary (dist/phantom-exporter-amd64)
  -h, --help Show this help
EOF
}

for arg in "$@"; do
    case "$arg" in
        --amd64) BUILD_AMD64=1 ;;
        -h|--help) usage; exit 0 ;;
        *) echo "error: unknown option: $arg" >&2; usage >&2; exit 1 ;;
    esac
done

build_binary() {
    local out="$1"
    local goos="${2:-}"
    local goarch="${3:-}"

    if [[ -n "$goos" && -n "$goarch" ]]; then
        echo "==> building phantom-exporter (${goos}/${goarch})"
        CGO_ENABLED=0 GOOS="$goos" GOARCH="$goarch" \
            go build -trimpath -ldflags="-s -w" -o "$out" ./cmd/exporter
    else
        echo "==> building phantom-exporter (native)"
        CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o "$out" ./cmd/exporter
    fi
}

build_binary "$DIST/phantom-exporter"
if [[ "$BUILD_AMD64" -eq 1 ]]; then
    build_binary "$DIST/phantom-exporter-amd64" linux amd64
fi

echo "==> preparing dist layout"
mkdir -p "$DIST/settings" "$DIST/web" "$DIST/logs"

echo "==> copying web assets"
for f in index.html style.css app.js; do
    cp "web/$f" "$DIST/web/$f"
done

echo "==> copying settings"
if compgen -G "dist/settings/*.yml" >/dev/null 2>&1 || \
   compgen -G "dist/settings/*.yaml" >/dev/null 2>&1; then
    echo "    kept existing dist/settings (not overwritten)"
elif compgen -G "settings/*.yml" >/dev/null 2>&1 || \
     compgen -G "settings/*.yaml" >/dev/null 2>&1; then
    cp settings/*.yml "$DIST/settings/" 2>/dev/null || true
    cp settings/*.yaml "$DIST/settings/" 2>/dev/null || true
    echo "    copied from ./settings"
else
    cp settings.example/*.yml "$DIST/settings/" 2>/dev/null || true
    cp settings.example/*.yaml "$DIST/settings/" 2>/dev/null || true
    echo "    copied from ./settings.example"
fi

chmod +x "$DIST/phantom-exporter" "$DIST/run.sh" "$DIST/stop.sh" "$DIST/status.sh"
[[ -f "$DIST/phantom-exporter-amd64" ]] && chmod +x "$DIST/phantom-exporter-amd64"

echo ""
echo "done:"
echo "  native : $DIST/phantom-exporter"
if [[ -f "$DIST/phantom-exporter-amd64" ]]; then
    echo "  amd64  : $DIST/phantom-exporter-amd64"
fi
echo "  run    : ./dist/run.sh"
echo "  stop   : ./dist/stop.sh"
echo "  status : ./dist/status.sh"
