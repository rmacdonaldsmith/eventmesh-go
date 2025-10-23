package eventlog

import (
	"context"
	"testing"

	"github.com/rmacdonaldsmith/eventmesh-go/pkg/eventlog"
)

// TestEventLog_AppendEvent tests appending events to specific topics
func TestEventLog_AppendEvent(t *testing.T) {
	log := NewInMemoryEventLog()
	defer log.Close()

	ctx := context.Background()

	// Create test events
	event1 := eventlog.NewEvent("topicA", []byte("payload1"))
	event2 := eventlog.NewEvent("topicA", []byte("payload2"))
	event3 := eventlog.NewEvent("topicB", []byte("payload3"))

	// Test appending to topicA
	result1, err := log.AppendEvent(ctx, "topicA", event1)
	if err != nil {
		t.Fatalf("Expected no error appending to topicA, got: %v", err)
	}
	if result1.Offset != 0 {
		t.Errorf("Expected first topicA event offset 0, got %d", result1.Offset)
	}

	result2, err := log.AppendEvent(ctx, "topicA", event2)
	if err != nil {
		t.Fatalf("Expected no error appending second event to topicA, got: %v", err)
	}
	if result2.Offset != 1 {
		t.Errorf("Expected second topicA event offset 1, got %d", result2.Offset)
	}

	// Test appending to topicB (should start at offset 0)
	result3, err := log.AppendEvent(ctx, "topicB", event3)
	if err != nil {
		t.Fatalf("Expected no error appending to topicB, got: %v", err)
	}
	if result3.Offset != 0 {
		t.Errorf("Expected first topicB event offset 0, got %d", result3.Offset)
	}
}

// TestEventLog_TopicIsolation tests that topics have independent offset sequences
func TestEventLog_TopicIsolation(t *testing.T) {
	log := NewInMemoryEventLog()
	defer log.Close()

	ctx := context.Background()

	// Create events for different topics
	event1 := eventlog.NewEvent("users", []byte("user1"))
	event2 := eventlog.NewEvent("orders", []byte("order1"))
	event3 := eventlog.NewEvent("users", []byte("user2"))

	// Append in mixed order
	result1, _ := log.AppendEvent(ctx, "users", event1)
	result2, _ := log.AppendEvent(ctx, "orders", event2)
	result3, _ := log.AppendEvent(ctx, "users", event3)

	// Verify independent offset sequences
	if result1.Offset != 0 {
		t.Errorf("Expected first users event offset 0, got %d", result1.Offset)
	}
	if result2.Offset != 0 {
		t.Errorf("Expected first orders event offset 0, got %d", result2.Offset)
	}
	if result3.Offset != 1 {
		t.Errorf("Expected second users event offset 1, got %d", result3.Offset)
	}
}

// TestEventLog_ReadEvents tests reading events from topics
func TestEventLog_ReadEvents(t *testing.T) {
	log := NewInMemoryEventLog()
	defer log.Close()

	ctx := context.Background()
	topic := "test-topic"

	// Add some events
	for i := 0; i < 5; i++ {
		event := eventlog.NewEvent(topic, []byte("payload"))
		log.AppendEvent(ctx, topic, event)
	}

	// Test reading from start
	events, err := log.ReadEvents(ctx, topic, 0, 3)
	if err != nil {
		t.Fatalf("Expected no error reading events, got: %v", err)
	}
	if len(events) != 3 {
		t.Errorf("Expected 3 events, got %d", len(events))
	}

	// Test reading from middle
	events, err = log.ReadEvents(ctx, topic, 2, 2)
	if err != nil {
		t.Fatalf("Expected no error reading from middle, got: %v", err)
	}
	if len(events) != 2 {
		t.Errorf("Expected 2 events from middle, got %d", len(events))
	}
	if events[0].Offset != 2 {
		t.Errorf("Expected first event offset 2, got %d", events[0].Offset)
	}

	// Test reading beyond end
	events, err = log.ReadEvents(ctx, topic, 10, 5)
	if err != nil {
		t.Fatalf("Expected no error reading beyond end, got: %v", err)
	}
	if len(events) != 0 {
		t.Errorf("Expected 0 events beyond end, got %d", len(events))
	}
}

// TestEventLog_ReplayEvents tests the replay channel functionality
func TestEventLog_ReplayEvents(t *testing.T) {
	log := NewInMemoryEventLog()
	defer log.Close()

	ctx := context.Background()
	topic := "replay-topic"

	// Add events
	for i := 0; i < 3; i++ {
		event := eventlog.NewEvent(topic, []byte("payload"))
		log.AppendEvent(ctx, topic, event)
	}

	// Replay from start
	eventChan, errChan := log.ReplayEvents(ctx, topic, 0)

	var events []*eventlog.Event
	for event := range eventChan {
		events = append(events, event)
	}

	// Check for errors
	select {
	case err := <-errChan:
		if err != nil {
			t.Fatalf("Unexpected error during replay: %v", err)
		}
	default:
	}

	if len(events) != 3 {
		t.Errorf("Expected 3 replayed events, got %d", len(events))
	}

	for i, event := range events {
		if event.Offset != int64(i) {
			t.Errorf("Expected event %d to have offset %d, got %d", i, i, event.Offset)
		}
	}
}

// TestEventLog_GetTopicEndOffset tests getting topic end offsets
func TestEventLog_GetTopicEndOffset(t *testing.T) {
	log := NewInMemoryEventLog()
	defer log.Close()

	ctx := context.Background()

	// Test empty topic
	offset, err := log.GetTopicEndOffset(ctx, "empty-topic")
	if err != nil {
		t.Fatalf("Expected no error for empty topic, got: %v", err)
	}
	if offset != 0 {
		t.Errorf("Expected end offset 0 for empty topic, got %d", offset)
	}

	// Add events and check offset
	topic := "test-topic"
	for i := 0; i < 3; i++ {
		event := eventlog.NewEvent(topic, []byte("payload"))
		log.AppendEvent(ctx, topic, event)
	}

	offset, err = log.GetTopicEndOffset(ctx, topic)
	if err != nil {
		t.Fatalf("Expected no error getting end offset, got: %v", err)
	}
	if offset != 3 {
		t.Errorf("Expected end offset 3, got %d", offset)
	}
}

// TestEventLog_TopicValidation tests topic validation
func TestEventLog_TopicValidation(t *testing.T) {
	log := NewInMemoryEventLog()
	defer log.Close()

	ctx := context.Background()
	event := eventlog.NewEvent("valid", []byte("payload"))

	// Test empty topic
	_, err := log.AppendEvent(ctx, "", event)
	if err == nil {
		t.Error("Expected error for empty topic")
	}

	// Test nil event
	_, err = log.AppendEvent(ctx, "valid", nil)
	if err == nil {
		t.Error("Expected error for nil event")
	}
}

// TestInMemoryEventLog_GetStatistics tests statistics functionality
func TestInMemoryEventLog_GetStatistics(t *testing.T) {
	log := NewInMemoryEventLog()
	defer log.Close()

	ctx := context.Background()

	t.Run("empty_log_returns_zero_statistics", func(t *testing.T) {
		stats, err := log.GetStatistics(ctx)
		if err != nil {
			t.Fatalf("Expected no error getting statistics, got: %v", err)
		}
		if stats.TotalEvents != 0 {
			t.Errorf("Expected 0 total events, got %d", stats.TotalEvents)
		}
		if stats.TopicCount != 0 {
			t.Errorf("Expected 0 topics, got %d", stats.TopicCount)
		}
	})

	t.Run("log_with_events_returns_correct_statistics", func(t *testing.T) {
		// Add events to different topics
		topics := []string{"users", "orders", "products"}
		eventCounts := []int{3, 2, 1}

		for i, topic := range topics {
			for j := 0; j < eventCounts[i]; j++ {
				event := eventlog.NewEvent(topic, []byte("payload"))
				log.AppendEvent(ctx, topic, event)
			}
		}

		stats, err := log.GetStatistics(ctx)
		if err != nil {
			t.Fatalf("Expected no error getting statistics, got: %v", err)
		}

		if stats.TotalEvents != 6 {
			t.Errorf("Expected 6 total events, got %d", stats.TotalEvents)
		}
		if stats.TopicCount != 3 {
			t.Errorf("Expected 3 topics, got %d", stats.TopicCount)
		}

		for i, topic := range topics {
			if count, exists := stats.TopicCounts[topic]; !exists || count != int64(eventCounts[i]) {
				t.Errorf("Expected %d events for topic %s, got %d", eventCounts[i], topic, count)
			}
		}
	})

	t.Run("closed_log_returns_error", func(t *testing.T) {
		closedLog := NewInMemoryEventLog()
		closedLog.Close()

		_, err := closedLog.GetStatistics(ctx)
		if err == nil {
			t.Error("Expected error for closed log")
		}
	})
}
