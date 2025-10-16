# EventMesh CLI Usage Examples

This directory contains practical examples and scripts demonstrating common EventMesh CLI workflows.

## Prerequisites

1. EventMesh server running (see `../single-node/` for setup)
2. EventMesh CLI built (`make build` from project root)

## Common Workflows

### Basic Authentication and Health Checks

```bash
# Check if server is healthy
./bin/eventmesh-cli health --server http://localhost:8081 --client-id health-check

# Authenticate a client
./bin/eventmesh-cli auth --server http://localhost:8081 --client-id my-client

# Store the token for reuse (the CLI automatically stores it)
# Subsequent commands will use the stored token
```

### Publishing Events

```bash
# Simple event publishing
./bin/eventmesh-cli publish \
  --server http://localhost:8081 \
  --client-id publisher \
  --topic "user.registered" \
  --payload '{"user_id": "12345", "email": "user@example.com", "timestamp": "2024-01-15T10:30:00Z"}'

# Publishing structured business events
./bin/eventmesh-cli publish \
  --server http://localhost:8081 \
  --client-id order-service \
  --topic "orders.created" \
  --payload '{
    "order_id": "ORD-789",
    "customer_id": "CUST-456",
    "items": [
      {"sku": "WIDGET-A", "quantity": 2, "price": 29.99},
      {"sku": "GADGET-B", "quantity": 1, "price": 49.99}
    ],
    "total": 109.97,
    "currency": "USD",
    "created_at": "2024-01-15T10:30:00Z"
  }'
```

### Subscription Management

```bash
# Subscribe to specific topic
./bin/eventmesh-cli subscribe \
  --server http://localhost:8081 \
  --client-id subscriber \
  --topic "orders.created"

# Subscribe to topic pattern (wildcard)
./bin/eventmesh-cli subscribe \
  --server http://localhost:8081 \
  --client-id subscriber \
  --topic "orders.*"

# Subscribe to all user events
./bin/eventmesh-cli subscribe \
  --server http://localhost:8081 \
  --client-id user-service \
  --topic "user.*"

# List all subscriptions
./bin/eventmesh-cli subscriptions list \
  --server http://localhost:8081 \
  --client-id subscriber

# Delete specific subscription
./bin/eventmesh-cli subscriptions delete \
  --server http://localhost:8081 \
  --client-id subscriber \
  --id "sub-abc123"
```

### Real-time Event Streaming

```bash
# Stream all events for a topic pattern
./bin/eventmesh-cli stream \
  --server http://localhost:8081 \
  --client-id stream-consumer \
  --topic "orders.*"

# Stream specific topic events
./bin/eventmesh-cli stream \
  --server http://localhost:8081 \
  --client-id notification-service \
  --topic "user.password_reset"

# Stream all events (use with caution in production)
./bin/eventmesh-cli stream \
  --server http://localhost:8081 \
  --client-id debug-consumer \
  --topic "*"
```

### Admin Operations

```bash
# Get server statistics
./bin/eventmesh-cli admin stats \
  --server http://localhost:8081 \
  --client-id admin-user

# List all connected clients
./bin/eventmesh-cli admin clients \
  --server http://localhost:8081 \
  --client-id admin-user

# List all subscriptions across all clients
./bin/eventmesh-cli admin subscriptions \
  --server http://localhost:8081 \
  --client-id admin-user
```

## Example Scripts

### 1. Simple Publisher (`simple-publisher.sh`)

Publishes a series of events to demonstrate basic publishing:

```bash
./simple-publisher.sh
```

### 2. Pattern Subscriber (`pattern-subscriber.sh`)

Demonstrates wildcard subscription patterns:

```bash
./pattern-subscriber.sh
```

### 3. Order Processing Workflow (`order-workflow.sh`)

Shows a complete order processing lifecycle with multiple event types:

```bash
./order-workflow.sh
```

### 4. Multi-Client Demo (`multi-client-demo.sh`)

Demonstrates multiple clients publishing and consuming simultaneously:

```bash
./multi-client-demo.sh
```

## Advanced Usage Patterns

### Error Handling

The CLI provides detailed error messages. Common scenarios:

```bash
# Authentication required
./bin/eventmesh-cli publish --topic test --payload "{}" --client-id test-client
# Error: not authenticated - call 'auth' command first

# Invalid topic
./bin/eventmesh-cli publish --topic "" --payload "{}" --client-id test-client
# Error: topic cannot be empty

# Server not available
./bin/eventmesh-cli health --server http://localhost:9999 --client-id test
# Error: failed to connect to server
```

### Token Reuse

```bash
# Get token from successful auth
TOKEN=$(./bin/eventmesh-cli auth --server http://localhost:8081 --client-id my-client | grep "Token:" | cut -d' ' -f2)

# Use token directly in subsequent commands
./bin/eventmesh-cli publish \
  --server http://localhost:8081 \
  --client-id my-client \
  --token "$TOKEN" \
  --topic "test.topic" \
  --payload '{"test": true}'
```

### Batch Operations

```bash
# Publish multiple events in sequence
for i in {1..10}; do
  ./bin/eventmesh-cli publish \
    --server http://localhost:8081 \
    --client-id batch-publisher \
    --topic "batch.events" \
    --payload "{\"batch_id\": \"batch-001\", \"sequence\": $i, \"timestamp\": \"$(date -Iseconds)\"}"
done
```

### Configuration via Environment Variables

```bash
# Set common configuration via environment
export EVENTMESH_SERVER="http://localhost:8081"
export EVENTMESH_CLIENT_ID="my-app"
export EVENTMESH_TIMEOUT="60s"

# Commands will use these defaults
./bin/eventmesh-cli auth
./bin/eventmesh-cli publish --topic "test" --payload '{"msg": "hello"}'
```

## Integration with Other Tools

### Using with `jq` for JSON Processing

```bash
# Pretty-print event payloads
./bin/eventmesh-cli stream --server http://localhost:8081 --client-id debug --topic "orders.*" | \
  grep "Payload:" | \
  sed 's/.*Payload: //' | \
  jq .

# Extract specific fields
./bin/eventmesh-cli subscriptions list --server http://localhost:8081 --client-id test | \
  grep "ID:" | \
  awk '{print $2}'
```

### Using with `curl` for Comparison

The CLI wraps EventMesh's HTTP API. Equivalent curl commands:

```bash
# Health check
curl http://localhost:8081/api/v1/health

# Authenticate (get token)
curl -X POST http://localhost:8081/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"clientId": "my-client"}'

# Publish event
curl -X POST http://localhost:8081/api/v1/events \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer YOUR_TOKEN" \
  -d '{"topic": "test.topic", "payload": {"message": "hello"}}'
```

## Troubleshooting

### Common Issues

1. **"not authenticated" errors**: Run `auth` command first
2. **"connection refused" errors**: Check server is running and URL is correct
3. **"topic not found" errors**: Create subscription first, or check topic spelling
4. **Timeout errors**: Increase `--timeout` value for slow operations

### Debug Mode

Set log level for more verbose output:

```bash
# Server-side debugging (restart server with debug logging)
./bin/eventmesh --log-level debug --http --http-port 8081 --http-secret "secret" --node-id "debug-node" --listen ":8082" --peer-listen ":8083"

# Client-side debugging (add --verbose flag if available, or check error messages)
```

### Performance Considerations

- **Streaming**: Streaming consumes server resources. Close streams when not needed.
- **Batch Publishing**: For high throughput, consider using the HTTP client library instead of CLI
- **Topic Patterns**: Broad patterns (like `*`) can impact performance with many topics

## Next Steps

- See `../http-client/` for programmatic access via Go HTTP client library
- See `../end-to-end/` for complete application examples
- Review the HTTP API documentation for advanced usage