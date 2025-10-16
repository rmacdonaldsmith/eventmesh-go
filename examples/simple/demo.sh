#!/bin/bash

# Simple all-in-one EventMesh demo

set -e

CLI="../../bin/eventmesh-cli"

echo "EventMesh Simple Demo"
echo "===================="

# Check CLI exists
if [ ! -f "$CLI" ]; then
    echo "Error: Please run 'make build' from project root first"
    exit 1
fi

# Check server is running
echo "Checking server..."
if ! $CLI health --server http://localhost:8081 --client-id health > /dev/null 2>&1; then
    echo "Error: Server not running. Start it first with:"
    echo "  ./start-server.sh"
    exit 1
fi

# Setup subscriber
echo "Setting up subscriber..."
$CLI auth --server http://localhost:8081 --client-id subscriber
$CLI subscribe --server http://localhost:8081 --client-id subscriber --topic "news.*"

# Setup publisher
echo "Setting up publisher..."
$CLI auth --server http://localhost:8081 --client-id publisher

echo ""
echo "Starting 5-second event stream..."

# Start streaming in background
$CLI stream --server http://localhost:8081 --client-id subscriber --topic "news.*" &
STREAM_PID=$!

sleep 2

# Publish events
echo "Publishing events..."
$CLI publish --server http://localhost:8081 --client-id publisher \
    --topic "news.sports" --payload '{"headline": "Game tonight!"}'

sleep 1

$CLI publish --server http://localhost:8081 --client-id publisher \
    --topic "news.weather" --payload '{"forecast": "Sunny day ahead"}'

sleep 2

# Stop streaming
kill $STREAM_PID 2>/dev/null || true

echo ""
echo "Demo complete! You should see the published events above."