# Performance checks

This repository includes repeatable Go benchmarks and opt-in smoke tests for
performance confidence. They are intended to identify bottlenecks and regressions;
they are not production capacity guarantees.

## Benchmark commands

Run from the repository root:

```bash
go test -bench='BenchmarkInMemoryRoutingTable_GetSubscribers|BenchmarkInMemoryRoutingTable_GetSubscribersWildcard' -benchmem ./internal/routingtable -run '^$' -benchtime=1s
go test -bench='BenchmarkGRPCMeshNode_(PublishOnly|LocalFanout100|ManySubscriptionsMixedTopics)$' -benchmem ./internal/meshnode -run '^$' -benchtime=1s
go test -bench='BenchmarkGRPCPeerLink_ReceiveEvents' -benchmem ./internal/peerlink -run '^$' -benchtime=1s -timeout 2m
```

Opt-in sustained smoke tests are skipped by default so normal `go test ./...`
stays fast and deterministic:

```bash
EVENTMESH_PERF_SMOKE=1 go test ./internal/meshnode ./internal/peerlink -run 'LoadSmoke|SustainedResourceSmoke' -timeout 2m -v
```

## Baseline run

Environment:

- Date: 2026-05-20
- OS/arch: darwin/arm64
- CPU: Apple M1
- Storage backend: in-memory EventLog unless noted by the benchmark

Observed benchmark output:

```text
BenchmarkInMemoryRoutingTable_GetSubscribers-8                        97182     12556 ns/op    51520 B/op      10 allocs/op
BenchmarkInMemoryRoutingTable_GetSubscribersWildcard_1KPatterns-8     17925     67144 ns/op    63920 B/op    1811 allocs/op
BenchmarkInMemoryRoutingTable_GetSubscribersWildcard_10KPatterns-8     1688    709200 ns/op   627633 B/op   18014 allocs/op

BenchmarkGRPCMeshNode_PublishOnly-8                                 4100704       277.7 ns/op    290 B/op       5 allocs/op
BenchmarkGRPCMeshNode_LocalFanout100-8                               519404      2734 ns/op     6491 B/op      12 allocs/op
BenchmarkGRPCMeshNode_ManySubscriptionsMixedTopics-8                  99925     13402 ns/op    20718 B/op     174 allocs/op

BenchmarkGRPCPeerLink_ReceiveEvents-8                              15224962        79.23 ns/op     0 B/op       0 allocs/op
BenchmarkGRPCPeerLink_ReceiveEventsParallelSenders-8                5403418       225.2 ns/op      0 B/op       0 allocs/op
```

## Early interpretation

- Exact-topic routing is cheap enough for the current MVP.
- Wildcard routing scales linearly with pattern count today. A 10k wildcard-ish
  pattern table is already around 0.7 ms per lookup on this machine, so a routing
  index will likely be the first optimization if wildcard-heavy workloads matter.
- Local fanout remains cheap when subscriber delivery is non-blocking and not
  constrained by client channel buffers.
- The PeerLink `ReceiveEvents` subscriber fanout path itself is allocation-free;
  full gRPC transport throughput should be measured separately when transport
  tuning becomes the priority.

## What these checks cover

- Routing table exact and wildcard lookup cost.
- MeshNode publish-only path.
- MeshNode local fanout with 100 subscribers.
- MeshNode mixed workload with 1,000 subscriptions over 100 topics.
- PeerLink `ReceiveEvents` subscriber distribution path, including parallel senders.
- Opt-in multi-client and sustained resource smoke tests.

## What they do not yet cover

- Full HTTP API throughput.
- SSE network throughput with many real HTTP clients.
- Pebble-backed disk performance.
- Long-duration multi-node soak testing.
- End-to-end gRPC transport saturation across many peers.
