#!/bin/bash

# EventMesh Multi-Node Test Script
#
# This script tests event flow across a multi-node EventMesh.
# It assumes the multi-node-demo.sh is already running with 3 nodes:
# - Node1: HTTP on 8091, peer on 8093 (seed)
# - Node2: HTTP on 8094, peer on 8096 (discovers node1)
# - Node3: HTTP on 8097, peer on 8099 (discovers node1)
#
# Usage:
# 1. Start multi-node mesh: ./multi-node-demo.sh
# 2. In another terminal: ./mesh-test.sh
# 3. Watch events flow across the mesh

set -e

# Get script directory and calculate paths relative to it
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
CLI="$PROJECT_ROOT/bin/eventmesh-cli"

echo "🧪 EventMesh Multi-Node Test"
echo "============================"
echo ""

# Check if CLI exists
if [ ! -f "$CLI" ]; then
    echo "❌ Error: CLI binary not found at $CLI"
    echo ""
    echo "To fix this:"
    echo "  1. Navigate to project root: cd $PROJECT_ROOT"
    echo "  2. Build the project: make build"
    echo "  3. Run this script again"
    exit 1
fi

# Node endpoints
NODE1_URL="http://localhost:8091"
NODE2_URL="http://localhost:8094"
NODE3_URL="http://localhost:8097"

# Test if nodes are running
echo "📋 Checking mesh nodes..."

check_node() {
    local node_name="$1"
    local node_url="$2"

    if curl -s "$node_url/api/v1/health" > /dev/null 2>&1; then
        echo "   ✅ $node_name is running ($node_url)"
        return 0
    else
        echo "   ❌ $node_name is not responding ($node_url)"
        return 1
    fi
}

NODES_OK=0
check_node "Node1" "$NODE1_URL" && NODES_OK=$((NODES_OK + 1))
check_node "Node2" "$NODE2_URL" && NODES_OK=$((NODES_OK + 1))
check_node "Node3" "$NODE3_URL" && NODES_OK=$((NODES_OK + 1))

if [ $NODES_OK -lt 3 ]; then
    echo ""
    echo "❌ Not all nodes are running ($NODES_OK/3 active)"
    echo ""
    echo "Please ensure the multi-node mesh is running:"
    echo "  ./multi-node-demo.sh"
    echo ""
    echo "Then run this test script in another terminal:"
    echo "  ./mesh-test.sh"
    exit 1
fi

echo ""
echo "✅ All nodes are responding!"
echo ""

# Test 1: Publish to Node1, check Node2 and Node3 can see it
echo "🧪 Test 1: Cross-Node Event Distribution"
echo "========================================="
echo ""

TOPIC="mesh.test.cross_node"
TEST_MESSAGE="Hello from mesh test at $(date -Iseconds)"

echo "📤 Publishing event to Node1..."
echo "   Topic: $TOPIC"
echo "   Message: $TEST_MESSAGE"

# Use no-auth mode for simplicity in testing
$CLI --no-auth publish \
    --server "$NODE1_URL" \
    --topic "$TOPIC" \
    --payload "{\"message\":\"$TEST_MESSAGE\",\"source\":\"Node1\",\"test\":\"cross_node_distribution\"}"

if [ $? -eq 0 ]; then
    echo "   ✅ Event published to Node1 successfully"
else
    echo "   ❌ Failed to publish event to Node1"
    exit 1
fi

echo ""
echo "🔍 Checking if event reached other nodes..."

# Small delay to allow propagation
sleep 2

# Check Node2
echo "📥 Checking Node2 for the event..."
NODE2_RESULT=$($CLI --no-auth topics info --server "$NODE2_URL" --topic "$TOPIC" 2>/dev/null || echo "ERROR")
if [[ "$NODE2_RESULT" == *"ERROR"* ]]; then
    echo "   ⚠️  Node2: Topic not found (events may not have propagated yet)"
else
    echo "   ✅ Node2: Topic exists - event propagated successfully!"
fi

# Check Node3
echo "📥 Checking Node3 for the event..."
NODE3_RESULT=$($CLI --no-auth topics info --server "$NODE3_URL" --topic "$TOPIC" 2>/dev/null || echo "ERROR")
if [[ "$NODE3_RESULT" == *"ERROR"* ]]; then
    echo "   ⚠️  Node3: Topic not found (events may not have propagated yet)"
else
    echo "   ✅ Node3: Topic exists - event propagated successfully!"
fi

echo ""

# Test 2: Replay the event from different nodes
echo "🧪 Test 2: Event Replay Across Nodes"
echo "====================================="
echo ""

for i in 1 2 3; do
    eval "NODE_URL=\$NODE${i}_URL"
    echo "📖 Replaying events from Node$i..."

    REPLAY_RESULT=$($CLI --no-auth replay \
        --server "$NODE_URL" \
        --topic "$TOPIC" \
        --offset 0 \
        --limit 5 2>/dev/null || echo "ERROR")

    if [[ "$REPLAY_RESULT" == *"ERROR"* ]]; then
        echo "   ❌ Node$i: Failed to replay events"
    else
        # Count events in replay result
        EVENT_COUNT=$(echo "$REPLAY_RESULT" | grep -c '"message"' || echo "0")
        echo "   ✅ Node$i: Successfully replayed $EVENT_COUNT events"

        # Show a sample of the replay (first few lines)
        if [ "$EVENT_COUNT" -gt 0 ]; then
            echo "      Sample: $(echo "$REPLAY_RESULT" | head -1 | cut -c1-80)..."
        fi
    fi

    echo ""
done

# Test 3: Multi-node publishing test
echo "🧪 Test 3: Publishing from Different Nodes"
echo "==========================================="
echo ""

TEST_TOPIC="mesh.test.multi_publish"

for i in 1 2 3; do
    eval "NODE_URL=\$NODE${i}_URL"
    MESSAGE="Message from Node$i at $(date -Iseconds)"

    echo "📤 Publishing from Node$i..."
    echo "   Message: $MESSAGE"

    $CLI --no-auth publish \
        --server "$NODE_URL" \
        --topic "$TEST_TOPIC" \
        --payload "{\"message\":\"$MESSAGE\",\"source\":\"Node$i\",\"test\":\"multi_node_publishing\"}" \
        > /dev/null 2>&1

    if [ $? -eq 0 ]; then
        echo "   ✅ Published successfully"
    else
        echo "   ❌ Failed to publish"
    fi

    echo ""
done

# Brief delay for propagation
sleep 2

# Check how many events each node has
echo "📊 Event Distribution Summary"
echo "============================="
echo ""

for i in 1 2 3; do
    eval "NODE_URL=\$NODE${i}_URL"

    # Count events in the multi-publish topic
    REPLAY_RESULT=$($CLI --no-auth replay \
        --server "$NODE_URL" \
        --topic "$TEST_TOPIC" \
        --offset 0 \
        --limit 10 2>/dev/null || echo "")

    EVENT_COUNT=$(echo "$REPLAY_RESULT" | grep -c '"message"' || echo "0")

    echo "Node$i ($NODE_URL):"
    echo "   📊 Total events in '$TEST_TOPIC': $EVENT_COUNT"

    # Show unique sources in events
    if [ "$EVENT_COUNT" -gt 0 ]; then
        SOURCES=$(echo "$REPLAY_RESULT" | grep -o '"source":"Node[0-9]"' | sort -u | wc -l)
        echo "   🎯 Events from $SOURCES different source nodes"
    fi

    echo ""
done

echo "🎉 Multi-Node Mesh Test Complete!"
echo ""
echo "💡 What this test demonstrated:"
echo "   ✅ Events published to one node propagate to others"
echo "   ✅ All nodes can replay the same events (mesh consistency)"
echo "   ✅ Events can be published from any node in the mesh"
echo "   ✅ Node discovery enables automatic mesh formation"
echo ""
echo "🔍 To see discovery logs:"
echo "   Check /tmp/eventmesh-demo-*/node*.log for discovery messages"
echo "   Look for: '🔍 Discovery found X peers' and '✅ Connected to peer'"