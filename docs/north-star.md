# EventMesh North Star

EventMesh should grow toward a lightweight event mesh for observable,
replayable, real-time application coordination.

The project is not trying to become a smaller Kafka. Kafka, Pulsar, RabbitMQ,
NATS, Redis Streams, MQTT, and cloud pub/sub systems already cover many mature
broker and data-platform needs. EventMesh is more interesting as a small,
understandable, HTTP-native event layer that can run near applications, persist
locally first, stream to clients, and eventually coordinate with peers.

The strongest product direction is agent-friendly by design:

> EventMesh should be easy for humans, services, and AI agents to inspect,
> understand, replay, validate, publish into, and debug safely.

## Positioning

EventMesh is best explored as:

- an application event mesh for small distributed systems
- a real-time delivery layer with local replay and client isolation
- a learning-friendly implementation whose internals stay understandable
- an agent-operable coordination fabric rather than a general data platform

It should not claim to replace Kafka-class systems for high-throughput durable
logs, long-term retention, consumer groups, schema governance, compliance-grade
audit trails, or mature production operations until those capabilities exist.

## Agent-Friendly Principles

Agent-friendly does not mean letting an LLM do anything it wants. It means the
system exposes enough structure for an agent to reason about it, and enough
guardrails for the agent to act safely.

- Self-describing events: events should carry clear metadata, stable IDs,
  timestamps, source information, schema references, correlation IDs, and
  eventually optional human/agent-readable summaries.
- Discoverability: agents should be able to list topics, inspect schemas,
  understand who publishes or subscribes, and discover supported capabilities
  through explicit APIs.
- Safe publishing: agents should be able to validate a draft event before
  publishing, with schema, authorization, and topic checks performed by
  EventMesh.
- Replay as context: agents should be able to recover recent history by topic,
  source, client, correlation ID, trace ID, or time window.
- Explainable errors: API errors should be structured and specific enough for a
  client or agent to correct the request without hidden operator knowledge.
- Tool-native access: EventMesh should eventually expose an MCP server or
  equivalent tool surface for agent workflows.

## Near-Term Design Questions

These are roadmap concerns, not current implementation facts:

- What is the minimum event metadata envelope that helps agents without making
  normal application publishing awkward?
- Should schema support start as JSON Schema references, inline schema
  registration, or documentation-only examples?
- What discovery endpoints are useful before persistent storage exists?
- How should draft/validate/publish work without creating a second publishing
  path?
- What replay and timeline views are most helpful for debugging and agent
  context?
- Which operations belong in an MCP server versus the HTTP API and CLI?

## Design Bias

Prefer small, testable increments that make the system more inspectable before
making it more magical. The agent-friendly surface should be built out of
ordinary, documented APIs first, then exposed through CLI and MCP tooling.
