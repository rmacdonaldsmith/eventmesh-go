# Event Mesh Design Document

## 1. Introduction

This document outlines the architectural vision and design considerations for building a distributed Event Mesh system in Go. The goal is to capture the high-level operating models, trade-offs, and implementation roadmap, proceeding from abstract concepts down to concrete technical approaches.

---

## 2. Goals and Principles

* **Flexibility:** Support both lightweight delivery and durable event log use cases.
* **Configurable:** Allow operators to choose routing and durability modes without API changes.
* **Simplicity First:** MVP should favor broadcast simplicity and evolve to more complex routing.
* **Observability:** Provide clear metrics for delivery, lag, and replication health.
* **Scalability:** Operate efficiently across nodes and zones, with bandwidth awareness.

---

## 3. Operating Models

### 3.1 Delivery Fabric Mode

* **Definition:** Event Mesh as a transient delivery buffer.
* **Durability:** Responsibility lies with applications/systems consuming events.
* **Characteristics:**

  * Low overhead and fast propagation.
  * Event logs are ephemeral and divergent across nodes.
  * Good for pipelines where durability exists elsewhere (DB, Kafka, S3).

### 3.2 Event Sink Mode

* **Definition:** Special nodes designated as **event sinks**.
* **Durability:** Mesh guarantees delivery of all events to sinks.
* **Characteristics:**

  * Sinks persist authoritative, durable logs.
  * Support replay for missed events.
  * Useful for compliance, audit, and inter-zone replication.

### 3.3 Hybrid Configurability

* System must allow operators to configure:

  * `mode: delivery-fabric` or `event-sinks`
  * Broadcast vs. smart routing
  * Sink replication strategies (local vs. cross-zone)

---

## 4. Routing Approaches

### 4.1 Broadcast Routing (MVP)

* All events are propagated to all peers.
* Ensures simple, consistent visibility.
* High bandwidth overhead, but lowest complexity.

### 4.2 Smart Routing (Future)

* Subscribers register interests.
* Publishers optionally register topics/contracts.
* Events are routed only to interested peers.
* Saves bandwidth but leads to divergent logs.

### 4.3 Sink-aware Routing

* Regardless of routing mode, sinks always receive **all events**.
* Ensures durable coverage even in smart routing mode.

---

## 5. Multi-Zone / Geo Distribution

* **Edge Sinks per Zone:** Persist all local events.
* **Cross-Zone Replication:** Asynchronous, compressed, batched.
* **Ordering:** Global total ordering not guaranteed; per-key ordering preserved.
* **Resilience:** Durable queues, catch-up range pulls, deduplication.

---

## 6. Observability and Verification

* **Coverage Metrics:** Confirm all sinks receive full event streams.
* **Replication Lag:** Track cross-zone delays.
* **Gap Detection:** Use sequence tracking, digest beacons, and gap reports.

---

## 7. Security

* **Authentication:** mTLS between nodes.
* **Authorization:** ACLs for publisher/subscriber roles.
* **Audit:** Sinks serve as authoritative event history.

---

## 8. Implementation Roadmap

### Phase 1: MVP

* Broadcast mode.
* Delivery fabric semantics.
* Minimal sink prototype (subscribe-to-all).

### Phase 2: Smart Routing

* Subscription gossip.
* Routing tables.
* Sink inclusion logic.

### Phase 3: Sink Durability

* Durable storage.
* Replay RPCs.
* Coverage verification.

### Phase 4: Geo Distribution

* Zone-local sinks.
* Asynchronous inter-zone replication.
* Metrics for replication lag.

### Phase 5: Advanced Features

* QoS tiers.
* Tiered storage.
* Fine-grained ACLs.

---

## 9. Open Questions

* Should sinks provide strong guarantees (ACK before publish returns) or eventual guarantees?
* How do we balance simplicity vs. early introduction of durability features?
* Should sinks expose a Kafka-like API for replay?

---

## 10. Next Steps

* Finalize MVP scope: broadcast + delivery fabric.
* Define proto schema for subscriptions, events, and sink RPCs.
* Sketch gossip message formats.
* Decide on replay protocol details for sinks.

---

**Author:** Rob Macdonald Smith
**Date:** October 2025
