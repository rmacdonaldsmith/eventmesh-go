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

### **Phase 3.1: Protobuf & gRPC Foundation** ✅ COMPLETED
**Goal**: Create protobuf schema and generate gRPC code using TDD approach

**Sub-Phases (Small TDD Batches):**

#### **Phase 3.1a: Configuration Component** ✅ COMPLETED
- ✅ Created `Config` struct with validation and defaults
- ✅ Added comprehensive tests (2 tests passing)
- ✅ Clean TDD cycle: RED → GREEN → REFACTOR → COMMIT
- ✅ **Committed**: e382109 - PeerLink Phase 3.1: Add Configuration Component (TDD)

#### **Phase 3.1b: Protobuf Schema** ✅ COMPLETED
- ✅ Created `proto/peerlink/v1/peerlink.proto` with exact schema from design.md
- ✅ Added gRPC dependencies to go.mod
- ✅ Ready for code generation

#### **Phase 3.1c: Protobuf Code Generation** ✅ COMPLETED
**Goal**: Generate working Go gRPC code and verify compilation
**Tasks:**
1. ✅ Updated protobuf schema to align with EventLog (removed partitions, added headers)
2. ✅ Generated Go gRPC client/server code from protobuf
3. ✅ Created comprehensive tests verifying message creation and schema changes
4. ✅ Fixed basic example to work with topic-scoped EventLog API
5. ✅ **Committed**: e5eb829 - Phase 1: Update protobuf schema to match EventLog and design spec

#### **Phase 3.1d: Basic GRPCPeerLink Structure** ✅ COMPLETED
**Goal**: Create minimal GRPCPeerLink struct with interface compliance
**Tasks:**
1. ✅ Created interface compliance test
2. ✅ Created basic `GRPCPeerLink` struct
3. ✅ Implemented minimal interface methods (return "not implemented" errors)
4. ✅ Made interface compliance test pass
5. ✅ Basic structure working and tested

#### **Phase 3.1e: Constructor and Lifecycle** ✅ COMPLETED
**Goal**: Add constructor and basic lifecycle methods
**Tasks:**
1. ✅ Created tests for `NewGRPCPeerLink` and `Close()`
2. ✅ Implemented constructor with config validation
3. ✅ Implemented basic `Close()` method
4. ✅ All tests passing with proper lifecycle management

### **Phase 3.2: Basic Connection Lifecycle (REQ-PL-001)** ✅ COMPLETED
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

### **Phase 3.3: Message Flow & Bounded Queues (REQ-PL-002)** ✅ COMPLETED
**Goal**: Implement bounded send queue to prevent memory growth (per design.md REQ-PL-002)

**Simple Implementation (MVP):**
- **Bounded send queue**: Add configurable queue size to GRPCPeerLink (default 100)
- **Drop policy**: Block with timeout, then drop oldest or newest (configurable)
- **Basic metrics**: Track queue depth and drops with simple counters
- **Keep sending as stub**: Queue messages but don't implement actual gRPC streaming yet

**Scope:**
- Modify `SendEvent()` to use bounded queue instead of immediate return
- Add queue management (block, timeout, drop policies)
- Add basic metrics tracking
- **Defer**: Actual gRPC streaming implementation (Phase 3.4+)

### **Phase 3.4: Heartbeats & Health Monitoring (REQ-PL-003)** ✅ COMPLETED
**Goal**: Basic failure detection (per design.md REQ-PL-003)

**Simple Implementation:**
- **Health states**: Healthy/Unhealthy/Disconnected per peer
- **Failure detection**: 3 consecutive missed heartbeats = mark peer Unhealthy
- **Heartbeat timing**: Configurable interval (default 5s, range 2s-10s per design)
- **Basic logging**: Log state transitions for observability

**Scope:**
- Add health state and failure counting to existing peer tracking
- Add methods: `GetPeerHealth(peerID)` and `SetPeerHealth(peerID, state)`
- Keep heartbeat sending as stub (don't implement actual periodic sending yet)
- **Defer**: Actual heartbeat protocol, complex state machine, "event emission"

### ~~**Phase 3.5: At-Least-Once Delivery (REQ-PL-004)**~~ ❌ **DROPPED FROM MVP**
**Rationale**: Too complex for MVP - requires deep EventLog integration, persistent state management, and complex replay logic. This is a perfect post-MVP feature that can be added once the basic mesh is working.

### **Phase 3.6: Configuration & Observability (REQ-PL-006, REQ-PL-007)** ⏳
**Goal**: Extend existing Config with queue/heartbeat settings (per design.md REQ-PL-007)

**Extended Config (build on existing):**
- Add `SendQueueSize int` (default 100)
- Add `HeartbeatInterval time.Duration` (default 5s)
- Add `SendTimeout time.Duration` (default 1s)

**Basic Metrics (REQ-PL-006):**
- Queue depth gauge (from Phase 3.3)
- Drops counter (from Phase 3.3)
- Connection state per peer (from Phase 3.4)

**Scope:**
- Extend existing Config struct, don't create new one
- **Defer**: Advanced metrics (latency histograms, bytes counters), structured logging

### **Phase 3.7: Final Integration** ⏳
**Goal**: Combine all phases into working PeerLink component

**Simple Integration:**
- Ensure all interface methods work together
- Basic end-to-end test (start server, connect peers, send events)
- Verify all implemented requirements (REQ-PL-001 through REQ-PL-007)

**Scope:**
- Keep testing incremental (done in each phase)
- **Defer**: Advanced integration testing, performance benchmarks, failure injection

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

**Current Phase**: 3.5 - At-Least-Once Delivery Stub
**Last Completed**: 3.4 - Heartbeats & Health Monitoring ✅
**Next Action**: Begin Phase 3.5 implementation of ACK/Resume framework
**Target**: Working PeerLink component enabling basic EventMesh node communication

### **Progress Summary**
- **Completed Phases**: 3.1 (Protobuf) + 3.2 (Lifecycle) + 3.3 (Bounded Queues) + 3.4 (Health Monitoring)
- **Test Count**: 15 passing (Config, lifecycle, queues, health states, metrics)
- **Commits**: 5 total (Config + Protobuf + Lifecycle + Queues + Health monitoring)
- **TDD Approach**: Strict RED → GREEN → REFACTOR → COMMIT cycle
- **Current Focus**: Basic ACK/Resume framework for Phase 3.5

### **Completed Work**
✅ **Phase 3.1: Protobuf & gRPC Foundation**
- **3.1a-e**: Complete protobuf schema, gRPC generation, and basic structure

✅ **Phase 3.2: Basic Connection Lifecycle**
- **Start/Stop Methods**: gRPC server lifecycle with graceful shutdown
- **Connection Management**: Connect, Disconnect, GetConnectedPeers with peer tracking
- **Stub Event Routing**: SendEvent and ReceiveEvents with basic implementations
- **Thread Safety**: Proper mutex usage and state management

✅ **Phase 3.3: Message Flow & Bounded Queues (REQ-PL-002)**
- **Bounded Send Queues**: Per-peer queues with configurable size (default 100)
- **Timeout & Drop Policy**: Block with timeout, drop messages when queue full
- **Basic Metrics**: Queue depth tracking and drops counter per peer
- **Queue Management**: Automatic queue creation/cleanup on connect/disconnect

✅ **Phase 3.4: Heartbeats & Health Monitoring (REQ-PL-003)**
- **Health State Enum**: Healthy/Unhealthy/Disconnected with String() method
- **State Tracking**: Health state and failure counters in peer metrics
- **Health Methods**: GetPeerHealth(), GetPeerHealthState(), SetPeerHealth()
- **State Transitions**: Automatic Healthy on Connect, Disconnected on Disconnect
- **Comprehensive Testing**: 15 tests covering all health state scenarios

### **Next Immediate Steps**
🔄 **Phase 3.5: At-Least-Once Delivery Stub** (Next phase)
1. Add basic ACK tracking framework per peer
2. Add methods for tracking highest acknowledged offset per topic
3. Add placeholder logic for resume semantics
4. Keep actual ACK protocol as stub for now
5. Focus on framework over full implementation

### **Disciplined Approach Validation**
- ✅ Small focused batches (single component per commit)
- ✅ TDD with failing tests first
- ✅ All tests passing before commit
- ✅ Clear commit messages with progress tracking
- ✅ Incremental progress with working code at each step

---

*This plan will be updated as implementation progresses and requirements evolve.*