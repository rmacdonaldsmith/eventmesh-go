package eventlog

import (
	"bytes"
	"context"
	"sync"
	"testing"
	"time"

	eventlogpkg "github.com/rmacdonaldsmith/eventmesh-go/pkg/eventlog"
)

// Helper function to create test context with timeout
func testContext() context.Context {
	ctx, _ := context.WithTimeout(context.Background(), 5*time.Second)
	return ctx
}

// Basic functionality tests

func TestAppend_ShouldReturnSequentialOffsets(t *testing.T) {
	// Arrange
	log := NewInMemoryEventLog()
	defer log.Close()

	eventData := []byte("test event")
	record1 := NewRecord("test-topic", eventData)
	record2 := NewRecord("test-topic", eventData)

	// Act
	result1, err1 := log.Append(testContext(), record1)
	result2, err2 := log.Append(testContext(), record2)

	// Assert
	if err1 != nil {
		t.Fatalf("Expected no error for first append, got %v", err1)
	}
	if err2 != nil {
		t.Fatalf("Expected no error for second append, got %v", err2)
	}

	if result1.Offset() != 0 {
		t.Errorf("Expected first offset to be 0, got %d", result1.Offset())
	}
	if result2.Offset() != 1 {
		t.Errorf("Expected second offset to be 1, got %d", result2.Offset())
	}

	if !bytes.Equal(result1.Payload(), eventData) {
		t.Errorf("Expected first payload to match original data")
	}
	if !bytes.Equal(result2.Payload(), eventData) {
		t.Errorf("Expected second payload to match original data")
	}

	if result1.Topic() != "test-topic" {
		t.Errorf("Expected first topic to be 'test-topic', got '%s'", result1.Topic())
	}
	if result2.Topic() != "test-topic" {
		t.Errorf("Expected second topic to be 'test-topic', got '%s'", result2.Topic())
	}
}

func TestReadFrom_ShouldReturnEventsFromOffset(t *testing.T) {
	// Arrange
	log := NewInMemoryEventLog()
	defer log.Close()

	record1 := NewRecord("topic1", []byte("event1"))
	record2 := NewRecord("topic2", []byte("event2"))
	record3 := NewRecord("topic3", []byte("event3"))

	_, err := log.Append(testContext(), record1)
	if err != nil {
		t.Fatalf("Failed to append first record: %v", err)
	}
	_, err = log.Append(testContext(), record2)
	if err != nil {
		t.Fatalf("Failed to append second record: %v", err)
	}
	_, err = log.Append(testContext(), record3)
	if err != nil {
		t.Fatalf("Failed to append third record: %v", err)
	}

	// Act
	results, err := log.ReadFrom(testContext(), 1, 10)

	// Assert
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if len(results) != 2 {
		t.Errorf("Expected 2 results, got %d", len(results))
	}

	if results[0].Offset() != 1 {
		t.Errorf("Expected first result offset to be 1, got %d", results[0].Offset())
	}
	if results[1].Offset() != 2 {
		t.Errorf("Expected second result offset to be 2, got %d", results[1].Offset())
	}

	if results[0].Topic() != "topic2" {
		t.Errorf("Expected first result topic to be 'topic2', got '%s'", results[0].Topic())
	}
	if results[1].Topic() != "topic3" {
		t.Errorf("Expected second result topic to be 'topic3', got '%s'", results[1].Topic())
	}
}

func TestReadFrom_ShouldRespectMaxCount(t *testing.T) {
	// Arrange
	log := NewInMemoryEventLog()
	defer log.Close()

	_, err := log.Append(testContext(), NewRecord("topic1", []byte("event1")))
	if err != nil {
		t.Fatalf("Failed to append first record: %v", err)
	}
	_, err = log.Append(testContext(), NewRecord("topic2", []byte("event2")))
	if err != nil {
		t.Fatalf("Failed to append second record: %v", err)
	}
	_, err = log.Append(testContext(), NewRecord("topic3", []byte("event3")))
	if err != nil {
		t.Fatalf("Failed to append third record: %v", err)
	}

	// Act
	results, err := log.ReadFrom(testContext(), 0, 2)

	// Assert
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if len(results) != 2 {
		t.Errorf("Expected 2 results, got %d", len(results))
	}

	if results[0].Offset() != 0 {
		t.Errorf("Expected first result offset to be 0, got %d", results[0].Offset())
	}
	if results[1].Offset() != 1 {
		t.Errorf("Expected second result offset to be 1, got %d", results[1].Offset())
	}
}

func TestReadFrom_ShouldReturnEmptyWhenOffsetTooHigh(t *testing.T) {
	// Arrange
	log := NewInMemoryEventLog()
	defer log.Close()

	_, err := log.Append(testContext(), NewRecord("topic1", []byte("event1")))
	if err != nil {
		t.Fatalf("Failed to append record: %v", err)
	}

	// Act
	results, err := log.ReadFrom(testContext(), 10, 5)

	// Assert
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if len(results) != 0 {
		t.Errorf("Expected empty results, got %d items", len(results))
	}
}

func TestGetEndOffset_ShouldReturnZeroForEmptyLog(t *testing.T) {
	// Arrange
	log := NewInMemoryEventLog()
	defer log.Close()

	// Act
	endOffset, err := log.GetEndOffset(testContext())

	// Assert
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if endOffset != 0 {
		t.Errorf("Expected end offset to be 0 for empty log, got %d", endOffset)
	}
}

func TestGetEndOffset_ShouldReturnNextAppendPosition(t *testing.T) {
	// Arrange
	log := NewInMemoryEventLog()
	defer log.Close()

	_, err := log.Append(testContext(), NewRecord("topic1", []byte("event1")))
	if err != nil {
		t.Fatalf("Failed to append first record: %v", err)
	}
	_, err = log.Append(testContext(), NewRecord("topic2", []byte("event2")))
	if err != nil {
		t.Fatalf("Failed to append second record: %v", err)
	}

	// Act
	endOffset, err := log.GetEndOffset(testContext())

	// Assert
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if endOffset != 2 {
		t.Errorf("Expected end offset to be 2 (next append position after offsets 0 and 1), got %d", endOffset)
	}
}

func TestReplay_ShouldReturnAllEventsFromOffset(t *testing.T) {
	// Arrange
	log := NewInMemoryEventLog()
	defer log.Close()

	_, err := log.Append(testContext(), NewRecord("topic1", []byte("event1")))
	if err != nil {
		t.Fatalf("Failed to append first record: %v", err)
	}
	_, err = log.Append(testContext(), NewRecord("topic2", []byte("event2")))
	if err != nil {
		t.Fatalf("Failed to append second record: %v", err)
	}
	_, err = log.Append(testContext(), NewRecord("topic3", []byte("event3")))
	if err != nil {
		t.Fatalf("Failed to append third record: %v", err)
	}

	// Act
	eventChan, errChan := log.Replay(testContext(), 1)
	var replayedEvents []interface{}

	// Collect all events from the channels
	for {
		select {
		case event, ok := <-eventChan:
			if !ok {
				// Channel closed, we're done
				goto assertResults
			}
			replayedEvents = append(replayedEvents, event)
		case err := <-errChan:
			if err != nil {
				t.Fatalf("Unexpected error during replay: %v", err)
			}
		case <-time.After(1 * time.Second):
			t.Fatal("Replay took too long")
		}
	}

assertResults:
	// Assert
	if len(replayedEvents) != 2 {
		t.Errorf("Expected 2 replayed events, got %d", len(replayedEvents))
	}

	if len(replayedEvents) >= 1 {
		event1 := replayedEvents[0].(eventlogpkg.EventRecord)
		if event1.Offset() != 1 {
			t.Errorf("Expected first replayed event offset to be 1, got %d", event1.Offset())
		}
		if event1.Topic() != "topic2" {
			t.Errorf("Expected first replayed event topic to be 'topic2', got '%s'", event1.Topic())
		}
	}

	if len(replayedEvents) >= 2 {
		event2 := replayedEvents[1].(eventlogpkg.EventRecord)
		if event2.Offset() != 2 {
			t.Errorf("Expected second replayed event offset to be 2, got %d", event2.Offset())
		}
		if event2.Topic() != "topic3" {
			t.Errorf("Expected second replayed event topic to be 'topic3', got '%s'", event2.Topic())
		}
	}
}

func TestReplay_ShouldReturnEmptyWhenOffsetTooHigh(t *testing.T) {
	// Arrange
	log := NewInMemoryEventLog()
	defer log.Close()

	_, err := log.Append(testContext(), NewRecord("topic1", []byte("event1")))
	if err != nil {
		t.Fatalf("Failed to append record: %v", err)
	}

	// Act
	eventChan, errChan := log.Replay(testContext(), 10)
	var replayedEvents []interface{}

	// Collect all events from the channels
	for {
		select {
		case event, ok := <-eventChan:
			if !ok {
				// Channel closed, we're done
				goto assertResults
			}
			replayedEvents = append(replayedEvents, event)
		case err := <-errChan:
			if err != nil {
				t.Fatalf("Unexpected error during replay: %v", err)
			}
		case <-time.After(1 * time.Second):
			t.Fatal("Replay took too long")
		}
	}

assertResults:
	// Assert
	if len(replayedEvents) != 0 {
		t.Errorf("Expected no replayed events, got %d", len(replayedEvents))
	}
}

func TestReplay_ShouldSupportCancellation(t *testing.T) {
	// Arrange
	log := NewInMemoryEventLog()
	defer log.Close()

	_, err := log.Append(testContext(), NewRecord("topic1", []byte("event1")))
	if err != nil {
		t.Fatalf("Failed to append first record: %v", err)
	}
	_, err = log.Append(testContext(), NewRecord("topic2", []byte("event2")))
	if err != nil {
		t.Fatalf("Failed to append second record: %v", err)
	}

	// Create a context that's immediately cancelled
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	// Act
	eventChan, errChan := log.Replay(ctx, 0)
	var replayedEvents []interface{}
	var replayError error

	// Collect results with timeout
	done := make(chan bool)
	go func() {
		for {
			select {
			case event, ok := <-eventChan:
				if !ok {
					done <- true
					return
				}
				replayedEvents = append(replayedEvents, event)
			case err := <-errChan:
				replayError = err
				done <- true
				return
			}
		}
	}()

	select {
	case <-done:
		// Replay completed
	case <-time.After(1 * time.Second):
		t.Fatal("Replay should have completed quickly due to cancellation")
	}

	// Assert
	// Should get context.Canceled error or no events due to cancellation
	if replayError != nil && replayError != context.Canceled {
		t.Errorf("Expected context.Canceled error or nil, got %v", replayError)
	}
	// We expect either no events or the cancellation to prevent events
	if len(replayedEvents) > 0 {
		t.Logf("Got %d events before cancellation (this is acceptable)", len(replayedEvents))
	}
}

func TestCompact_ShouldCompleteSuccessfully(t *testing.T) {
	// Arrange
	log := NewInMemoryEventLog()
	defer log.Close()

	_, err := log.Append(testContext(), NewRecord("topic1", []byte("event1")))
	if err != nil {
		t.Fatalf("Failed to append record: %v", err)
	}

	// Act & Assert - Should not return error
	err = log.Compact(testContext())
	if err != nil {
		t.Errorf("Expected no error from Compact, got %v", err)
	}

	// Verify log still works after compaction
	endOffset, err := log.GetEndOffset(testContext())
	if err != nil {
		t.Fatalf("Failed to get end offset after compaction: %v", err)
	}

	if endOffset != 1 {
		t.Errorf("Expected end offset to be 1 after compaction, got %d", endOffset)
	}
}

// Parameter validation tests

func TestAppend_WithNilRecord_ShouldReturnError(t *testing.T) {
	// Arrange
	log := NewInMemoryEventLog()
	defer log.Close()

	// Act
	result, err := log.Append(testContext(), nil)

	// Assert
	if err == nil {
		t.Error("Expected error for nil record, got nil")
	}
	if err != ErrNilRecord {
		t.Errorf("Expected ErrNilRecord, got %v", err)
	}
	if result != nil {
		t.Error("Expected nil result for error case")
	}
}

func TestReadFrom_WithNegativeOffset_ShouldReturnError(t *testing.T) {
	// Arrange
	log := NewInMemoryEventLog()
	defer log.Close()

	// Act
	results, err := log.ReadFrom(testContext(), -1, 10)

	// Assert
	if err == nil {
		t.Error("Expected error for negative offset, got nil")
	}
	if err != ErrNegativeOffset {
		t.Errorf("Expected ErrNegativeOffset, got %v", err)
	}
	if results != nil {
		t.Error("Expected nil results for error case")
	}
}

func TestReadFrom_WithNegativeMaxCount_ShouldReturnError(t *testing.T) {
	// Arrange
	log := NewInMemoryEventLog()
	defer log.Close()

	// Act
	results, err := log.ReadFrom(testContext(), 0, -1)

	// Assert
	if err == nil {
		t.Error("Expected error for negative max count, got nil")
	}
	if err != ErrNegativeMaxCount {
		t.Errorf("Expected ErrNegativeMaxCount, got %v", err)
	}
	if results != nil {
		t.Error("Expected nil results for error case")
	}
}

func TestReadFrom_WithZeroMaxCount_ShouldReturnEmpty(t *testing.T) {
	// Arrange
	log := NewInMemoryEventLog()
	defer log.Close()

	_, err := log.Append(testContext(), NewRecord("test", []byte("test")))
	if err != nil {
		t.Fatalf("Failed to append record: %v", err)
	}

	// Act
	results, err := log.ReadFrom(testContext(), 0, 0)

	// Assert
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if len(results) != 0 {
		t.Errorf("Expected empty results for zero max count, got %d items", len(results))
	}
}

func TestReplay_WithNegativeOffset_ShouldReturnError(t *testing.T) {
	// Arrange
	log := NewInMemoryEventLog()
	defer log.Close()

	// Act
	eventChan, errChan := log.Replay(testContext(), -1)

	// Wait for error
	select {
	case err := <-errChan:
		if err != ErrNegativeOffset {
			t.Errorf("Expected ErrNegativeOffset, got %v", err)
		}
	case <-eventChan:
		t.Error("Expected error, but got an event")
	case <-time.After(1 * time.Second):
		t.Fatal("Expected error quickly")
	}
}

// Edge case tests

func TestAppend_WithEmptyTopic_ShouldWork(t *testing.T) {
	// Arrange
	log := NewInMemoryEventLog()
	defer log.Close()

	record := NewRecord("", []byte("test"))

	// Act
	result, err := log.Append(testContext(), record)

	// Assert
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if result.Offset() != 0 {
		t.Errorf("Expected offset 0, got %d", result.Offset())
	}
	if result.Topic() != "" {
		t.Errorf("Expected empty topic, got '%s'", result.Topic())
	}
}

func TestAppend_WithEmptyPayload_ShouldWork(t *testing.T) {
	// Arrange
	log := NewInMemoryEventLog()
	defer log.Close()

	record := NewRecord("test-topic", []byte{})

	// Act
	result, err := log.Append(testContext(), record)

	// Assert
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if result.Offset() != 0 {
		t.Errorf("Expected offset 0, got %d", result.Offset())
	}
	if result.Topic() != "test-topic" {
		t.Errorf("Expected topic 'test-topic', got '%s'", result.Topic())
	}
	if len(result.Payload()) != 0 {
		t.Errorf("Expected empty payload, got %d bytes", len(result.Payload()))
	}
}

func TestAppend_WithNilPayload_ShouldWork(t *testing.T) {
	// Arrange
	log := NewInMemoryEventLog()
	defer log.Close()

	record := NewRecord("test-topic", nil)

	// Act
	result, err := log.Append(testContext(), record)

	// Assert
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if result.Offset() != 0 {
		t.Errorf("Expected offset 0, got %d", result.Offset())
	}
	if result.Topic() != "test-topic" {
		t.Errorf("Expected topic 'test-topic', got '%s'", result.Topic())
	}
	if result.Payload() != nil {
		t.Error("Expected nil payload to remain nil")
	}
}

func TestAppend_WithNilHeaders_ShouldWork(t *testing.T) {
	// Arrange
	log := NewInMemoryEventLog()
	defer log.Close()

	record := NewRecordWithHeaders("test-topic", []byte("test"), nil)

	// Act
	result, err := log.Append(testContext(), record)

	// Assert
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if result.Offset() != 0 {
		t.Errorf("Expected offset 0, got %d", result.Offset())
	}
	if result.Topic() != "test-topic" {
		t.Errorf("Expected topic 'test-topic', got '%s'", result.Topic())
	}

	headers := result.Headers()
	if headers == nil {
		t.Error("Expected non-nil headers (should be converted to empty map)")
	}
	if len(headers) != 0 {
		t.Errorf("Expected empty headers map, got %d items", len(headers))
	}
}

// Concurrent access tests

func TestConcurrentAppends_ShouldHaveUniqueOffsets(t *testing.T) {
	// Arrange
	log := NewInMemoryEventLog()
	defer log.Close()

	const taskCount = 10
	var wg sync.WaitGroup
	results := make([]eventlogpkg.EventRecord, taskCount)
	errors := make([]error, taskCount)

	// Act - Run concurrent appends
	for i := 0; i < taskCount; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			record := NewRecord("topic"+string(rune('0'+index)), []byte("event"+string(rune('0'+index))))
			result, err := log.Append(testContext(), record)
			results[index] = result
			errors[index] = err
		}(i)
	}

	wg.Wait()

	// Assert - All operations should succeed
	for i, err := range errors {
		if err != nil {
			t.Errorf("Append %d failed: %v", i, err)
		}
	}

	// Assert - All offsets should be unique
	offsets := make(map[int64]bool)
	for i, result := range results {
		if result == nil {
			t.Errorf("Result %d is nil", i)
			continue
		}
		offset := result.Offset()
		if offsets[offset] {
			t.Errorf("Duplicate offset %d found", offset)
		}
		offsets[offset] = true

		if offset < 0 || offset >= taskCount {
			t.Errorf("Offset %d is out of expected range [0, %d)", offset, taskCount)
		}
	}

	// Should have exactly taskCount unique offsets
	if len(offsets) != taskCount {
		t.Errorf("Expected %d unique offsets, got %d", taskCount, len(offsets))
	}
}

func TestConcurrentReads_ShouldReturnConsistentResults(t *testing.T) {
	// Arrange
	log := NewInMemoryEventLog()
	defer log.Close()

	// Add some test data
	_, err := log.Append(testContext(), NewRecord("topic1", []byte("event1")))
	if err != nil {
		t.Fatalf("Failed to append event1: %v", err)
	}
	_, err = log.Append(testContext(), NewRecord("topic2", []byte("event2")))
	if err != nil {
		t.Fatalf("Failed to append event2: %v", err)
	}
	_, err = log.Append(testContext(), NewRecord("topic3", []byte("event3")))
	if err != nil {
		t.Fatalf("Failed to append event3: %v", err)
	}

	const readerCount = 5
	var wg sync.WaitGroup
	results := make([][]eventlogpkg.EventRecord, readerCount)
	readErrors := make([]error, readerCount)

	// Act - Run concurrent reads
	for i := 0; i < readerCount; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			result, err := log.ReadFrom(testContext(), 0, 10)
			results[index] = result
			readErrors[index] = err
		}(i)
	}

	wg.Wait()

	// Assert - All reads should succeed
	for i, err := range readErrors {
		if err != nil {
			t.Errorf("Read %d failed: %v", i, err)
		}
	}

	// Assert - All reads should return the same data
	if len(results) == 0 {
		t.Fatal("No results to compare")
	}

	firstResult := results[0]
	for i := 1; i < len(results); i++ {
		result := results[i]
		if len(result) != len(firstResult) {
			t.Errorf("Result %d has different length: expected %d, got %d", i, len(firstResult), len(result))
			continue
		}

		for j := 0; j < len(firstResult); j++ {
			if result[j].Offset() != firstResult[j].Offset() {
				t.Errorf("Result %d[%d] has different offset: expected %d, got %d", i, j, firstResult[j].Offset(), result[j].Offset())
			}
			if result[j].Topic() != firstResult[j].Topic() {
				t.Errorf("Result %d[%d] has different topic: expected '%s', got '%s'", i, j, firstResult[j].Topic(), result[j].Topic())
			}
			if !bytes.Equal(result[j].Payload(), firstResult[j].Payload()) {
				t.Errorf("Result %d[%d] has different payload", i, j)
			}
		}
	}
}

// Disposal/cleanup tests

func TestClose_ShouldClearEvents(t *testing.T) {
	// Arrange
	log := NewInMemoryEventLog()

	_, err := log.Append(testContext(), NewRecord("topic1", []byte("event1")))
	if err != nil {
		t.Fatalf("Failed to append record: %v", err)
	}

	// Verify we have events before disposal
	endOffsetBefore, err := log.GetEndOffset(testContext())
	if err != nil {
		t.Fatalf("Failed to get end offset before close: %v", err)
	}
	if endOffsetBefore != 1 {
		t.Errorf("Expected end offset 1 before close, got %d", endOffsetBefore)
	}

	// Act
	err = log.Close()
	if err != nil {
		t.Fatalf("Close should not return error, got %v", err)
	}

	// Assert - After disposal, should act like empty log
	endOffsetAfter, err := log.GetEndOffset(testContext())
	if err != nil {
		t.Fatalf("Failed to get end offset after close: %v", err)
	}
	if endOffsetAfter != 0 {
		t.Errorf("Expected end offset 0 after close, got %d", endOffsetAfter)
	}

	results, err := log.ReadFrom(testContext(), 0, 10)
	if err != nil {
		t.Fatalf("ReadFrom should not return error after close, got %v", err)
	}
	if len(results) != 0 {
		t.Errorf("Expected empty results after close, got %d items", len(results))
	}
}

func TestClose_ShouldBeIdempotent(t *testing.T) {
	// Arrange
	log := NewInMemoryEventLog()

	// Act & Assert - Multiple closes should not return error
	err := log.Close()
	if err != nil {
		t.Errorf("First close should not return error, got %v", err)
	}

	err = log.Close()
	if err != nil {
		t.Errorf("Second close should not return error, got %v", err)
	}

	err = log.Close()
	if err != nil {
		t.Errorf("Third close should not return error, got %v", err)
	}
}

func TestOperations_AfterClose_ShouldStillWork(t *testing.T) {
	// Note: For InMemoryEventLog, operations after close should still work
	// but return empty/default results since we cleared the data

	// Arrange
	log := NewInMemoryEventLog()
	_, err := log.Append(testContext(), NewRecord("test", []byte("test")))
	if err != nil {
		t.Fatalf("Failed to append record: %v", err)
	}

	// Act
	err = log.Close()
	if err != nil {
		t.Fatalf("Close should not return error, got %v", err)
	}

	// Assert - Should work but return empty results
	endOffset, err := log.GetEndOffset(testContext())
	if err != nil {
		t.Fatalf("GetEndOffset should work after close, got %v", err)
	}
	if endOffset != 0 {
		t.Errorf("Expected end offset 0 after close, got %d", endOffset)
	}

	readResults, err := log.ReadFrom(testContext(), 0, 10)
	if err != nil {
		t.Fatalf("ReadFrom should work after close, got %v", err)
	}
	if len(readResults) != 0 {
		t.Errorf("Expected empty read results after close, got %d items", len(readResults))
	}

	// Replay should return empty
	eventChan, errChan := log.Replay(testContext(), 0)
	var replayResults []eventlogpkg.EventRecord

	done := make(chan bool)
	go func() {
		for {
			select {
			case event, ok := <-eventChan:
				if !ok {
					done <- true
					return
				}
				replayResults = append(replayResults, event)
			case err := <-errChan:
				if err != nil {
					t.Errorf("Unexpected replay error after close: %v", err)
				}
				done <- true
				return
			}
		}
	}()

	select {
	case <-done:
		// Replay completed
	case <-time.After(1 * time.Second):
		t.Fatal("Replay took too long after close")
	}

	if len(replayResults) != 0 {
		t.Errorf("Expected empty replay results after close, got %d items", len(replayResults))
	}

	// Should be able to append new events after close
	newRecord, err := log.Append(testContext(), NewRecord("new", []byte("new")))
	if err != nil {
		t.Fatalf("Should be able to append after close, got %v", err)
	}
	if newRecord.Offset() != 0 {
		t.Errorf("Expected offset 0 for first append after close (starts fresh), got %d", newRecord.Offset())
	}
}