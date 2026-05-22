#!/usr/bin/env bash
set -euo pipefail

CITY="${1:-}"
case "$CITY" in
  sf|ny|chicago) ;;
  "") echo "usage: $0 <sf|ny|chicago>" >&2; exit 2 ;;
  *) echo "unknown city '$CITY' (expected sf, ny, or chicago)" >&2; exit 2 ;;
esac

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
CLI="$ROOT_DIR/bin/eventmesh-cli"

if [[ ! -x "$CLI" ]]; then
  echo "Building eventmesh CLI..."
  (cd "$ROOT_DIR" && make build-cli)
fi

exec "$CLI" \
  --server http://localhost:8081 \
  --client-id "$CITY-dashboard" \
  --no-auth \
  stream --topic "weather.$CITY"
