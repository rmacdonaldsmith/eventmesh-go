# Simple EventMesh Examples

Basic examples showing how to run EventMesh server and connect clients for publish/subscribe.

## Quick Start

### 1. Build EventMesh
```bash
# From project root
make build
```

### 2. Start Server
```bash
./start-server.sh
```

### 3. In another terminal - Subscribe to events
```bash
./subscriber.sh
```

### 4. In another terminal - Publish events
```bash
./publisher.sh
```

That's it! You'll see events published by the publisher appear in the subscriber terminal.

## Advanced Features - Event Replay

EventMesh stores all events and allows you to replay them from any offset:

### 5. Try the replay demo (optional)
```bash
./replay-demo.sh
```

This demonstrates:
- Publishing historical events
- Inspecting topic metadata (`topics info`)
- Replaying events from specific offsets (`replay`)
- Comparing replay vs real-time streaming

## What's Happening

- **Server**: EventMesh server running on port 8081
- **Publisher**: Sends events to topics like `news.sports` and `news.weather`
- **Subscriber**: Listens to `news.*` pattern and receives all news events in real-time
- **Replay**: Read historical events from any point in time using offset-based queries

## Files

- `start-server.sh` - Starts EventMesh server with minimal config
- `publisher.sh` - Simple script that publishes a few events
- `subscriber.sh` - Simple script that subscribes and streams events
- `demo.sh` - Runs publisher and subscriber together
- `replay-demo.sh` - Demonstrates offset-based event replay features

## CLI Commands Reference

EventMesh CLI provides several commands for working with events:

### Real-time Operations
- `eventmesh-cli auth` - Authenticate with the server
- `eventmesh-cli publish` - Publish a single event
- `eventmesh-cli subscribe` - Create a subscription
- `eventmesh-cli stream` - Stream events in real-time

### Historical Operations (New!)
- `eventmesh-cli replay` - Replay historical events from a specific offset
- `eventmesh-cli topics info` - Show topic metadata and available offsets

### Admin & Monitoring
- `eventmesh-cli health` - Check server health
- `eventmesh-cli admin` - Admin operations (stats, clients, etc.)

Run any command with `--help` for detailed usage information.