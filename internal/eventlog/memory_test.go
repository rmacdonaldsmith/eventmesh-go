package eventlog

import (
	"context"
	"testing"
)

func TestImplementationCompilation(t *testing.T) {
	// This test verifies that our implementation compiles correctly
	// We'll add comprehensive tests in Phase 3

	// Test Record creation
	record := NewRecord("test-topic", []byte("test payload"))
	if record.Topic() != "test-topic" {
		t.Errorf("Expected topic 'test-topic', got '%s'", record.Topic())
	}

	// Test InMemoryEventLog creation
	log := NewInMemoryEventLog()
	if log == nil {
		t.Error("Expected non-nil InMemoryEventLog")
	}

	// Test basic append
	ctx := context.Background()
	appendedRecord, err := log.Append(ctx, record)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if appendedRecord.Offset() != 0 {
		t.Errorf("Expected offset 0, got %d", appendedRecord.Offset())
	}

	// Test close
	err = log.Close()
	if err != nil {
		t.Errorf("Expected no error on close, got %v", err)
	}

	t.Log("Record and InMemoryEventLog implementations compile and basic functionality works")
}