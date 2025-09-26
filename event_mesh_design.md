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

#### REQ-LOG-001 Append-Only Storage

**Description:** The Event Log must persist events in append-only segments, never overwriting existing records.\
**Success Criteria:**

- Appending never overwrites existing records.
- Events are durable after crashes.
- Corrupted writes are detected and truncated to the last valid record.

#### REQ-LOG-002 Replay Support

**Description:** The Event Log must support replay from any offset for local and remote subscribers.\
**Success Criteria:**

- Replay starts from the chosen offset.
- Ordering of events is preserved.
- Replay matches committed events with no gaps.

#### REQ-LOG-003 Crash Recovery

**Description:** On restart, the Event Log must recover to the last committed state without data loss.\
**Success Criteria:**

- Log validates and reopens cleanly.
- Corrupted tails are truncated.
- Replay works after restart.

#### REQ-LOG-004 Indexing

**Description:** Maintain an offset-based index for efficient seek during replay.\
**Success Criteria:**

- Seek time is O(log n) for offsets.
- Index survives restart and crash recovery.

### Routing Table Requirements

- REQ-RT-001 Wildcard Matching
- REQ-RT-002 Gossip-based Rebuild
- REQ-RT-003 Efficient Lookup

### Peer Link Requirements

- REQ-PL-001 Secure Connection Lifecycle
- REQ-PL-002 Backpressure and Flow Control
- REQ-PL-003 Heartbeats and Failure Detection

### Mesh Node Requirements

- REQ-MNODE-001 Authentication of Clients
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

