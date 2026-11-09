#!/usr/bin/env bash
set -euo pipefail

source "$(dirname "${BASH_SOURCE[0]}")/common.sh"

interval="${1:-2}"
cities=(sf ny)
if compose_with_chicago ps --services --filter status=running | grep -qx chicago; then
  cities+=(chicago)
fi

publish() {
  local city="$1"
  local topic="$2"
  local payload="$3"
  compose exec -T "$city" eventmesh-cli --no-auth --server http://localhost:8081 publish \
    --topic "$topic" \
    --payload "$payload" >/dev/null
}

echo "Publishing synthetic weather events every ${interval}s. Ctrl+C to stop."
echo "Cities: ${cities[*]}"

count=0
while true; do
  count=$((count + 1))
  for city in "${cities[@]}"; do
    temp=$((55 + RANDOM % 45))
    humidity=$((35 + RANDOM % 60))
    wind=$((2 + RANDOM % 30))
    payload=$(printf '{"city":"%s","temperatureF":%d,"humidity":%d,"windMph":%d,"sequence":%d}' \
      "$city" "$temp" "$humidity" "$wind" "$count")
    publish "$city" "weather.$city" "$payload"
    printf '[%s] weather.%s temp=%sF humidity=%s%% wind=%smph seq=%s\n' \
      "$(date +%H:%M:%S)" "$city" "$temp" "$humidity" "$wind" "$count"

    if (( temp >= 92 || wind >= 26 )); then
      alert_payload=$(printf '{"city":"%s","temperatureF":%d,"windMph":%d,"severity":"watch","sequence":%d}' \
        "$city" "$temp" "$wind" "$count")
      publish "$city" "alert.$city" "$alert_payload"
      printf '[%s] alert.%s severity=watch temp=%sF wind=%smph seq=%s\n' \
        "$(date +%H:%M:%S)" "$city" "$temp" "$wind" "$count"
    fi
  done
  sleep "$interval"
done
