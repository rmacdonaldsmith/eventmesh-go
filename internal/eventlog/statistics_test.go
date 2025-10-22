package eventlog

import (
	"context"
	"testing"

	"github.com/rmacdonaldsmith/eventmesh-go/pkg/eventlog"
)

func TestInMemoryEventLog_GetStatistics(t *testing.T) {
	t.Run("empty log returns zero statistics", func(t *testing.T) {
		log := NewInMemoryEventLog()
		defer log.Close()

		ctx := context.Background()
		stats, err := log.GetStatistics(ctx)
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		if stats.TotalEvents != 0 {
			t.Errorf("Expected 0 total events, got %d", stats.TotalEvents)
		}
		if stats.TopicCount != 0 {
			t.Errorf("Expected 0 topics, got %d", stats.TopicCount)
		}
		if len(stats.TopicCounts) != 0 {
			t.Errorf("Expected empty topic counts, got %d entries", len(stats.TopicCounts))
		}
	})

	t.Run("log with events returns correct statistics", func(t *testing.T) {
		log := NewInMemoryEventLog()
		defer log.Close()

		ctx := context.Background()

		// Add events to multiple topics
		_, err := log.AppendToTopic(ctx, "orders", eventlog.NewRecord("orders", []byte("order1")))
		if err != nil {
			t.Fatalf("Failed to append order1: %v", err)
		}
		_, err = log.AppendToTopic(ctx, "orders", eventlog.NewRecord("orders", []byte("order2")))
		if err != nil {
			t.Fatalf("Failed to append order2: %v", err)
		}
		_, err = log.AppendToTopic(ctx, "users", eventlog.NewRecord("users", []byte("user1")))
		if err != nil {
			t.Fatalf("Failed to append user1: %v", err)
		}

		// Get statistics
		stats, err := log.GetStatistics(ctx)
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		// Verify total counts
		if stats.TotalEvents != 3 {
			t.Errorf("Expected 3 total events, got %d", stats.TotalEvents)
		}
		if stats.TopicCount != 2 {
			t.Errorf("Expected 2 topics, got %d", stats.TopicCount)
		}

		// Verify per-topic counts
		if stats.TopicCounts["orders"] != 2 {
			t.Errorf("Expected 2 orders events, got %d", stats.TopicCounts["orders"])
		}
		if stats.TopicCounts["users"] != 1 {
			t.Errorf("Expected 1 users event, got %d", stats.TopicCounts["users"])
		}
	})

	t.Run("closed log returns error", func(t *testing.T) {
		log := NewInMemoryEventLog()
		log.Close()

		ctx := context.Background()
		_, err := log.GetStatistics(ctx)
		if err == nil {
			t.Error("Expected error for closed log, got nil")
		}
	})
}