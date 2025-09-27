package routingtable

import (
	"context"
	"fmt"
	"sync"
	"testing"

	"github.com/rmacdonaldsmith/eventmesh-go/pkg/routingtable"
)

func TestInMemoryRoutingTable_Subscribe(t *testing.T) {
	rt := NewInMemoryRoutingTable()
	defer rt.Close()
	ctx := context.Background()

	subscriber := routingtable.NewLocalSubscriber("client-1")

	err := rt.Subscribe(ctx, "orders.created", subscriber)
	if err != nil {
		t.Fatalf("Subscribe failed: %v", err)
	}

	subscribers, err := rt.GetSubscribers(ctx, "orders.created")
	if err != nil {
		t.Fatalf("GetSubscribers failed: %v", err)
	}

	if len(subscribers) != 1 {
		t.Fatalf("Expected 1 subscriber, got %d", len(subscribers))
	}

	if subscribers[0].ID() != "client-1" {
		t.Errorf("Expected subscriber ID 'client-1', got '%s'", subscribers[0].ID())
	}
}

func TestInMemoryRoutingTable_Subscribe_NilSubscriber(t *testing.T) {
	rt := NewInMemoryRoutingTable()
	defer rt.Close()
	ctx := context.Background()

	err := rt.Subscribe(ctx, "orders.created", nil)
	if err == nil {
		t.Fatal("Expected error for nil subscriber")
	}
}

func TestInMemoryRoutingTable_Subscribe_EmptyTopic(t *testing.T) {
	rt := NewInMemoryRoutingTable()
	defer rt.Close()
	ctx := context.Background()

	subscriber := routingtable.NewLocalSubscriber("client-1")

	err := rt.Subscribe(ctx, "", subscriber)
	if err == nil {
		t.Fatal("Expected error for empty topic")
	}
}

func TestInMemoryRoutingTable_Unsubscribe(t *testing.T) {
	rt := NewInMemoryRoutingTable()
	defer rt.Close()
	ctx := context.Background()

	subscriber := routingtable.NewLocalSubscriber("client-1")

	// Subscribe first
	err := rt.Subscribe(ctx, "orders.created", subscriber)
	if err != nil {
		t.Fatalf("Subscribe failed: %v", err)
	}

	// Unsubscribe
	err = rt.Unsubscribe(ctx, "orders.created", "client-1")
	if err != nil {
		t.Fatalf("Unsubscribe failed: %v", err)
	}

	// Verify no subscribers
	subscribers, err := rt.GetSubscribers(ctx, "orders.created")
	if err != nil {
		t.Fatalf("GetSubscribers failed: %v", err)
	}

	if len(subscribers) != 0 {
		t.Fatalf("Expected 0 subscribers after unsubscribe, got %d", len(subscribers))
	}
}

func TestInMemoryRoutingTable_GetSubscribers_NoMatch(t *testing.T) {
	rt := NewInMemoryRoutingTable()
	defer rt.Close()
	ctx := context.Background()

	subscribers, err := rt.GetSubscribers(ctx, "non.existent.topic")
	if err != nil {
		t.Fatalf("GetSubscribers failed: %v", err)
	}

	if len(subscribers) != 0 {
		t.Fatalf("Expected 0 subscribers for non-existent topic, got %d", len(subscribers))
	}
}

func TestInMemoryRoutingTable_MultipleSubscribers(t *testing.T) {
	rt := NewInMemoryRoutingTable()
	defer rt.Close()
	ctx := context.Background()

	local1 := routingtable.NewLocalSubscriber("client-1")
	local2 := routingtable.NewLocalSubscriber("client-2")
	peer1 := routingtable.NewPeerSubscriber("node-1")

	// Subscribe all to same topic
	err := rt.Subscribe(ctx, "orders.created", local1)
	if err != nil {
		t.Fatalf("Subscribe failed: %v", err)
	}

	err = rt.Subscribe(ctx, "orders.created", local2)
	if err != nil {
		t.Fatalf("Subscribe failed: %v", err)
	}

	err = rt.Subscribe(ctx, "orders.created", peer1)
	if err != nil {
		t.Fatalf("Subscribe failed: %v", err)
	}

	subscribers, err := rt.GetSubscribers(ctx, "orders.created")
	if err != nil {
		t.Fatalf("GetSubscribers failed: %v", err)
	}

	if len(subscribers) != 3 {
		t.Fatalf("Expected 3 subscribers, got %d", len(subscribers))
	}

	// Verify all subscriber IDs are present
	ids := make(map[string]bool)
	for _, sub := range subscribers {
		ids[sub.ID()] = true
	}

	expected := []string{"client-1", "client-2", "node-1"}
	for _, id := range expected {
		if !ids[id] {
			t.Errorf("Missing subscriber ID: %s", id)
		}
	}
}

func TestInMemoryRoutingTable_ConcurrentAccess(t *testing.T) {
	rt := NewInMemoryRoutingTable()
	defer rt.Close()
	ctx := context.Background()

	var wg sync.WaitGroup
	const numWorkers = 10

	// Concurrent subscribe operations
	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			subscriber := routingtable.NewLocalSubscriber(fmt.Sprintf("client-%d", id))
			err := rt.Subscribe(ctx, "orders.created", subscriber)
			if err != nil {
				t.Errorf("Subscribe failed for client-%d: %v", id, err)
			}
		}(i)
	}

	wg.Wait()

	// Verify all subscribers were added
	subscribers, err := rt.GetSubscribers(ctx, "orders.created")
	if err != nil {
		t.Fatalf("GetSubscribers failed: %v", err)
	}

	if len(subscribers) != numWorkers {
		t.Fatalf("Expected %d subscribers, got %d", numWorkers, len(subscribers))
	}
}

func TestInMemoryRoutingTable_Close(t *testing.T) {
	rt := NewInMemoryRoutingTable()

	err := rt.Close()
	if err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	// Operations after close should return error
	ctx := context.Background()
	subscriber := routingtable.NewLocalSubscriber("client-1")

	err = rt.Subscribe(ctx, "orders.created", subscriber)
	if err == nil {
		t.Fatal("Expected error when subscribing to closed routing table")
	}
}