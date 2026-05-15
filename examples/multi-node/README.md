# Multi-Node Example

This example shows how to start two local EventMesh nodes with static seed
discovery. It is for development inspection only, not production deployment
guidance.

Run commands from the repository root after building:

```bash
make build
```

## Terminal 1: Start Node 1

```bash
./bin/eventmesh --http --no-auth \
  --http-port 8091 \
  --node-id node1 \
  --listen :8092 \
  --peer-listen :8093
```

## Terminal 2: Start Node 2 Seeded From Node 1

```bash
./bin/eventmesh --http --no-auth \
  --http-port 8094 \
  --node-id node2 \
  --listen :8095 \
  --peer-listen :8096 \
  --seed-nodes localhost:8093
```

## Terminal 3: Exercise Node 1

```bash
./bin/eventmesh-cli --no-auth --server http://localhost:8091 publish \
  --topic mesh.demo \
  --payload '{"from":"node1"}'

./bin/eventmesh-cli --no-auth --server http://localhost:8091 replay \
  --topic mesh.demo \
  --offset 0
```

## Terminal 4: Exercise Node 2

```bash
./bin/eventmesh-cli --no-auth --server http://localhost:8094 publish \
  --topic mesh.demo \
  --payload '{"from":"node2"}'

./bin/eventmesh-cli --no-auth --server http://localhost:8094 replay \
  --topic mesh.demo \
  --offset 0
```

For real cross-node routing, create subscriptions before publishing and use the
CLI `stream --topic` command against the subscribing node. Mesh behavior is still
MVP-level; prefer the Go integration tests for authoritative coverage.

## Current Limits

- Discovery is static seed based.
- Peer transport is gRPC without mTLS.
- Durable cross-node replay/ACK semantics are roadmap work.
- In-memory EventLog means restart loses events.
