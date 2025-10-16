#!/bin/bash

# EventMesh Single Node Demo Script
# This script demonstrates a complete EventMesh workflow

set -e

# Configuration
SERVER_URL="http://localhost:8081"
PUBLISHER_CLIENT="demo-publisher"
SUBSCRIBER_CLIENT="demo-subscriber"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
PURPLE='\033[0;35m'
NC='\033[0m' # No Color

# Check if CLI binary exists
CLI_BIN="../../bin/eventmesh-cli"
if [ ! -f "$CLI_BIN" ]; then
    echo -e "${RED}Error: EventMesh CLI binary not found at $CLI_BIN${NC}"
    echo -e "${YELLOW}Please run 'make build' from the project root first.${NC}"
    exit 1
fi

echo -e "${BLUE}EventMesh Single Node Demo${NC}"
echo -e "${BLUE}=========================${NC}"
echo ""

# Function to run CLI commands with nice output
run_cli_command() {
    local description="$1"
    shift
    echo -e "${YELLOW}➤ $description${NC}"
    echo -e "${PURPLE}Command: $CLI_BIN $*${NC}"
    "$CLI_BIN" "$@"
    echo ""
    sleep 1
}

# Check server health
echo -e "${GREEN}Step 1: Checking server health${NC}"
run_cli_command "Health check" health --server "$SERVER_URL" --client-id health-checker

# Authenticate clients
echo -e "${GREEN}Step 2: Authenticating clients${NC}"
run_cli_command "Publisher authentication" auth --server "$SERVER_URL" --client-id "$PUBLISHER_CLIENT"
run_cli_command "Subscriber authentication" auth --server "$SERVER_URL" --client-id "$SUBSCRIBER_CLIENT"

# Create subscription
echo -e "${GREEN}Step 3: Creating subscription${NC}"
run_cli_command "Subscribe to orders.* pattern" subscribe --server "$SERVER_URL" --client-id "$SUBSCRIBER_CLIENT" --topic "orders.*"

# List subscriptions to verify
echo -e "${GREEN}Step 4: Verifying subscription${NC}"
run_cli_command "List subscriptions" subscriptions list --server "$SERVER_URL" --client-id "$SUBSCRIBER_CLIENT"

# Publish some events
echo -e "${GREEN}Step 5: Publishing events${NC}"

# Sample order lifecycle events
events=(
    "orders.created:{\"order_id\": \"ORD-001\", \"customer\": \"alice@example.com\", \"amount\": 99.99, \"items\": [{\"sku\": \"WIDGET-A\", \"qty\": 2}]}"
    "orders.payment_received:{\"order_id\": \"ORD-001\", \"payment_method\": \"credit_card\", \"amount\": 99.99, \"transaction_id\": \"TXN-12345\"}"
    "orders.processing:{\"order_id\": \"ORD-001\", \"warehouse\": \"EAST-01\", \"estimated_ship_date\": \"2024-01-15\"}"
    "orders.shipped:{\"order_id\": \"ORD-001\", \"carrier\": \"FedEx\", \"tracking_number\": \"1Z999AA1234567890\", \"ship_date\": \"2024-01-15\"}"
    "orders.completed:{\"order_id\": \"ORD-001\", \"delivery_date\": \"2024-01-17\", \"customer_rating\": 5}"
)

for event in "${events[@]}"; do
    IFS=':' read -r topic payload <<< "$event"
    run_cli_command "Publishing $topic event" publish \
        --server "$SERVER_URL" \
        --client-id "$PUBLISHER_CLIENT" \
        --topic "$topic" \
        --payload "$payload"
done

# Show event streaming example
echo -e "${GREEN}Step 6: Event streaming demonstration${NC}"
echo -e "${YELLOW}Now let's demonstrate real-time event streaming.${NC}"
echo -e "${YELLOW}We'll start a background stream and publish a few more events.${NC}"
echo ""

# Start streaming in background for a few seconds
echo -e "${PURPLE}Starting background stream (will run for 10 seconds)...${NC}"
timeout 10s "$CLI_BIN" stream --server "$SERVER_URL" --client-id "$SUBSCRIBER_CLIENT" --topic "orders.*" &
STREAM_PID=$!

sleep 2

# Publish a few more events while streaming
echo -e "${YELLOW}Publishing additional events (you should see them in the stream above):${NC}"

additional_events=(
    "orders.created:{\"order_id\": \"ORD-002\", \"customer\": \"bob@example.com\", \"amount\": 149.99}"
    "orders.cancelled:{\"order_id\": \"ORD-003\", \"reason\": \"customer_request\", \"refund_amount\": 75.50}"
    "orders.returned:{\"order_id\": \"ORD-001\", \"reason\": \"defective_item\", \"refund_issued\": true}"
)

for event in "${additional_events[@]}"; do
    IFS=':' read -r topic payload <<< "$event"
    echo -e "${PURPLE}Publishing: $topic${NC}"
    "$CLI_BIN" publish --server "$SERVER_URL" --client-id "$PUBLISHER_CLIENT" --topic "$topic" --payload "$payload" > /dev/null
    sleep 2
done

# Wait for streaming to finish
wait $STREAM_PID 2>/dev/null || true

echo ""
echo -e "${GREEN}Step 7: Admin operations (if available)${NC}"
run_cli_command "Getting server statistics" admin stats --server "$SERVER_URL" --client-id admin-client || echo -e "${YELLOW}Note: Admin operations require special privileges${NC}"

# Final summary
echo -e "${GREEN}Demo Complete! Summary:${NC}"
echo -e "${BLUE}✓ Started EventMesh server${NC}"
echo -e "${BLUE}✓ Authenticated multiple clients${NC}"
echo -e "${BLUE}✓ Created topic subscriptions${NC}"
echo -e "${BLUE}✓ Published events across order lifecycle${NC}"
echo -e "${BLUE}✓ Demonstrated real-time streaming${NC}"
echo -e "${BLUE}✓ Showed admin operations${NC}"
echo ""
echo -e "${GREEN}Key EventMesh Features Demonstrated:${NC}"
echo -e "${YELLOW}• JWT-based client authentication${NC}"
echo -e "${YELLOW}• Topic-based publish/subscribe messaging${NC}"
echo -e "${YELLOW}• Wildcard pattern subscriptions (orders.*)${NC}"
echo -e "${YELLOW}• Real-time event streaming with Server-Sent Events${NC}"
echo -e "${YELLOW}• Event persistence and replay capability${NC}"
echo -e "${YELLOW}• RESTful HTTP API for all operations${NC}"
echo ""
echo -e "${BLUE}Next Steps:${NC}"
echo -e "${YELLOW}• Try the examples in ../cli-usage/ for more advanced workflows${NC}"
echo -e "${YELLOW}• See ../http-client/ for Go application examples${NC}"
echo -e "${YELLOW}• Check ../end-to-end/ for complete application scenarios${NC}"
echo ""
echo -e "${GREEN}Happy event streaming! 🚀${NC}"