#!/bin/bash

# Simple EventMesh subscriber

set -e

CLI="../../bin/eventmesh-cli"

echo "EventMesh Subscriber"
echo "==================="

# Check CLI exists
if [ ! -f "$CLI" ]; then
    echo "Error: Please run 'make build' from project root first"
    exit 1
fi

# Authenticate
echo "Authenticating..."
$CLI auth --server http://localhost:8081 --client-id subscriber

# Subscribe to news topics
echo "Creating subscription to 'news.*' topics..."
$CLI subscribe --server http://localhost:8081 --client-id subscriber --topic "news.*"

echo ""
echo "Streaming events (Press Ctrl+C to stop):"
echo "Run ./publisher.sh in another terminal to see events here"
echo ""

# Stream events
$CLI stream --server http://localhost:8081 --client-id subscriber --topic "news.*"