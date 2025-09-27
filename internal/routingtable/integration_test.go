package routingtable

import (
	"context"
	"testing"

	"github.com/rmacdonaldsmith/eventmesh-go/pkg/routingtable"
)

func TestInMemoryRoutingTable_GetAllSubscriptions(t *testing.T) {
	rt := NewInMemoryRoutingTable()
	defer rt.Close()
	ctx := context.Background()

	// Add some subscriptions
	local1 := routingtable.NewLocalSubscriber("client-1")
	local2 := routingtable.NewLocalSubscriber("client-2")
	peer1 := routingtable.NewPeerSubscriber("node-1")

	rt.Subscribe(ctx, "orders.created", local1)
	rt.Subscribe(ctx, "orders.updated", local1)
	rt.Subscribe(ctx, "orders.created", local2)
	rt.Subscribe(ctx, "inventory.updated", peer1)

	subscriptions, err := rt.GetAllSubscriptions(ctx)
	if err != nil {
		t.Fatalf("GetAllSubscriptions failed: %v", err)
	}

	if len(subscriptions) != 4 {
		t.Fatalf("Expected 4 subscriptions, got %d", len(subscriptions))
	}

	// Verify subscription counts by topic
	topicCounts := make(map[string]int)
	for _, sub := range subscriptions {
		topicCounts[sub.Topic]++
	}

	if topicCounts["orders.created"] != 2 {
		t.Errorf("Expected 2 subscribers for orders.created, got %d", topicCounts["orders.created"])
	}
	if topicCounts["orders.updated"] != 1 {
		t.Errorf("Expected 1 subscriber for orders.updated, got %d", topicCounts["orders.updated"])
	}
	if topicCounts["inventory.updated"] != 1 {
		t.Errorf("Expected 1 subscriber for inventory.updated, got %d", topicCounts["inventory.updated"])
	}
}

func TestInMemoryRoutingTable_RebuildFromGossip(t *testing.T) {
	rt := NewInMemoryRoutingTable()
	defer rt.Close()
	ctx := context.Background()

	// Add initial subscriptions
	local1 := routingtable.NewLocalSubscriber("client-1")
	rt.Subscribe(ctx, "orders.created", local1)

	// Verify initial state
	count, err := rt.GetTopicCount(ctx)
	if err != nil {
		t.Fatalf("GetTopicCount failed: %v", err)
	}
	if count != 1 {
		t.Fatalf("Expected 1 topic initially, got %d", count)
	}

	// Rebuild from gossip data
	peer1 := routingtable.NewPeerSubscriber("node-1")
	peer2 := routingtable.NewPeerSubscriber("node-2")

	gossipData := []routingtable.Subscription{
		{Topic: "inventory.updated", Subscriber: peer1},
		{Topic: "user.created", Subscriber: peer2},
		{Topic: "user.created", Subscriber: peer1}, // Same topic, different subscriber
	}

	err = rt.RebuildFromGossip(ctx, gossipData)
	if err != nil {
		t.Fatalf("RebuildFromGossip failed: %v", err)
	}

	// Verify the table was rebuilt (original data should be gone)
	count, err = rt.GetTopicCount(ctx)
	if err != nil {
		t.Fatalf("GetTopicCount failed: %v", err)
	}
	if count != 2 {
		t.Fatalf("Expected 2 topics after rebuild, got %d", count)
	}

	// Verify specific subscriptions
	subscribers, err := rt.GetSubscribers(ctx, "user.created")
	if err != nil {
		t.Fatalf("GetSubscribers failed: %v", err)
	}
	if len(subscribers) != 2 {
		t.Fatalf("Expected 2 subscribers for user.created, got %d", len(subscribers))
	}

	subscribers, err = rt.GetSubscribers(ctx, "orders.created")
	if err != nil {
		t.Fatalf("GetSubscribers failed: %v", err)
	}
	if len(subscribers) != 0 {
		t.Fatalf("Expected 0 subscribers for orders.created after rebuild, got %d", len(subscribers))
	}
}

func TestInMemoryRoutingTable_GetTopicCount(t *testing.T) {
	rt := NewInMemoryRoutingTable()
	defer rt.Close()
	ctx := context.Background()

	// Initially empty
	count, err := rt.GetTopicCount(ctx)
	if err != nil {
		t.Fatalf("GetTopicCount failed: %v", err)
	}
	if count != 0 {
		t.Fatalf("Expected 0 topics initially, got %d", count)
	}

	// Add subscriptions to different topics
	local1 := routingtable.NewLocalSubscriber("client-1")
	local2 := routingtable.NewLocalSubscriber("client-2")

	rt.Subscribe(ctx, "orders.created", local1)
	rt.Subscribe(ctx, "orders.created", local2) // Same topic
	rt.Subscribe(ctx, "inventory.updated", local1) // Different topic

	count, err = rt.GetTopicCount(ctx)
	if err != nil {
		t.Fatalf("GetTopicCount failed: %v", err)
	}
	if count != 2 {
		t.Fatalf("Expected 2 unique topics, got %d", count)
	}
}

func TestInMemoryRoutingTable_GetSubscriberCount(t *testing.T) {
	rt := NewInMemoryRoutingTable()
	defer rt.Close()
	ctx := context.Background()

	// Initially empty
	count, err := rt.GetSubscriberCount(ctx)
	if err != nil {
		t.Fatalf("GetSubscriberCount failed: %v", err)
	}
	if count != 0 {
		t.Fatalf("Expected 0 subscribers initially, got %d", count)
	}

	// Add multiple subscriptions
	local1 := routingtable.NewLocalSubscriber("client-1")
	local2 := routingtable.NewLocalSubscriber("client-2")
	peer1 := routingtable.NewPeerSubscriber("node-1")

	rt.Subscribe(ctx, "orders.created", local1)
	rt.Subscribe(ctx, "orders.created", local2)
	rt.Subscribe(ctx, "inventory.updated", local1) // Same subscriber, different topic
	rt.Subscribe(ctx, "user.created", peer1)

	count, err = rt.GetSubscriberCount(ctx)
	if err != nil {
		t.Fatalf("GetSubscriberCount failed: %v", err)
	}
	if count != 4 {
		t.Fatalf("Expected 4 total subscriptions, got %d", count)
	}

	// Unsubscribe one and verify count decreases
	rt.Unsubscribe(ctx, "orders.created", "client-1")

	count, err = rt.GetSubscriberCount(ctx)
	if err != nil {
		t.Fatalf("GetSubscriberCount failed: %v", err)
	}
	if count != 3 {
		t.Fatalf("Expected 3 subscriptions after unsubscribe, got %d", count)
	}
}

func TestInMemoryRoutingTable_InterfaceCompliance(t *testing.T) {
	// Verify that InMemoryRoutingTable implements the RoutingTable interface
	var _ routingtable.RoutingTable = (*InMemoryRoutingTable)(nil)
}

func TestInMemoryRoutingTable_DuplicateSubscriptions(t *testing.T) {
	rt := NewInMemoryRoutingTable()
	defer rt.Close()
	ctx := context.Background()

	subscriber := routingtable.NewLocalSubscriber("client-1")

	// Subscribe twice to the same topic
	err := rt.Subscribe(ctx, "orders.created", subscriber)
	if err != nil {
		t.Fatalf("First subscribe failed: %v", err)
	}

	err = rt.Subscribe(ctx, "orders.created", subscriber)
	if err != nil {
		t.Fatalf("Second subscribe failed: %v", err)
	}

	// Should only have one subscription
	subscribers, err := rt.GetSubscribers(ctx, "orders.created")
	if err != nil {
		t.Fatalf("GetSubscribers failed: %v", err)
	}

	if len(subscribers) != 1 {
		t.Fatalf("Expected 1 subscriber (no duplicates), got %d", len(subscribers))
	}

	count, err := rt.GetSubscriberCount(ctx)
	if err != nil {
		t.Fatalf("GetSubscriberCount failed: %v", err)
	}
	if count != 1 {
		t.Fatalf("Expected total count of 1, got %d", count)
	}
}