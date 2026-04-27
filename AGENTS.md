# AGENTS.md

Note for all agents:
We track work in Beads instead of Markdown. Run \`bd quickstart\` to see how.

**Purpose:**
Provide structured guidance to Codex and human contributors when working on the **EventMesh** Go codebase — a distributed event-streaming system built for secure, real-time communication between applications.

---

## 🧠 1. Project Overview

**EventMesh** is a Go event-streaming project with a working HTTP/CLI MVP and
an in-progress peer-to-peer mesh layer. Treat production claims as roadmap work
unless the code and tests prove the behavior today.

**Core Goals**

* 🔐 Client isolation via JWT authentication
* 📡 Real-time client streaming via SSE and peer transport via gRPC
* 🧱 Modular architecture (EventLog, MeshNode, RoutingTable, PeerLink)
* 🧪 Test-driven, interface-first design
* 🧭 Production-grade orchestration (graceful shutdown, observability)

---

## ⚙️ 2. Architecture Summary

**Core Components**

| Component        | Purpose                                                | Status         |
| ---------------- | ------------------------------------------------------ | -------------- |
| **EventLog**     | In-memory append-only event storage with replay        | ✅ MVP implemented |
| **MeshNode**     | Core orchestration: clients, routes, lifecycle         | ✅ MVP implemented |
| **RoutingTable** | Topic subscriptions with single-segment wildcards      | ✅ Implemented  |
| **PeerLink**     | gRPC peer communication layer                          | 🚧 In progress |
| **HTTP API**     | REST + SSE interface for external clients              | ✅ Implemented  |

**Design Principles**

1. **Local persistence first** – events are written before being forwarded.
2. **Smart propagation** – subscriptions shared across peers for efficiency.
3. **Interface-driven design** – public contracts defined before implementations.
4. **Eventual consistency** – delivery guarantees are local-first and
   at-least-once, not exactly-once.
5. **Observability built-in** – metrics, health checks, structured logging.

**Delivery guarantee contract:**

* A successful publish means the event was appended to the local EventLog before
  local subscriber delivery or peer forwarding.
* Peer-to-peer mesh delivery is at-least-once and duplicate-tolerant. Retries,
  reconnects, and future replay/resync flows may produce duplicates.
* Live SSE/client delivery is best-effort while connected. Durable catch-up uses
  topic replay from the EventLog by offset.
* Do not write tests or docs that imply exactly-once delivery unless the feature
  is explicitly designed and implemented later.

---

## 🧩 3. Codebase Layout

```
eventmesh-go/
├── cmd/eventmesh/         # Server binary
├── cmd/eventmesh-cli/     # CLI binary
├── pkg/                   # Public interfaces and clients
│   ├── httpclient/        # HTTP client and SSE streaming
│   ├── eventlog/          # EventLog contracts
│   └── meshnode/          # MeshNode interfaces
├── internal/              # Implementations
│   ├── eventlog/          # In-memory EventLog
│   ├── meshnode/          # Core orchestration
│   ├── routingtable/      # Subscription routing
│   ├── peerlink/          # Networking layer
│   └── httpapi/           # HTTP API implementation
├── tests/                 # Integration tests
└── docs/                  # Current docs and future design notes
```

---

## 🧪 4. Development Workflow

**Guiding Philosophy:**

> “Move in small, test-driven increments. Let tests drive the design.”

### Workflow (for both Human + Codex)

1. **TDD Loop:**

   * **RED:** Write or update a failing test first for behavior changes.
   * **GREEN:** Implement minimal code to pass it.
   * **REFACTOR:** Simplify while keeping all tests green.

   Do not implement from prose alone when a meaningful test can express the
   expected behavior. If the desired behavior is hard to test, capture the
   missing test seam in Beads before or alongside the implementation.

2. **Work in Small Batches:**

   * Implement one small feature or fix at a time.
   * Keep PRs under ~200 lines when possible.
   * Avoid parallel feature work; maintain focus.

3. **Frequent Commits:**

   * Commit after each small passing test or logical unit.
   * Prefer multiple small commits over large ones.
   * Push regularly to keep the branch in sync.

4. **Pairing with Codex:**

   * Use Codex to generate scaffolding, docstrings, or table-driven tests.
   * Human reviews for design clarity and idiomatic Go style.
   * Codex never commits directly; the human must review and test first.

5. **Validation Before Commit:**

   ```bash
   make        # Recommended: runs fmt + vet + test + build
   # OR
   go fmt ./... && go vet ./... && go test ./...
   ```

   All tests **must pass** before merging.

### Closing the Loop

Agents should not stop at writing code. Every change should end with a local
feedback loop that proves the work is correct, or with a clear note explaining
what could not be verified.

**Default loop for code changes:**

```bash
bd --no-db show <issue-id>        # Confirm the active Bead and acceptance criteria
go fmt ./...                      # Or make fmt
go test ./... -timeout 2m         # Or make test
make build                        # Builds both server and CLI
bd --no-db close <issue-id> --reason "..."  # Only after verification passes
```

**Sandboxed agent loop:**

Some agent sandboxes cannot write to the normal Go cache or module cache. In
that case, keep the human's Go environment untouched and use temporary caches:

```bash
env GOCACHE=/tmp/eventmesh-go-build-cache \
    GOMODCACHE=/tmp/eventmesh-go-mod-cache \
    go test ./... -timeout 2m
```

Networking tests use local loopback listeners. If a sandbox blocks bind/connect
operations, rerun the same test with the tool's approved elevated/network
permission rather than weakening the test.

**Fast lanes:**

```bash
go test ./internal/httpapi ./internal/meshnode -timeout 2m
go test ./cmd/eventmesh ./cmd/eventmesh-cli -timeout 2m
```

**Full local CI:**

```bash
make
```

Before handing work back, agents should report the exact commands run and their
results. If a command fails, capture the failure in Beads or create a follow-up
issue rather than leaving it as conversation-only context.

---

## 🧰 5. Build & Test Commands

```bash
# Use Makefile (recommended)
make help           # Show all available targets
make                # Run full pipeline: fmt + vet + test + build
make build          # Build server and CLI binaries
make build-server   # Build server binary to bin/eventmesh
make build-cli      # Build CLI binary to bin/eventmesh-cli
make test           # Run all tests with coverage
make test-coverage  # Generate detailed HTML coverage report
make fmt            # Format Go code
make vet            # Run go vet static analysis
make clean          # Remove build artifacts
make run            # Build and start development server

# Direct Go commands (if needed)
go build -o bin/eventmesh ./cmd/eventmesh
go build -o bin/eventmesh-cli ./cmd/eventmesh-cli
go test ./... -timeout 2m -v
go test ./... -timeout 2m -cover
go fmt ./...
go vet ./...

# Advanced linting (optional)
golangci-lint run   # Comprehensive linting (43 minor issues as of latest)
```

Tests are organized by concern:

* **Unit tests:** internal/component_name/*_test.go
* **Integration tests:** tests/*_test.go
* **Benchmarks:** included in performance-critical modules

---

## 🌐 6. API Overview

**Client APIs (JWT-protected):**

```http
POST /api/v1/auth/login              # Authenticate client
POST /api/v1/events                  # Publish event
GET  /api/v1/events/stream           # SSE for active subscriptions
GET  /api/v1/topics/{topic}/events   # Read/replay events by topic and offset
POST /api/v1/subscriptions           # Add topic subscription
GET  /api/v1/subscriptions           # List client subscriptions
DELETE /api/v1/subscriptions/{id}    # Remove subscription
```

`GET /api/v1/events/stream?topic=...` is intentionally rejected. Use
`POST /api/v1/subscriptions` first, or use the CLI/Go client `StreamConfig.Topic`
convenience that creates a temporary subscription and cleans it up on close.

**Admin APIs:**

```http
GET /api/v1/admin/clients            # List connected clients
GET /api/v1/admin/subscriptions      # List all subscriptions
GET /api/v1/health                   # Health check
```

---

## 🧩 7. Development Patterns

**Error Handling**

* Never panic; always return explicit `error`s.
* Include context in error messages.

**Concurrency**

* Use `context.Context` for all long-running operations.
* Use channels for event delivery, mutexes for shared state.

**Interface Verification**

```go
var _ EventLog = (*InMemoryEventLog)(nil)
```

**Logging**

* Use structured logging (zap/log) for consistent observability.

---

## ✅ 8. Quality Standards

| Principle              | Description                                |
| ---------------------- | ------------------------------------------ |
| **Interface-first**    | Define contracts before implementations    |
| **Defensive design**   | Protect against misuse and race conditions |
| **Zero circular deps** | Keep pkg → internal separation clean       |
| **Resource cleanup**   | Always implement `Close()` properly        |
| **Readable first**     | Optimize for clarity, not cleverness       |

---

## 🧭 9. Troubleshooting

```bash
# Run specific tests
go test ./internal/meshnode -v -run TestGRPCMeshNode_AuthenticateClient

# Clean & rebuild
make clean && make build
# OR
go clean ./... && go build ./...

# Fix import issues
go mod tidy && go mod verify

# Check code quality
make fmt && make vet
golangci-lint run  # Shows 43 minor style issues (non-critical)
```

---

## 🤖 10. AI Collaboration Guidelines

When assisting in this repository, **Codex should:**

1. **Follow the TDD Workflow** — write or update a failing test before implementing behavior changes.
2. **Propose incremental changes** — modify one function or module per iteration.
3. **Explain design rationale** — add short docstrings summarizing intent.
4. **Avoid assumptions** — ask for clarification when design is ambiguous.
5. **Maintain readability** — match idiomatic Go style and comments.
6. **Prefer correctness over performance** in early iterations.
7. **Never overwrite tests** — new code must extend existing coverage, not replace it.

### Documentation Maintenance

* Prefer documenting current behavior before roadmap ideas.
* Mark future design explicitly as future or planned.
* Keep commands copy/pasteable from the repository root unless noted.
* When API behavior changes, update the root README, design doc, examples, and
  agent guidance in the same change.

---

## 📘 11. Key References

* `docs/design.md` — current architecture, limits, and roadmap
* `docs/discovery.md` — future discovery design notes
* `README.md` — quickstart and current API summary

---
