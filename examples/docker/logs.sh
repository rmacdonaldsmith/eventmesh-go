#!/usr/bin/env bash
set -euo pipefail

source "$(dirname "${BASH_SOURCE[0]}")/common.sh"

service="${1:-all}"
case "$service" in
  all) compose_with_chicago logs -f ;;
  hub|sf|ny|chicago) compose_with_chicago logs -f "$service" ;;
  *)
    echo "usage: ./logs.sh [all|hub|sf|ny|chicago]" >&2
    exit 1
    ;;
esac
