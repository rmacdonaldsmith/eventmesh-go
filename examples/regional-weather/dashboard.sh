#!/usr/bin/env bash
set -euo pipefail

if ! command -v tmux >/dev/null 2>&1; then
  echo "tmux is required for dashboard.sh. Run the scripts manually instead:"
  echo "  ./start.sh"
  echo "  ./subscribe-analytics.sh"
  echo "  ./subscribe-alerts.sh"
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

tmux new-session -d -s "$SESSION" -c "$DEMO_DIR" \
  "$(labeled_command 'NODES + HUB LOG' './start.sh; echo; echo "Nodes started. Press Ctrl+C here or run ./stop.sh to stop nodes."; tail -f .run/logs/hub.log')"
tmux split-window -h -t "$SESSION:0" -c "$DEMO_DIR" \
  "$(labeled_command 'ANALYTICS SUBSCRIBER: weather.*' './subscribe-analytics.sh')"
tmux split-window -v -t "$SESSION:0.1" -c "$DEMO_DIR" \
  "$(labeled_command 'ALERT SUBSCRIBER: alert.*' './subscribe-alerts.sh')"
tmux select-pane -t "$SESSION:0.0"
tmux split-window -v -t "$SESSION:0.0" -c "$DEMO_DIR" \
  "$(labeled_command 'WEATHER GENERATOR: city nodes publish events' 'sleep 3; ./generate-weather.py')"
tmux select-pane -t "$SESSION:0.2"
tmux split-window -v -t "$SESSION:0.2" -c "$DEMO_DIR" \
  "$(labeled_command 'MESH STATS: subscriptions + peer interest' 'sleep 3; ./stats.py')"

tmux select-layout -t "$SESSION:0" tiled

# Show pane titles in borders when the installed tmux supports pane-border-status.
tmux set-option -t "$SESSION" pane-border-status top >/dev/null 2>&1 || true
tmux set-option -t "$SESSION" pane-border-format ' #{pane_index}: #{pane_title} ' >/dev/null 2>&1 || true

tmux attach -t "$SESSION"
