# EventMesh Examples

These examples are development demos for the current EventMesh implementation.
They assume you have already run `make build` from the repository root.

## Recommended Path

1. Start with [single-node](single-node/) to learn the CLI and API.
2. Use [cli-usage](cli-usage/) for longer command workflows.
3. Use [multi-node](multi-node/) to inspect peer discovery and mesh behavior.

## Development Mode

Most scripts use `--no-auth` because they are demos. In no-auth mode:

- non-admin routes accept a dummy token from the CLI
- admin routes still require an admin JWT
- this mode is not safe for production

Start a dev server:

```bash
./bin/eventmesh --http --no-auth --node-id dev-node
```

Use the CLI:

```bash
./bin/eventmesh-cli --no-auth publish \
  --topic test.events \
  --payload '{"message":"hello"}'

./bin/eventmesh-cli --no-auth replay --topic test.events --offset 0
```

## Example Directories

### single-node

One server, one HTTP API, and CLI commands for publish, subscribe, stream, and
replay.

### cli-usage

Standalone scripts for common CLI workflows such as publishing event batches,
creating wildcard subscriptions, and simulating business event flows.

### multi-node

Scripts for starting multiple local nodes with static seed discovery. This is
useful for development and test inspection, but the multi-node behavior is still
MVP-level and not a production deployment recipe.

## Topic Patterns

Current wildcard behavior supports `*` for one dot-separated segment:

```text
orders.*      matches orders.created
orders.*      does not match orders.eu.created
*             matches one-segment topics only
```

Multi-segment wildcards such as `orders.#` are not implemented.
