package routingtable

import (
	"context"
	"testing"

	"github.com/rmacdonaldsmith/eventmesh-go/pkg/routingtable"
)

// TestInMemoryRoutingTable_WildcardMatching tests wildcard pattern matching
// Phase 2 implementation of REQ-RT-001
func TestInMemoryRoutingTable_WildcardMatching(t *testing.T) {

	rt := NewInMemoryRoutingTable()
	defer rt.Close()
	ctx := context.Background()

	subscriber := routingtable.NewLocalSubscriber("client-1")

	// Test single-level wildcard matching: "orders.*" should match "orders.created", "orders.updated"
	err := rt.Subscribe(ctx, "orders.*", subscriber)
	if err != nil {
		t.Fatalf("Subscribe to wildcard pattern failed: %v", err)
	}

	// These exact topics should match the wildcard pattern
	subscribers, err := rt.GetSubscribers(ctx, "orders.created")
	if err != nil {
		t.Fatalf("GetSubscribers failed: %v", err)
	}
	if len(subscribers) != 1 {
		t.Errorf("Expected 1 subscriber for 'orders.created' matching 'orders.*', got %d", len(subscribers))
	}

	subscribers, err = rt.GetSubscribers(ctx, "orders.updated")
	if err != nil {
		t.Fatalf("GetSubscribers failed: %v", err)
	}
	if len(subscribers) != 1 {
		t.Errorf("Expected 1 subscriber for 'orders.updated' matching 'orders.*', got %d", len(subscribers))
	}

	// This should NOT match the pattern
	subscribers, err = rt.GetSubscribers(ctx, "inventory.created")
	if err != nil {
		t.Fatalf("GetSubscribers failed: %v", err)
	}
	if len(subscribers) != 0 {
		t.Errorf("Expected 0 subscribers for 'inventory.created' (should not match 'orders.*'), got %d", len(subscribers))
	}
}

// TestInMemoryRoutingTable_WildcardPrecedence tests precedence between exact and wildcard matches
// Phase 2 implementation of REQ-RT-001
func TestInMemoryRoutingTable_WildcardPrecedence(t *testing.T) {

	rt := NewInMemoryRoutingTable()
	defer rt.Close()
	ctx := context.Background()

	wildcardSub := routingtable.NewLocalSubscriber("wildcard-client")
	exactSub := routingtable.NewLocalSubscriber("exact-client")

	// Subscribe to both wildcard pattern and exact match
	err := rt.Subscribe(ctx, "orders.*", wildcardSub)
	if err != nil {
		t.Fatalf("Subscribe to wildcard failed: %v", err)
	}

	err = rt.Subscribe(ctx, "orders.created", exactSub)
	if err != nil {
		t.Fatalf("Subscribe to exact topic failed: %v", err)
	}

	// Both should match "orders.created"
	subscribers, err := rt.GetSubscribers(ctx, "orders.created")
	if err != nil {
		t.Fatalf("GetSubscribers failed: %v", err)
	}

	if len(subscribers) != 2 {
		t.Fatalf("Expected 2 subscribers (exact + wildcard match), got %d", len(subscribers))
	}

	// Verify both subscriber types are present
	foundExact := false
	foundWildcard := false
	for _, sub := range subscribers {
		if sub.ID() == "exact-client" {
			foundExact = true
		}
		if sub.ID() == "wildcard-client" {
			foundWildcard = true
		}
	}

	if !foundExact {
		t.Error("Missing exact match subscriber")
	}
	if !foundWildcard {
		t.Error("Missing wildcard match subscriber")
	}
}

// TestInMemoryRoutingTable_ComplexWildcardPatterns tests various wildcard patterns
// Phase 2 implementation of REQ-RT-001
func TestInMemoryRoutingTable_ComplexWildcardPatterns(t *testing.T) {

	rt := NewInMemoryRoutingTable()
	defer rt.Close()
	ctx := context.Background()

	subscriber := routingtable.NewLocalSubscriber("client-1")

	testCases := []struct {
		pattern     string
		shouldMatch []string
		shouldNotMatch []string
	}{
		{
			pattern:     "*.created",
			shouldMatch: []string{"orders.created", "user.created", "inventory.created"},
			shouldNotMatch: []string{"orders.updated", "orders.created.v2", "created"},
		},
		{
			pattern:     "orders.*.event",
			shouldMatch: []string{"orders.payment.event", "orders.shipping.event"},
			shouldNotMatch: []string{"orders.created", "inventory.payment.event", "orders.payment.event.v2"},
		},
		{
			pattern:     "*",
			shouldMatch: []string{"orders", "users", "inventory"},
			shouldNotMatch: []string{"orders.created", "users.updated"},
		},
	}

	for _, tc := range testCases {
		// Subscribe to the pattern
		err := rt.Subscribe(ctx, tc.pattern, subscriber)
		if err != nil {
			t.Fatalf("Subscribe to pattern '%s' failed: %v", tc.pattern, err)
		}

		// Test topics that should match
		for _, topic := range tc.shouldMatch {
			subscribers, err := rt.GetSubscribers(ctx, topic)
			if err != nil {
				t.Fatalf("GetSubscribers for '%s' failed: %v", topic, err)
			}
			if len(subscribers) == 0 {
				t.Errorf("Pattern '%s' should match topic '%s' but found no subscribers", tc.pattern, topic)
			}
		}

		// Test topics that should NOT match
		for _, topic := range tc.shouldNotMatch {
			subscribers, err := rt.GetSubscribers(ctx, topic)
			if err != nil {
				t.Fatalf("GetSubscribers for '%s' failed: %v", topic, err)
			}
			if len(subscribers) > 0 {
				t.Errorf("Pattern '%s' should NOT match topic '%s' but found %d subscribers", tc.pattern, topic, len(subscribers))
			}
		}

		// Clean up for next test case
		err = rt.Unsubscribe(ctx, tc.pattern, subscriber.ID())
		if err != nil {
			t.Fatalf("Unsubscribe from pattern '%s' failed: %v", tc.pattern, err)
		}
	}
}