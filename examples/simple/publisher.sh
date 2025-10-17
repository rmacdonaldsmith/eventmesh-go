#!/bin/bash

# Simple EventMesh publisher

set -e

# Get script directory and calculate paths relative to it
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
CLI="$PROJECT_ROOT/bin/eventmesh-cli"
SERVER="http://localhost:8081"
CLIENT_ID="publisher"

echo "EventMesh Publisher"
echo "=================="

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

# Publish some events
echo ""
echo "Publishing events..."

echo "Publishing to news.sports (subscriber should see this)..."
$CLI publish --server "$SERVER" --client-id "$CLIENT_ID" --token "$TOKEN" \
    --topic "news.sports" \
    --payload '{"headline": "Local team wins championship!", "category": "sports"}'

sleep 1

echo "Publishing to news.weather..."
$CLI publish --server "$SERVER" --client-id "$CLIENT_ID" --token "$TOKEN" \
    --topic "news.weather" \
    --payload '{"forecast": "Sunny with high of 75F", "category": "weather"}'

sleep 1

echo "Publishing to news.tech..."
$CLI publish --server "$SERVER" --client-id "$CLIENT_ID" --token "$TOKEN" \
    --topic "news.tech" \
    --payload '{"headline": "New programming language released", "category": "tech"}'

echo ""
echo "Published 3 events!"
echo "Check the subscriber terminal to see them."