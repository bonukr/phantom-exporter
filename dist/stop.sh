#!/usr/bin/env bash
# Stop the phantom-exporter background daemon.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")" && pwd)"
PID_FILE="$ROOT/phantom-exporter.pid"

if [[ ! -f "$PID_FILE" ]]; then
    echo "not running (no pid file)"
    exit 0
fi

pid="$(cat "$PID_FILE")"
if ! kill -0 "$pid" 2>/dev/null; then
    echo "not running (stale pid $pid)"
    rm -f "$PID_FILE" "$ROOT/phantom-exporter.env"
    exit 0
fi

kill "$pid" 2>/dev/null || true

for _ in $(seq 1 20); do
    if ! kill -0 "$pid" 2>/dev/null; then
        rm -f "$PID_FILE" "$ROOT/phantom-exporter.env"
        echo "stopped (pid $pid)"
        exit 0
    fi
    sleep 0.25
done

echo "force killing pid $pid"
kill -9 "$pid" 2>/dev/null || true
rm -f "$PID_FILE" "$ROOT/phantom-exporter.env"
echo "stopped (pid $pid)"
