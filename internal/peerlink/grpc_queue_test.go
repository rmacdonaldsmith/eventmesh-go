package peerlink

import (
	"context"
	"testing"
	"time"

	"github.com/rmacdonaldsmith/eventmesh-go/pkg/eventlog"
	"github.com/rmacdonaldsmith/eventmesh-go/pkg/peerlink"
)

func TestGRPCPeerLink_ControlMessagesAreNotBlockedByFullDataQueue(t *testing.T) {
	peerLink := newQueueTestPeerLink(t)
	defer peerLink.Close()

	ctx := context.Background()
	if err := peerLink.SendEvent(ctx, "peer-1", eventlog.NewEvent("data.one", []byte("one"))); err != nil {
		t.Fatalf("failed to fill data queue: %v", err)
	}

	if err := peerLink.SendEvent(ctx, "peer-1", eventlog.NewEvent("data.two", []byte("two"))); err == nil {
		t.Fatal("expected second data-plane event to fail because the data queue is full")
	}

	change := &peerlink.SubscriptionChange{
		Action:   "subscribe",
		ClientId: "client-1",
		Topic:    "orders.*",
		NodeId:   "node-1",
	}
	if err := peerLink.SendSubscriptionChange(ctx, "peer-1", change); err != nil {
		t.Fatalf("control-plane subscription change should not be blocked by full data queue: %v", err)
	}
}

func TestGRPCPeerLink_HeartbeatIsNotBlockedByFullDataQueue(t *testing.T) {
	peerLink := newQueueTestPeerLink(t)
	defer peerLink.Close()

	ctx := context.Background()
	if err := peerLink.SendEvent(ctx, "peer-1", eventlog.NewEvent("data.one", []byte("one"))); err != nil {
		t.Fatalf("failed to fill data queue: %v", err)
	}

	if err := peerLink.SendHeartbeat(ctx, "peer-1"); err != nil {
		t.Fatalf("heartbeat should not be blocked by full data queue: %v", err)
	}
}

func TestGRPCPeerLink_ControlQueueIsSelectedBeforeDataQueue(t *testing.T) {
	peerLink := newQueueTestPeerLink(t)
	defer peerLink.Close()

	ctx := context.Background()
	if err := peerLink.SendEvent(ctx, "peer-1", eventlog.NewEvent("data.one", []byte("one"))); err != nil {
		t.Fatalf("failed to queue data message: %v", err)
	}
	if err := peerLink.SendHeartbeat(ctx, "peer-1"); err != nil {
		t.Fatalf("failed to queue heartbeat: %v", err)
	}

	msg, ok := peerLink.nextOutboundMessage(ctx, "peer-1")
	if !ok {
		t.Fatal("expected queued outbound message")
	}
	if _, ok := msg.(*HeartbeatMsg); !ok {
		t.Fatalf("expected control-plane heartbeat to be selected first, got %T", msg)
	}
}

func newQueueTestPeerLink(t *testing.T) *GRPCPeerLink {
	t.Helper()

	peerLink, err := NewGRPCPeerLink(&Config{
		NodeID:            "queue-test-node",
		ListenAddress:     "localhost:0",
		SendQueueSize:     1,
		SendTimeout:       10 * time.Millisecond,
		HeartbeatInterval: 5 * time.Second,
	})
	if err != nil {
		t.Fatalf("failed to create PeerLink: %v", err)
	}

	peerLink.mu.Lock()
	peerLink.registerPeer("peer-1")
	peerLink.connections["peer-1"] = &connectionInfo{
		node: &simplePeerNode{id: "peer-1", address: "localhost:0", healthy: true},
	}
	peerLink.mu.Unlock()

	return peerLink
}
