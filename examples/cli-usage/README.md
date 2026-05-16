# CLI Usage Examples

This directory contains a small script and copy/pasteable commands for the
`eventmesh-cli` binary.

From the repository root:

```bash
make build
./bin/eventmesh --http --no-auth --node-id cli-demo
```

Then in another terminal:

```bash
cd examples/cli-usage
./simple-publisher.sh
```

## Script

- `simple-publisher.sh` publishes sample user, product, and system events.

Larger workflow scripts were intentionally removed. They were useful demos, but
became maintenance-heavy while the codebase is still evolving.

## Common Commands

Health:

```bash
../../bin/eventmesh-cli --no-auth health
```

Publish:

```bash
../../bin/eventmesh-cli --no-auth publish \
  --topic user.registered \
  --payload '{"user_id":"12345","email":"user@example.com"}'
```

Create a stored client subscription:

```bash
../../bin/eventmesh-cli --no-auth subscribe --topic 'orders.*'
../../bin/eventmesh-cli --no-auth subscriptions list
```

Stream with a temporary subscription:

```bash
../../bin/eventmesh-cli --no-auth stream --topic 'orders.*'
```

Replay stored events:

```bash
../../bin/eventmesh-cli --no-auth replay --topic orders.created --offset 0
```

Inspect a topic:

```bash
../../bin/eventmesh-cli --no-auth topics info --topic orders.created
```

## Authenticated Mode

When the server is not running with `--no-auth`, authenticate first:

```bash
../../bin/eventmesh-cli auth --client-id my-client
../../bin/eventmesh-cli publish \
  --client-id my-client \
  --topic test.events \
  --payload '{"ok":true}'
```

You can also pass a token directly:

```bash
../../bin/eventmesh-cli publish \
  --client-id my-client \
  --token "$EVENTMESH_TOKEN" \
  --topic test.events \
  --payload '{"ok":true}'
```

## Curl Equivalents

Health:

```bash
curl http://localhost:8081/api/v1/health
```

Login:

```bash
curl -X POST http://localhost:8081/api/v1/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"clientId":"my-client"}'
```

Publish:

```bash
curl -X POST http://localhost:8081/api/v1/events \
  -H 'Content-Type: application/json' \
  -H "Authorization: Bearer $TOKEN" \
  -d '{"topic":"test.events","payload":{"message":"hello"}}'
```

Read events:

```bash
curl 'http://localhost:8081/api/v1/topics/test.events/events?offset=0&limit=100' \
  -H "Authorization: Bearer $TOKEN"
```

## Notes

- `*` matches one topic segment, not an arbitrary suffix.
- `stream --topic` can be repeated; the CLI creates, re-ensures across reconnects, and later removes temporary subscriptions it created.
- `GET /api/v1/events/stream?topic=...` is not a supported HTTP contract.
- Broad streams such as `--topic '*'` are useful for demos but can be noisy.
