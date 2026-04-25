# Benchmark Tests

EventMesh does not have a full benchmark story yet. The current benchmark tests
are useful for local development feedback, not production throughput claims.

## Current Benchmarks

The active benchmark file is:

- `internal/routingtable/benchmark_test.go`

It exercises routing-table subscription and lookup behavior, including
concurrent operations and larger subscriber/topic counts.

## Run Them

From the repository root:

```bash
go test -bench=. ./internal/routingtable -benchmem
```

Sandbox-friendly form:

```bash
env GOCACHE=/tmp/eventmesh-go-build-cache \
    GOMODCACHE=/tmp/eventmesh-go-mod-cache \
    go test -bench=. ./internal/routingtable -benchmem
```

## Later

Rebuild this document when the implementation is ready for broader performance
work, especially around persistent EventLog storage, HTTP/SSE fan-out, and
multi-node PeerLink behavior.
