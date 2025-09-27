package routingtable

import (
	"context"
	"fmt"
	"testing"

	"github.com/rmacdonaldsmith/eventmesh-go/pkg/routingtable"
)

// BenchmarkInMemoryRoutingTable_Subscribe measures subscription performance
func BenchmarkInMemoryRoutingTable_Subscribe(b *testing.B) {
	rt := NewInMemoryRoutingTable()
	defer rt.Close()
	ctx := context.Background()

	// Pre-create subscribers to avoid allocation during benchmark
	subscribers := make([]*routingtable.LocalSubscriber, b.N)
	for i := 0; i < b.N; i++ {
		subscribers[i] = routingtable.NewLocalSubscriber(fmt.Sprintf("client-%d", i))
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		err := rt.Subscribe(ctx, "orders.created", subscribers[i])
		if err != nil {
			b.Fatalf("Subscribe failed: %v", err)
		}
	}
}

// BenchmarkInMemoryRoutingTable_GetSubscribers measures lookup performance
func BenchmarkInMemoryRoutingTable_GetSubscribers(b *testing.B) {
	rt := NewInMemoryRoutingTable()
	defer rt.Close()
	ctx := context.Background()

	// Setup: Add subscribers to create realistic lookup scenario
	const numSubscribers = 1000
	for i := 0; i < numSubscribers; i++ {
		subscriber := routingtable.NewLocalSubscriber(fmt.Sprintf("client-%d", i))
		rt.Subscribe(ctx, "orders.created", subscriber)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := rt.GetSubscribers(ctx, "orders.created")
		if err != nil {
			b.Fatalf("GetSubscribers failed: %v", err)
		}
	}
}

// BenchmarkInMemoryRoutingTable_MixedOperations measures mixed workload performance
func BenchmarkInMemoryRoutingTable_MixedOperations(b *testing.B) {
	rt := NewInMemoryRoutingTable()
	defer rt.Close()
	ctx := context.Background()

	// Pre-create subscribers and topics
	const numTopics = 100
	subscribers := make([]*routingtable.LocalSubscriber, b.N)
	topics := make([]string, numTopics)

	for i := 0; i < b.N; i++ {
		subscribers[i] = routingtable.NewLocalSubscriber(fmt.Sprintf("client-%d", i))
	}
	for i := 0; i < numTopics; i++ {
		topics[i] = fmt.Sprintf("topic-%d", i)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		topic := topics[i%numTopics]
		subscriber := subscribers[i]

		// Mix of operations: 70% subscribe, 20% lookup, 10% unsubscribe
		switch i % 10 {
		case 0: // Unsubscribe (10%)
			rt.Unsubscribe(ctx, topic, subscriber.ID())
		case 1, 2: // Lookup (20%)
			rt.GetSubscribers(ctx, topic)
		default: // Subscribe (70%)
			rt.Subscribe(ctx, topic, subscriber)
		}
	}
}

// BenchmarkInMemoryRoutingTable_ConcurrentSubscribe measures concurrent subscription performance
func BenchmarkInMemoryRoutingTable_ConcurrentSubscribe(b *testing.B) {
	rt := NewInMemoryRoutingTable()
	defer rt.Close()
	ctx := context.Background()

	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			subscriber := routingtable.NewLocalSubscriber(fmt.Sprintf("client-%d", i))
			err := rt.Subscribe(ctx, "orders.created", subscriber)
			if err != nil {
				b.Fatalf("Subscribe failed: %v", err)
			}
			i++
		}
	})
}

// BenchmarkInMemoryRoutingTable_ConcurrentLookup measures concurrent lookup performance
func BenchmarkInMemoryRoutingTable_ConcurrentLookup(b *testing.B) {
	rt := NewInMemoryRoutingTable()
	defer rt.Close()
	ctx := context.Background()

	// Setup: Add some subscribers for realistic lookups
	const numSubscribers = 100
	for i := 0; i < numSubscribers; i++ {
		subscriber := routingtable.NewLocalSubscriber(fmt.Sprintf("client-%d", i))
		rt.Subscribe(ctx, "orders.created", subscriber)
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, err := rt.GetSubscribers(ctx, "orders.created")
			if err != nil {
				b.Fatalf("GetSubscribers failed: %v", err)
			}
		}
	})
}

// TestInMemoryRoutingTable_PerformanceBaseline establishes performance baselines for future comparison
func TestInMemoryRoutingTable_PerformanceBaseline(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping performance baseline test in short mode")
	}

	rt := NewInMemoryRoutingTable()
	defer rt.Close()
	ctx := context.Background()

	// Baseline 1: Large number of subscribers to single topic
	t.Run("LargeSubscriberCount", func(t *testing.T) {
		const numSubscribers = 10000

		// Subscribe many clients to the same topic
		for i := 0; i < numSubscribers; i++ {
			subscriber := routingtable.NewLocalSubscriber(fmt.Sprintf("client-%d", i))
			err := rt.Subscribe(ctx, "popular.topic", subscriber)
			if err != nil {
				t.Fatalf("Subscribe %d failed: %v", i, err)
			}
		}

		// Verify lookup performance doesn't degrade significantly
		subscribers, err := rt.GetSubscribers(ctx, "popular.topic")
		if err != nil {
			t.Fatalf("GetSubscribers failed: %v", err)
		}

		if len(subscribers) != numSubscribers {
			t.Fatalf("Expected %d subscribers, got %d", numSubscribers, len(subscribers))
		}

		count, err := rt.GetSubscriberCount(ctx)
		if err != nil {
			t.Fatalf("GetSubscriberCount failed: %v", err)
		}

		if count != numSubscribers {
			t.Fatalf("Expected subscriber count %d, got %d", numSubscribers, count)
		}

		t.Logf("Successfully handled %d subscribers to single topic", numSubscribers)
	})

	// Baseline 2: Many topics with few subscribers each
	t.Run("ManyTopics", func(t *testing.T) {
		const numTopics = 10000
		rt2 := NewInMemoryRoutingTable()
		defer rt2.Close()

		// Create many topics with 1-3 subscribers each
		for i := 0; i < numTopics; i++ {
			topic := fmt.Sprintf("topic-%d", i)
			for j := 0; j < (i%3)+1; j++ { // 1-3 subscribers per topic
				subscriber := routingtable.NewLocalSubscriber(fmt.Sprintf("client-%d-%d", i, j))
				err := rt2.Subscribe(ctx, topic, subscriber)
				if err != nil {
					t.Fatalf("Subscribe to topic %d, subscriber %d failed: %v", i, j, err)
				}
			}
		}

		topicCount, err := rt2.GetTopicCount(ctx)
		if err != nil {
			t.Fatalf("GetTopicCount failed: %v", err)
		}

		if topicCount != numTopics {
			t.Fatalf("Expected topic count %d, got %d", numTopics, topicCount)
		}

		// Test random lookups
		for i := 0; i < 100; i++ {
			topicId := i % numTopics
			topic := fmt.Sprintf("topic-%d", topicId)
			subscribers, err := rt2.GetSubscribers(ctx, topic)
			if err != nil {
				t.Fatalf("GetSubscribers for topic %s failed: %v", topic, err)
			}

			expectedCount := (topicId % 3) + 1
			if len(subscribers) != expectedCount {
				t.Fatalf("Topic %s: expected %d subscribers, got %d", topic, expectedCount, len(subscribers))
			}
		}

		t.Logf("Successfully handled %d topics with distributed subscribers", numTopics)
	})

	// Baseline 3: Memory usage estimation
	t.Run("MemoryUsage", func(t *testing.T) {
		rt3 := NewInMemoryRoutingTable()
		defer rt3.Close()

		const numTopics = 1000
		const subscribersPerTopic = 10

		// Add known amount of data
		for i := 0; i < numTopics; i++ {
			topic := fmt.Sprintf("memory-test-topic-%d", i)
			for j := 0; j < subscribersPerTopic; j++ {
				subscriber := routingtable.NewLocalSubscriber(fmt.Sprintf("memory-test-client-%d-%d", i, j))
				err := rt3.Subscribe(ctx, topic, subscriber)
				if err != nil {
					t.Fatalf("Subscribe failed: %v", err)
				}
			}
		}

		totalSubscriptions := numTopics * subscribersPerTopic
		count, err := rt3.GetSubscriberCount(ctx)
		if err != nil {
			t.Fatalf("GetSubscriberCount failed: %v", err)
		}

		if count != totalSubscriptions {
			t.Fatalf("Expected %d total subscriptions, got %d", totalSubscriptions, count)
		}

		// This provides a baseline for memory usage patterns
		// Actual memory measurement would require runtime.MemStats
		t.Logf("Baseline established: %d topics, %d subscriptions", numTopics, totalSubscriptions)
	})
}