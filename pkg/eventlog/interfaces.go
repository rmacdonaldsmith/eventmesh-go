package eventlog

import (
	"context"
	"io"
)

// EventLog defines the interface for topic-scoped append-only event storage.
// Each topic has its own independent offset sequence starting from 0.
type EventLog interface {
	io.Closer

	// AppendEvent appends a new event to a specific topic.
	// The Offset will be assigned by the log per topic and set on the returned event.
	AppendEvent(ctx context.Context, topic string, event *Event) (*Event, error)

	// ReadEvents reads events from a specific topic starting at a given offset, up to a max count.
	ReadEvents(ctx context.Context, topic string, startOffset int64, maxCount int) ([]*Event, error)

	// GetTopicEndOffset gets the current end offset for a specific topic (next append position).
	GetTopicEndOffset(ctx context.Context, topic string) (int64, error)

	// ReplayEvents replays events from a specific topic starting at a given offset via a channel.
	// The channel will be closed when all events are sent or context is cancelled.
	ReplayEvents(ctx context.Context, topic string, startOffset int64) (<-chan *Event, <-chan error)

	// Compact performs log compaction or cleanup (future use).
	Compact(ctx context.Context) error

	// GetStatistics returns overall statistics about the event log.
	GetStatistics(ctx context.Context) (EventLogStatistics, error)
}

// EventLogStatistics provides aggregate statistics about the event log
type EventLogStatistics struct {
	TotalEvents int64            // Total number of events across all topics
	TopicCounts map[string]int64 // Number of events per topic
	TopicCount  int              // Number of distinct topics
}
