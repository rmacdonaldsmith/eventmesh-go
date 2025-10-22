#!/bin/bash

# Multi-Node EventMesh Discovery Demo
#
# This script demonstrates EventMesh node discovery by starting a 3-node mesh:
# - Node1 (seed): Acts as the seed node that others discover
# - Node2: Discovers Node1 using --seed-nodes flag
# - Node3: Discovers Node1 using --seed-nodes flag
#
# Prerequisites:
# 1. Run 'make build' from project root to build binaries
# 2. Run this script: ./multi-node-demo.sh [--follow-logs]
# 3. Watch the discovery logs as nodes find and connect to each other
# 4. Use Ctrl+C to stop all nodes
#
# Options:
#   --follow-logs    Show live logs from all nodes (can be noisy)

set -e

# Parse command line options
FOLLOW_LOGS=false
if [[ "${1:-}" == "--follow-logs" ]]; then
    FOLLOW_LOGS=true
fi

# Get script directory and calculate paths relative to it
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
EVENTMESH="$PROJECT_ROOT/bin/eventmesh"

echo "🔗 EventMesh Multi-Node Discovery Demo"
echo "====================================="
echo ""

# Check if binary exists
if [ ! -f "$EVENTMESH" ]; then
    echo "❌ Error: EventMesh binary not found at $EVENTMESH"
    echo ""
    echo "To fix this:"
    echo "  1. Navigate to project root: cd $PROJECT_ROOT"
    echo "  2. Build the project: make build"
    echo "  3. Run this script again"
    exit 1
fi

# Create static log directory for easy tailing
LOG_DIR="/tmp/eventmesh-demo"
rm -rf "$LOG_DIR"
mkdir -p "$LOG_DIR"
echo "📁 Logs will be written to: $LOG_DIR"
echo "💡 Pro tip: Run 'tail -f $LOG_DIR/node*.log' in another terminal for live logs"
echo ""

# Function to cleanup background processes
cleanup() {
    echo ""
    echo "🧹 Shutting down nodes and log tails..."

    # Kill any tail processes first
    for tail_pid in ${TAIL1_PID:-} ${TAIL2_PID:-} ${TAIL3_PID:-} ${TAIL_ALL_PID:-}; do
        if [[ -n "$tail_pid" ]] && kill -0 "$tail_pid" 2>/dev/null; then
            kill "$tail_pid" 2>/dev/null || true
        fi
    done

    # Kill all background processes in our process group
    if [[ -n "${PIDS:-}" ]]; then
        for pid in $PIDS; do
            if kill -0 "$pid" 2>/dev/null; then
                echo "   Stopping process $pid"
                kill "$pid" 2>/dev/null || true
            fi
        done
    fi

    # Wait a moment for graceful shutdown
    sleep 2

    # Force kill any remaining processes
    if [[ -n "${PIDS:-}" ]]; then
        for pid in $PIDS; do
            if kill -0 "$pid" 2>/dev/null; then
                echo "   Force stopping process $pid"
                kill -9 "$pid" 2>/dev/null || true
            fi
        done
    fi

    echo "✅ All nodes stopped"
    echo ""
    echo "💡 Log files are available in: /tmp/eventmesh-demo"
    echo "   - node1.log (seed node)"
    echo "   - node2.log (discovers node1)"
    echo "   - node3.log (discovers node1)"
    echo ""
    echo "💡 Pro tip: Keep 'tail -f /tmp/eventmesh-demo/node*.log' running for next demo run!"
}

# Set trap to cleanup on exit
trap cleanup EXIT INT TERM

# Start Node1 (Seed Node)
echo "🌱 Starting Node1 (Seed Node) on ports 8091/8092/8093..."
EVENTMESH_JWT_SECRET="demo-secret-1" $EVENTMESH \
    --http \
    --http-port 8091 \
    --node-id "demo-node-1" \
    --listen ":8092" \
    --peer-listen ":8093" \
    --no-auth \
    --log-level debug \
    > "$LOG_DIR/node1.log" 2>&1 &
NODE1_PID=$!
PIDS="$NODE1_PID"

echo "   Node1 PID: $NODE1_PID"
echo "   HTTP API: http://localhost:8091"
echo "   Logs: $LOG_DIR/node1.log"

# Show startup progress
echo "   Waiting for Node1 to start..."
sleep 2

# Show key startup messages from logs
if [[ -f "$LOG_DIR/node1.log" ]]; then
    echo "   📋 Startup status:"
    # Show key startup lines
    grep -E "(Starting EventMesh|WARNING.*NO-AUTH|Started successfully|HTTP API)" "$LOG_DIR/node1.log" 2>/dev/null | sed 's/^/      /' || true
fi

# Check if Node1 is running
if ! kill -0 "$NODE1_PID" 2>/dev/null; then
    echo "❌ Node1 failed to start. Check logs: $LOG_DIR/node1.log"
    exit 1
fi

# Verify Node1 HTTP API is responding
HTTP_READY=false
for i in {1..10}; do
    if curl -s http://localhost:8091/api/v1/health > /dev/null 2>&1; then
        HTTP_READY=true
        break
    fi
    echo "   Waiting for Node1 HTTP API... (attempt $i/10)"
    sleep 1
done

if [ "$HTTP_READY" = false ]; then
    echo "❌ Node1 HTTP API not responding. Check logs: $LOG_DIR/node1.log"
    exit 1
fi

echo "   ✅ Node1 is running and ready"
echo ""

# Start Node2 (Discovers Node1)
echo "🔍 Starting Node2 (discovers Node1) on ports 8094/8095/8096..."
EVENTMESH_JWT_SECRET="demo-secret-2" $EVENTMESH \
    --http \
    --http-port 8094 \
    --node-id "demo-node-2" \
    --listen ":8095" \
    --peer-listen ":8096" \
    --seed-nodes "localhost:8093" \
    --no-auth \
    --log-level debug \
    > "$LOG_DIR/node2.log" 2>&1 &
NODE2_PID=$!
PIDS="$PIDS $NODE2_PID"

echo "   Node2 PID: $NODE2_PID"
echo "   HTTP API: http://localhost:8094"
echo "   Discovery: Will discover Node1 at localhost:8093"
echo "   Logs: $LOG_DIR/node2.log"

# Wait for Node2 to start and discover
echo "   Waiting for Node2 to start and discover Node1..."
sleep 3

# Start Node3 (Also discovers Node1)
echo ""
echo "🔍 Starting Node3 (discovers Node1) on ports 8097/8098/8099..."
EVENTMESH_JWT_SECRET="demo-secret-3" $EVENTMESH \
    --http \
    --http-port 8097 \
    --node-id "demo-node-3" \
    --listen ":8098" \
    --peer-listen ":8099" \
    --seed-nodes "localhost:8093" \
    --no-auth \
    --log-level debug \
    > "$LOG_DIR/node3.log" 2>&1 &
NODE3_PID=$!
PIDS="$PIDS $NODE3_PID"

echo "   Node3 PID: $NODE3_PID"
echo "   HTTP API: http://localhost:8097"
echo "   Discovery: Will discover Node1 at localhost:8093"
echo "   Logs: $LOG_DIR/node3.log"

# Wait for Node3 to start and discover
echo "   Waiting for Node3 to start and discover Node1..."
sleep 3

echo ""
echo "🎯 Multi-Node Mesh Status"
echo "========================"

# Check which nodes are still running
RUNNING_NODES=0
for i in 1 2 3; do
    eval "PID=\$NODE${i}_PID"
    if kill -0 "$PID" 2>/dev/null; then
        echo "✅ Node$i (PID $PID): Running"
        RUNNING_NODES=$((RUNNING_NODES + 1))
    else
        echo "❌ Node$i (PID $PID): Not running"
    fi
done

echo ""
if [ $RUNNING_NODES -eq 3 ]; then
    echo "🎉 SUCCESS: All 3 nodes are running!"
    echo ""
    echo "Discovery Status:"
    echo "=================="
    echo "Check the logs to see discovery in action:"
    echo ""
    echo "Node2 discovering Node1:"
    echo "  grep -A5 -B5 'Discovery found' /tmp/eventmesh-demo/node2.log"
    echo ""
    echo "Node3 discovering Node1:"
    echo "  grep -A5 -B5 'Discovery found' /tmp/eventmesh-demo/node3.log"
    echo ""
    echo "Testing the Mesh:"
    echo "=================="
    echo "You can now test the mesh by running (in separate terminals):"
    echo ""
    echo "# Test Node1 -> Node2 communication:"
    echo "./publisher.sh   # (uses Node1 at port 8091)"
    echo ""
    echo "# Test cross-node event delivery:"
    echo "curl -X POST http://localhost:8091/api/v1/events \\"
    echo "     -H 'Content-Type: application/json' \\"
    echo "     -d '{\"topic\":\"mesh.test\",\"payload\":{\"msg\":\"Hello mesh!\"}}'"
    echo ""
    echo "Press Ctrl+C to stop all nodes..."
    echo ""
    echo "💡 To follow live logs from all nodes, run in another terminal:"
    echo "   tail -f /tmp/eventmesh-demo/node*.log"
    echo ""
    echo "💡 Or restart with: ./multi-node-demo.sh --follow-logs"
else
    echo "⚠️  WARNING: Only $RUNNING_NODES/3 nodes running"
    echo ""
    echo "Check logs for errors:"
    for i in 1 2 3; do
        echo "  tail /tmp/eventmesh-demo/node$i.log"
    done
fi

echo ""

# If --follow-logs was specified, start tailing all logs
if [ "$FOLLOW_LOGS" = true ]; then
    echo "📋 Following logs from all nodes (use Ctrl+C to stop)..."
    echo "======================================================="
    tail -f "$LOG_DIR"/node*.log &
    TAIL_ALL_PID=$!
fi

# Keep script running until interrupted
while true; do
    sleep 5

    # Check if any nodes have died
    CURRENT_RUNNING=0
    for i in 1 2 3; do
        eval "PID=\$NODE${i}_PID"
        if kill -0 "$PID" 2>/dev/null; then
            CURRENT_RUNNING=$((CURRENT_RUNNING + 1))
        fi
    done

    if [ $CURRENT_RUNNING -lt $RUNNING_NODES ]; then
        echo "⚠️  WARNING: Some nodes have stopped running ($CURRENT_RUNNING/$RUNNING_NODES active)"
        break
    fi
done