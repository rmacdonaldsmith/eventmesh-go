# EventMesh

A distributed event streaming platform for real-time messaging between applications.

## What is EventMesh?

EventMesh is a Go-based event streaming system that enables secure, scalable communication across distributed applications. It provides real-time event publishing and subscription through HTTP APIs with automatic multi-node discovery and mesh networking.

**Key Features:**
- HTTP REST API with Server-Sent Events streaming
- Multi-node mesh with automatic peer discovery
- JWT authentication and client isolation
- Event persistence with offset-based replay
- Pattern-based topic subscriptions (`news.*`, `orders.#`)
- Command-line interface for all operations

## Quick Start

**Prerequisites:** Go 1.25.1+

**Build:**
```bash
make build
```

**Try the Multi-Node Demo:**
```bash
cd examples/multi-node
./start-server.sh     # Terminal 1
./subscriber.sh       # Terminal 2
./publisher.sh        # Terminal 3
```

This starts an EventMesh server, subscribes to `news.*` events, and publishes sample events.

## Architecture

EventMesh consists of:

- **EventMesh Server**: Core event routing and storage engine
- **HTTP API**: RESTful interface with Server-Sent Events for real-time streaming
- **CLI Tool**: Command-line interface for publishing, subscribing, and administration
- **Multi-Node Mesh**: Automatic peer discovery and cross-node event distribution

```
┌─────────────┐    HTTP API    ┌─────────────┐    Peer Network    ┌─────────────┐
│  Publisher  │──────────────→ │ EventMesh   │←──────────────────→│ EventMesh   │
│   Client    │                │   Node 1    │                    │   Node 2    │
└─────────────┘                └─────────────┘                    └─────────────┘
                                       │                                  │
                               Event Stream (SSE)                Event Stream (SSE)
                                       ↓                                  ↓
┌─────────────┐               ┌─────────────┐                    ┌─────────────┐
│ Subscriber  │←──────────────│ Subscriber  │                    │ Subscriber  │
│  Client #1  │               │  Client #2  │                    │  Client #3  │
└─────────────┘               └─────────────┘                    └─────────────┘
```

## Features

- **Event Streaming**: Real-time publish/subscribe messaging
- **Multi-Node Mesh**: Automatic discovery and cross-node event routing
- **Event Persistence**: All events stored with offset-based replay capability
- **Topic Patterns**: Flexible routing with wildcard subscriptions
- **JWT Authentication**: Secure client isolation and access control
- **HTTP API**: RESTful endpoints for all operations
- **CLI Interface**: Full command-line tool for operational tasks
- **Development Mode**: No-auth mode for testing and development

## Getting Started

### Build Commands

```bash
make            # Full build: format, vet, test, build
make build      # Build server binary only
make test       # Run tests with coverage
make clean      # Clean build artifacts
make run        # Build and start development server
```

### Server Usage

```bash
# Basic server
./bin/eventmesh --http --http-port 8081

# Development mode (no authentication)
./bin/eventmesh --http --no-auth

# Multi-node with discovery
./bin/eventmesh --http --node-id node1 --peer-listen :9090
./bin/eventmesh --http --node-id node2 --peer-listen :9091 --seed-nodes "localhost:9090"
```

### CLI Usage

```bash
# Authenticate (production)
./bin/eventmesh-cli auth --client-id my-client

# Development mode (no auth required)
./bin/eventmesh-cli --no-auth publish --topic test.events --payload '{"msg":"hello"}'
./bin/eventmesh-cli --no-auth stream --topic "test.*"
./bin/eventmesh-cli --no-auth topics info --topic test.events
```

## Examples & Demos

### Multi-Node Examples (`examples/multi-node/`)
Best starting point - demonstrates core functionality:
- Server startup and basic pub/sub
- Multi-node mesh with automatic discovery
- Event replay and historical queries
- Cross-node event delivery testing

### Single-Node Setup (`examples/single-node/`)
Production-ready deployment patterns:
- Comprehensive server configuration
- Health monitoring and operational guides
- Authentication setup

### CLI Usage Patterns (`examples/cli-usage/`)
Advanced command-line workflows:
- Authentication and token management
- Business workflow examples
- Multi-service integration patterns

## API Reference

### HTTP Endpoints

**Events:**
- `POST /api/v1/events` - Publish event
- `GET /api/v1/events/stream?topic=pattern` - Subscribe via Server-Sent Events

**Topics:**
- `GET /api/v1/topics/{topic}/info` - Topic metadata and offsets
- `GET /api/v1/topics/{topic}/replay?offset=N` - Replay from offset

**Admin:**
- `GET /api/v1/health` - Server health check
- `GET /api/v1/admin/clients` - Connected clients
- `GET /api/v1/admin/subscriptions` - Active subscriptions

**Authentication:**
- `POST /api/v1/auth/login` - Client authentication

### CLI Commands

- `eventmesh-cli auth` - Authentication management
- `eventmesh-cli publish` - Publish events
- `eventmesh-cli stream` - Real-time event streaming
- `eventmesh-cli replay` - Historical event replay
- `eventmesh-cli topics` - Topic inspection and management
- `eventmesh-cli health` - Server health checks
- `eventmesh-cli admin` - Administrative operations

## Development

**Requirements:**
- Go 1.25.1 or later
- No external dependencies for core functionality

**Project Structure:**
```
eventmesh-go/
├── cmd/eventmesh/          # Server binary
├── pkg/                    # Public interfaces
├── internal/               # Private implementations
├── examples/               # Working examples and demos
├── tests/                  # Integration tests
├── bin/                    # Built binaries
└── proto/                  # gRPC protocol definitions
```

**Testing:**
```bash
make test                   # Run all tests
make test-coverage          # Generate HTML coverage report
go test ./tests -v          # Integration tests only
```

## License

MIT License - see LICENSE file for details.

## Documentation

- `docs/design.md` - System design and architecture
- `examples/` - Working demonstrations
- `CLAUDE.md` - Development guidelines