#!/usr/bin/env bash
set -euo pipefail

source "$(dirname "${BASH_SOURCE[0]}")/common.sh"

with_chicago=false
if [[ "${1:-}" == "--with-chicago" || "${1:-}" == "--profile=chicago" ]]; then
  with_chicago=true
elif [[ $# -gt 0 ]]; then
  echo "usage: ./start.sh [--with-chicago]" >&2
  exit 1
fi

if [[ "$with_chicago" == true ]]; then
  compose_with_chicago up -d --build
else
  compose up -d --build
fi

cat <<MSG

Docker weather mesh is running.

Nodes:
  hub      http://localhost:8081 peer localhost:9100
  sf       http://localhost:8082 peer localhost:9101
  ny       http://localhost:8083 peer localhost:9102
MSG
if [[ "$with_chicago" == true ]]; then
  echo "  chicago  http://localhost:8084 peer localhost:9103"
fi
cat <<'MSG'

Try these in separate terminals or tmux panes:
  ./subscribe-weather.sh       # analytics subscribes to weather.* on hub
  ./subscribe-alerts.sh        # alerting subscribes to alert.* on hub
  ./generate-events.sh         # city nodes publish weather/alert events
  ./stats.py --view control    # watch interest gossip and routing state
  ./logs.sh hub                # follow one node's logs

Stop with:
  ./stop.sh
MSG
