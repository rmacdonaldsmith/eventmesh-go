#!/bin/bash

# EventMesh Replay Demo - demonstrates offset-based event reading
#
# PREREQUISITES:
# 1. Run 'make build' from project root to build binaries
# 2. Start EventMesh server: examples/simple/start-server.sh (in another terminal)
#    OR for easier testing: ../../bin/eventmesh --http --no-auth
# 3. Run this script from anywhere: ./examples/simple/replay-demo.sh
#
# This script demonstrates:
# - Publishing events to a topic
# - Reading historical events from specific offsets
# - Using the CLI replay and topics info commands
# - Practical use cases: checkpoint processing, debugging, data recovery
# - Development workflow with --no-auth mode

set -e

# Get script directory and calculate paths relative to it
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
CLI="$PROJECT_ROOT/bin/eventmesh-cli"
SERVER_SCRIPT="$SCRIPT_DIR/start-server.sh"
SERVER="http://localhost:8081"
CLIENT_ID="replay-demo"

echo "EventMesh Replay Demo"
echo "===================="
echo ""
echo "This demo shows offset-based event replay capabilities."
echo "Prerequisites:"
echo "  1. EventMesh binaries built (make build)"
echo "  2. EventMesh server running ($SERVER_SCRIPT)"
echo ""

# Check CLI exists
echo "🔍 Checking for CLI binary..."
if [ ! -f "$CLI" ]; then
    echo "❌ Error: CLI binary not found at $CLI"
    echo ""
    echo "To fix this:"
    echo "  1. Navigate to project root: cd $PROJECT_ROOT"
    echo "  2. Build the project: make build"
    echo "  3. Run this script again from anywhere"
    exit 1
fi
echo "✅ CLI binary found"

# Check server is running and detect no-auth mode
echo "🔍 Checking server connection..."
NO_AUTH_MODE=false

# Try with no-auth first (common in development)
if $CLI --no-auth health --server "$SERVER" > /dev/null 2>&1; then
    echo "✅ Server is running in NO-AUTH mode (development)"
    NO_AUTH_MODE=true
elif ! $CLI health --server "$SERVER" --client-id health > /dev/null 2>&1; then
    echo "❌ Error: Cannot connect to EventMesh server at $SERVER"
    echo ""
    echo "To fix this:"
    echo "  1. Open a new terminal"
    echo "  2. Start the server with one of these commands:"
    echo "     • Regular mode: $SERVER_SCRIPT"
    echo "     • Development mode: ../../bin/eventmesh --http --no-auth"
    echo "  3. Wait for startup message and run this script again"
    exit 1
else
    echo "✅ Server is running with authentication enabled"
fi

# Set authentication flags based on server mode
if [ "$NO_AUTH_MODE" = true ]; then
    echo "🔓 Using no-auth mode - skipping authentication"
    AUTH_FLAGS="--no-auth"
    TOKEN=""
else
    echo "🔐 Authenticating with server..."
    AUTH_OUTPUT=$($CLI auth --server "$SERVER" --client-id "$CLIENT_ID")
    echo "$AUTH_OUTPUT"

    # Extract token from output
    TOKEN=$(echo "$AUTH_OUTPUT" | grep "Token:" | awk '{print $2}')

    if [ -z "$TOKEN" ]; then
        echo "Error: Failed to get authentication token"
        exit 1
    fi

    AUTH_FLAGS="--client-id $CLIENT_ID --token $TOKEN"
fi

echo ""
echo "📚 STEP 1: Publishing some historical events..."

# Publish several events to create history
for i in {0..4}; do
    echo "Publishing event $i..."
    $CLI publish --server "$SERVER" $AUTH_FLAGS \
        --topic "demo.events" \
        --payload "{\"eventNumber\": $i, \"message\": \"This is event number $i\", \"timestamp\": \"$(date -Iseconds)\"}"
    sleep 0.5
done

echo ""
echo "📊 STEP 2: Inspect topic metadata..."

$CLI topics info --server "$SERVER" $AUTH_FLAGS \
    --topic "demo.events"

echo ""
echo "🔄 STEP 3: Replay events from the beginning..."

$CLI replay --server "$SERVER" $AUTH_FLAGS \
    --topic "demo.events" --offset 0 --limit 10 --pretty

echo ""
echo "🔄 STEP 4: Replay only the last 2 events..."

$CLI replay --server "$SERVER" $AUTH_FLAGS \
    --topic "demo.events" --offset 3 --limit 2 --pretty

echo ""
echo "📊 STEP 5: Compare with real-time streaming..."
echo "Starting a 5-second stream to show the difference..."

# Start streaming in background for 5 seconds
timeout 5s $CLI stream --server "$SERVER" $AUTH_FLAGS \
    --topic "demo.events" &
STREAM_PID=$!

sleep 1

# Publish one more event during streaming
echo "Publishing new real-time event..."
$CLI publish --server "$SERVER" $AUTH_FLAGS \
    --topic "demo.events" \
    --payload "{\"eventNumber\": 5, \"message\": \"This is a new real-time event\", \"timestamp\": \"$(date -Iseconds)\"}"

# Wait for stream to finish
wait $STREAM_PID 2>/dev/null || true

echo ""
echo "🔄 STEP 6: Now replay everything including the new event..."

$CLI replay --server "$SERVER" $AUTH_FLAGS \
    --topic "demo.events" --offset 0 --limit 10 --pretty

echo ""
echo "🏗️  STEP 7: Practical Use Case Examples..."
echo ""

echo "📈 Use Case 1: Checkpoint-based Processing"
echo "Simulating processing events with checkpoints..."
CHECKPOINT_OFFSET=2
echo "• Processing from checkpoint offset: $CHECKPOINT_OFFSET"
$CLI replay --server "$SERVER" $AUTH_FLAGS \
    --topic "demo.events" --offset "$CHECKPOINT_OFFSET" --limit 3
echo "• Would save new checkpoint at offset: 5"

echo ""
echo "🔍 Use Case 2: Debugging - Find Specific Event"
echo "Looking for event with specific content..."
echo "• Searching for event number 3 in recent history"
$CLI replay --server "$SERVER" $AUTH_FLAGS \
    --topic "demo.events" --offset 0 --limit 10 | grep "eventNumber.*3" || echo "Event found in replay output above"

echo ""
echo "🔄 Use Case 3: Data Recovery - Replay After Downtime"
echo "Simulating recovery from known good state..."
echo "• System was last healthy at offset 1"
echo "• Replaying all events since then for recovery:"
$CLI replay --server "$SERVER" $AUTH_FLAGS \
    --topic "demo.events" --offset 1 --limit 10

echo ""
echo "✅ Demo Complete!"
echo ""
echo "💡 Key Takeaways:"
echo "   - 'replay' fetches historical events as a batch and exits"
echo "   - 'stream' subscribes to real-time events and keeps running"
echo "   - 'topics info' helps you understand available offsets"
echo "   - Offsets enable checkpoint processing, debugging, and data recovery"
echo "   - Use --no-auth mode for faster development iteration"
if [ "$NO_AUTH_MODE" = true ]; then
echo "   - Currently running in NO-AUTH mode (development only!)"
fi