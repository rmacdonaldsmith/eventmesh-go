#!/usr/bin/env bash
set -euo pipefail

DEMO_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "$DEMO_DIR/../.." && pwd)"
COMPOSE_FILE="$DEMO_DIR/docker-compose.yml"
PROJECT_NAME="eventmesh-weather"

compose() {
  docker compose -p "$PROJECT_NAME" -f "$COMPOSE_FILE" "$@"
}

compose_with_chicago() {
  docker compose -p "$PROJECT_NAME" -f "$COMPOSE_FILE" --profile chicago "$@"
}

city_server_url() {
  case "${1:-}" in
    hub) echo "http://localhost:8081" ;;
    sf) echo "http://localhost:8082" ;;
    ny) echo "http://localhost:8083" ;;
    chicago) echo "http://localhost:8084" ;;
    *)
      echo "unknown city/node: ${1:-}" >&2
      echo "expected one of: hub, sf, ny, chicago" >&2
      return 1
      ;;
  esac
}

city_topic() {
  case "${1:-}" in
    sf|ny|chicago) echo "weather.$1" ;;
    *)
      echo "unknown city: ${1:-}" >&2
      echo "expected one of: sf, ny, chicago" >&2
      return 1
      ;;
  esac
}
