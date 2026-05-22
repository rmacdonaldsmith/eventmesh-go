#!/usr/bin/env python3
"""Poll admin stats from the regional weather demo nodes."""

import json
import time
import urllib.request
from datetime import datetime

NODES = {
    "hub": "http://localhost:8081",
    "sf": "http://localhost:8082",
    "ny": "http://localhost:8083",
    "chicago": "http://localhost:8084",
}


def post_json(url, body):
    data = json.dumps(body).encode("utf-8")
    req = urllib.request.Request(url, data=data, headers={"Content-Type": "application/json"}, method="POST")
    with urllib.request.urlopen(req, timeout=5) as resp:
        return json.loads(resp.read().decode("utf-8"))


def get_json(url, token):
    req = urllib.request.Request(url, headers={"Authorization": f"Bearer {token}"})
    with urllib.request.urlopen(req, timeout=5) as resp:
        return json.loads(resp.read().decode("utf-8"))


def token_for(server):
    return post_json(f"{server}/api/v1/auth/login", {"clientId": "admin"})["token"]


def summarize(name, stats):
    event_log = stats.get("eventLog", {})
    subs = stats.get("subscriptions", {})
    peer_interest = stats.get("peerInterest", {})
    gossip = stats.get("interestGossip", {})

    topics = subs.get("localInterestTopics", [])
    peers = peer_interest.get("peers", {})
    peer_bits = []
    for peer_id, peer_stats in peers.items():
        peer_topics = peer_stats.get("topics", [])
        peer_bits.append(f"{peer_id}:{','.join(peer_topics) if peer_topics else '-'}")

    return (
        f"{name:<8} events={event_log.get('totalEvents', 0):<4} "
        f"localInterest={','.join(topics) if topics else '-':<20} "
        f"peerInterest={' | '.join(peer_bits) if peer_bits else '-':<36} "
        f"gossip sent/recv={gossip.get('updatesSent', 0)}/{gossip.get('updatesReceived', 0)} "
        f"snap={gossip.get('snapshotsSent', 0)}/{gossip.get('snapshotsReceived', 0)}"
    )


def main():
    tokens = {name: token_for(server) for name, server in NODES.items()}
    print("Polling admin stats. Ctrl+C to stop.\n")
    try:
        while True:
            print("\033[2J\033[H", end="")
            print(f"EventMesh regional weather stats @ {datetime.now().strftime('%H:%M:%S')}\n")
            for name, server in NODES.items():
                try:
                    stats = get_json(f"{server}/api/v1/admin/stats", tokens[name])
                    print(summarize(name, stats))
                except Exception as exc:  # demo script: keep polling even if one node is down
                    print(f"{name:<8} unavailable: {exc}")
            print("\nTip: analytics subscribes to weather.*; alerting subscribes to alert.* on the hub.")
            time.sleep(2)
    except KeyboardInterrupt:
        print("\nStats stopped.")


if __name__ == "__main__":
    main()
