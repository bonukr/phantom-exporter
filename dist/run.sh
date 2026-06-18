#!/usr/bin/env bash
# Start phantom-exporter as a background daemon.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")" && pwd)"
cd "$ROOT"

BIN="$ROOT/phantom-exporter"
PID_FILE="$ROOT/phantom-exporter.pid"
ENV_FILE="$ROOT/phantom-exporter.env"

H2H_PORT="${H2H_PORT:-8088}"
H2H_SETTINGS_DIR="${H2H_SETTINGS_DIR:-$ROOT/settings}"
H2H_LOG_FILE="${H2H_LOG_FILE:-$ROOT/logs/phantom-exporter.log}"
H2H_LOG_LEVEL="${H2H_LOG_LEVEL:-info}"

if [[ ! -x "$BIN" ]]; then
    echo "error: $BIN not found. Run ../dist.sh from the project root first." >&2
    exit 1
fi

if [[ -f "$PID_FILE" ]]; then
    pid="$(cat "$PID_FILE")"
    if kill -0 "$pid" 2>/dev/null; then
        echo "already running (pid $pid)"
        exit 0
    fi
    rm -f "$PID_FILE"
fi

mkdir -p "$ROOT/logs" "$H2H_SETTINGS_DIR"

export H2H_PORT H2H_SETTINGS_DIR H2H_LOG_FILE H2H_LOG_LEVEL

cat >"$ENV_FILE" <<EOF
H2H_PORT=$H2H_PORT
H2H_SETTINGS_DIR=$H2H_SETTINGS_DIR
H2H_LOG_FILE=$H2H_LOG_FILE
H2H_LOG_LEVEL=$H2H_LOG_LEVEL
EOF

nohup "$BIN" >>"$ROOT/logs/nohup.log" 2>&1 &
echo $! >"$PID_FILE"

sleep 0.3
pid="$(cat "$PID_FILE")"
if kill -0 "$pid" 2>/dev/null; then
    echo "started (pid $pid)"
    echo "  listen  : http://127.0.0.1:${H2H_PORT}/"
    echo "  settings: ${H2H_SETTINGS_DIR}"
    echo "  log     : ${H2H_LOG_FILE}"
else
    echo "error: failed to start. See $ROOT/logs/nohup.log" >&2
    rm -f "$PID_FILE" "$ENV_FILE"
    exit 1
fi
