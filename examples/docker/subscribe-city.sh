#!/usr/bin/env bash
set -euo pipefail

source "$(dirname "${BASH_SOURCE[0]}")/common.sh"

city="${1:-}"
if [[ -z "$city" ]]; then
  echo "usage: ./subscribe-city.sh <sf|ny|chicago>" >&2
  exit 1
fi

topic="$(city_topic "$city")"
compose exec "$city" eventmesh-cli --no-auth --server http://localhost:8081 stream --topic "$topic"
