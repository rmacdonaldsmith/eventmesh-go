#!/usr/bin/env python3
"""Poll admin stats from the Docker weather mesh demo nodes."""

import argparse
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


def peer_interest_summary(stats):
    peers = stats.get("peerInterest", {}).get("peers", {}) or {}
    peer_bits = []
    for peer_id, peer_topics in peers.items():
        topics = peer_topics or []
        peer_bits.append(f"{peer_id}:{','.join(topics) if topics else '-'}")
    return " | ".join(peer_bits) if peer_bits else "-"


def local_interest_summary(stats):
    topics = stats.get("subscriptions", {}).get("localInterestTopics", []) or []
    return ",".join(topics) if topics else "-"


def summarize_data(name, stats):
    event_log = stats.get("eventLog", {})
    sse = stats.get("sseResume", {})
    return (
        f"{name:<8} DATA  "
        f"events={event_log.get('totalEvents', 0):<5} "
        f"topics={event_log.get('topicCount', 0):<3} "
        f"clients={stats.get('connectedClients', 0):<2} "
        f"peers={stats.get('connectedPeers', 0):<2} "
        f"sseResume req/replay={sse.get('requests', 0)}/{sse.get('eventsReplayed', 0)}"
    )


def summarize_control(name, stats):
    subs = stats.get("subscriptions", {})
    peer_interest = stats.get("peerInterest", {})
    gossip = stats.get("interestGossip", {})
    return (
        f"{name:<8} CTRL  "
        f"local={local_interest_summary(stats):<18} "
        f"peerInterest={peer_interest_summary(stats):<30} "
        f"peerTopics={peer_interest.get('topicCount', 0):<2} "
        f"subs={subs.get('localSubscriptions', 0):<2} "
        f"gossip upd/snap sent={gossip.get('updatesSent', 0)}/{gossip.get('snapshotsSent', 0)} "
        f"recv={gossip.get('updatesReceived', 0)}/{gossip.get('snapshotsReceived', 0)} "
        f"emptySnap={gossip.get('emptySnapshotsReceived', 0)}"
    )


def summarize_all(name, stats):
    event_log = stats.get("eventLog", {})
    gossip = stats.get("interestGossip", {})
    return (
        f"{name:<8} events={event_log.get('totalEvents', 0):<4} "
        f"localInterest={local_interest_summary(stats):<20} "
        f"peerInterest={peer_interest_summary(stats):<36} "
        f"gossip upd={gossip.get('updatesSent', 0)}/{gossip.get('updatesReceived', 0)} "
        f"snap={gossip.get('snapshotsSent', 0)}/{gossip.get('snapshotsReceived', 0)}"
    )


def summarize(name, stats, view):
    if view == "data":
        return summarize_data(name, stats)
    if view == "control":
        return summarize_control(name, stats)
    return summarize_all(name, stats)


def main():
    parser = argparse.ArgumentParser(description="Poll Docker weather mesh admin stats")
    parser.add_argument("--view", choices=["all", "data", "control"], default="all")
    args = parser.parse_args()

    tokens = {}
    for name, server in NODES.items():
        try:
            tokens[name] = token_for(server)
        except Exception:
            pass

    print("Polling admin stats. Ctrl+C to stop.\n")
    try:
        while True:
            print("\033[2J\033[H", end="")
            title = {
                "all": "Docker weather mesh stats",
                "data": "Docker weather mesh DATA PLANE metrics",
                "control": "Docker weather mesh CONTROL PLANE / GOSSIP metrics",
            }[args.view]
            print(f"{title} @ {datetime.now().strftime('%H:%M:%S')}\n")
            for name, server in NODES.items():
                try:
                    token = tokens.get(name) or token_for(server)
                    tokens[name] = token
                    stats = get_json(f"{server}/api/v1/admin/stats", token)
                    print(summarize(name, stats, args.view))
                except Exception as exc:
                    print(f"{name:<8} unavailable: {exc}")
            print("\nTip: subscribe-weather.sh creates hub interest in weather.*; city nodes then forward matching events.")
            time.sleep(2)
    except KeyboardInterrupt:
        print("\nStats stopped.")


if __name__ == "__main__":
    main()
