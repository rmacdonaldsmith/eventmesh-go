#!/bin/bash

# Simple all-in-one EventMesh demo

set -e

# Get script directory and calculate paths relative to it
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
CLI="$PROJECT_ROOT/bin/eventmesh-cli"
SERVER_SCRIPT="$SCRIPT_DIR/start-server.sh"
SERVER="http://localhost:8081"

echo "EventMesh Simple Demo"
echo "===================="

# Check CLI exists
if [ ! -f "$CLI" ]; then
    echo "Error: CLI binary not found at $CLI"
    echo "Please run 'make build' from project root: $PROJECT_ROOT"
    exit 1
fi

# Check server is running
echo "Checking server..."
if ! $CLI health --server "$SERVER" --client-id health > /dev/null 2>&1; then
    echo "Error: Server not running. Start it first with:"
    echo "  $SERVER_SCRIPT"
    exit 1
fi

# Setup subscriber
echo "Setting up subscriber..."
SUB_AUTH=$($CLI auth --server "$SERVER" --client-id subscriber)
SUB_TOKEN=$(echo "$SUB_AUTH" | grep "Token:" | awk '{print $2}')

$CLI subscribe --server "$SERVER" --client-id subscriber --token "$SUB_TOKEN" --topic "news.*"

# Setup publisher
echo "Setting up publisher..."
PUB_AUTH=$($CLI auth --server "$SERVER" --client-id publisher)
PUB_TOKEN=$(echo "$PUB_AUTH" | grep "Token:" | awk '{print $2}')

echo ""
echo "Starting 5-second event stream..."

# Start streaming in background (using specific topic since wildcard streaming not supported yet)
$CLI stream --server "$SERVER" --client-id subscriber --token "$SUB_TOKEN" --topic "news.sports" &
STREAM_PID=$!

sleep 2

# Publish events
echo "Publishing events..."
$CLI publish --server "$SERVER" --client-id publisher --token "$PUB_TOKEN" \
    --topic "news.sports" --payload '{"headline": "Game tonight!"}'

sleep 1

$CLI publish --server "$SERVER" --client-id publisher --token "$PUB_TOKEN" \
    --topic "news.weather" --payload '{"forecast": "Sunny day ahead"}'

sleep 2

# Stop streaming
kill $STREAM_PID 2>/dev/null || true

echo ""
echo "Demo complete! You should see the published events above."