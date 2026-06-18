#!/usr/bin/env bash
# Show phantom-exporter daemon status.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")" && pwd)"
PID_FILE="$ROOT/phantom-exporter.pid"
ENV_FILE="$ROOT/phantom-exporter.env"

if [[ -f "$ENV_FILE" ]]; then
    # shellcheck disable=SC1090
    source "$ENV_FILE"
fi
H2H_PORT="${H2H_PORT:-8080}"

running=0
pid=""

if [[ -f "$PID_FILE" ]]; then
    pid="$(cat "$PID_FILE")"
    if kill -0 "$pid" 2>/dev/null; then
        running=1
    fi
fi

if [[ "$running" -eq 1 ]]; then
    echo "status : running"
    echo "pid    : $pid"
    echo "port   : $H2H_PORT"
    echo "url    : http://127.0.0.1:${H2H_PORT}/"
    if command -v curl >/dev/null 2>&1; then
        if resp="$(curl -fsS --max-time 2 "http://127.0.0.1:${H2H_PORT}/api/status" 2>/dev/null)"; then
            echo "api    : $resp"
        else
            echo "api    : (no response yet)"
        fi
    fi
    exit 0
fi

echo "status : stopped"
if [[ -n "$pid" ]]; then
    echo "note   : stale pid file ($pid)"
fi
exit 1
