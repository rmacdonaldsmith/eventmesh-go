# Event Mesh Design

## Introduction

This document describes the design of a lightweight event mesh system. The goal is to enable multiple mesh nodes to interconnect securely and route published events to subscribers across the network. It covers the core functionality expected in the MVP (publish/subscribe messaging, routing, durability, subscription management, and client SDK), the non-functional requirements (performance, availability, security, observability, etc.), and the detailed roles of each core component. Diagrams and sequence flows illustrate how events move through the system and how modules interact to provide reliable delivery and resilience.

## Problem Statement & Goals

Modern distributed applications need reliable, low-latency event delivery across multiple services, regions, and environments. Existing messaging systems often trade off durability, replay, or multi-node scalability. The purpose of this design is to create a lightweight event mesh MVP that fills this gap: simple to operate, secure by default, and extensible for future replication and advanced features.

At the same time, systems are generating ever-larger volumes of data and increasingly shifting toward event-driven architectures (microservices, AI/ML pipelines, IoT). These trends demand real-time ingestion, routing, and processing. Building an event mesh directly serves this trajectory, providing a foundation for scalable, event-first systems. Industry leaders have validated this direction through platforms like Apache Kafka and newer Confluent products (Cluster Linking, ksqlDB, etc.), which demonstrate the value of durable event logs, replay, and global-scale event fabrics. This MVP positions itself as a lightweight alternative in the same lineage, designed for simplicity and extensibility.



## Core Functionality

- **Publish/Subscribe Messaging:** Clients must be able to publish events to topics and subscribe to topics. System guarantees at-least-once delivery. Message ordering is preserved per topic partition.
- **Multi-node Event Routing:** Nodes must forward events across the mesh to reach interested subscribers. Routing decisions are based on topic subscriptions disseminated via gossip.
- **Durable Logging:** Each node persists locally published events and remote events if they have local subscribers or peers needing them (hybrid model). Persistence must survive process restarts.
- **Subscription Management:** Support topic-based subscriptions with wildcard matching (e.g., `foo.*`). Subscriptions must propagate quickly across the mesh.
- **Security:** Basic transport-level security (mTLS) and client authentication via tokens or API keys. Unauthorized clients cannot publish or subscribe.
- **Client SDK Support:** Provide a client library exposing publish/subscribe APIs, handling retries, backoff, and reconnections transparently.



## Non Functional Requirements

- **Performance:** Handle at least X events per second per node (target TBD). Latency for local publish→subscribe delivery should be < Y ms.
- **Availability:** System should tolerate node failures without data loss of locally published events; subscribers should be able to reconnect and resume.
- **Scalability:** Must scale horizontally by adding more nodes; routing and gossip must remain efficient as cluster grows.
- **Durability:** Events from local publishers must not be lost once acknowledged; log persistence must survive restarts.
- **Security:** All inter-node and client-node communication must use TLS/mTLS. Tokens or API keys should be used for client authentication.
- **Observability:** Expose metrics, logging, and tracing hooks to monitor throughput, latency, errors, and peer health.
- **Operability:** Deployment must be container-friendly; support for orchestration (Kubernetes, Helm) expected.
- **Extensibility:** Code should be modular to support replication, archiving, and new features without major refactor.
- **Testability:** System must be testable in unit, integration, and distributed scenarios; tests should be automated and repeatable.

---

## Core Components

- **Mesh Node:** Hosts the event log, routing table, and peer links. Orchestrates publishing, subscribing, and routing. Manages authentication and client connections.
- **Event Log:** Append-only log of locally published events and relevant remote events (hybrid model). Ensures durability and supports replay.
- **Routing Table:** In-memory mapping of topics → local subscribers and peer nodes. Updated dynamically based on gossip messages. Must support wildcard matching and efficient lookups. For MVP, it remains memory-only and can be reconstructed on restart via gossip resync; the routing table is rebuilt from gossip on restart. Persistence to disk is not required initially but may be added in later versions for faster recovery.
- **Peer Links:** A component within each mesh node that abstracts and manages the connection between peer mesh nodes. These are persistent streaming connections, typically implemented over gRPC with mTLS. They handle connection lifecycle, data streaming, retransmission, health monitoring, and control data.
- **Membership & Gossip Protocol:** Disseminates cluster membership and subscription data. Ensures that new nodes can rebuild routing tables and integrate quickly.
- **Client SDK:** Language-specific libraries exposing publish/subscribe APIs. Must handle retries, exponential backoff, reconnections, and abstract protocol details. SDK should also expose hooks for metrics and logging. Clients connect to mesh nodes using **gRPC over HTTP/2 with mTLS** as the baseline protocol. This ensures efficient streaming, bi-directional communication, and security. Alternative lightweight protocols (e.g., WebSockets, MQTT) are explicitly out of scope for the MVP and will not be supported initially.
- **Security Layer:** Provides TLS/mTLS, client authentication, and optional ACLs for topic-level authorization.
- **Monitoring & Metrics Module:** Exposes Prometheus-style metrics, health checks, logs, and traces for observability.

---

## Module Interaction Diagram

```
+-----------+       +-------------+       +-----------------+
|  Clients  | <---> | Mesh Node   | <---> | Peer Links      |
+-----------+       |             |       | (gRPC/mTLS)     |
                    |             |       +-----------------+
                    |  +---------+|
                    |  |Routing  ||
                    |  | Table   ||
                    |  +---------+|
                    |  +---------+|
                    |  |Event Log||
                    |  +---------+|
                    |             |
                    +-------------+
                          ^
                          |
                   +--------------+
                   | Monitoring & |
                   |  Metrics     |
                   +--------------+
                          ^
                          |
                   +--------------+
                   | Security     |
                   | Layer        |
                   +--------------+
```

This diagram illustrates the interaction between modules: clients interact with mesh nodes, which manage routing and logs. Nodes interconnect via secure peer links. Cross-cutting concerns like monitoring and security layer support the system.

---

## Detailed Component Requirements

Each component has a set of detailed, testable requirements expressed using the schema defined earlier. These requirements will guide implementation and validation.

### Event Log Requirements

#### REQ-LOG-001 Append-Only Storage (Per Topic/Partition)

**Description:**  
The Event Log must persist events in append-only segments on a per-topic (or per-topic/partition) basis, never overwriting existing records.  

**Success Criteria:**  
- Appending never overwrites existing records within a topic/partition.  
- Events are durable after crashes.  
- Corrupted writes are detected and truncated to the last valid record.  

#### REQ-LOG-002 Replay Support (Per Topic/Partition)

**Description:**  
The Event Log must support replay from any offset within a topic/partition for local and remote subscribers.  

**Success Criteria:**  
- Replay starts from the chosen offset within a topic.  
- Ordering of events is preserved per topic/partition.  
- Replay matches committed events with no gaps.  

#### REQ-LOG-003 Crash Recovery

**Description:**  
On restart, the Event Log must recover to the last committed state without data loss.  

**Success Criteria:**  
- Log validates and reopens cleanly.  
- Corrupted tails are truncated.  
- Replay works after restart.  

#### REQ-LOG-004 Indexing (Per Topic/Partition)

**Description:**  
Maintain an offset-based index per topic/partition for efficient seek during replay.  

**Success Criteria:**  
- Seek time is O(log n) for offsets within a topic.  
- Index survives restart and crash recovery.  

#### REQ-LOG-005 Topic-Based Retrieval

**Description:**  
The Event Log must support efficient retrieval of events scoped to a given topic or topic/partition.  

**Success Criteria:**  
- Queries for a topic only scan/index that topic’s log.  
- Retrieval latency is predictable even as the number of topics grows.  
- Retrieval works for both sequential read and offset-based seek.  

#### REQ-LOG-006 Topic-Based Replay Semantics

**Description:**  
The Event Log must allow subscribers to re-consume events from a chosen topic (or topic/partition) starting from a stored offset or timestamp.  

**Success Criteria:**  
- Subscribers can resume from their last known offset within a topic.  
- Replay preserves per-topic ordering guarantees.  
- Replay across multiple topics is independent; a slow replay on one topic does not block others.  


### Routing Table Requirements

- REQ-RT-001 Wildcard Matching
- REQ-RT-002 Gossip-based Rebuild
- REQ-RT-003 Efficient Lookup

### Peer Link Requirements

Each mesh node uses **PeerLinks** to maintain persistent connections to other mesh nodes. These connections form the backbone of the mesh, allowing events and subscription data to flow between nodes. PeerLinks are expected to handle:

- Connection lifecycle (establish, maintain, reconnect, close).
- Data streaming (events, subscription updates).
- Backpressure and retransmission.
- Health monitoring (heartbeats, failure detection).

Security (mTLS) is **deferred from MVP**. An appendix in the main design doc outlines a future approach using Smallstep.

- **REQ-PL-001 Connection Lifecycle**  
  *Guarantee a single, long‑lived connection per peer with automatic reconnects.*  
  **Acceptance Criteria**  
  - gRPC/HTTP2 implementation for peer to peer communication
  - Peer identity (`node_id`) exchanged during handshake.  
  - Exactly one active connection between any two peers (de‑dup rules when both dial: keep higher `node_id` as dialer, close the other).  
  - Reconnect with exponential backoff + jitter; max backoff configurable; cancellation via context.  
  - Graceful shutdown closes stream and drains in‑flight sends.

- **REQ-PL-002 Minimal Bounded Flow Control (MVP)**  
  *Prevent unbounded memory growth; advanced credit/priority control deferred to backlog.*  
  **Acceptance Criteria**  
  - Outbound `sendQueue` is bounded (configurable).  
  - If queue is full: block with timeout; on timeout either drop oldest or drop new (policy configurable).  
  - gRPC/HTTP2 stream backpressure is leveraged; no busy loops when peer is slow.  
  - Metrics for queue depth, drops, and send latency are exposed.

- **REQ-PL-003 Heartbeats & Failure Detection**  
  *Detect peer liveness and surface state changes to membership/routing.*  
  **Acceptance Criteria**  
  - Periodic ping (e.g., 2s–10s configurable) with deadline; 3 consecutive failures ⇒ mark `Unhealthy`.  
  - State transitions (Healthy/Unhealthy/Disconnected) emitted as events.  

- **DROPPED FROM THE PLAN - REQ-PL-004 At‑Least‑Once Cross‑Node Delivery (ACK/Resume)**  
  *Ensure resend after disconnect without gaps; duplicates are acceptable.*  
  **Acceptance Criteria**  
  - Per (topic) highest‑acked offset tracked per peer.  
  - Receiver acks contiguously applied offsets (coalesced).  
  - On reconnect, sender resumes from last ack+1 using EventLog replay.  
  - Receiver applies idempotently (drop duplicates by offset) and preserves per‑partition ordering.

- **REQ-PL-005 Protocol Version & Peer Identity**  
  *Make wire protocol evolvable and safe.*  
  **Acceptance Criteria**  
  - Handshake exchanges `protocol_version`, `node_id`, and optional `features`.  
  - Reject incompatible major versions.  
  - Minor versions may carry feature flags for optional capabilities.  
  - **Feature negotiation**: during handshake, peers advertise supported feature flags (e.g., compression, batching). MVP only requires validating protocol version and exchanging `node_id`; full feature negotiation can be deferred. This provides a forward‑compatible hook without implementing complex branching logic in MVP.

- **REQ-PL-006 Observability**  
  *Expose minimal metrics/logs for ops.*  
  **Acceptance Criteria**  
  - Metrics: connection_state, reconnects_total, bytes_sent/received, messages_sent/received, queue_depth, drops_total, rtt_ms (heartbeat), send_latency_ms p50/p95.  
  - Structured logs for connect/reconnect/close, backoff, errors; trace spans around send/recv.

- **REQ-PL-007 Config & Limits**  
  *Centralize knobs and safe defaults.*  
  **Acceptance Criteria**  
  - Static peer list (host:port, node_id), queue sizes, timeouts, max_message_bytes, backoff params.  
  - Hard caps on concurrent streams (1 per peer in MVP), and message size enforcement.

### PeerLink Protocol Definition

```proto
syntax = "proto3";

service PeerLink {
  rpc EventStream(stream PeerMessage) returns (stream PeerMessage);
}

message PeerMessage { // Wrapper frame type. Acts as an envelope type that wraps all possible protocol messages (handshake, event, ack, heartbeat) into a single frame so they can all be sent over one gRPC streaming RPC. This avoids defining multiple RPC methods or streams.
  oneof msg {
    Handshake handshake = 1;
    Event event = 2;
    Ack ack = 3;
    Heartbeat heartbeat = 4;
  }
}

message Handshake {
  string node_id = 1;
  int32 protocol_version = 2;
  repeated string features = 3; // optional
}

message Event {
  string topic = 1;
  int64 offset = 2;
  bytes payload = 3;
  map<string,string> headers = 4;
}

message Ack {
  string topic = 1;
  int64 offset = 2;
}

message Heartbeat {
  int64 timestamp = 1;
}
```

### Peer Link Protocol Stages

#### Stage A: Connection & Handshake

After both handshake messages are exchanged and validated, there’s nothing extra to do at the gRPC/HTTP2 layer. The bidirectional stream is already established at the transport level. At the application layer, you simply mark the connection state as Connected and begin normal message exchange (events, acks, heartbeats). Any further protocol work—such as replaying missed events—should happen at the app layer, not the gRPC transport level.

- **Initiator → Responder**: Sends `PeerMessage{handshake}` with `node_id`, `protocol_version`. The expectation is that the Initiator always sends first after dialing.
- **Responder → Initiator**: Replies with its own `handshake`. The expectation is that the Responder waits for the Initiator’s handshake, then responds.
- Both sides validate:
  - Major version must match.
  - `node_id` must not equal local (self-loop).
- If invalid → close stream.
- On success → transition state to **Connected**.

#### Stage B: Event Delivery

- For each event that should be forwarded:
  - Wrap as `PeerMessage{event}` with topic, offset, payload, headers.
  - Send over the stream.
- Receiver, on receiving `event`:
  - Persist via the EventLog into local log - durable write.
  - Apply to local subscribers (asynchronously; ACK is not delayed until downstream delivery completes).
  - Issue `Ack` with highest contiguous offset applied for that topic. ACK is sent after durable append, not after full propagation, to keep latency bounded.

#### Stage C: Acknowledgements & Resume

- Sender tracks the last `Ack` **per topic** so it can resume correctly for each stream of events.
- ACKs include both the topic name and highest contiguous offset applied for that topic.
- Even though the EventLog is append-only, the application maintains an in-memory mapping of per-topic offsets to support replay-by-topic.
- If connection drops:
  - On reconnect, sender replays from `lastAck+1` for each topic using `EventLog.Replay(topic, offset)`.
- Duplicate events are safe — idempotent application ensures correctness.

#### Stage D: Heartbeats

- Periodically send `Heartbeat{timestamp}`.
- Receiver replies with an ack heartbeat or measures RTT.
- Missed heartbeats beyond threshold → mark peer unhealthy, trigger reconnect.

#### Stage E: Graceful Shutdown

- Either side may close the stream.
- Sender flushes events, receiver sends final acks.
- Transition state → Disconnected, schedule reconnect with backoff.

### Mesh Node Requirements

- REQ-MNODE-001 Authentication of Clients (descoped from MVP - will add post MVP)
- REQ-MNODE-002 Local Persistence Before Forwarding
- REQ-MNODE-003 Subscription Propagation

### Client SDK Requirements

- REQ-SDK-001 Transparent Retry and Reconnect
- REQ-SDK-002 Expose Publish/Subscribe APIs
- REQ-SDK-003 Provide Metrics Hooks

---

## Technical Decisions

### TD-001 Event Log Backend: RocksDB

**Decision:** Use RocksDB as the underlying storage engine for the Event Log in the MVP.\
**Rationale:** RocksDB is optimized for high-throughput, write-heavy workloads and supports efficient sequential appends and replay, aligning closely with the requirements of an event log. It provides durability, compaction, and proven production use in similar distributed systems.\
**Alternatives Considered:** SQLite in WAL mode (simpler integration, but less suited to high-throughput streaming append workloads), file-based logs, or building a custom log. These were deferred in favor of RocksDB for performance and reliability.\
**Status:** Accepted.

---

## Post MVP Work

- **Retention Policy:** Support time-based or size-based log retention, ensuring expired segments are removed safely without affecting active readers.
- **Log Compaction:** Provide optional log compaction to discard older events or keep only the latest event per key for space efficiency.
- **Peer Link:** follow up work to support secutity and flow / backpressure controls.
- mTLS support (handshake, certs via Smallstep).  
- Advanced flow control (credits, prioritization, rate limiting).  
- Multi‑stream per peer (control vs data; per‑partition streams).  
- Automatic peer discovery; NAT traversal.  
- Cert rotation; mutual auth via SPIFFE/SPIRE.  
- Adaptive batching and compression.  
- Per‑topic quotas and fairness.

---

## Appendix A: Publish→Subscribe Sequence Diagram

```
Client → Mesh Node A: Publish(Event)
Mesh Node A → Mesh Node A: Append to Event Log
Mesh Node A → Mesh Node A: Update Routing Tbl
Mesh Node A → Peer Link: Forward Event
Peer Link → Mesh Node B: Deliver Event
Mesh Node B → Mesh Node B: Append to Event Log
Mesh Node B → Subscriber: Deliver Event
```

### Commentary

1. Client publishes an event to Mesh Node A.
2. Mesh Node A authenticates, validates, and appends it to its Event Log.
3. Mesh Node A updates its Routing Table.
4. Mesh Node A passes the event to its Peer Link, which streams it securely to Mesh Node B.
5. Mesh Node B appends the event to its local Event Log.
6. Mesh Node B delivers the event to its subscriber.

This sequence shows clear separation of responsibilities: nodes handle persistence and routing decisions; peer links only transmit events; and subscribers receive events once they are safely logged on their connected node.

## Appendix B: Secure mTLS Between Mesh Nodes

### mTLS & PKI Requirements

**CA Setup**
Use smallstep step-ca as the internal Certificate Authority.
Single containerized service running inside the mesh.
Root CA kept offline; Intermediate CA used for signing.

**Node Enrollment**
Each Go node (container) generates its own keypair on startup.
Certificates are issued by step-ca via provisioners (password/JWK/secret).
All nodes are provisioned with the root CA certificate to establish trust.

**mTLS Configuration**
Services use tls.RequireAndVerifyClientCert.
Clients verify server certificates against the CA bundle.
Node identities encoded in certificate SANs (e.g., spiffe://mesh/prod/node-42).
Optional Go hook (VerifyPeerCertificate) to enforce authorization policies.

**Rotation & Revocation**
Certificates are short-lived (e.g., 24h).
Nodes run step ca renew --daemon for automatic renewal.
Revocation handled implicitly by expiration (no CRL/OCSP needed).

**Rationale**
Smallstep is lightweight, battle-tested PKI.
Minimal infra overhead compared to Vault/Istio.
Automated lifecycle management, secure by default.
Scales well for dozens–hundreds of nodes in an MVP.