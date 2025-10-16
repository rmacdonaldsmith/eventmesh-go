#!/bin/bash

# EventMesh Order Processing Workflow Example
# Demonstrates a realistic business process with multiple event types and subscribers

set -e

# Configuration
SERVER_URL=${SERVER_URL:-"http://localhost:8081"}
ORDER_SERVICE="order-service"
PAYMENT_SERVICE="payment-service"
INVENTORY_SERVICE="inventory-service"
SHIPPING_SERVICE="shipping-service"
NOTIFICATION_SERVICE="notification-service"
AUDIT_SERVICE="audit-service"

# Colors
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
PURPLE='\033[0;35m'
RED='\033[0;31m'
NC='\033[0m'

CLI_BIN="../../bin/eventmesh-cli"

echo -e "${BLUE}EventMesh Order Processing Workflow${NC}"
echo -e "${BLUE}===================================${NC}"
echo ""

# Check server health
echo -e "${GREEN}Step 1: Checking server health...${NC}"
"$CLI_BIN" health --server "$SERVER_URL" --client-id health-check

# Authenticate all services
echo -e "${GREEN}Step 2: Authenticating all services...${NC}"
services=("$ORDER_SERVICE" "$PAYMENT_SERVICE" "$INVENTORY_SERVICE" "$SHIPPING_SERVICE" "$NOTIFICATION_SERVICE" "$AUDIT_SERVICE")

for service in "${services[@]}"; do
    echo -e "${YELLOW}Authenticating: $service${NC}"
    "$CLI_BIN" auth --server "$SERVER_URL" --client-id "$service"
done

# Set up subscriptions for each service
echo -e "${GREEN}Step 3: Setting up service subscriptions...${NC}"

# Payment service listens for new orders
echo -e "${YELLOW}Payment service subscribing to orders.created${NC}"
"$CLI_BIN" subscribe --server "$SERVER_URL" --client-id "$PAYMENT_SERVICE" --topic "orders.created"

# Inventory service listens for payment confirmations
echo -e "${YELLOW}Inventory service subscribing to orders.payment_confirmed${NC}"
"$CLI_BIN" subscribe --server "$SERVER_URL" --client-id "$INVENTORY_SERVICE" --topic "orders.payment_confirmed"

# Shipping service listens for inventory allocations
echo -e "${YELLOW}Shipping service subscribing to orders.inventory_allocated${NC}"
"$CLI_BIN" subscribe --server "$SERVER_URL" --client-id "$SHIPPING_SERVICE" --topic "orders.inventory_allocated"

# Notification service listens to all order events
echo -e "${YELLOW}Notification service subscribing to orders.*${NC}"
"$CLI_BIN" subscribe --server "$SERVER_URL" --client-id "$NOTIFICATION_SERVICE" --topic "orders.*"

# Audit service listens to all events
echo -e "${YELLOW}Audit service subscribing to *${NC}"
"$CLI_BIN" subscribe --server "$SERVER_URL" --client-id "$AUDIT_SERVICE" --topic "*"

sleep 1

# Start background streaming for audit/monitoring
echo -e "${GREEN}Step 4: Starting audit monitoring stream...${NC}"
echo -e "${PURPLE}(Audit stream will run in background to show all events)${NC}"
"$CLI_BIN" stream --server "$SERVER_URL" --client-id "$AUDIT_SERVICE" --topic "*" > /tmp/audit-log.txt 2>&1 &
AUDIT_STREAM_PID=$!

sleep 2

# Process the order workflow
echo -e "${GREEN}Step 5: Processing order workflow...${NC}"

ORDER_ID="ORD-$(date +%s)"
CUSTOMER_EMAIL="customer@example.com"

# Step 1: Order created
echo -e "${BLUE}━━━ Order Creation ━━━${NC}"
ORDER_DATA="{
  \"order_id\": \"$ORDER_ID\",
  \"customer_email\": \"$CUSTOMER_EMAIL\",
  \"items\": [
    {\"sku\": \"LAPTOP-001\", \"quantity\": 1, \"price\": 1299.99},
    {\"sku\": \"MOUSE-001\", \"quantity\": 1, \"price\": 29.99}
  ],
  \"total_amount\": 1329.98,
  \"currency\": \"USD\",
  \"shipping_address\": {
    \"street\": \"123 Main St\",
    \"city\": \"Anytown\",
    \"state\": \"CA\",
    \"zip\": \"12345\"
  },
  \"created_at\": \"$(date -Iseconds)\"
}"

echo -e "${YELLOW}Publishing: orders.created${NC}"
"$CLI_BIN" publish --server "$SERVER_URL" --client-id "$ORDER_SERVICE" --topic "orders.created" --payload "$ORDER_DATA"
sleep 2

# Step 2: Payment processing
echo -e "${BLUE}━━━ Payment Processing ━━━${NC}"
PAYMENT_DATA="{
  \"order_id\": \"$ORDER_ID\",
  \"payment_method\": \"credit_card\",
  \"card_last_four\": \"4242\",
  \"amount\": 1329.98,
  \"transaction_id\": \"txn_$(date +%s)\",
  \"processed_at\": \"$(date -Iseconds)\"
}"

echo -e "${YELLOW}Publishing: orders.payment_confirmed${NC}"
"$CLI_BIN" publish --server "$SERVER_URL" --client-id "$PAYMENT_SERVICE" --topic "orders.payment_confirmed" --payload "$PAYMENT_DATA"
sleep 2

# Step 3: Inventory allocation
echo -e "${BLUE}━━━ Inventory Allocation ━━━${NC}"
INVENTORY_DATA="{
  \"order_id\": \"$ORDER_ID\",
  \"allocations\": [
    {\"sku\": \"LAPTOP-001\", \"quantity\": 1, \"warehouse\": \"WEST-01\", \"reserved_at\": \"$(date -Iseconds)\"},
    {\"sku\": \"MOUSE-001\", \"quantity\": 1, \"warehouse\": \"WEST-01\", \"reserved_at\": \"$(date -Iseconds)\"}
  ],
  \"estimated_ship_date\": \"$(date -d '+2 days' -Iseconds)\"
}"

echo -e "${YELLOW}Publishing: orders.inventory_allocated${NC}"
"$CLI_BIN" publish --server "$SERVER_URL" --client-id "$INVENTORY_SERVICE" --topic "orders.inventory_allocated" --payload "$INVENTORY_DATA"
sleep 2

# Step 4: Shipping
echo -e "${BLUE}━━━ Shipping ━━━${NC}"
SHIPPING_DATA="{
  \"order_id\": \"$ORDER_ID\",
  \"carrier\": \"FedEx\",
  \"service_type\": \"Ground\",
  \"tracking_number\": \"1Z999AA$(date +%s)\",
  \"estimated_delivery\": \"$(date -d '+5 days' -Iseconds)\",
  \"shipped_at\": \"$(date -Iseconds)\"
}"

echo -e "${YELLOW}Publishing: orders.shipped${NC}"
"$CLI_BIN" publish --server "$SERVER_URL" --client-id "$SHIPPING_SERVICE" --topic "orders.shipped" --payload "$SHIPPING_DATA"
sleep 2

# Step 5: Delivery confirmation
echo -e "${BLUE}━━━ Delivery Confirmation ━━━${NC}"
DELIVERY_DATA="{
  \"order_id\": \"$ORDER_ID\",
  \"delivered_at\": \"$(date -Iseconds)\",
  \"delivery_location\": \"Front door\",
  \"signature_required\": false,
  \"delivered_by\": \"FedEx Driver\",
  \"photos\": [\"delivery_photo_1.jpg\"]
}"

echo -e "${YELLOW}Publishing: orders.delivered${NC}"
"$CLI_BIN" publish --server "$SERVER_URL" --client-id "$SHIPPING_SERVICE" --topic "orders.delivered" --payload "$DELIVERY_DATA"
sleep 2

# Optional: Customer feedback
echo -e "${BLUE}━━━ Customer Feedback ━━━${NC}"
FEEDBACK_DATA="{
  \"order_id\": \"$ORDER_ID\",
  \"customer_email\": \"$CUSTOMER_EMAIL\",
  \"rating\": 5,
  \"review\": \"Excellent service, fast delivery!\",
  \"feedback_categories\": [\"shipping_speed\", \"product_quality\", \"customer_service\"],
  \"would_recommend\": true,
  \"submitted_at\": \"$(date -Iseconds)\"
}"

echo -e "${YELLOW}Publishing: orders.feedback_received${NC}"
"$CLI_BIN" publish --server "$SERVER_URL" --client-id "$ORDER_SERVICE" --topic "orders.feedback_received" --payload "$FEEDBACK_DATA"
sleep 2

# Stop audit stream
echo -e "${GREEN}Step 6: Stopping audit stream...${NC}"
kill $AUDIT_STREAM_PID 2>/dev/null || true

# Show audit log
echo -e "${GREEN}Step 7: Audit log summary${NC}"
echo -e "${BLUE}━━━ Audit Trail ━━━${NC}"
if [ -f /tmp/audit-log.txt ]; then
    echo -e "${PURPLE}Events captured by audit service:${NC}"
    grep -E "(orders\.|Event #)" /tmp/audit-log.txt | head -20 || echo "No events captured yet"
    rm -f /tmp/audit-log.txt
fi

# Show subscription status
echo -e "${GREEN}Step 8: Service subscription status${NC}"
for service in "${services[@]}"; do
    echo -e "${YELLOW}$service subscriptions:${NC}"
    "$CLI_BIN" subscriptions list --server "$SERVER_URL" --client-id "$service" | grep -E "(Topic|ID)" | head -4
    echo ""
done

echo -e "${GREEN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "${GREEN}Order Workflow Completed Successfully!${NC}"
echo -e "${GREEN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"

echo ""
echo -e "${BLUE}Workflow Summary:${NC}"
echo -e "${YELLOW}✓ Order created by Order Service${NC}"
echo -e "${YELLOW}✓ Payment processed by Payment Service${NC}"
echo -e "${YELLOW}✓ Inventory allocated by Inventory Service${NC}"
echo -e "${YELLOW}✓ Order shipped by Shipping Service${NC}"
echo -e "${YELLOW}✓ Order delivered (confirmed by Shipping Service)${NC}"
echo -e "${YELLOW}✓ Customer feedback received${NC}"
echo -e "${YELLOW}✓ All events captured by Audit Service${NC}"

echo ""
echo -e "${BLUE}Event Types Published:${NC}"
echo "  • orders.created"
echo "  • orders.payment_confirmed"
echo "  • orders.inventory_allocated"
echo "  • orders.shipped"
echo "  • orders.delivered"
echo "  • orders.feedback_received"

echo ""
echo -e "${BLUE}Services Involved:${NC}"
echo "  • Order Service (orchestrator)"
echo "  • Payment Service (payment processing)"
echo "  • Inventory Service (stock management)"
echo "  • Shipping Service (logistics)"
echo "  • Notification Service (customer communications)"
echo "  • Audit Service (compliance & monitoring)"

echo ""
echo -e "${GREEN}Key EventMesh Features Demonstrated:${NC}"
echo -e "${YELLOW}• Event-driven microservice architecture${NC}"
echo -e "${YELLOW}• Topic-based routing with wildcard patterns${NC}"
echo -e "${YELLOW}• Multiple subscribers per event type${NC}"
echo -e "${YELLOW}• Service-to-service communication via events${NC}"
echo -e "${YELLOW}• Audit trail and compliance logging${NC}"
echo -e "${YELLOW}• Asynchronous workflow orchestration${NC}"

echo ""
echo -e "${GREEN}Order ID for reference: $ORDER_ID${NC}"