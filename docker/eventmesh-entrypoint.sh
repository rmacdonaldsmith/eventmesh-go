#!/bin/sh
set -eu

# If arguments are provided, run them as the eventmesh command arguments. This
# keeps the image useful for one-off flag-driven local runs, while the default
# path below supports environment-driven container configuration.
if [ "$#" -gt 0 ]; then
	exec /usr/local/bin/eventmesh "$@"
fi

is_true() {
	case "${1:-}" in
		1|true|TRUE|yes|YES|on|ON) return 0 ;;
		*) return 1 ;;
	esac
}

NODE_ID="${EVENTMESH_NODE_ID:-eventmesh-node}"
LISTEN="${EVENTMESH_LISTEN:-:8080}"
PEER_LISTEN="${EVENTMESH_PEER_LISTEN:-:9090}"
HTTP_PORT="${EVENTMESH_HTTP_PORT:-8081}"
LOG_LEVEL="${EVENTMESH_LOG_LEVEL:-info}"
EVENTLOG_BACKEND="${EVENTMESH_EVENTLOG_BACKEND:-memory}"
EVENTLOG_PATH="${EVENTMESH_EVENTLOG_PATH:-}"
SEED_NODES="${EVENTMESH_SEED_NODES:-}"
CONNECT_PEER="${EVENTMESH_CONNECT_PEER:-}"

set -- \
	--node-id "$NODE_ID" \
	--listen "$LISTEN" \
	--peer-listen "$PEER_LISTEN" \
	--log-level "$LOG_LEVEL" \
	--eventlog-backend "$EVENTLOG_BACKEND"

if is_true "${EVENTMESH_HTTP_ENABLED:-true}"; then
	set -- "$@" --http --http-port "$HTTP_PORT"
fi

if is_true "${EVENTMESH_NO_AUTH:-true}"; then
	set -- "$@" --no-auth
fi

if [ -n "$EVENTLOG_PATH" ]; then
	set -- "$@" --eventlog-path "$EVENTLOG_PATH"
fi

if [ -n "$SEED_NODES" ]; then
	set -- "$@" --seed-nodes "$SEED_NODES"
fi

if [ -n "$CONNECT_PEER" ]; then
	set -- "$@" --connect-peer "$CONNECT_PEER"
fi

exec /usr/local/bin/eventmesh "$@"
