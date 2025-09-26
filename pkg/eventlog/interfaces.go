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

// EventLog defines the interface for append-only event storage.
// This is equivalent to IEventLog in the C# implementation.
type EventLog interface {
	io.Closer // Go equivalent of IDisposable

	// AppendAsync appends a new event to the log.
	// The Offset will be assigned by the log and set on the returned record.
	Append(ctx context.Context, record EventRecord) (EventRecord, error)

	// ReadFrom reads events starting at a given offset, up to a max count.
	ReadFrom(ctx context.Context, startOffset int64, maxCount int) ([]EventRecord, error)

	// GetEndOffset gets the current end offset (next append position).
	GetEndOffset(ctx context.Context) (int64, error)

	// Replay replays events from a given offset via a channel.
	// The channel will be closed when all events are sent or context is cancelled.
	Replay(ctx context.Context, startOffset int64) (<-chan EventRecord, <-chan error)

	// Compact performs log compaction or cleanup (future use).
	Compact(ctx context.Context) error
}