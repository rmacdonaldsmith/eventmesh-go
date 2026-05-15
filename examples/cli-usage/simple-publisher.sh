#!/bin/bash

# Simple EventMesh Publisher Example
# Demonstrates basic event publishing patterns

set -e

# Configuration
SERVER_URL=${SERVER_URL:-"http://localhost:8081"}

# Colors
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
NC='\033[0m'

CLI_BIN="../../bin/eventmesh-cli"

echo -e "${BLUE}EventMesh Simple Publisher Example${NC}"
echo -e "${BLUE}=================================${NC}"

# Check server health
echo -e "${GREEN}Checking server health...${NC}"
"$CLI_BIN" --no-auth health --server "$SERVER_URL"

echo -e "${GREEN}Publishing sample events...${NC}"

# Publish various types of events
events=(
    # User events
    "user.registered:{\"user_id\": \"user-001\", \"email\": \"alice@example.com\", \"plan\": \"premium\"}"
    "user.login:{\"user_id\": \"user-001\", \"ip_address\": \"192.168.1.100\", \"user_agent\": \"Mozilla/5.0\"}"
    "user.profile_updated:{\"user_id\": \"user-001\", \"fields_changed\": [\"name\", \"phone\"], \"timestamp\": \"2024-01-15T10:30:00Z\"}"

    # Product events
    "product.created:{\"product_id\": \"prod-123\", \"name\": \"Wireless Headphones\", \"category\": \"Electronics\", \"price\": 99.99}"
    "product.price_changed:{\"product_id\": \"prod-123\", \"old_price\": 99.99, \"new_price\": 89.99, \"reason\": \"sale\"}"
    "product.out_of_stock:{\"product_id\": \"prod-123\", \"last_quantity\": 0, \"restock_date\": \"2024-01-20\"}"

    # System events
    "system.maintenance_start:{\"maintenance_id\": \"maint-456\", \"duration_minutes\": 30, \"affected_services\": [\"api\", \"web\"]}"
    "system.backup_completed:{\"backup_id\": \"backup-789\", \"size_mb\": 1024, \"duration_seconds\": 300, \"success\": true}"
)

for event in "${events[@]}"; do
    IFS=':' read -r topic payload <<< "$event"

    echo -e "${YELLOW}Publishing: $topic${NC}"
    "$CLI_BIN" --no-auth publish \
        --server "$SERVER_URL" \
        --topic "$topic" \
        --payload "$payload"

    sleep 1
done

echo -e "${GREEN}Published ${#events[@]} events successfully!${NC}"
echo ""
echo -e "${BLUE}Event Topics Published:${NC}"
for event in "${events[@]}"; do
    IFS=':' read -r topic payload <<< "$event"
    echo -e "  • $topic"
done

echo ""
echo -e "${GREEN}Next steps:${NC}"
echo -e "${YELLOW}• Inspect one topic: $CLI_BIN --no-auth topics info --server $SERVER_URL --topic user.registered${NC}"
echo -e "${YELLOW}• Replay events: $CLI_BIN --no-auth replay --server $SERVER_URL --topic user.registered --offset 0${NC}"
echo -e "${YELLOW}• Stream matching events: $CLI_BIN --no-auth stream --server $SERVER_URL --topic \"user.*\"${NC}"
