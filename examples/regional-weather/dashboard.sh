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

tmux new-session -d -s "$SESSION" -c "$DEMO_DIR" './start.sh; echo; echo "Nodes started. Press Ctrl+C here or run ./stop.sh to stop nodes."; tail -f .run/logs/hub.log'
tmux split-window -h -t "$SESSION:0" -c "$DEMO_DIR" './subscribe-analytics.sh'
tmux split-window -v -t "$SESSION:0.1" -c "$DEMO_DIR" './subscribe-alerts.sh'
tmux select-pane -t "$SESSION:0.0"
tmux split-window -v -t "$SESSION:0.0" -c "$DEMO_DIR" 'sleep 3; ./generate-weather.py'
tmux select-pane -t "$SESSION:0.2"
tmux split-window -v -t "$SESSION:0.2" -c "$DEMO_DIR" 'sleep 3; ./stats.py'
tmux select-layout -t "$SESSION:0" tiled

tmux attach -t "$SESSION"
