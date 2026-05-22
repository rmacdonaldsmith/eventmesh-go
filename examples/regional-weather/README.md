# Regional Weather Mesh Demo

This demo shows EventMesh as a small regional telemetry mesh.

Three city nodes publish local weather observations. A hub node hosts two
consumers:

- `analytics-dashboard` subscribes to `weather.*`
- `alerting-service` subscribes to `alert.*`

The city nodes only forward events that match the hub's aggregate topic
interest. Admin stats show local subscriptions, peer-interest views, and gossip
counters while events move through the mesh.

## Topology

```text
SF node      weather.sf / alert.sf      \
NY node      weather.ny / alert.ny       -> hub node -> analytics: weather.*
Chicago node weather.chicago / alert.chicago /          alerting:  alert.*
```

Topics intentionally use two segments because EventMesh currently supports
single-segment wildcards:

```text
weather.sf
weather.ny
weather.chicago
alert.sf
alert.ny
alert.chicago
```

## Quick start with tmux

```bash
cd examples/regional-weather
./dashboard.sh
```

This opens panes for nodes, generator, analytics subscriber, alert subscriber,
and stats. Stop the nodes afterward with:

```bash
./stop.sh
```

## Manual start

Terminal 1:

```bash
cd examples/regional-weather
./start.sh
```

Terminal 2, analytics consumer on the hub:

```bash
./subscribe-analytics.sh
```

Terminal 3, alerting consumer on the hub:

```bash
./subscribe-alerts.sh
```

Terminal 4, weather generators publishing to the city nodes:

```bash
./generate-weather.py
```

Terminal 5, admin stats view:

```bash
./stats.py
```

Stop all nodes:

```bash
./stop.sh
```

## Live Open-Meteo mode

Synthetic weather is the default so the demo works offline. To fetch current
weather from Open-Meteo instead:

```bash
./generate-weather.py --mode openmeteo --interval 10
```

Open-Meteo does not require an API key, but this mode needs internet access and
changes more slowly than synthetic mode.

## What to watch

- The analytics stream receives `weather.sf`, `weather.ny`, and
  `weather.chicago` events.
- The alert stream receives only `alert.*` events.
- `stats.py` shows the hub's local interest and the city nodes' peer-interest
  view after the subscriptions are established.
- If you stop a subscriber and restart it, the managed CLI stream recreates its
  temporary subscription on reconnect.

## Ports

| Node | HTTP | Peer |
| --- | --- | --- |
| hub | `localhost:8081` | `127.0.0.1:9100` |
| sf | `localhost:8082` | `127.0.0.1:9101` |
| ny | `localhost:8083` | `127.0.0.1:9102` |
| chicago | `localhost:8084` | `127.0.0.1:9103` |
