#!/usr/bin/env bash
set -euo pipefail

if ! command -v tmux >/dev/null 2>&1; then
  echo "tmux is required for dashboard.sh. Run the scripts manually instead:"
  echo "  ./start.sh"
  echo "  ./subscribe-weather.sh"
  echo "  ./subscribe-alerts.sh"
  echo "  ./generate-events.sh"
  echo "  ./stats.py --view control"
  echo "  ./logs.sh hub"
  exit 1
fi

DEMO_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SESSION="eventmesh-docker-weather"

tmux has-session -t "$SESSION" 2>/dev/null && tmux kill-session -t "$SESSION"

labeled_command() {
  local title="$1"
  shift
  printf 'printf "\\033]2;%s\\033\\\\"; clear; printf "==== %s ====\\n\\n"; %s' "$title" "$title" "$*"
}

nodes_pane=$(tmux new-session -d -s "$SESSION" -n mesh -c "$DEMO_DIR" -P -F '#{pane_id}' \
  "$(labeled_command 'START + HUB LOGS' './start.sh; ./logs.sh hub')")
sf_logs=$(tmux split-window -h -t "$nodes_pane" -c "$DEMO_DIR" -P -F '#{pane_id}' \
  "$(labeled_command 'SF NODE LOGS' 'sleep 4; ./logs.sh sf')")
ny_logs=$(tmux split-window -v -t "$sf_logs" -c "$DEMO_DIR" -P -F '#{pane_id}' \
  "$(labeled_command 'NY NODE LOGS' 'sleep 4; ./logs.sh ny')")
tmux split-window -v -t "$nodes_pane" -c "$DEMO_DIR" -P -F '#{pane_id}' \
  "$(labeled_command 'CONTROL / GOSSIP STATS' 'sleep 5; ./stats.py --view control')" >/dev/null
tmux select-layout -t "$SESSION:mesh" tiled

weather_pane=$(tmux new-window -t "$SESSION" -n streams -c "$DEMO_DIR" -P -F '#{pane_id}' \
  "$(labeled_command 'ANALYTICS: weather.* ON HUB' 'sleep 5; ./subscribe-weather.sh')")
alerts_pane=$(tmux split-window -h -t "$weather_pane" -c "$DEMO_DIR" -P -F '#{pane_id}' \
  "$(labeled_command 'ALERTING: alert.* ON HUB' 'sleep 5; ./subscribe-alerts.sh')")
sf_sub=$(tmux split-window -v -t "$weather_pane" -c "$DEMO_DIR" -P -F '#{pane_id}' \
  "$(labeled_command 'SF LOCAL DASHBOARD: weather.sf' 'sleep 5; ./subscribe-city.sh sf')")
tmux split-window -v -t "$alerts_pane" -c "$DEMO_DIR" -P -F '#{pane_id}' \
  "$(labeled_command 'GENERATOR: CITY NODES PUBLISH' 'sleep 8; ./generate-events.sh')" >/dev/null
tmux select-layout -t "$SESSION:streams" tiled

# Show pane titles when supported.
tmux set-option -t "$SESSION" pane-border-status top >/dev/null 2>&1 || true
tmux set-option -t "$SESSION" pane-border-format ' #{pane_index}: #{pane_title} ' >/dev/null 2>&1 || true

tmux select-window -t "$SESSION:streams"
tmux select-pane -t "$sf_sub"
tmux attach -t "$SESSION"
