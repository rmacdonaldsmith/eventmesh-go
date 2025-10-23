package eventlog

import (
	"time"
)

// Event represents a single event in the event log.
type Event struct {
	// Offset is the unique, sequential position of this event in the log
	Offset int64

	// Topic is the topic/category this event belongs to
	Topic string

	// Payload is the raw event data as bytes (immutable after creation)
	Payload []byte

	// Timestamp is when this event was created
	Timestamp time.Time

	// Headers are key-value metadata associated with this event (immutable after creation)
	Headers map[string]string
}

// NewEvent creates a new Event with the given properties.
// The payload and headers are copied to ensure immutability.
func NewEvent(topic string, payload []byte) *Event {
	// Copy payload to prevent external mutation
	payloadCopy := make([]byte, len(payload))
	copy(payloadCopy, payload)

	return &Event{
		Offset:    0, // Will be set by the EventLog when appending
		Topic:     topic,
		Payload:   payloadCopy,
		Timestamp: time.Now().UTC(),
		Headers:   make(map[string]string),
	}
}

// NewEventWithHeaders creates a new Event with headers.
// Both payload and headers are copied to ensure immutability.
func NewEventWithHeaders(topic string, payload []byte, headers map[string]string) *Event {
	// Copy payload to prevent external mutation
	payloadCopy := make([]byte, len(payload))
	copy(payloadCopy, payload)

	// Copy headers to prevent external mutation
	headersCopy := make(map[string]string, len(headers))
	for k, v := range headers {
		headersCopy[k] = v
	}

	return &Event{
		Offset:    0, // Will be set by the EventLog when appending
		Topic:     topic,
		Payload:   payloadCopy,
		Timestamp: time.Now().UTC(),
		Headers:   headersCopy,
	}
}

// WithOffset returns a new Event with the specified offset.
// This is used internally by the EventLog when storing events.
func (e *Event) WithOffset(offset int64) *Event {
	return &Event{
		Offset:    offset,
		Topic:     e.Topic,
		Payload:   e.Payload, // Already immutable from construction
		Timestamp: e.Timestamp,
		Headers:   e.Headers, // Already immutable from construction
	}
}

// Copy returns a deep copy of the Event.
// Useful when you need to modify an event while preserving the original.
func (e *Event) Copy() *Event {
	payloadCopy := make([]byte, len(e.Payload))
	copy(payloadCopy, e.Payload)

	headersCopy := make(map[string]string, len(e.Headers))
	for k, v := range e.Headers {
		headersCopy[k] = v
	}

	return &Event{
		Offset:    e.Offset,
		Topic:     e.Topic,
		Payload:   payloadCopy,
		Timestamp: e.Timestamp,
		Headers:   headersCopy,
	}
}
