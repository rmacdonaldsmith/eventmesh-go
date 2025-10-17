#!/bin/bash

# EventMesh Replay Demo - demonstrates offset-based event reading
#
# PREREQUISITES:
# 1. Run 'make build' from project root to build binaries
# 2. Start EventMesh server: ./start-server.sh (in another terminal)
# 3. Run this script: ./replay-demo.sh
#
# This script demonstrates:
# - Publishing events to a topic
# - Reading historical events from specific offsets
# - Using the CLI replay and topics info commands

set -e

CLI="../../bin/eventmesh-cli"
SERVER="http://localhost:8081"
CLIENT_ID="replay-demo"

echo "EventMesh Replay Demo"
echo "===================="
echo ""
echo "This demo shows offset-based event replay capabilities."
echo "Prerequisites:"
echo "  1. EventMesh binaries built (make build)"
echo "  2. EventMesh server running (./start-server.sh)"
echo ""

# Check CLI exists
echo "🔍 Checking for CLI binary..."
if [ ! -f "$CLI" ]; then
    echo "❌ Error: CLI binary not found at $CLI"
    echo ""
    echo "To fix this:"
    echo "  1. Navigate to project root: cd ../.."
    echo "  2. Build the project: make build"
    echo "  3. Return here: cd examples/simple"
    echo "  4. Run this script again: ./replay-demo.sh"
    exit 1
fi
echo "✅ CLI binary found"

# Check server is running
echo "🔍 Checking server connection..."
if ! $CLI health --server "$SERVER" --client-id health > /dev/null 2>&1; then
    echo "❌ Error: Cannot connect to EventMesh server at $SERVER"
    echo ""
    echo "To fix this:"
    echo "  1. Open a new terminal"
    echo "  2. Navigate to: cd examples/simple"
    echo "  3. Start the server: ./start-server.sh"
    echo "  4. Wait for 'HTTP server listening on :8081' message"
    echo "  5. Return to this terminal and run: ./replay-demo.sh"
    echo ""
    echo "Server startup command: ./start-server.sh"
    exit 1
fi
echo "✅ Server is running and accessible"

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

echo ""
echo "📚 STEP 1: Publishing some historical events..."

# Publish several events to create history
for i in {0..4}; do
    echo "Publishing event $i..."
    $CLI publish --server "$SERVER" --client-id "$CLIENT_ID" --token "$TOKEN" \
        --topic "demo.events" \
        --payload "{\"eventNumber\": $i, \"message\": \"This is event number $i\", \"timestamp\": \"$(date -Iseconds)\"}"
    sleep 0.5
done

echo ""
echo "📊 STEP 2: Inspect topic metadata..."

$CLI topics info --server "$SERVER" --client-id "$CLIENT_ID" --token "$TOKEN" \
    --topic "demo.events"

echo ""
echo "🔄 STEP 3: Replay events from the beginning..."

$CLI replay --server "$SERVER" --client-id "$CLIENT_ID" --token "$TOKEN" \
    --topic "demo.events" --offset 0 --limit 10 --pretty

echo ""
echo "🔄 STEP 4: Replay only the last 2 events..."

$CLI replay --server "$SERVER" --client-id "$CLIENT_ID" --token "$TOKEN" \
    --topic "demo.events" --offset 3 --limit 2 --pretty

echo ""
echo "📊 STEP 5: Compare with real-time streaming..."
echo "Starting a 5-second stream to show the difference..."

# Start streaming in background for 5 seconds
timeout 5s $CLI stream --server "$SERVER" --client-id "$CLIENT_ID" --token "$TOKEN" \
    --topic "demo.events" &
STREAM_PID=$!

sleep 1

# Publish one more event during streaming
echo "Publishing new real-time event..."
$CLI publish --server "$SERVER" --client-id "$CLIENT_ID" --token "$TOKEN" \
    --topic "demo.events" \
    --payload "{\"eventNumber\": 5, \"message\": \"This is a new real-time event\", \"timestamp\": \"$(date -Iseconds)\"}"

# Wait for stream to finish
wait $STREAM_PID 2>/dev/null || true

echo ""
echo "🔄 STEP 6: Now replay everything including the new event..."

$CLI replay --server "$SERVER" --client-id "$CLIENT_ID" --token "$TOKEN" \
    --topic "demo.events" --offset 0 --limit 10 --pretty

echo ""
echo "✅ Demo Complete!"
echo ""
echo "💡 Key Takeaways:"
echo "   - 'replay' fetches historical events as a batch and exits"
echo "   - 'stream' subscribes to real-time events and keeps running"
echo "   - 'topics info' helps you understand available offsets"
echo "   - Offsets let you start reading from any point in the event history"