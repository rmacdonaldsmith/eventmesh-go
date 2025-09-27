package eventlog

import (
	"time"
)

// Record implements the eventlog.EventRecord interface.
// This is equivalent to the C# EventRecord class.
type Record struct {
	offset    int64
	topic     string
	payload   []byte
	timestamp time.Time
	headers   map[string]string
}

// NewRecord creates a new Record with the given properties.
// This is a constructor function since Go doesn't have constructors.
func NewRecord(topic string, payload []byte) *Record {
	return &Record{
		offset:    0, // Will be set by the EventLog when appending
		topic:     topic,
		payload:   payload,
		timestamp: time.Now().UTC(),
		headers:   make(map[string]string),
	}
}

// NewRecordWithHeaders creates a new Record with headers.
func NewRecordWithHeaders(topic string, payload []byte, headers map[string]string) *Record {
	if headers == nil {
		headers = make(map[string]string)
	}
	return &Record{
		offset:    0, // Will be set by the EventLog when appending
		topic:     topic,
		payload:   payload,
		timestamp: time.Now().UTC(),
		headers:   headers,
	}
}

// WithOffset returns a new Record with the specified offset.
// This is used internally by the EventLog when storing events.
func (r *Record) WithOffset(offset int64) *Record {
	return &Record{
		offset:    offset,
		topic:     r.topic,
		payload:   r.payload,
		timestamp: r.timestamp,
		headers:   r.headers,
	}
}

// Offset returns the unique, sequential position of this event in the log.
func (r *Record) Offset() int64 {
	return r.offset
}

// Topic returns the topic/category this event belongs to.
func (r *Record) Topic() string {
	return r.topic
}

// Payload returns the raw event data as bytes.
func (r *Record) Payload() []byte {
	// Return a copy to prevent mutation
	if r.payload == nil {
		return nil
	}
	result := make([]byte, len(r.payload))
	copy(result, r.payload)
	return result
}

// Timestamp returns when this event was created.
func (r *Record) Timestamp() time.Time {
	return r.timestamp
}

// Headers returns key-value metadata associated with this event.
func (r *Record) Headers() map[string]string {
	// Return a copy to prevent mutation
	if r.headers == nil {
		return make(map[string]string)
	}
	result := make(map[string]string, len(r.headers))
	for k, v := range r.headers {
		result[k] = v
	}
	return result
}

// Verify that Record implements the EventRecord interface at compile time
var _ EventRecord = (*Record)(nil)