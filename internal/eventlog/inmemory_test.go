package eventlog

import (
	"context"
	"testing"

	"github.com/rmacdonaldsmith/eventmesh-go/pkg/eventlog"
)

// TestEventLog_AppendToTopic tests appending events to specific topics
func TestEventLog_AppendToTopic(t *testing.T) {
	log := NewInMemoryEventLog()
	defer log.Close()

	ctx := context.Background()

	// Create test records
	record1 := eventlog.NewRecord("topicA", []byte("payload1"))
	record2 := eventlog.NewRecord("topicA", []byte("payload2"))
	record3 := eventlog.NewRecord("topicB", []byte("payload3"))

	// Test appending to topicA
	result1, err := log.AppendToTopic(ctx, "topicA", record1)
	if err != nil {
		t.Fatalf("Expected no error appending to topicA, got: %v", err)
	}
	if result1.Offset() != 0 {
		t.Errorf("Expected first topicA record offset 0, got %d", result1.Offset())
	}

	result2, err := log.AppendToTopic(ctx, "topicA", record2)
	if err != nil {
		t.Fatalf("Expected no error appending to topicA, got: %v", err)
	}
	if result2.Offset() != 1 {
		t.Errorf("Expected second topicA record offset 1, got %d", result2.Offset())
	}

	// Test appending to topicB (should have independent offset sequence)
	result3, err := log.AppendToTopic(ctx, "topicB", record3)
	if err != nil {
		t.Fatalf("Expected no error appending to topicB, got: %v", err)
	}
	if result3.Offset() != 0 {
		t.Errorf("Expected first topicB record offset 0, got %d", result3.Offset())
	}
}

// TestEventLog_TopicIsolation verifies that topics have independent offset sequences
func TestEventLog_TopicIsolation(t *testing.T) {
	log := NewInMemoryEventLog()
	defer log.Close()

	ctx := context.Background()

	// Add events to multiple topics
	for i := 0; i < 3; i++ {
		record := eventlog.NewRecord("topicA", []byte("dataA"))
		result, err := log.AppendToTopic(ctx, "topicA", record)
		if err != nil {
			t.Fatalf("Error appending to topicA: %v", err)
		}
		if result.Offset() != int64(i) {
			t.Errorf("Expected topicA offset %d, got %d", i, result.Offset())
		}
	}

	for i := 0; i < 2; i++ {
		record := eventlog.NewRecord("topicB", []byte("dataB"))
		result, err := log.AppendToTopic(ctx, "topicB", record)
		if err != nil {
			t.Fatalf("Error appending to topicB: %v", err)
		}
		if result.Offset() != int64(i) {
			t.Errorf("Expected topicB offset %d, got %d", i, result.Offset())
		}
	}

	// Verify topic end offsets are independent
	endOffsetA, err := log.GetTopicEndOffset(ctx, "topicA")
	if err != nil {
		t.Fatalf("Error getting topicA end offset: %v", err)
	}
	if endOffsetA != 3 {
		t.Errorf("Expected topicA end offset 3, got %d", endOffsetA)
	}

	endOffsetB, err := log.GetTopicEndOffset(ctx, "topicB")
	if err != nil {
		t.Fatalf("Error getting topicB end offset: %v", err)
	}
	if endOffsetB != 2 {
		t.Errorf("Expected topicB end offset 2, got %d", endOffsetB)
	}
}

// TestEventLog_ReadFromTopic tests reading events from specific topics
func TestEventLog_ReadFromTopic(t *testing.T) {
	log := NewInMemoryEventLog()
	defer log.Close()

	ctx := context.Background()

	// Add test data to different topics
	log.AppendToTopic(ctx, "topicA", eventlog.NewRecord("topicA", []byte("A0")))
	log.AppendToTopic(ctx, "topicA", eventlog.NewRecord("topicA", []byte("A1")))
	log.AppendToTopic(ctx, "topicB", eventlog.NewRecord("topicB", []byte("B0")))
	log.AppendToTopic(ctx, "topicA", eventlog.NewRecord("topicA", []byte("A2")))

	// Read from topicA starting at offset 1
	results, err := log.ReadFromTopic(ctx, "topicA", 1, 10)
	if err != nil {
		t.Fatalf("Error reading from topicA: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("Expected 2 records from topicA, got %d", len(results))
	}
	if string(results[0].Payload()) != "A1" {
		t.Errorf("Expected payload 'A1', got '%s'", string(results[0].Payload()))
	}
	if string(results[1].Payload()) != "A2" {
		t.Errorf("Expected payload 'A2', got '%s'", string(results[1].Payload()))
	}

	// Read from topicB
	results, err = log.ReadFromTopic(ctx, "topicB", 0, 10)
	if err != nil {
		t.Fatalf("Error reading from topicB: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("Expected 1 record from topicB, got %d", len(results))
	}
	if string(results[0].Payload()) != "B0" {
		t.Errorf("Expected payload 'B0', got '%s'", string(results[0].Payload()))
	}
}

// TestEventLog_ReplayTopic tests replaying events from specific topics
func TestEventLog_ReplayTopic(t *testing.T) {
	log := NewInMemoryEventLog()
	defer log.Close()

	ctx := context.Background()

	// Add test data
	log.AppendToTopic(ctx, "topicA", eventlog.NewRecord("topicA", []byte("A0")))
	log.AppendToTopic(ctx, "topicB", eventlog.NewRecord("topicB", []byte("B0")))
	log.AppendToTopic(ctx, "topicA", eventlog.NewRecord("topicA", []byte("A1")))

	// Replay topicA from offset 0
	eventChan, errChan := log.ReplayTopic(ctx, "topicA", 0)

	var events []eventlog.EventRecord
	var replayErr error

	// Collect all events and errors
	for eventChan != nil || errChan != nil {
		select {
		case event, ok := <-eventChan:
			if !ok {
				eventChan = nil
			} else {
				events = append(events, event)
			}
		case err, ok := <-errChan:
			if !ok {
				errChan = nil
			} else if err != nil {
				replayErr = err
			}
		}
	}

	if replayErr != nil {
		t.Fatalf("Error during topicA replay: %v", replayErr)
	}

	if len(events) != 2 {
		t.Fatalf("Expected 2 events from topicA replay, got %d", len(events))
	}

	if string(events[0].Payload()) != "A0" {
		t.Errorf("Expected first event payload 'A0', got '%s'", string(events[0].Payload()))
	}
	if string(events[1].Payload()) != "A1" {
		t.Errorf("Expected second event payload 'A1', got '%s'", string(events[1].Payload()))
	}
}

// TestEventLog_GetTopicEndOffset tests getting end offsets for specific topics
func TestEventLog_GetTopicEndOffset(t *testing.T) {
	log := NewInMemoryEventLog()
	defer log.Close()

	ctx := context.Background()

	// Initially, non-existent topic should have end offset 0
	endOffset, err := log.GetTopicEndOffset(ctx, "nonexistent")
	if err != nil {
		t.Fatalf("Error getting end offset for nonexistent topic: %v", err)
	}
	if endOffset != 0 {
		t.Errorf("Expected end offset 0 for nonexistent topic, got %d", endOffset)
	}

	// Add events and verify end offsets
	log.AppendToTopic(ctx, "topicA", eventlog.NewRecord("topicA", []byte("data")))
	log.AppendToTopic(ctx, "topicA", eventlog.NewRecord("topicA", []byte("data")))

	endOffset, err = log.GetTopicEndOffset(ctx, "topicA")
	if err != nil {
		t.Fatalf("Error getting end offset for topicA: %v", err)
	}
	if endOffset != 2 {
		t.Errorf("Expected end offset 2 for topicA, got %d", endOffset)
	}
}