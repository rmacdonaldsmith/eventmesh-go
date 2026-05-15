# EventMesh Examples

These examples are intentionally small development aids for the current
EventMesh implementation. They assume you have already run `make build` from
the repository root.

The examples avoid large demo workflows because the API and mesh behavior are
still evolving. Prefer copy/pasteable commands and short scripts that are easy
to keep current.

## Recommended Path

1. [single-node](single-node/) — start one local server and use core CLI commands.
2. [cli-usage](cli-usage/) — publish a small batch of sample events.
3. [multi-node](multi-node/) — manually start two nodes with static seed discovery.

## Development Mode

Most examples use `--no-auth` because they are demos. In no-auth mode:

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

One short script for publishing sample events plus copy/pasteable CLI commands.

### multi-node

Manual two-node startup commands using static seed discovery. This is useful for
inspection, but the mesh layer is still MVP-level and not a production
deployment recipe.

## Topic Patterns

Current wildcard behavior supports `*` for one dot-separated segment:

```text
orders.*      matches orders.created
orders.*      does not match orders.eu.created
*             matches one-segment topics only
```

Multi-segment wildcards such as `orders.#` are not implemented.
