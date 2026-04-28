package eventlog

import (
	"testing"
	"time"

	"github.com/rmacdonaldsmith/eventmesh-go/pkg/eventlog"
)

func TestPebbleStoredEventRoundTrip(t *testing.T) {
	original := eventlog.NewEventWithHeaders("orders.created", []byte("payload"), map[string]string{
		"trace-id": "abc-123",
	}).WithOffset(42)
	original.Timestamp = time.Date(2026, 4, 27, 10, 11, 12, 13, time.UTC)

	encoded, err := marshalPebbleStoredEvent(original)
	if err != nil {
		t.Fatalf("marshalPebbleStoredEvent failed: %v", err)
	}

	decoded, err := unmarshalPebbleStoredEvent(encoded)
	if err != nil {
		t.Fatalf("unmarshalPebbleStoredEvent failed: %v", err)
	}

	if decoded.Topic != original.Topic {
		t.Fatalf("Expected topic %q, got %q", original.Topic, decoded.Topic)
	}
	if decoded.Offset != original.Offset {
		t.Fatalf("Expected offset %d, got %d", original.Offset, decoded.Offset)
	}
	if string(decoded.Payload) != string(original.Payload) {
		t.Fatalf("Expected payload %q, got %q", original.Payload, decoded.Payload)
	}
	if decoded.Headers["trace-id"] != "abc-123" {
		t.Fatalf("Expected trace-id header to round-trip")
	}
	if !decoded.Timestamp.Equal(original.Timestamp) {
		t.Fatalf("Expected timestamp %s, got %s", original.Timestamp, decoded.Timestamp)
	}
}
