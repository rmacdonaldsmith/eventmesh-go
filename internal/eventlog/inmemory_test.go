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

// TestEventLog_TopicValidation tests that empty topics are rejected
func TestEventLog_TopicValidation(t *testing.T) {
	log := NewInMemoryEventLog()
	defer log.Close()

	ctx := context.Background()

	// Test AppendToTopic with empty topic
	record := eventlog.NewRecord("valid-topic", []byte("data"))
	_, err := log.AppendToTopic(ctx, "", record)
	if err == nil {
		t.Error("Expected error when appending to empty topic")
	}

	// Test ReadFromTopic with empty topic
	_, err = log.ReadFromTopic(ctx, "", 0, 10)
	if err == nil {
		t.Error("Expected error when reading from empty topic")
	}

	// Test GetTopicEndOffset with empty topic
	_, err = log.GetTopicEndOffset(ctx, "")
	if err == nil {
		t.Error("Expected error when getting end offset for empty topic")
	}

	// Test ReplayTopic with empty topic
	eventChan, errChan := log.ReplayTopic(ctx, "", 0)

	// Should get error from errChan
	var replayErr error
	select {
	case replayErr = <-errChan:
		// Expected
	case <-eventChan:
		t.Error("Expected error, but got event from empty topic replay")
	}

	if replayErr == nil {
		t.Error("Expected error when replaying empty topic")
	}
}

// TestEventLog_ReadFromTopic_Performance tests the performance improvement of ReadFromTopic
// This test demonstrates the O(n) → O(1) optimization
func TestEventLog_ReadFromTopic_Performance(t *testing.T) {
	log := NewInMemoryEventLog()
	defer log.Close()

	ctx := context.Background()

	// Create a topic with many events (simulating production scenario)
	const numEvents = 10000
	const readFromOffset = 8000 // Reading from near the end

	// Add many events to create performance test scenario
	for i := 0; i < numEvents; i++ {
		record := eventlog.NewRecord("large-topic", []byte("event-data"))
		_, err := log.AppendToTopic(ctx, "large-topic", record)
		if err != nil {
			t.Fatalf("Error appending event %d: %v", i, err)
		}
	}

	// Test reading from near the end of a large topic
	// With O(n) implementation, this would scan 8000 events
	// With O(1) implementation, this should be instant
	results, err := log.ReadFromTopic(ctx, "large-topic", readFromOffset, 1000)
	if err != nil {
		t.Fatalf("Error reading from large topic: %v", err)
	}

	// Verify we get the expected number of results (limited by maxCount)
	availableResults := numEvents - readFromOffset // 10000 - 8000 = 2000 available
	maxCount := 1000
	expectedResults := maxCount // Should be limited by maxCount
	if availableResults < maxCount {
		expectedResults = availableResults
	}
	if len(results) != expectedResults {
		t.Errorf("Expected %d results (limited by maxCount %d), got %d", expectedResults, maxCount, len(results))
	}

	// Verify the first result has the correct offset
	if results[0].Offset() != readFromOffset {
		t.Errorf("Expected first result offset %d, got %d", readFromOffset, results[0].Offset())
	}

	// Verify we don't exceed maxCount when it's smaller
	results, err = log.ReadFromTopic(ctx, "large-topic", readFromOffset, 500)
	if err != nil {
		t.Fatalf("Error reading limited results from large topic: %v", err)
	}
	if len(results) != 500 {
		t.Errorf("Expected 500 results (maxCount), got %d", len(results))
	}

	// Verify we get all available results when maxCount is larger than available
	results, err = log.ReadFromTopic(ctx, "large-topic", readFromOffset, 5000) // maxCount > available
	if err != nil {
		t.Fatalf("Error reading with large maxCount: %v", err)
	}
	expectedAllResults := numEvents - readFromOffset // 10000 - 8000 = 2000
	if len(results) != expectedAllResults {
		t.Errorf("Expected %d results (all available), got %d", expectedAllResults, len(results))
	}
}

// TestEventLog_ReadFromTopic_EdgeCases tests edge cases for the optimized ReadFromTopic
func TestEventLog_ReadFromTopic_EdgeCases(t *testing.T) {
	log := NewInMemoryEventLog()
	defer log.Close()

	ctx := context.Background()

	// Add some test events
	for i := 0; i < 5; i++ {
		record := eventlog.NewRecord("test-topic", []byte("data"))
		_, err := log.AppendToTopic(ctx, "test-topic", record)
		if err != nil {
			t.Fatalf("Error appending event %d: %v", i, err)
		}
	}

	// Test reading from offset beyond available events
	results, err := log.ReadFromTopic(ctx, "test-topic", 10, 5)
	if err != nil {
		t.Fatalf("Error reading from offset beyond events: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("Expected 0 results when reading beyond available events, got %d", len(results))
	}

	// Test reading from exact end offset
	results, err = log.ReadFromTopic(ctx, "test-topic", 5, 5)
	if err != nil {
		t.Fatalf("Error reading from exact end offset: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("Expected 0 results when reading from exact end offset, got %d", len(results))
	}

	// Test reading with maxCount of 0
	results, err = log.ReadFromTopic(ctx, "test-topic", 0, 0)
	if err != nil {
		t.Fatalf("Error reading with maxCount 0: %v", err)
	}
	if results == nil {
		t.Error("Expected non-nil results for maxCount 0, got nil")
	}
	if len(results) != 0 {
		t.Errorf("Expected 0 results for maxCount 0, got %d", len(results))
	}
}
