#!/bin/bash

# Simple EventMesh subscriber

set -e

# Get script directory and calculate paths relative to it
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
CLI="$PROJECT_ROOT/bin/eventmesh-cli"
SERVER="http://localhost:8091"
CLIENT_ID="subscriber"

echo "EventMesh Subscriber"
echo "==================="

# Check CLI exists
if [ ! -f "$CLI" ]; then
    echo "Error: CLI binary not found at $CLI"
    echo "Please run 'make build' from project root: $PROJECT_ROOT"
    exit 1
fi

# Authenticate and capture token
echo "Authenticating..."
AUTH_OUTPUT=$($CLI auth --server "$SERVER" --client-id "$CLIENT_ID")
echo "$AUTH_OUTPUT"

# Extract token from output
TOKEN=$(echo "$AUTH_OUTPUT" | grep "Token:" | awk '{print $2}')

if [ -z "$TOKEN" ]; then
    echo "Error: Failed to get authentication token"
    exit 1
fi

# Subscribe to news topics (using wildcard pattern)
echo "Creating subscription to 'news.*' topics..."
$CLI subscribe --server "$SERVER" --client-id "$CLIENT_ID" --token "$TOKEN" --topic "news.*"

echo ""
echo "Streaming events (Press Ctrl+C to stop):"
echo "Run ./publisher.sh in another terminal to see events here"
echo ""
echo "Note: Streaming from news.sports topic"
echo "(Subscription covers news.* pattern, but streaming requires specific topics)"
echo ""

# Stream from one specific topic to avoid output conflicts
# The subscription will still catch events from all news.* topics
$CLI stream --server "$SERVER" --client-id "$CLIENT_ID" --token "$TOKEN" --topic "news.sports"