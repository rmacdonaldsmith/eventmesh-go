# Docker Weather Mesh Demo

This is the recommended local multi-node EventMesh demo. It runs a small weather
telemetry mesh in Docker Compose, with one hub node and city nodes that publish
local weather observations.

The demo is intentionally small: easy to start, easy to inspect, and easy to map
back to EventMesh concepts. It runs with `--no-auth`, static peer connections,
and in-memory EventLogs by default, so it is a development harness rather than a
production deployment recipe.

## Topology

Default mesh:

```text
sf node  weather.sf / alert.sf  \
                                  -> hub node -> analytics subscriber: weather.*
ny node  weather.ny / alert.ny  /              alerting subscriber:  alert.*
```

Optional Chicago node:

```text
chicago node weather.chicago / alert.chicago -> hub node
```

Topics use two segments because EventMesh currently supports single-segment
wildcards:

```text
weather.sf
weather.ny
weather.chicago
alert.sf
alert.ny
alert.chicago
```

## Quick Start With tmux

```bash
cd examples/docker
./dashboard.sh
```

The dashboard starts the Docker mesh and opens panes for:

- hub, SF, and NY node logs
- control-plane / gossip stats
- `weather.*` analytics stream on the hub
- `alert.*` alerting stream on the hub
- an SF local city stream
- synthetic event generation from the city nodes

Stop the mesh afterward:

```bash
./stop.sh
```

## Manual Start

Start the default hub + SF + NY mesh:

```bash
cd examples/docker
./start.sh
```

Start with the optional Chicago node:

```bash
./start.sh --with-chicago
```

Follow logs for one node:

```bash
./logs.sh hub
./logs.sh sf
./logs.sh ny
./logs.sh chicago
```

Follow all logs:

```bash
./logs.sh
```

## Streaming Demo

Terminal 1, analytics subscriber on the hub:

```bash
./subscribe-weather.sh
```

Terminal 2, alerting subscriber on the hub:

```bash
./subscribe-alerts.sh
```

Terminal 3, optional city-local subscriber:

```bash
./subscribe-city.sh sf
```

Terminal 4, city event generator:

```bash
./generate-events.sh
```

The generator publishes `weather.<city>` events every two seconds. It also emits
`alert.<city>` events when synthetic temperature or wind values cross simple
thresholds.

## Stats

Watch all node stats:

```bash
./stats.py
```

Focus on data-plane counters:

```bash
./stats.py --view data
```

Focus on topic-interest gossip and peer-interest views:

```bash
./stats.py --view control
```

What to look for:

- the hub has local interest in `weather.*` and/or `alert.*` while subscribers run
- city nodes learn the hub's interest through peer-interest gossip
- city nodes forward only matching events toward the hub
- hub event counts increase as routed events arrive

## Ports

| Node | HTTP API | Peer gRPC |
| ---- | -------- | --------- |
| hub | `localhost:8081` | `localhost:9100` |
| sf | `localhost:8082` | `localhost:9101` |
| ny | `localhost:8083` | `localhost:9102` |
| chicago | `localhost:8084` | `localhost:9103` |

Inside the Docker network every node listens on HTTP port `8081` and peer port
`9090`. The hub connects to `sf:9090` and `ny:9090`; Chicago, when enabled,
seeds from `hub:9090`.

## Configuration

The Compose file maps common environment variables into each container:

| Variable | Purpose | Default |
| -------- | ------- | ------- |
| `EVENTMESH_NODE_ID` | unique node identity | per service |
| `EVENTMESH_HTTP_ENABLED` | enable HTTP API | `true` |
| `EVENTMESH_HTTP_PORT` | in-container HTTP port | `8081` |
| `EVENTMESH_LISTEN` | client listen flag value | `:8080` |
| `EVENTMESH_PEER_LISTEN` | peer gRPC listen address | `:9090` |
| `EVENTMESH_CONNECT_PEER` | explicit peer connections | hub only |
| `EVENTMESH_SEED_NODES` | comma-separated peer seed addresses | Chicago only |
| `EVENTMESH_NO_AUTH` | run in insecure development mode | `true` |
| `EVENTMESH_LOG_LEVEL` | `debug`, `info`, `warn`, or `error` | `info` |
| `EVENTMESH_EVENTLOG_BACKEND` | `memory` or `pebble` | `memory` |
| `EVENTMESH_EVENTLOG_PATH` | Pebble storage path | empty |

Host ports can be overridden with a local `.env` file in this directory. Start
from the example file:

```bash
cp .env.example .env
```

## Scaling Notes

This demo uses explicit `hub`, `sf`, `ny`, and optional `chicago` services rather
than `docker compose up --scale` because EventMesh nodes need stable node IDs and
predictable peer topology. To add another city, copy the Chicago service, give it
unique host ports and `EVENTMESH_NODE_ID`, and seed/connect it to the hub.

## Raw Docker Compose Commands

From the repository root, if you do not want to use the helper scripts:

```bash
docker compose -p eventmesh-weather -f examples/docker/docker-compose.yml up -d --build
docker compose -p eventmesh-weather -f examples/docker/docker-compose.yml --profile chicago up -d --build
docker compose -p eventmesh-weather -f examples/docker/docker-compose.yml --profile chicago down --remove-orphans
```
