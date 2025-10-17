#!/bin/bash

# Start EventMesh server with simple configuration

set -e

# Get script directory and calculate paths relative to it
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
EVENTMESH="$PROJECT_ROOT/bin/eventmesh"

echo "Starting EventMesh server..."
echo ""
echo "Once running, try:"
echo "  $SCRIPT_DIR/publisher.sh    (in another terminal)"
echo "  $SCRIPT_DIR/subscriber.sh   (in another terminal)"
echo ""
echo "Press Ctrl+C to stop"
echo ""

# Check if binary exists
if [ ! -f "$EVENTMESH" ]; then
    echo "Error: EventMesh binary not found at $EVENTMESH"
    echo "Please run 'make build' from project root: $PROJECT_ROOT"
    exit 1
fi

# Start server
$EVENTMESH \
    --http \
    --http-port 8081 \
    --http-secret "simple-demo" \
    --node-id "simple-node" \
    --listen ":8082" \
    --peer-listen ":8083"