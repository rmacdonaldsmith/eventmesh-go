# Single-Node Example

This example shows the simplest useful EventMesh setup: one server, one HTTP
API, and CLI clients.

Run commands from the repository root unless noted.

## Build

```bash
make build
```

## Start A Development Server

```bash
./examples/single-node/start-server.sh
```

The script starts `bin/eventmesh` with:

- HTTP API on `http://localhost:8081`
- node ID `single-node-demo`
- `--no-auth` enabled for development
- default ports `8082` and `8083` for internal/peer listeners

Override ports if needed:

```bash
HTTP_PORT=8181 LISTEN_PORT=8182 PEER_LISTEN_PORT=8183 \
  ./examples/single-node/start-server.sh
```

## Try The CLI

In another terminal:

```bash
./bin/eventmesh-cli --no-auth health

./bin/eventmesh-cli --no-auth publish \
  --topic orders.created \
  --payload '{"order_id":"001","total":149.99}'

./bin/eventmesh-cli --no-auth topics info --topic orders.created

./bin/eventmesh-cli --no-auth replay --topic orders.created --offset 0
```

## Stream Events

Terminal 2:

```bash
./bin/eventmesh-cli --no-auth stream --topic 'orders.*'
```

Terminal 3:

```bash
./bin/eventmesh-cli --no-auth publish \
  --topic orders.updated \
  --payload '{"order_id":"001","status":"processing"}'
```

The stream command creates a temporary subscription for `orders.*`, opens the
SSE stream, filters matching events locally, and removes the temporary
subscription when it exits.

## Authenticated Mode

For JWT mode, start the server manually without `--no-auth`:

```bash
EVENTMESH_JWT_SECRET="demo-secret" \
  ./bin/eventmesh --http --node-id demo-node
```

Then authenticate:

```bash
./bin/eventmesh-cli auth --client-id demo-client
./bin/eventmesh-cli publish \
  --client-id demo-client \
  --topic orders.created \
  --payload '{"order_id":"002"}'
```

Admin commands require the `admin` client ID:

```bash
./bin/eventmesh-cli auth --client-id admin
./bin/eventmesh-cli admin stats --client-id admin
```

## Troubleshooting

- If the script cannot find `bin/eventmesh`, run `make build`.
- If ports are busy, override `HTTP_PORT`, `LISTEN_PORT`, or
  `PEER_LISTEN_PORT`.
- If authenticated commands fail, run `auth` again or provide `--token`.
- If a stream receives nothing, publish to a topic that matches the stream topic
  pattern.
