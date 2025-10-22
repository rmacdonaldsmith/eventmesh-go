# EventMesh Examples

This directory contains practical examples demonstrating how to use EventMesh for event-driven applications.

## Quick Start - Multi-Node Pub/Sub

The fastest way to see EventMesh in action:

```bash
cd examples/multi-node
./start-server.sh     # Terminal 1 - starts server
./subscriber.sh       # Terminal 2 - listens for events
./publisher.sh        # Terminal 3 - sends events
```

## Available Examples

### [`multi-node/`](multi-node/) - Multi-Node Mesh Demo
**Best starting point** - Shows core EventMesh functionality:
- Start a server
- Subscribe to topic patterns
- Publish events
- See real-time event streaming
- Multi-node mesh discovery

### [`single-node/`](single-node/) - Complete Server Setup
Comprehensive single-node deployment with:
- Detailed server configuration
- Step-by-step operational guide
- Health monitoring examples
- Production-ready startup scripts

### [`cli-usage/`](cli-usage/) - Advanced CLI Workflows
Advanced CLI patterns and scripts:
- Authentication and token management
- Pattern-based subscriptions
- Business workflow examples (order processing)
- Multi-service demonstrations

## What is EventMesh?

EventMesh is a distributed event streaming platform that provides:

- **Event Streaming**: Real-time publish/subscribe messaging
- **HTTP API**: RESTful interface for all operations
- **CLI Tool**: Command-line interface for easy interaction
- **JWT Authentication**: Secure client isolation
- **Topic Patterns**: Flexible routing with wildcards
- **Event Persistence**: Reliable message storage and replay

## Common Use Cases

- **Microservice Communication**: Decouple services with async messaging
- **Event-Driven Architecture**: Build reactive applications
- **Real-time Notifications**: Stream updates to clients
- **Audit Trails**: Capture and replay business events
- **Integration Hub**: Connect disparate systems

## Architecture Overview

```
┌─────────────┐    HTTP API    ┌─────────────┐
│  Publisher  │ ──────────────► │ EventMesh   │
│   Client    │                │   Server    │
└─────────────┘                └─────────────┘
                                       │
                               Event Stream (SSE)
                                       ▼
┌─────────────┐               ┌─────────────┐
│ Subscriber  │ ◄───────────── │ Subscriber  │
│  Client #1  │               │  Client #2  │
└─────────────┘               └─────────────┘
```

## Getting Started

1. **Build EventMesh**:
   ```bash
   make build
   ```

2. **Try the Multi-Node Example**:
   ```bash
   cd examples/multi-node
   ./start-server.sh
   # In new terminals:
   ./subscriber.sh
   ./publisher.sh
   ```

3. **Explore More Examples**: Check other directories for advanced patterns

## Development Workflow

For faster development and testing, EventMesh supports a no-authentication mode:

### Quick Development Setup
```bash
# Start server without authentication (INSECURE - dev only)
./bin/eventmesh --http --no-auth

# Use CLI without tokens (in another terminal)
./bin/eventmesh-cli --no-auth publish --topic test --payload '{"data":"value"}'
./bin/eventmesh-cli --no-auth topics info --topic test
./bin/eventmesh-cli --no-auth replay --topic test --offset 0
```

**⚠️ Important**: No-auth mode disables security and should **never** be used in production. Admin endpoints always require proper JWT authentication even in no-auth mode.

## Need Help?

- Start with `examples/multi-node/` for basic pub/sub and mesh demos
- Check `examples/single-node/README.md` for operational guidance
- See `examples/cli-usage/README.md` for advanced CLI patterns
- Review the main project README for API documentation