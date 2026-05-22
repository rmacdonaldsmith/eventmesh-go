#!/usr/bin/env bash
set -euo pipefail

if ! command -v tmux >/dev/null 2>&1; then
  echo "tmux is required for dashboard.sh. Run the scripts manually instead:"
  echo "  ./start.sh"
  echo "  ./subscribe-analytics.sh"
  echo "  ./subscribe-alerts.sh"
  echo "  ./subscribe-city.sh sf"
  echo "  ./subscribe-city.sh ny"
  echo "  ./subscribe-city.sh chicago"
  echo "  ./generate-weather.py"
  echo "  ./stats.py"
  exit 1
fi

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
DEMO_DIR="$ROOT_DIR/examples/regional-weather"
SESSION="eventmesh-weather"

tmux has-session -t "$SESSION" 2>/dev/null && tmux kill-session -t "$SESSION"

labeled_command() {
  local title="$1"
  shift
  printf 'printf "\\033]2;%s\\033\\\\"; clear; printf "==== %s ====\\n\\n"; %s' "$title" "$title" "$*"
}

# Window 1: mesh processes and metrics.
nodes_pane=$(tmux new-session -d -s "$SESSION" -n overview -c "$DEMO_DIR" -P -F '#{pane_id}' \
  "$(labeled_command 'NODES + HUB LOG' './start.sh; echo; echo "Nodes started. Press Ctrl+C here or run ./stop.sh to stop nodes."; tail -f .run/logs/hub.log')")
generator_pane=$(tmux split-window -h -t "$nodes_pane" -c "$DEMO_DIR" -P -F '#{pane_id}' \
  "$(labeled_command 'WEATHER GENERATOR: city nodes publish events' 'sleep 3; ./generate-weather.py')")
data_pane=$(tmux split-window -v -t "$nodes_pane" -c "$DEMO_DIR" -P -F '#{pane_id}' \
  "$(labeled_command 'DATA PLANE METRICS: event flow + SSE' 'sleep 3; ./stats.py --view data')")
tmux split-window -v -t "$generator_pane" -c "$DEMO_DIR" -P -F '#{pane_id}' \
  "$(labeled_command 'CONTROL PLANE METRICS: peer interest gossip' 'sleep 3; ./stats.py --view control')" >/dev/null
tmux select-layout -t "$SESSION:overview" tiled

# Window 2: subscribers/consumers. Includes one pane per city-specific consumer.
analytics_pane=$(tmux new-window -t "$SESSION" -n subscribers -c "$DEMO_DIR" -P -F '#{pane_id}' \
  "$(labeled_command 'ANALYTICS SUBSCRIBER: weather.*' './subscribe-analytics.sh')")
alerts_pane=$(tmux split-window -h -t "$analytics_pane" -c "$DEMO_DIR" -P -F '#{pane_id}' \
  "$(labeled_command 'ALERT SUBSCRIBER: alert.*' './subscribe-alerts.sh')")
sf_pane=$(tmux split-window -v -t "$analytics_pane" -c "$DEMO_DIR" -P -F '#{pane_id}' \
  "$(labeled_command 'SF DASHBOARD: weather.sf' './subscribe-city.sh sf')")
ny_pane=$(tmux split-window -v -t "$alerts_pane" -c "$DEMO_DIR" -P -F '#{pane_id}' \
  "$(labeled_command 'NY DASHBOARD: weather.ny' './subscribe-city.sh ny')")
tmux split-window -v -t "$ny_pane" -c "$DEMO_DIR" -P -F '#{pane_id}' \
  "$(labeled_command 'CHICAGO DASHBOARD: weather.chicago' './subscribe-city.sh chicago')" >/dev/null
tmux select-layout -t "$SESSION:subscribers" tiled

# Show pane titles in borders when the installed tmux supports pane-border-status.
tmux set-option -t "$SESSION" pane-border-status top >/dev/null 2>&1 || true
tmux set-option -t "$SESSION" pane-border-format ' #{pane_index}: #{pane_title} ' >/dev/null 2>&1 || true

tmux select-window -t "$SESSION:subscribers"
tmux select-pane -t "$sf_pane"
tmux attach -t "$SESSION"
