# Single Node EventMesh Server Example

This example demonstrates how to run a single-node EventMesh server and interact with it using the CLI tool.

## Overview

EventMesh provides a distributed event streaming platform with:
- **HTTP API** for publishing events and managing subscriptions
- **Server-Sent Events (SSE)** for real-time event streaming
- **JWT authentication** for client isolation
- **CLI tool** for easy interaction

## Quick Start

### 1. Build the Binaries

First, build both the server and CLI:

```bash
# From the project root
make build

# This creates:
# - bin/eventmesh (server)
# - bin/eventmesh-cli (CLI tool)
```

### 2. Start the EventMesh Server

```bash
./bin/eventmesh \
  --http \
  --http-port 8081 \
  --http-secret "my-secret-key" \
  --node-id "demo-node" \
  --listen ":8082" \
  --peer-listen ":8083"
```

**Server Options Explained:**
- `--http`: Enable HTTP API
- `--http-port 8081`: HTTP API listens on port 8081
- `--http-secret "my-secret-key"`: JWT signing secret
- `--node-id "demo-node"`: Unique identifier for this node
- `--listen ":8082"`: Internal mesh communication port
- `--peer-listen ":8083"`: Peer-to-peer communication port

### 3. Verify Server is Running

Check the server health:

```bash
./bin/eventmesh-cli health \
  --server http://localhost:8081 \
  --client-id demo-client
```

Expected output:
```
✅ Server is healthy
Connected Clients: 0
Connected Peers: 0
Event Log: Healthy
Routing Table: Healthy
Peer Link: Healthy
```

## Basic Operations

### Authentication

Before publishing or subscribing, authenticate with the server:

```bash
./bin/eventmesh-cli auth \
  --server http://localhost:8081 \
  --client-id demo-client
```

This returns a JWT token that's automatically stored for subsequent commands.

### Publishing Events

Publish an event to a topic:

```bash
./bin/eventmesh-cli publish \
  --server http://localhost:8081 \
  --client-id demo-client \
  --topic "orders.created" \
  --payload '{"order_id": "12345", "customer": "john@example.com", "amount": 99.99}'
```

### Creating Subscriptions

Subscribe to a topic pattern:

```bash
./bin/eventmesh-cli subscribe \
  --server http://localhost:8081 \
  --client-id demo-client \
  --topic "orders.*"
```

### Listing Subscriptions

View all your subscriptions:

```bash
./bin/eventmesh-cli subscriptions list \
  --server http://localhost:8081 \
  --client-id demo-client
```

### Streaming Events (Real-time)

Stream events in real-time using Server-Sent Events:

```bash
./bin/eventmesh-cli stream \
  --server http://localhost:8081 \
  --client-id demo-client \
  --topic "orders.*"
```

This will show live events as they're published. Press Ctrl+C to stop.

## Complete Workflow Example

Here's a complete workflow demonstrating EventMesh capabilities:

### Terminal 1: Start Server
```bash
./bin/eventmesh --http --http-port 8081 --http-secret "demo-secret" --node-id "demo-node" --listen ":8082" --peer-listen ":8083"
```

### Terminal 2: Setup Subscriber
```bash
# Authenticate
./bin/eventmesh-cli auth --server http://localhost:8081 --client-id subscriber

# Create subscription
./bin/eventmesh-cli subscribe --server http://localhost:8081 --client-id subscriber --topic "orders.*"

# Start streaming (this will block and show live events)
./bin/eventmesh-cli stream --server http://localhost:8081 --client-id subscriber --topic "orders.*"
```

### Terminal 3: Publish Events
```bash
# Authenticate
./bin/eventmesh-cli auth --server http://localhost:8081 --client-id publisher

# Publish some events
./bin/eventmesh-cli publish --server http://localhost:8081 --client-id publisher --topic "orders.created" --payload '{"order_id": "001", "customer": "alice@example.com"}'

./bin/eventmesh-cli publish --server http://localhost:8081 --client-id publisher --topic "orders.updated" --payload '{"order_id": "001", "status": "processing"}'

./bin/eventmesh-cli publish --server http://localhost:8081 --client-id publisher --topic "orders.completed" --payload '{"order_id": "001", "total": 149.99}'
```

You should see these events appear in real-time in Terminal 2!

## Configuration Options

### Server Configuration

The EventMesh server supports various configuration options:

```bash
./bin/eventmesh --help
```

Key options:
- `--http`: Enable HTTP API (required for CLI interaction)
- `--http-port`: HTTP API port (default: 8081)
- `--http-secret`: JWT signing secret (required for authentication)
- `--node-id`: Unique node identifier (required)
- `--listen`: Internal mesh communication address
- `--peer-listen`: Peer-to-peer communication address
- `--log-level`: Logging level (debug, info, warn, error)

### CLI Configuration

The CLI tool supports global flags:

```bash
./bin/eventmesh-cli --help
```

Key options:
- `--server`: EventMesh server URL (default: http://localhost:8081)
- `--client-id`: Client identifier (required for most operations)
- `--token`: JWT token (if you want to reuse a previous token)
- `--timeout`: Request timeout (default: 30s)

## Advanced Topics

### Topic Patterns

EventMesh supports wildcard patterns in subscriptions:

- `orders.*` - matches `orders.created`, `orders.updated`, etc.
- `user.*.login` - matches `user.123.login`, `user.456.login`, etc.
- `*` - matches all topics

### Event Persistence

All events are persisted locally by the EventMesh server. This means:
- Events are not lost if subscribers are offline
- New subscribers can replay historical events
- The server can restart without losing events (in production with persistent storage)

### Authentication & Security

- Each client must authenticate with a unique `client-id`
- JWT tokens are used for subsequent API calls
- Clients are isolated from each other
- Admin operations require special privileges

## Next Steps

- See `examples/cli-usage/` for more advanced CLI workflows
- See `examples/http-client/` for Go applications using the HTTP client library
- See `examples/end-to-end/` for complete application examples

## Troubleshooting

### Server Won't Start
- Check if ports 8081, 8082, 8083 are available
- Ensure you have the `--http-secret` flag set
- Check logs for specific error messages

### Authentication Fails
- Verify the server URL is correct
- Ensure the server is running and healthy
- Check that you're using a unique `client-id`

### Events Not Received
- Verify you have an active subscription to the topic
- Check that the topic pattern matches published topics
- Ensure the subscriber is authenticated

### Connection Issues
- Check network connectivity to the server
- Verify the server is accepting HTTP connections
- Try the health check command first