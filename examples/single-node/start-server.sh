#!/bin/bash

# EventMesh Single Node Server Startup Script
# This script starts a single-node EventMesh server with sensible defaults

set -e

# Configuration
HTTP_PORT=${HTTP_PORT:-8081}
EVENTMESH_JWT_SECRET=${EVENTMESH_JWT_SECRET:-"eventmesh-demo-secret-key"}
NODE_ID=${NODE_ID:-"single-node-demo"}
LISTEN_PORT=${LISTEN_PORT:-8082}
PEER_LISTEN_PORT=${PEER_LISTEN_PORT:-8083}
LOG_LEVEL=${LOG_LEVEL:-"info"}

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

echo -e "${BLUE}Starting EventMesh Single Node Server${NC}"
echo -e "${BLUE}====================================${NC}"

# Check if eventmesh binary exists
EVENTMESH_BIN="../../bin/eventmesh"
if [ ! -f "$EVENTMESH_BIN" ]; then
    echo -e "${RED}Error: EventMesh binary not found at $EVENTMESH_BIN${NC}"
    echo -e "${YELLOW}Please run 'make build' from the project root first.${NC}"
    exit 1
fi

# Show configuration
echo -e "${GREEN}Configuration:${NC}"
echo -e "  HTTP API Port: ${HTTP_PORT}"
echo -e "  Node ID: ${NODE_ID}"
echo -e "  Internal Port: ${LISTEN_PORT}"
echo -e "  Peer Port: ${PEER_LISTEN_PORT}"
echo -e "  Log Level: ${LOG_LEVEL}"
echo ""

# Check if ports are available
check_port() {
    local port=$1
    local name=$2
    if lsof -Pi :$port -sTCP:LISTEN -t >/dev/null 2>&1; then
        echo -e "${RED}Error: Port $port ($name) is already in use${NC}"
        echo -e "${YELLOW}Please stop the process using this port or change the port configuration.${NC}"
        exit 1
    fi
}

echo -e "${BLUE}Checking port availability...${NC}"
check_port $HTTP_PORT "HTTP API"
check_port $LISTEN_PORT "Internal"
check_port $PEER_LISTEN_PORT "Peer Communication"

echo -e "${GREEN}All ports available. Starting server...${NC}"
echo ""

# Start the server
echo -e "${BLUE}Server starting with command:${NC}"
echo "EVENTMESH_JWT_SECRET=\"$EVENTMESH_JWT_SECRET\" $EVENTMESH_BIN --http --http-port $HTTP_PORT --node-id \"$NODE_ID\" --listen \":$LISTEN_PORT\" --peer-listen \":$PEER_LISTEN_PORT\" --log-level $LOG_LEVEL"
echo ""

# Show next steps
echo -e "${GREEN}Server starting! Once running, you can:${NC}"
echo ""
echo -e "${YELLOW}Check server health:${NC}"
echo "  ../../bin/eventmesh-cli health --server http://localhost:$HTTP_PORT --client-id demo-client"
echo ""
echo -e "${YELLOW}Authenticate a client:${NC}"
echo "  ../../bin/eventmesh-cli auth --server http://localhost:$HTTP_PORT --client-id demo-client"
echo ""
echo -e "${YELLOW}Publish an event:${NC}"
echo "  ../../bin/eventmesh-cli publish --server http://localhost:$HTTP_PORT --client-id demo-client --topic orders.created --payload '{\"id\": 123}'"
echo ""
echo -e "${YELLOW}Press Ctrl+C to stop the server${NC}"
echo ""
echo -e "${BLUE}Server Output:${NC}"
echo "----------------------------------------"

# Execute the server (export the environment variable for the process)
export EVENTMESH_JWT_SECRET
exec "$EVENTMESH_BIN" \
    --http \
    --http-port "$HTTP_PORT" \
    --node-id "$NODE_ID" \
    --listen ":$LISTEN_PORT" \
    --peer-listen ":$PEER_LISTEN_PORT" \
    --log-level "$LOG_LEVEL" \
    --no-auth