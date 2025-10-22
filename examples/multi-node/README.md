# Multi-Node EventMesh Examples

Examples showing EventMesh server operations, multi-node mesh discovery, and client publish/subscribe patterns.

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

## Multi-Node Discovery

EventMesh nodes can automatically discover each other to form a mesh network:

### 6. Multi-Node Mesh Demo
```bash
./multi-node-demo.sh
```

This starts a 3-node mesh with automatic discovery:
- **Node1** (seed): Acts as the discovery seed node
- **Node2**: Automatically discovers and connects to Node1
- **Node3**: Automatically discovers and connects to Node1

**What you'll see:**
```
🔗 EventMesh Multi-Node Discovery Demo
🌱 Starting Node1 (Seed Node) on ports 8091/8092/8093...
🔍 Starting Node2 (discovers Node1) on ports 8094/8095/8096...
🔍 Starting Node3 (discovers Node1) on ports 8097/8098/8099...
🎉 SUCCESS: All 3 nodes are running!
```

### 7. Test Multi-Node Event Flow
```bash
./mesh-test.sh    # (run in another terminal while multi-node-demo.sh is running)
```

This demonstrates:
- Events published to one node reach all other nodes
- Cross-node event replay and consistency
- Publishing from any node in the mesh
- Automatic mesh formation through discovery

## What's Happening

- **Multi-Node Setup**: 3 EventMesh nodes running on ports 8091, 8094, 8097
- **Node1 (Seed)**: Discovery seed node on port 8091
- **Node2 & Node3**: Auto-discover and connect to Node1
- **Publisher**: Sends events to topics like `news.sports` and `news.weather`
- **Subscriber**: Listens to `news.*` pattern and receives all news events in real-time
- **Cross-Node Events**: Events published to any node are delivered to all connected nodes
- **Replay**: Read historical events from any point in time using offset-based queries

## Files

- `start-server.sh` - Starts EventMesh server with minimal config
- `publisher.sh` - Simple script that publishes a few events
- `subscriber.sh` - Simple script that subscribes and streams events
- `demo.sh` - Runs publisher and subscriber together
- `replay-demo.sh` - Demonstrates offset-based event replay features
- `multi-node-demo.sh` - Starts a 3-node mesh for discovery testing
- `mesh-test.sh` - Tests cross-node event delivery in multi-node mesh

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

## Node Discovery Reference

### Manual Multi-Node Setup

You can also start nodes manually with discovery:

```bash
# Terminal 1: Start seed node
../../bin/eventmesh --http --http-port 8091 --node-id node1 --peer-listen ":8093"

# Terminal 2: Start node that discovers node1
../../bin/eventmesh --http --http-port 8094 --node-id node2 --peer-listen ":8096" \
  --seed-nodes "localhost:8093"

# Terminal 3: Start another node that discovers node1
../../bin/eventmesh --http --http-port 8097 --node-id node3 --peer-listen ":8099" \
  --seed-nodes "localhost:8093"
```

### Discovery Process

When a node starts with `--seed-nodes`:

1. **Bootstrap Configuration**: Parses seed node addresses
2. **Discovery Service**: Creates static discovery from seed list
3. **Peer Connection**: Automatically connects to discovered peers
4. **Mesh Formation**: Nodes can now exchange events

**Discovery Logs:**
```
time=22:32:50.853 level=INFO msg="starting peer discovery" node_id=demo-node-2 seed_nodes=1
time=22:32:50.853 level=INFO msg="peers discovered" node_id=demo-node-2 peers_found=1
time=22:32:50.853 level=INFO msg="connecting to peer" peer_id=localhost:8093 peer_address=localhost:8093 local_node_id=demo-node-2
time=22:32:50.853 level=INFO msg="peer connected successfully" peer_id=localhost:8093 peer_address=localhost:8093 local_node_id=demo-node-2
time=22:32:50.853 level=INFO msg="discovery complete" node_id=demo-node-2 peers_found=1 peers_connected=1
```

**Enhanced Startup Summary:**
```
time=22:32:53.876 level=INFO msg="🚀 Node Ready Summary:"
time=22:32:53.876 level=INFO msg="   Node ID: demo-node-3"
time=22:32:53.876 level=INFO msg="   Version: 0.1.0"
time=22:32:53.876 level=INFO msg="   Startup Time: 2ms"
time=22:32:53.876 level=INFO msg="   Peer Listen: localhost:8099"
time=22:32:53.876 level=INFO msg="   HTTP API: localhost:8097"
time=22:32:53.876 level=INFO msg="   Connected Peers (1):"
time=22:32:53.876 level=INFO msg="     • localhost:8093@localhost:8093"
time=22:32:53.876 level=INFO msg="   Auth Mode: disabled"
```

### Key Benefits

- **Zero Configuration**: Nodes find each other automatically
- **Fault Tolerant**: Discovery continues even if some connections fail
- **Scalable**: Add new nodes by pointing them to any existing seed
- **Simple**: Just add `--seed-nodes="host:port,host2:port2"` to any node

## Enhanced Logging & Observability

EventMesh now includes comprehensive structured logging for better operational visibility:

### Log Levels
- `--log-level debug` - Detailed debug information (used in demos)
- `--log-level info` - Standard operational information (default)
- `--log-level warn` - Warning messages only
- `--log-level error` - Error messages only

### Key Logging Features

**Structured Logging**: All logs use structured key-value format for easy parsing:
```bash
time=22:32:50.853 level=INFO msg="peer connected successfully" peer_id=localhost:8093 local_node_id=demo-node-2
```

**Pretty-Printed Startup Summary**: Each node logs a comprehensive startup summary showing:
- Node ID, version, and startup time
- Actual listening addresses (not just configured ones)
- Connected peer information
- Authentication mode and configuration

**Event Publication Logging**: Track events as they flow through the mesh:
```bash
time=22:33:28.470 level=INFO msg="event published" client_id=dev-client topic=news.sports event_offset=0 payload_size=64
```

**Static Log Files**: Demo scripts write to `/tmp/eventmesh-demo/nodeX.log` for consistent log monitoring

## Development Mode (No Authentication)

For development and testing, EventMesh supports a no-authentication mode that bypasses JWT token requirements:

### Start Server in No-Auth Mode
```bash
# Instead of ./start-server.sh, run:
../../bin/eventmesh --http --no-auth
```

### Use CLI Without Authentication
```bash
# No need for auth/tokens - just add --no-auth flag:
../../bin/eventmesh-cli --no-auth publish --topic test.events --payload '{"msg":"hello"}'
../../bin/eventmesh-cli --no-auth topics info --topic test.events
../../bin/eventmesh-cli --no-auth replay --topic test.events --offset 0
```

**⚠️ Security Note**: No-auth mode is **INSECURE** and should only be used for development/testing. Admin endpoints always require proper authentication even in no-auth mode.