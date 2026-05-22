#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
RUN_DIR="$ROOT_DIR/examples/regional-weather/.run"
PID_FILE="$RUN_DIR/pids"

if [[ ! -f "$PID_FILE" ]]; then
  echo "No regional weather demo PID file found."
  exit 0
fi

while read -r pid name; do
  [[ -z "${pid:-}" ]] && continue
  if kill -0 "$pid" 2>/dev/null; then
    echo "Stopping $name ($pid)"
    kill "$pid" 2>/dev/null || true
  fi
done < "$PID_FILE"

sleep 1
while read -r pid name; do
  [[ -z "${pid:-}" ]] && continue
  if kill -0 "$pid" 2>/dev/null; then
    echo "Force stopping $name ($pid)"
    kill -9 "$pid" 2>/dev/null || true
  fi
done < "$PID_FILE"

rm -f "$PID_FILE"
echo "Regional weather demo stopped."
