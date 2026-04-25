# Multi-Node Examples

These scripts start multiple local EventMesh nodes using static seed discovery.
They are useful for development inspection, not production deployment guidance.

Run from this directory after building from the repository root:

```bash
make build
cd examples/multi-node
```

## Single Development Server

```bash
./start-server.sh
```

In separate terminals:

```bash
./subscriber.sh
./publisher.sh
```

The starter scripts use `--no-auth` and the default HTTP API on
`http://localhost:8081`.

`publisher.sh`, `subscriber.sh`, and `demo.sh` read `EVENTMESH_SERVER` when you
want to point them somewhere else.

## Replay Demo

```bash
./replay-demo.sh
```

This publishes events, reads topic information, and replays events from offsets
through the CLI.

## Three-Node Discovery Demo

```bash
./multi-node-demo.sh
```

This starts:

- node1 on HTTP `8091`, peer `8093`
- node2 on HTTP `8094`, peer `8096`, seeded with `localhost:8093`
- node3 on HTTP `8097`, peer `8099`, seeded with `localhost:8093`

Logs are written to:

```text
/tmp/eventmesh-demo/node1.log
/tmp/eventmesh-demo/node2.log
/tmp/eventmesh-demo/node3.log
```

Follow logs in another terminal:

```bash
tail -f /tmp/eventmesh-demo/node*.log
```

## Mesh Test

While `multi-node-demo.sh` is running:

```bash
./mesh-test.sh
```

The test script exercises event publishing and reads across the local nodes.
Expect this area to evolve as PeerLink and subscription-gossip semantics are
hardened.

To use the simple publisher or subscriber against node1 from the three-node
demo:

```bash
EVENTMESH_SERVER=http://localhost:8091 ./publisher.sh
EVENTMESH_SERVER=http://localhost:8091 ./subscriber.sh
```

## Manual Node Startup

```bash
../../bin/eventmesh --http --no-auth \
  --http-port 8091 \
  --node-id node1 \
  --peer-listen :8093

../../bin/eventmesh --http --no-auth \
  --http-port 8094 \
  --node-id node2 \
  --peer-listen :8096 \
  --seed-nodes localhost:8093
```

## Current Limits

- Discovery is static seed based.
- Peer transport is gRPC without mTLS.
- Durable cross-node replay/ACK semantics are roadmap work.
- In-memory EventLog means restart loses events.
