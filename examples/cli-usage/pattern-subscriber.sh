#!/bin/bash

# EventMesh Pattern Subscriber Example
# Demonstrates wildcard subscription patterns and real-time streaming

set -e

# Configuration
SERVER_URL=${SERVER_URL:-"http://localhost:8081"}
CLIENT_ID="pattern-subscriber"

# Colors
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
PURPLE='\033[0;35m'
NC='\033[0m'

CLI_BIN="../../bin/eventmesh-cli"

echo -e "${BLUE}EventMesh Pattern Subscriber Example${NC}"
echo -e "${BLUE}===================================${NC}"

# Check server health
echo -e "${GREEN}Checking server health...${NC}"
"$CLI_BIN" health --server "$SERVER_URL" --client-id health-check

# Authenticate
echo -e "${GREEN}Authenticating subscriber...${NC}"
"$CLI_BIN" auth --server "$SERVER_URL" --client-id "$CLIENT_ID"

echo -e "${GREEN}Creating pattern-based subscriptions...${NC}"

# Create multiple pattern subscriptions
patterns=("user.*" "product.*" "system.*" "order.*")

for pattern in "${patterns[@]}"; do
    echo -e "${YELLOW}Subscribing to pattern: $pattern${NC}"
    "$CLI_BIN" subscribe --server "$SERVER_URL" --client-id "$CLIENT_ID" --topic "$pattern"
    sleep 0.5
done

# List all subscriptions to verify
echo -e "${GREEN}Verifying subscriptions...${NC}"
"$CLI_BIN" subscriptions list --server "$SERVER_URL" --client-id "$CLIENT_ID"

echo ""
echo -e "${GREEN}Starting real-time event streaming demo...${NC}"
echo -e "${YELLOW}This will stream events matching our patterns for 15 seconds.${NC}"
echo -e "${YELLOW}Open another terminal and run simple-publisher.sh to see events!${NC}"
echo ""
echo -e "${PURPLE}Press Ctrl+C to stop streaming early, or wait 15 seconds...${NC}"
echo ""

# Start streaming with timeout
echo -e "${BLUE}Streaming events (user.*, product.*, system.*, order.*):${NC}"
echo "----------------------------------------"

# Use timeout to limit streaming duration
timeout 15s "$CLI_BIN" stream --server "$SERVER_URL" --client-id "$CLIENT_ID" --topic "user.*" &
STREAM_PID1=$!

# Wait a moment and start additional streams for other patterns
sleep 1
timeout 15s "$CLI_BIN" stream --server "$SERVER_URL" --client-id "${CLIENT_ID}-2" --topic "product.*" &
STREAM_PID2=$!

sleep 1
timeout 15s "$CLI_BIN" stream --server "$SERVER_URL" --client-id "${CLIENT_ID}-3" --topic "system.*" &
STREAM_PID3=$!

sleep 1
timeout 15s "$CLI_BIN" stream --server "$SERVER_URL" --client-id "${CLIENT_ID}-4" --topic "order.*" &
STREAM_PID4=$!

# Wait for all streams to finish or timeout
wait $STREAM_PID1 2>/dev/null || true
wait $STREAM_PID2 2>/dev/null || true
wait $STREAM_PID3 2>/dev/null || true
wait $STREAM_PID4 2>/dev/null || true

echo ""
echo "----------------------------------------"
echo -e "${GREEN}Streaming demo completed!${NC}"

echo ""
echo -e "${GREEN}Pattern Matching Summary:${NC}"
echo -e "${BLUE}• user.* matches:${NC} user.registered, user.login, user.profile_updated, etc."
echo -e "${BLUE}• product.* matches:${NC} product.created, product.price_changed, product.out_of_stock, etc."
echo -e "${BLUE}• system.* matches:${NC} system.maintenance_start, system.backup_completed, etc."
echo -e "${BLUE}• order.* matches:${NC} order.created, order.shipped, order.completed, etc."

echo ""
echo -e "${GREEN}Advanced Pattern Examples:${NC}"
echo -e "${YELLOW}Single-level wildcards:${NC}"
echo "  user.login.* - matches user.login.success, user.login.failed"
echo "  *.created - matches user.created, order.created, product.created"
echo ""
echo -e "${YELLOW}Multi-level wildcards:${NC}"
echo "  * - matches ALL topics (use carefully!)"

echo ""
echo -e "${GREEN}Cleanup: Removing subscriptions...${NC}"

# Get list of subscription IDs and delete them
echo -e "${YELLOW}Listing current subscriptions for cleanup...${NC}"
SUBSCRIPTIONS_OUTPUT=$("$CLI_BIN" subscriptions list --server "$SERVER_URL" --client-id "$CLIENT_ID")

# Extract subscription IDs (this is a simple approach - in production, use proper JSON parsing)
if echo "$SUBSCRIPTIONS_OUTPUT" | grep -q "ID:"; then
    echo "$SUBSCRIPTIONS_OUTPUT" | grep "ID:" | while read -r line; do
        SUB_ID=$(echo "$line" | awk '{print $2}')
        if [ ! -z "$SUB_ID" ]; then
            echo -e "${YELLOW}Deleting subscription: $SUB_ID${NC}"
            "$CLI_BIN" subscriptions delete --server "$SERVER_URL" --client-id "$CLIENT_ID" --id "$SUB_ID"
        fi
    done
fi

echo ""
echo -e "${GREEN}Pattern subscriber demo completed!${NC}"
echo ""
echo -e "${BLUE}Key Takeaways:${NC}"
echo -e "${YELLOW}✓ Wildcard patterns enable flexible event routing${NC}"
echo -e "${YELLOW}✓ Multiple clients can subscribe to overlapping patterns${NC}"
echo -e "${YELLOW}✓ Real-time streaming works with pattern matching${NC}"
echo -e "${YELLOW}✓ Subscriptions can be managed dynamically${NC}"

echo ""
echo -e "${GREEN}Next steps:${NC}"
echo -e "${YELLOW}• Try order-workflow.sh for a complete business process example${NC}"
echo -e "${YELLOW}• Run multi-client-demo.sh to see multiple publishers and subscribers${NC}"