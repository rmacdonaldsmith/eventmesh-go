package meshnode

import (
	"context"
	"reflect"
	"testing"

	peerlinkpkg "github.com/rmacdonaldsmith/eventmesh-go/pkg/peerlink"
)

func TestGRPCMeshNode_GetRoutingStatsLocalAggregateInterest(t *testing.T) {
	ctx := context.Background()
	node := newStartedIntegrationNode(t, ctx, "routing-stats-local")

	first := NewTrustedClient("stats-client-1")
	second := NewTrustedClient("stats-client-2")
	third := NewTrustedClient("stats-client-3")

	if err := node.Subscribe(ctx, first, "orders.*"); err != nil {
		t.Fatalf("subscribe first: %v", err)
	}
	if err := node.Subscribe(ctx, second, "orders.*"); err != nil {
		t.Fatalf("subscribe second: %v", err)
	}
	if err := node.Subscribe(ctx, third, "payments.*"); err != nil {
		t.Fatalf("subscribe third: %v", err)
	}

	stats := node.GetRoutingStats(ctx)
	if stats.LocalSubscriptions != 3 {
		t.Fatalf("Expected 3 local subscriptions, got %d", stats.LocalSubscriptions)
	}
	if stats.LocalClientsWithSubscriptions != 3 {
		t.Fatalf("Expected 3 clients with subscriptions, got %d", stats.LocalClientsWithSubscriptions)
	}
	expectedTopics := []string{"orders.*", "payments.*"}
	if !reflect.DeepEqual(stats.LocalInterestTopics, expectedTopics) {
		t.Fatalf("Expected local interest topics %#v, got %#v", expectedTopics, stats.LocalInterestTopics)
	}
}

func TestGRPCMeshNode_GetRoutingStatsPeerSnapshotAndCounters(t *testing.T) {
	ctx := context.Background()
	node := newStartedIntegrationNode(t, ctx, "routing-stats-peer")

	node.processIncomingInterestSnapshot(ctx, &peerlinkpkg.InterestSnapshot{
		NodeId: "node-b",
		Topics: []string{"payments.*", "orders.*"},
	})
	node.processIncomingInterestUpdate(ctx, &peerlinkpkg.InterestUpdate{
		NodeId: "node-c",
		Action: "subscribe",
		Topic:  "inventory.low",
	})

	stats := node.GetRoutingStats(ctx)
	if !reflect.DeepEqual(stats.PeerInterest["node-b"], []string{"orders.*", "payments.*"}) {
		t.Fatalf("Expected sorted node-b peer interest, got %#v", stats.PeerInterest["node-b"])
	}
	if !reflect.DeepEqual(stats.PeerInterest["node-c"], []string{"inventory.low"}) {
		t.Fatalf("Expected node-c peer interest, got %#v", stats.PeerInterest["node-c"])
	}
	if stats.InterestSnapshotsReceived != 1 {
		t.Fatalf("Expected 1 received snapshot, got %d", stats.InterestSnapshotsReceived)
	}
	if stats.SnapshotTopicsReceived != 2 {
		t.Fatalf("Expected 2 snapshot topics received, got %d", stats.SnapshotTopicsReceived)
	}
	if stats.InterestUpdatesReceived != 1 {
		t.Fatalf("Expected 1 received update, got %d", stats.InterestUpdatesReceived)
	}
}

func TestGRPCMeshNode_GetRoutingStatsEmptySnapshotClearsPeerInterest(t *testing.T) {
	ctx := context.Background()
	node := newStartedIntegrationNode(t, ctx, "routing-stats-empty-snapshot")

	node.processIncomingInterestSnapshot(ctx, &peerlinkpkg.InterestSnapshot{
		NodeId: "node-b",
		Topics: []string{"orders.*"},
	})
	node.processIncomingInterestSnapshot(ctx, &peerlinkpkg.InterestSnapshot{
		NodeId: "node-b",
	})

	stats := node.GetRoutingStats(ctx)
	if _, exists := stats.PeerInterest["node-b"]; exists {
		t.Fatalf("Expected empty snapshot to clear node-b peer interest, got %#v", stats.PeerInterest["node-b"])
	}
	if stats.InterestSnapshotsReceived != 2 {
		t.Fatalf("Expected 2 received snapshots, got %d", stats.InterestSnapshotsReceived)
	}
	if stats.EmptySnapshotsReceived != 1 {
		t.Fatalf("Expected 1 empty snapshot, got %d", stats.EmptySnapshotsReceived)
	}
}
