#!/usr/bin/env python3
"""Publish regional weather events into a four-node EventMesh demo.

Default mode is synthetic so the demo works offline. Use --mode openmeteo to
fetch live-ish weather from Open-Meteo without an API key.
"""

import argparse
import json
import math
import random
import sys
import time
import urllib.error
import urllib.request
from datetime import datetime, timezone

CITIES = {
    "sf": {
        "name": "San Francisco",
        "server": "http://localhost:8082",
        "lat": 37.7749,
        "lon": -122.4194,
        "base_temp": 58,
    },
    "ny": {
        "name": "New York",
        "server": "http://localhost:8083",
        "lat": 40.7128,
        "lon": -74.0060,
        "base_temp": 67,
    },
    "chicago": {
        "name": "Chicago",
        "server": "http://localhost:8084",
        "lat": 41.8781,
        "lon": -87.6298,
        "base_temp": 62,
    },
}


def post_json(url, body):
    data = json.dumps(body).encode("utf-8")
    req = urllib.request.Request(
        url,
        data=data,
        headers={"Content-Type": "application/json"},
        method="POST",
    )
    with urllib.request.urlopen(req, timeout=5) as resp:
        return json.loads(resp.read().decode("utf-8"))


def publish(server, topic, payload):
    return post_json(f"{server}/api/v1/events", {"topic": topic, "payload": payload})


def synthetic_weather(city_key, tick):
    city = CITIES[city_key]
    phase = tick / 3.0 + len(city_key)
    temp = city["base_temp"] + math.sin(phase) * 12 + random.uniform(-2.5, 2.5)
    wind = 8 + abs(math.cos(phase)) * 18 + random.uniform(-2, 4)
    humidity = max(25, min(95, 55 + math.sin(phase / 2) * 25 + random.uniform(-5, 5)))
    return round(temp, 1), round(wind, 1), round(humidity, 1)


def openmeteo_weather(city_key):
    city = CITIES[city_key]
    url = (
        "https://api.open-meteo.com/v1/forecast"
        f"?latitude={city['lat']}&longitude={city['lon']}"
        "&current=temperature_2m,relative_humidity_2m,wind_speed_10m"
        "&temperature_unit=fahrenheit&wind_speed_unit=mph"
    )
    with urllib.request.urlopen(url, timeout=8) as resp:
        data = json.loads(resp.read().decode("utf-8"))
    current = data["current"]
    return (
        round(float(current["temperature_2m"]), 1),
        round(float(current["wind_speed_10m"]), 1),
        round(float(current["relative_humidity_2m"]), 1),
    )


def alert_reason(temp, wind, tick, city_key):
    # Synthetic mode intentionally emits occasional alerts so the alert path is visible.
    if wind >= 24:
        return "high_wind"
    if temp >= 85:
        return "heat"
    if temp <= 35:
        return "cold"
    if tick % 9 == 0 and city_key == "chicago":
        return "demo_storm_cell"
    return None


def main():
    parser = argparse.ArgumentParser(description="Generate regional weather events for EventMesh")
    parser.add_argument("--mode", choices=["synthetic", "openmeteo"], default="synthetic")
    parser.add_argument("--interval", type=float, default=1.5, help="seconds between city batches")
    parser.add_argument("--count", type=int, default=0, help="number of batches, 0 means forever")
    args = parser.parse_args()

    print("Publishing regional weather events. Ctrl+C to stop.", flush=True)
    print("Cities publish to their local nodes; hub consumers subscribe to weather.* and alert.*", flush=True)

    tick = 0
    try:
        while args.count == 0 or tick < args.count:
            tick += 1
            for city_key, city in CITIES.items():
                try:
                    if args.mode == "openmeteo":
                        temp, wind, humidity = openmeteo_weather(city_key)
                    else:
                        temp, wind, humidity = synthetic_weather(city_key, tick)

                    payload = {
                        "city": city["name"],
                        "cityCode": city_key,
                        "temperatureF": temp,
                        "windMph": wind,
                        "humidityPct": humidity,
                        "source": args.mode,
                        "observedAt": datetime.now(timezone.utc).isoformat(),
                    }
                    topic = f"weather.{city_key}"
                    result = publish(city["server"], topic, payload)
                    print(f"{topic:<16} temp={temp:>5}F wind={wind:>4}mph offset={result.get('offset')}", flush=True)

                    reason = alert_reason(temp, wind, tick, city_key)
                    if reason:
                        alert_payload = dict(payload)
                        alert_payload["reason"] = reason
                        alert_topic = f"alert.{city_key}"
                        alert_result = publish(city["server"], alert_topic, alert_payload)
                        print(f"{alert_topic:<16} reason={reason:<15} offset={alert_result.get('offset')}", flush=True)
                except (urllib.error.URLError, TimeoutError, KeyError, ValueError) as exc:
                    print(f"failed to publish {city_key}: {exc}", file=sys.stderr, flush=True)
            time.sleep(args.interval)
    except KeyboardInterrupt:
        print("\nGenerator stopped.")


if __name__ == "__main__":
    main()
