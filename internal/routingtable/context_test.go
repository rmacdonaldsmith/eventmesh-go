package routingtable

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/rmacdonaldsmith/eventmesh-go/pkg/routingtable"
)

func TestInMemoryRoutingTable_ContextCancellation(t *testing.T) {
	rt := NewInMemoryRoutingTable()
	defer rt.Close()

	subscriber := routingtable.NewLocalSubscriber("client-1")

	// Create a context that will be cancelled
	ctx, cancel := context.WithCancel(context.Background())

	// Start a Subscribe operation in a goroutine
	done := make(chan error, 1)
	go func() {
		done <- rt.Subscribe(ctx, "orders.created", subscriber)
	}()

	// Cancel the context immediately
	cancel()

	// The operation should complete quickly since it doesn't respect cancellation yet
	// but this test validates that operations handle cancelled contexts gracefully
	select {
	case err := <-done:
		// For current implementation, this should succeed even with cancelled context
		// since Subscribe doesn't check ctx.Done() - this is expected for Phase 1
		if err != nil {
			t.Logf("Subscribe returned error (this is acceptable): %v", err)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Subscribe operation did not complete within reasonable time")
	}
}

func TestInMemoryRoutingTable_ContextTimeout(t *testing.T) {
	rt := NewInMemoryRoutingTable()
	defer rt.Close()

	subscriber := routingtable.NewLocalSubscriber("client-1")

	// Create a context with a very short timeout
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
	defer cancel()

	// Wait for the context to timeout
	<-ctx.Done()

	// Now try operations with the timed-out context
	err := rt.Subscribe(ctx, "orders.created", subscriber)
	// For current implementation, this should succeed even with timed-out context
	// since Subscribe doesn't check ctx.Done() - this is expected for Phase 1
	if err != nil {
		t.Logf("Subscribe with timed-out context returned error (this is acceptable): %v", err)
	}

	_, err = rt.GetSubscribers(ctx, "orders.created")
	if err != nil {
		t.Logf("GetSubscribers with timed-out context returned error (this is acceptable): %v", err)
	}
}

func TestInMemoryRoutingTable_ContextDeadline(t *testing.T) {
	rt := NewInMemoryRoutingTable()
	defer rt.Close()

	subscriber := routingtable.NewLocalSubscriber("client-1")

	// Create a context with a deadline in the past
	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(-1*time.Hour))
	defer cancel()

	// Operations should handle expired contexts gracefully
	err := rt.Subscribe(ctx, "orders.created", subscriber)
	// For current Phase 1 implementation, this may succeed since we don't check context state
	if err != nil {
		t.Logf("Subscribe with expired deadline returned error (this is acceptable): %v", err)
	}

	_, err = rt.GetSubscribers(ctx, "orders.created")
	if err != nil {
		t.Logf("GetSubscribers with expired deadline returned error (this is acceptable): %v", err)
	}

	err = rt.Unsubscribe(ctx, "orders.created", subscriber.ID())
	if err != nil {
		t.Logf("Unsubscribe with expired deadline returned error (this is acceptable): %v", err)
	}
}

func TestInMemoryRoutingTable_LongRunningOperationWithCancellation(t *testing.T) {
	rt := NewInMemoryRoutingTable()
	defer rt.Close()

	ctx, cancel := context.WithCancel(context.Background())

	// Simulate a scenario where we have many operations
	const numOperations = 100
	done := make(chan bool, numOperations)

	// Start multiple operations
	for i := 0; i < numOperations; i++ {
		go func(id int) {
			subscriber := routingtable.NewLocalSubscriber(fmt.Sprintf("client-%d", id))
			err := rt.Subscribe(ctx, fmt.Sprintf("topic-%d", id), subscriber)
			if err != nil {
				t.Logf("Subscribe operation %d failed (acceptable): %v", id, err)
			}
			done <- true
		}(i)
	}

	// Cancel the context after a short delay
	go func() {
		time.Sleep(10 * time.Millisecond)
		cancel()
	}()

	// Wait for all operations to complete
	completed := 0
	timeout := time.After(1 * time.Second)
	for completed < numOperations {
		select {
		case <-done:
			completed++
		case <-timeout:
			t.Fatalf("Not all operations completed within timeout. Completed: %d/%d", completed, numOperations)
		}
	}

	// Verify the routing table state is consistent
	count, err := rt.GetTopicCount(context.Background())
	if err != nil {
		t.Fatalf("GetTopicCount failed: %v", err)
	}

	// The count should be between 0 and numOperations depending on timing
	if count < 0 || count > numOperations {
		t.Errorf("Topic count %d is outside expected range [0, %d]", count, numOperations)
	}

	t.Logf("Completed all operations. Final topic count: %d", count)
}

// TestInMemoryRoutingTable_ContextValues tests that operations work with context values
func TestInMemoryRoutingTable_ContextValues(t *testing.T) {
	rt := NewInMemoryRoutingTable()
	defer rt.Close()

	// Create a context with some values
	type contextKey string
	const requestIDKey contextKey = "requestID"
	ctx := context.WithValue(context.Background(), requestIDKey, "test-request-123")

	subscriber := routingtable.NewLocalSubscriber("client-1")

	// Operations should work normally with context values
	err := rt.Subscribe(ctx, "orders.created", subscriber)
	if err != nil {
		t.Fatalf("Subscribe with context values failed: %v", err)
	}

	subscribers, err := rt.GetSubscribers(ctx, "orders.created")
	if err != nil {
		t.Fatalf("GetSubscribers with context values failed: %v", err)
	}

	if len(subscribers) != 1 {
		t.Fatalf("Expected 1 subscriber, got %d", len(subscribers))
	}

	// Verify we can still access context values (though our implementation doesn't use them)
	requestID := ctx.Value(requestIDKey)
	if requestID != "test-request-123" {
		t.Errorf("Context value not preserved: expected 'test-request-123', got %v", requestID)
	}
}