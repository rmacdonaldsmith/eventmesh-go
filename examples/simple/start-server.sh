#!/bin/bash

# Start EventMesh server with simple configuration

set -e

echo "Starting EventMesh server..."
echo ""
echo "Once running, try:"
echo "  ./publisher.sh    (in another terminal)"
echo "  ./subscriber.sh   (in another terminal)"
echo ""
echo "Press Ctrl+C to stop"
echo ""

# Check if binary exists
if [ ! -f "../../bin/eventmesh" ]; then
    echo "Error: Please run 'make build' from project root first"
    exit 1
fi

# Start server
../../bin/eventmesh \
    --http \
    --http-port 8081 \
    --http-secret "simple-demo" \
    --node-id "simple-node" \
    --listen ":8082" \
    --peer-listen ":8083"