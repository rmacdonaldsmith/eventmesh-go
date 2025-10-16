#!/bin/bash

# Simple EventMesh publisher

set -e

CLI="../../bin/eventmesh-cli"

echo "EventMesh Publisher"
echo "=================="

# Check CLI exists
if [ ! -f "$CLI" ]; then
    echo "Error: Please run 'make build' from project root first"
    exit 1
fi

# Authenticate
echo "Authenticating..."
$CLI auth --server http://localhost:8081 --client-id publisher

# Publish some events
echo ""
echo "Publishing events..."

$CLI publish --server http://localhost:8081 --client-id publisher \
    --topic "news.sports" \
    --payload '{"headline": "Local team wins championship!", "category": "sports"}'

$CLI publish --server http://localhost:8081 --client-id publisher \
    --topic "news.weather" \
    --payload '{"forecast": "Sunny with high of 75F", "category": "weather"}'

$CLI publish --server http://localhost:8081 --client-id publisher \
    --topic "news.tech" \
    --payload '{"headline": "New programming language released", "category": "tech"}'

echo ""
echo "Published 3 events!"
echo "Check the subscriber terminal to see them."