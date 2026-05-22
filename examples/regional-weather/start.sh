#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
RUN_DIR="$ROOT_DIR/examples/regional-weather/.run"
LOG_DIR="$RUN_DIR/logs"
BIN="$ROOT_DIR/bin/eventmesh"

mkdir -p "$LOG_DIR"

if [[ ! -x "$BIN" ]]; then
  echo "Building eventmesh server..."
  (cd "$ROOT_DIR" && make build-server)
fi

if [[ -f "$RUN_DIR/pids" ]]; then
  echo "Existing demo PID file found. Run ./stop.sh first if the demo is already running."
fi
: > "$RUN_DIR/pids"

start_node() {
  local node_id="$1"
  local http_port="$2"
  local peer_addr="$3"
  shift 3

  echo "Starting $node_id: http=:$http_port peer=$peer_addr"
  "$BIN" \
    --node-id "$node_id" \
    --listen "127.0.0.1:0" \
    --peer-listen "$peer_addr" \
    --http \
    --http-port "$http_port" \
    --no-auth \
    --log-level warn \
    "$@" > "$LOG_DIR/$node_id.log" 2>&1 &
  echo "$! $node_id" >> "$RUN_DIR/pids"
}

start_node sf 8082 127.0.0.1:9101
start_node ny 8083 127.0.0.1:9102
start_node chicago 8084 127.0.0.1:9103
sleep 1
start_node hub 8081 127.0.0.1:9100 \
  --connect-peer sf=127.0.0.1:9101,ny=127.0.0.1:9102,chicago=127.0.0.1:9103

sleep 2
cat <<MSG

Regional weather mesh is running.

Nodes:
  hub      http://localhost:8081 peer 127.0.0.1:9100
  sf       http://localhost:8082 peer 127.0.0.1:9101
  ny       http://localhost:8083 peer 127.0.0.1:9102
  chicago  http://localhost:8084 peer 127.0.0.1:9103

Next terminals:
  ./subscribe-analytics.sh   # weather.* from the hub
  ./subscribe-alerts.sh      # alert.* from the hub
  ./generate-weather.py      # city nodes publish local weather
  ./stats.py                 # watch mesh interest/routing stats

Logs: $LOG_DIR
MSG
