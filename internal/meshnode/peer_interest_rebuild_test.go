package meshnode

import (
	"context"
	"testing"
	"time"
)

func TestPeerInterestIsAggregateAcrossLocalClients(t *testing.T) {
	ctx := context.Background()
	publisherNode := newStartedIntegrationNode(t, ctx, "aggregate-publisher")
	subscriberNode := newStartedIntegrationNode(t, ctx, "aggregate-subscriber")
	connectMeshNodes(t, ctx, publisherNode, subscriberNode)

	topic := "orders.aggregate"
	firstClient := NewTrustedClient("aggregate-subscriber-1")
	secondClient := NewTrustedClient("aggregate-subscriber-2")

	if err := subscriberNode.Subscribe(ctx, firstClient, topic); err != nil {
		t.Fatalf("Failed to subscribe first client: %v", err)
	}
	waitForInterestedPeer(t, ctx, publisherNode, topic, subscriberNode.GetNodeID())

	if err := subscriberNode.Subscribe(ctx, secondClient, topic); err != nil {
		t.Fatalf("Failed to subscribe second client: %v", err)
	}
	waitForInterestedPeer(t, ctx, publisherNode, topic, subscriberNode.GetNodeID())

	if err := subscriberNode.Unsubscribe(ctx, firstClient, topic); err != nil {
		t.Fatalf("Failed to unsubscribe first client: %v", err)
	}

	assertInterestedPeerRemains(t, ctx, publisherNode, topic, subscriberNode.GetNodeID(), 300*time.Millisecond)

	if err := subscriberNode.Unsubscribe(ctx, secondClient, topic); err != nil {
		t.Fatalf("Failed to unsubscribe second client: %v", err)
	}
	waitForNoInterestedPeer(t, ctx, publisherNode, topic, subscriberNode.GetNodeID())
}

func assertInterestedPeerRemains(t *testing.T, ctx context.Context, node *GRPCMeshNode, topic, peerID string, duration time.Duration) {
	t.Helper()

	deadline := time.NewTimer(duration)
	defer deadline.Stop()
	ticker := time.NewTicker(20 * time.Millisecond)
	defer ticker.Stop()

	for {
		if !meshNodeHasInterestedPeer(ctx, node, topic, peerID) {
			t.Fatalf("Expected %s to remain interested in %s for %s", peerID, topic, duration)
		}

		select {
		case <-deadline.C:
			return
		case <-ticker.C:
		}
	}
}

func TestPeerInterestSnapshotClearsUnsubscribedTopicsAfterReconnect(t *testing.T) {
	ctx := context.Background()
	publisherNode := newStartedIntegrationNode(t, ctx, "snapshot-publisher")
	subscriberNode := newStartedIntegrationNode(t, ctx, "snapshot-subscriber")
	connectMeshNodes(t, ctx, publisherNode, subscriberNode)

	topic := "orders.snapshot.removed"
	subscriber := NewTrustedClient("snapshot-subscriber-client")
	if err := subscriberNode.Subscribe(ctx, subscriber, topic); err != nil {
		t.Fatalf("Failed to subscribe: %v", err)
	}
	waitForInterestedPeer(t, ctx, publisherNode, topic, subscriberNode.GetNodeID())

	if err := publisherNode.GetPeerLink().Disconnect(ctx, subscriberNode.GetNodeID()); err != nil {
		t.Fatalf("Failed to disconnect publisher from subscriber: %v", err)
	}
	waitForMeshNodeCondition(t, 2*time.Second, func() bool {
		return !meshNodeHasConnectedPeer(t, ctx, publisherNode, subscriberNode.GetNodeID())
	}, "publisher to remove subscriber from connected peers")

	if err := subscriberNode.Unsubscribe(ctx, subscriber, topic); err != nil {
		t.Fatalf("Failed to unsubscribe while disconnected: %v", err)
	}

	connectMeshNodes(t, ctx, publisherNode, subscriberNode)
	waitForNoInterestedPeer(t, ctx, publisherNode, topic, subscriberNode.GetNodeID())
}

func TestPeerInterestEmptySnapshotClearsStaleInterestAfterSubscriberRestart(t *testing.T) {
	ctx := context.Background()
	publisherNode := newStartedIntegrationNode(t, ctx, "restart-publisher")
	subscriberNode := newStartedIntegrationNode(t, ctx, "restart-subscriber")
	connectMeshNodes(t, ctx, publisherNode, subscriberNode)

	topic := "orders.restart.stale"
	subscriber := NewTrustedClient("restart-subscriber-client")
	if err := subscriberNode.Subscribe(ctx, subscriber, topic); err != nil {
		t.Fatalf("Failed to subscribe: %v", err)
	}
	waitForInterestedPeer(t, ctx, publisherNode, topic, subscriberNode.GetNodeID())

	if err := publisherNode.GetPeerLink().Disconnect(ctx, subscriberNode.GetNodeID()); err != nil {
		t.Fatalf("Failed to disconnect publisher from subscriber: %v", err)
	}
	waitForMeshNodeCondition(t, 2*time.Second, func() bool {
		return !meshNodeHasConnectedPeer(t, ctx, publisherNode, subscriberNode.GetNodeID())
	}, "publisher to remove subscriber from connected peers")

	restartedSubscriberNode := newStartedIntegrationNode(t, ctx, subscriberNode.GetNodeID())
	connectMeshNodes(t, ctx, publisherNode, restartedSubscriberNode)

	waitForNoInterestedPeer(t, ctx, publisherNode, topic, restartedSubscriberNode.GetNodeID())
}

func TestPeerInterestSnapshotRestoresMultipleTopicsAfterReconnect(t *testing.T) {
	ctx := context.Background()
	publisherNode := newStartedIntegrationNode(t, ctx, "multi-snapshot-publisher")
	subscriberNode := newStartedIntegrationNode(t, ctx, "multi-snapshot-subscriber")

	topics := []string{"orders.multi.*", "payments.multi.*"}
	subscriber := NewTrustedClient("multi-snapshot-subscriber-client")
	for _, topic := range topics {
		if err := subscriberNode.Subscribe(ctx, subscriber, topic); err != nil {
			t.Fatalf("Failed to subscribe to %s: %v", topic, err)
		}
	}

	connectMeshNodes(t, ctx, publisherNode, subscriberNode)
	waitForInterestedPeer(t, ctx, publisherNode, "orders.multi.created", subscriberNode.GetNodeID())
	waitForInterestedPeer(t, ctx, publisherNode, "payments.multi.completed", subscriberNode.GetNodeID())
}
