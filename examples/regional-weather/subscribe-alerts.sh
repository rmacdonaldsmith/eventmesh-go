#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
CLI="$ROOT_DIR/bin/eventmesh-cli"

if [[ ! -x "$CLI" ]]; then
  echo "Building eventmesh CLI..."
  (cd "$ROOT_DIR" && make build-cli)
fi

exec "$CLI" \
  --server http://localhost:8081 \
  --client-id alerting-service \
  --no-auth \
  stream --topic alert.*
