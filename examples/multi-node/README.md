# Multi-Node Example

For day-to-day multi-node development, prefer the Docker Compose workflow in
[`../docker`](../docker/). It starts a small weather-style mesh with hub and city
nodes, gives each node stable Docker DNS names, and avoids manual port juggling.

This page keeps the direct binary commands as a low-level reference for debugging
startup flags or running nodes outside Docker. Both workflows use static seed
discovery and are for development inspection only, not production deployment
guidance.

## Recommended: Docker Compose

From the Docker example directory, start the default hub + SF + NY mesh:

```bash
cd examples/docker
./start.sh
```

Start with the optional Chicago node:

```bash
./start.sh --with-chicago
```

Follow logs per node in separate terminal tabs or tmux panes:

```bash
./logs.sh hub
./logs.sh sf
./logs.sh ny
./logs.sh chicago
```

Default HTTP endpoints:

```text
hub:     http://localhost:8081
sf:      http://localhost:8082
ny:      http://localhost:8083
chicago: http://localhost:8084  # only with --with-chicago
```

See [`../docker/README.md`](../docker/README.md) for the full Compose workflow,
configuration variables, streaming demo, stats, and scaling notes.

## Manual Reference: Two Local Nodes

Run commands from the repository root after building:

```bash
make build
```

### Terminal 1: Start Node 1

```bash
./bin/eventmesh --http --no-auth \
  --http-port 8091 \
  --node-id node1 \
  --listen :8092 \
  --peer-listen :8093
```

### Terminal 2: Start Node 2 Seeded From Node 1

```bash
./bin/eventmesh --http --no-auth \
  --http-port 8094 \
  --node-id node2 \
  --listen :8095 \
  --peer-listen :8096 \
  --seed-nodes localhost:8093
```

### Terminal 3: Stream From Node 2

Create a live stream on the subscribing node before publishing:

```bash
./bin/eventmesh-cli --no-auth --server http://localhost:8094 stream \
  --topic mesh.demo
```

### Terminal 4: Publish From Node 1

```bash
./bin/eventmesh-cli --no-auth --server http://localhost:8091 publish \
  --topic mesh.demo \
  --payload '{"from":"node1"}'
```

The event should appear in the stream attached to node 2. To inspect local
history on either node, use replay:

```bash
./bin/eventmesh-cli --no-auth --server http://localhost:8091 replay \
  --topic mesh.demo \
  --offset 0

./bin/eventmesh-cli --no-auth --server http://localhost:8094 replay \
  --topic mesh.demo \
  --offset 0
```

## Current Limits

- Discovery is static seed based.
- Peer transport is gRPC without mTLS.
- Durable cross-node replay/ACK semantics are roadmap work.
- In-memory EventLog means restart loses events unless Pebble is configured.
- Docker Compose is a development harness, not a production deployment recipe.
