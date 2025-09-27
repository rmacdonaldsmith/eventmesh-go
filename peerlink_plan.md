# PeerLink Implementation Plan (Phase 3 MVP)

## Overview
Implement gRPC-based PeerLink component for secure peer-to-peer communication between EventMesh nodes. Focus on MVP simplicity while addressing all 8 requirements from design.md.

## Key Simplifications for MVP
- **mTLS deferred**: Use plain gRPC for MVP (mTLS in post-MVP as per design.md)
- **Single stream per peer**: No multi-stream complexity
- **Simple backoff**: Exponential backoff without advanced jitter algorithms
- **Basic metrics**: Essential observability without complex tracing
- **Static peer lists**: No dynamic peer discovery
- **In-memory state**: No persistence of peer state/offsets

## Implementation Phases

### **Phase 3.1: Protobuf & gRPC Foundation** ⏳
**Goal**: Create protobuf schema and generate gRPC code

**Tasks:**
1. Create `proto/peerlink/v1/peerlink.proto` with exact schema from design.md
2. Set up protobuf generation in `go.mod` and build scripts
3. Generate Go gRPC client/server code
4. Create basic `PeerMessage`, `Handshake`, `Event`, `Ack`, `Heartbeat` message handling
5. Verify protobuf compilation and imports work correctly

**Success Criteria**: Clean protobuf generation with no compilation errors

### **Phase 3.2: Basic Connection Lifecycle (REQ-PL-001)** ⏳
**Goal**: Establish single bidirectional gRPC streams per peer

**Simple Implementation:**
- **Connection deduplication**: Simple rule - higher `node_id` wins as dialer
- **Reconnection**: Basic exponential backoff (1s, 2s, 4s, max 30s)
- **Handshake**: Exchange `node_id` and `protocol_version` only (no feature negotiation yet)
- **Graceful shutdown**: Close stream and wait for in-flight sends with timeout

**Key Components:**
- `PeerConnection` struct managing single gRPC stream per peer
- `ConnectionManager` handling peer lifecycle and deduplication
- Basic retry logic with `time.Sleep()` and context cancellation

### **Phase 3.3: Message Flow & Bounded Queues (REQ-PL-002)** ⏳
**Goal**: Implement simple flow control to prevent memory issues

**Simple Approach:**
- **Bounded send queue**: `chan PeerMessage` with configurable size (default 1000)
- **Drop policy**: Block with timeout, then drop oldest message on timeout
- **No advanced backpressure**: Rely on gRPC/HTTP2 built-in flow control
- **Basic metrics**: Queue depth gauge, drops counter

**Implementation:**
- `SendQueue` per peer connection with buffered channel
- Simple goroutine to drain queue and send via gRPC stream
- Configurable timeouts and drop policies

### **Phase 3.4: Heartbeats & Health Monitoring (REQ-PL-003)** ⏳
**Goal**: Detect peer failures and emit health state changes

**Simple Design:**
- **Periodic ping**: Send `Heartbeat` message every 5s (configurable)
- **Failure detection**: 3 consecutive missed heartbeats = Unhealthy
- **State transitions**: Healthy → Unhealthy → Disconnected
- **No piggyback optimization**: Always send dedicated heartbeats for MVP

**Components:**
- `HealthMonitor` per peer with ticker and failure counters
- Simple state machine with basic logging
- Channel-based health events for observability

### **Phase 3.5: At-Least-Once Delivery Stub (REQ-PL-004)** ⏳
**Goal**: Basic ACK/Resume framework (full implementation deferred)

**MVP Implementation:**
- **Event sending**: Include topic/partition/offset in Event messages
- **ACK reception**: Log received ACKs but don't implement full replay yet
- **Resume logic**: Placeholder - will integrate with EventLog replay in Phase 4
- **Idempotency**: Simple offset-based duplicate detection

**Rationale**: This is complex and tightly coupled with EventLog. Implement framework now, full logic when integrating with MeshNode.

### **Phase 3.6: Configuration & Observability (REQ-PL-006, REQ-PL-007)** ⏳
**Goal**: Basic config and essential metrics

**Simple Config Structure:**
```go
type PeerLinkConfig struct {
    NodeID           string
    ListenAddress    string
    Peers            []PeerConfig
    SendQueueSize    int
    HeartbeatInterval time.Duration
    MaxMessageSize   int
    ReconnectBackoff  []time.Duration
}
```

**Essential Metrics:**
- Connection state (connected/disconnected/unhealthy)
- Messages sent/received counters
- Queue depth gauge
- Reconnect attempts counter
- Basic latency histogram for heartbeats

### **Phase 3.7: Integration & Testing** ⏳
**Goal**: Comprehensive testing and interface compliance

**Test Strategy:**
1. **Unit tests**: Each component with mocked dependencies
2. **Integration tests**: Two-node scenarios with real gRPC
3. **Failure scenarios**: Network partitions, node restarts, backpressure
4. **Performance baselines**: Message throughput and connection overhead
5. **Interface compliance**: Verify implements `peerlink.PeerLink`

## File Structure
```
proto/
  peerlink/v1/peerlink.proto       # Protobuf schema
internal/peerlink/
  grpc.go                          # Main gRPC implementation
  connection.go                    # PeerConnection management
  health.go                        # HealthMonitor implementation
  config.go                        # Configuration structures
  grpc_test.go                     # Comprehensive test suite
pkg/peerlink/
  interfaces.go                    # Public interfaces (existing)
```

## Requirements Mapping

| Requirement | Phase | Status | Implementation Notes |
|-------------|-------|--------|---------------------|
| **REQ-PL-001** Connection Lifecycle | 3.2 | ⏳ | Basic gRPC streams with reconnection |
| **REQ-PL-002** Flow Control | 3.3 | ⏳ | Bounded queues with simple drop policy |
| **REQ-PL-003** Heartbeats | 3.4 | ⏳ | 5s intervals, 3-failure detection |
| **REQ-PL-004** At-Least-Once | 3.5 | ⏳ | Framework only, full logic in Phase 4 |
| **REQ-PL-005** Protocol Version | 3.2 | ⏳ | Basic handshake, no feature negotiation |
| **REQ-PL-006** Observability | 3.6 | ⏳ | Essential metrics and structured logs |
| **REQ-PL-007** Config & Limits | 3.6 | ⏳ | Static configuration with safe defaults |
| **REQ-PL-008** Protobuf Schema | 3.1 | ⏳ | Exact schema from design.md |

## Success Criteria for MVP
- ✅ All 8 PeerLink requirements have basic implementation
- ✅ Two EventMesh nodes can establish gRPC connections
- ✅ Basic event streaming works bidirectionally
- ✅ Heartbeats detect failures and trigger reconnection
- ✅ Bounded queues prevent memory growth
- ✅ Configuration is externalized and testable
- ✅ Essential metrics are exposed for monitoring

## What's NOT in MVP (Post-MVP)
- mTLS security (Smallstep integration deferred)
- Advanced flow control (credits, prioritization)
- Multi-stream per peer (control vs data streams)
- Dynamic peer discovery (static peer lists only)
- Full at-least-once replay (framework only)
- Complex feature negotiation (basic handshake only)

## Implementation Status

**Current Phase**: Not started
**Next Action**: Begin Phase 3.1 - Protobuf & gRPC Foundation
**Target**: Working PeerLink component enabling basic EventMesh node communication

---

*This plan will be updated as implementation progresses and requirements evolve.*