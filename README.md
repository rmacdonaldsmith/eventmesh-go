# EventMesh

EventMesh is a Go event-streaming project for experimenting with local event
persistence, topic-based routing, HTTP publishing, Server-Sent Events streaming,
and peer-to-peer mesh propagation. The idea is to let applications publish and
subscribe through nearby nodes while the mesh shares aggregate topic interest and
routes live events toward interested peers. Publishers do not need to know where
subscribers live, and subscribers can recover recent local history through replay
and SSE resume.

The north star is a lightweight event mesh for observable, replayable,
real-time application coordination: easy for humans, services, and AI agents to
inspect, understand, validate, publish into, and debug safely.

The codebase is functional as a development/MVP system. It is not yet a
production broker: the default storage is in-memory, Pebble is available for
local durability, peer security is not mTLS yet, and some multi-node behavior is
still being hardened.

## Delivery Guarantees

EventMesh does not claim exactly-once delivery. The current design target is
local-first durability with at-least-once, duplicate-tolerant delivery across
the mesh.

- **Publishers:** a successful publish means the event was appended to the
  local node's EventLog before local delivery or peer forwarding.
- **Peer-to-peer mesh:** node-to-node delivery is at-least-once. Retries,
  reconnects, and future replay/resync flows may produce duplicates, so
  receivers must tolerate duplicate event IDs or payloads. A receiving node
  appends an inbound peer event to its local EventLog before local subscriber
  delivery; append failure skips that delivery path.
- **Live subscribers:** active SSE/client streams are best-effort while
  connected. Durable catch-up should use topic replay from the EventLog by
  offset.
- **Exactly-once:** not a current goal. Supporting it would require durable
  acknowledgements, subscriber offsets, idempotency/deduplication storage, and
  stronger transactional boundaries.

## Current Capabilities

- HTTP API for publishing, reading, subscribing, and streaming events
- CLI for day-to-day development workflows
- JWT-based client identity, plus explicit `--no-auth` development mode
- In-memory append-only event log with per-topic offsets and replay
- Optional Pebble-backed durable EventLog for local restart survival
- Node-local SSE resume using `Last-Event-ID` cursors
- In-memory routing table with single-segment wildcard matching, such as
  `orders.*`
- gRPC PeerLink implementation for peer connections, event forwarding, and
  aggregate topic-interest gossip
- Local verification loop through `make`

PeerLink currently separates data-plane user events from control-plane aggregate
topic-interest gossip logically through typed `PeerMessage` frames, separate
interfaces, independent queues, and per-plane metrics. Peers gossip node-level
interest snapshots/deltas, not individual client identities. Both planes are
still multiplexed over one physical gRPC `EventStream`; separate physical data
and control streams are future work if tests or operations show they are needed.

## Quick Start

Requirements:

- Go 1.25.1 or newer
- `make`

Build both binaries:

```bash
make build
```

This creates:

```text
bin/eventmesh
bin/eventmesh-cli
```

Start a development server without authentication:

```bash
./bin/eventmesh --http --no-auth --node-id dev-node
```

Use Pebble for local durable event storage:

```bash
./bin/eventmesh --http --no-auth --node-id dev-node \
  --eventlog-backend pebble \
  --eventlog-path ./data/eventlog
```

In another terminal, publish and read events:

```bash
./bin/eventmesh-cli --no-auth publish \
  --topic test.events \
  --payload '{"message":"hello"}'

./bin/eventmesh-cli --no-auth topics info --topic test.events

./bin/eventmesh-cli --no-auth replay --topic test.events --offset 0
```

Stream a topic:

```bash
./bin/eventmesh-cli --no-auth stream --topic test.events
```

When `stream --topic` is used one or more times, the CLI creates temporary
subscriptions, opens the unified SSE stream, filters matching events locally,
re-ensures those subscriptions across reconnects, and removes subscriptions it
created when the stream closes. Raw HTTP clients must manage subscription
recreation themselves after node restarts.

## Authenticated Flow

For JWT mode, start the server without `--no-auth` and set a secret:

```bash
EVENTMESH_JWT_SECRET="dev-secret" \
  ./bin/eventmesh --http --node-id dev-node
```

Authenticate a client:

```bash
./bin/eventmesh-cli auth --client-id my-client
```

Then use the same `--client-id` for publish, subscribe, stream, replay, and
admin commands. Admin endpoints require a token for client ID `admin`.

## API Summary

Health and auth:

```http
GET  /api/v1/health
POST /api/v1/auth/login
```

Events:

```http
POST /api/v1/events
GET  /api/v1/topics/{topic}/events?offset=0&limit=100
GET  /api/v1/events/stream
```

Subscriptions:

```http
POST   /api/v1/subscriptions
GET    /api/v1/subscriptions
DELETE /api/v1/subscriptions/{id}
```

Admin:

```http
GET /api/v1/admin/clients
GET /api/v1/admin/subscriptions
GET /api/v1/admin/stats
```

Important SSE contract:

- `GET /api/v1/events/stream` streams events for the authenticated client's
  active subscriptions.
- `GET /api/v1/events/stream?topic=...` is intentionally rejected. Create a
  subscription first, or use the Go/CLI client convenience that does this for
  you.
- SSE resume is node-local: resume cursors refer to the serving node's local
  EventLog. Clients should reconnect to the same stable node ID; cross-node
  durable resume is future work.

## Build And Test

```bash
make              # fmt + vet + test + build
make build        # build server and CLI
make build-server # build bin/eventmesh
make build-cli    # build bin/eventmesh-cli
make test         # go test ./... -timeout 2m -v -cover
make test-coverage
```

Agent/sandbox-friendly test command:

```bash
env GOCACHE=/tmp/eventmesh-go-build-cache \
    GOMODCACHE=/tmp/eventmesh-go-mod-cache \
    go test ./... -timeout 2m
```

Some tests open local loopback listeners. In restricted agent sandboxes those
tests may need approved network/loopback permission rather than code changes.

## Development Workflow

EventMesh is developed in small TDD increments. For behavior changes, start
with a failing test, make it pass, then refactor while keeping the suite green.
See [AGENTS.md](AGENTS.md) for the full workflow and local verification loop
used by humans and coding agents.

## Project Layout

```text
cmd/eventmesh/          Server binary
cmd/eventmesh-cli/      CLI binary
internal/eventlog/      In-memory and Pebble EventLog implementations
internal/httpapi/       HTTP handlers, auth, middleware, SSE
internal/meshnode/      Node orchestration and routing decisions
internal/peerlink/      gRPC peer communication
internal/routingtable/  Topic subscription routing
pkg/                    Public interfaces and HTTP client
proto/                  PeerLink protobuf definitions
tests/                  Integration tests
docs/                   Design notes and development references
examples/               Runnable demo scripts
```

## Documentation Map

- [docs/north-star.md](docs/north-star.md) - positioning and agent-friendly
  product direction
- [docs/design.md](docs/design.md) - current architecture and roadmap
- [docs/discovery.md](docs/discovery.md) - future discovery design notes
- [docs/benchmark.md](docs/benchmark.md) - benchmark notes and how to rerun
- [examples/README.md](examples/README.md) - examples overview
- [AGENTS.md](AGENTS.md) - canonical agent workflow guidance
- [CLAUDE.md](CLAUDE.md) - Claude Code compatibility pointer

## Current Roadmap Themes

- Pebble storage hardening, retention, and compaction policy
- Agent-friendly event metadata, discovery, validation, replay, and tooling
- More reliable multi-node subscription propagation and delivery semantics
- mTLS and stronger peer identity
- Production metrics and operational endpoints
- Broader integration/load tests
