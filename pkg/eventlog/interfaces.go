package eventlog

import (
	"context"
	"io"
	"time"
)

// EventRecord represents a single event in the event log.
// This is equivalent to IEventRecord in the C# implementation.
type EventRecord interface {
	// Offset returns the unique, sequential position of this event in the log
	Offset() int64

	// Topic returns the topic/category this event belongs to
	Topic() string

	// Payload returns the raw event data as bytes
	Payload() []byte

	// Timestamp returns when this event was created
	Timestamp() time.Time

	// Headers returns key-value metadata associated with this event
	Headers() map[string]string
}

// EventLog defines the interface for topic-scoped append-only event storage.
// Each topic has its own independent offset sequence starting from 0.
type EventLog interface {
	io.Closer // Go equivalent of IDisposable

	// AppendToTopic appends a new event to a specific topic.
	// The Offset will be assigned by the log per topic and set on the returned record.
	AppendToTopic(ctx context.Context, topic string, record EventRecord) (EventRecord, error)

	// ReadFromTopic reads events from a specific topic starting at a given offset, up to a max count.
	ReadFromTopic(ctx context.Context, topic string, startOffset int64, maxCount int) ([]EventRecord, error)

	// GetTopicEndOffset gets the current end offset for a specific topic (next append position).
	GetTopicEndOffset(ctx context.Context, topic string) (int64, error)

	// ReplayTopic replays events from a specific topic starting at a given offset via a channel.
	// The channel will be closed when all events are sent or context is cancelled.
	ReplayTopic(ctx context.Context, topic string, startOffset int64) (<-chan EventRecord, <-chan error)

	// Compact performs log compaction or cleanup (future use).
	Compact(ctx context.Context) error
}
