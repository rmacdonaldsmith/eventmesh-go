package eventlog

import (
	"testing"
	"time"
)

func TestNewEvent(t *testing.T) {
	topic := "test-topic"
	payload := []byte("test payload")

	event := NewEvent(topic, payload)

	if event.Topic != topic {
		t.Errorf("Expected topic %s, got %s", topic, event.Topic)
	}

	if string(event.Payload) != string(payload) {
		t.Errorf("Expected payload %s, got %s", payload, event.Payload)
	}

	if event.Offset != 0 {
		t.Errorf("Expected offset 0, got %d", event.Offset)
	}

	if event.Headers == nil {
		t.Error("Expected headers to be initialized")
	}

	if len(event.Headers) != 0 {
		t.Errorf("Expected empty headers, got %d items", len(event.Headers))
	}

	// Verify timestamp is recent
	if time.Since(event.Timestamp) > time.Second {
		t.Error("Expected timestamp to be recent")
	}
}

func TestNewEventWithHeaders(t *testing.T) {
	topic := "test-topic"
	payload := []byte("test payload")
	headers := map[string]string{
		"source": "test",
		"type":   "user-action",
	}

	event := NewEventWithHeaders(topic, payload, headers)

	if event.Topic != topic {
		t.Errorf("Expected topic %s, got %s", topic, event.Topic)
	}

	if len(event.Headers) != 2 {
		t.Errorf("Expected 2 headers, got %d", len(event.Headers))
	}

	if event.Headers["source"] != "test" {
		t.Errorf("Expected header source=test, got %s", event.Headers["source"])
	}

	if event.Headers["type"] != "user-action" {
		t.Errorf("Expected header type=user-action, got %s", event.Headers["type"])
	}
}

func TestEvent_WithOffset(t *testing.T) {
	original := NewEvent("test", []byte("payload"))
	offset := int64(42)

	withOffset := original.WithOffset(offset)

	if withOffset.Offset != offset {
		t.Errorf("Expected offset %d, got %d", offset, withOffset.Offset)
	}

	// Original should be unchanged
	if original.Offset != 0 {
		t.Errorf("Original offset should remain 0, got %d", original.Offset)
	}

	// Other fields should be copied
	if withOffset.Topic != original.Topic {
		t.Error("Topic should be copied")
	}
}

func TestEvent_Immutability(t *testing.T) {
	originalPayload := []byte("original")
	originalHeaders := map[string]string{"key": "value"}

	event := NewEventWithHeaders("test", originalPayload, originalHeaders)

	// Mutate original data
	originalPayload[0] = 'X'
	originalHeaders["key"] = "modified"

	// Event should be unchanged
	if string(event.Payload) != "original" {
		t.Errorf("Payload should be immutable, got %s", event.Payload)
	}

	if event.Headers["key"] != "value" {
		t.Errorf("Headers should be immutable, got %s", event.Headers["key"])
	}
}

func TestEvent_Copy(t *testing.T) {
	original := NewEventWithHeaders("test", []byte("payload"), map[string]string{"key": "value"})
	original = original.WithOffset(42)

	copy := original.Copy()

	// Should have same values
	if copy.Topic != original.Topic {
		t.Error("Copy should have same topic")
	}
	if copy.Offset != original.Offset {
		t.Error("Copy should have same offset")
	}
	if string(copy.Payload) != string(original.Payload) {
		t.Error("Copy should have same payload")
	}
	if copy.Headers["key"] != original.Headers["key"] {
		t.Error("Copy should have same headers")
	}

	// Should be independent - mutating copy shouldn't affect original
	copy.Payload[0] = 'X'
	copy.Headers["key"] = "modified"

	if string(original.Payload) != "payload" {
		t.Error("Original payload should be unchanged")
	}
	if original.Headers["key"] != "value" {
		t.Error("Original headers should be unchanged")
	}
}
