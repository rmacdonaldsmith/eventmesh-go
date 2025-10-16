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

## What's Happening

- **Server**: EventMesh server running on port 8081
- **Publisher**: Sends events to topics like `news.sports` and `news.weather`
- **Subscriber**: Listens to `news.*` pattern and receives all news events in real-time

## Files

- `start-server.sh` - Starts EventMesh server with minimal config
- `publisher.sh` - Simple script that publishes a few events
- `subscriber.sh` - Simple script that subscribes and streams events
- `demo.sh` - Runs publisher and subscriber together