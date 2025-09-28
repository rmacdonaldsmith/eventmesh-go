package eventlog

import (
	"context"
	"errors"
	"sync"

	"github.com/rmacdonaldsmith/eventmesh-go/pkg/eventlog"
)

var (
	// ErrNegativeOffset is returned when a negative offset is provided
	ErrNegativeOffset = errors.New("offset cannot be negative")
	// ErrNegativeMaxCount is returned when a negative max count is provided
	ErrNegativeMaxCount = errors.New("max count cannot be negative")
	// ErrNilRecord is returned when a nil record is provided
	ErrNilRecord = errors.New("record cannot be nil")
)

// InMemoryEventLog implements the eventlog.EventLog interface using in-memory topic-partitioned storage.
// Each topic has its own independent event sequence and offset counter starting from 0.
// It is safe for concurrent use.
type InMemoryEventLog struct {
	mu                sync.RWMutex
	eventsByTopic     map[string][]*eventlog.Record // topic -> events
	nextOffsetByTopic map[string]int64              // topic -> nextOffset
	closed            bool
}

// NewInMemoryEventLog creates a new in-memory topic-partitioned event log.
func NewInMemoryEventLog() *InMemoryEventLog {
	return &InMemoryEventLog{
		eventsByTopic:     make(map[string][]*eventlog.Record),
		nextOffsetByTopic: make(map[string]int64),
		closed:            false,
	}
}

// AppendToTopic appends a new event to a specific topic.
// The Offset will be assigned by the log per topic and set on the returned record.
func (log *InMemoryEventLog) AppendToTopic(ctx context.Context, topic string, record eventlog.EventRecord) (eventlog.EventRecord, error) {
	if record == nil {
		return nil, ErrNilRecord
	}

	// Check if context is cancelled
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	log.mu.Lock()
	defer log.mu.Unlock()

	// Get the current offset for this topic (0 if topic doesn't exist yet)
	currentOffset := log.nextOffsetByTopic[topic]

	// Create a new Record with the assigned topic-specific offset
	// We need to ensure we have a concrete Record type to store
	var storedRecord *eventlog.Record
	if r, ok := record.(*eventlog.Record); ok {
		// If it's already our Record type, use WithOffset to create a copy
		storedRecord = r.WithOffset(currentOffset)
	} else {
		// If it's a different implementation, extract the data and create new Record
		baseRecord := eventlog.NewRecordWithHeaders(
			record.Topic(),
			record.Payload(),
			record.Headers(),
		)
		storedRecord = baseRecord.WithOffset(currentOffset)
	}

	// Append to the topic's event slice
	log.eventsByTopic[topic] = append(log.eventsByTopic[topic], storedRecord)
	log.nextOffsetByTopic[topic]++

	return storedRecord, nil
}

// ReadFromTopic reads events from a specific topic starting at a given offset, up to a max count.
func (log *InMemoryEventLog) ReadFromTopic(ctx context.Context, topic string, startOffset int64, maxCount int) ([]eventlog.EventRecord, error) {
	if startOffset < 0 {
		return nil, ErrNegativeOffset
	}
	if maxCount < 0 {
		return nil, ErrNegativeMaxCount
	}

	// Check if context is cancelled
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	log.mu.RLock()
	defer log.mu.RUnlock()

	// Handle zero maxCount case early
	if maxCount == 0 {
		return make([]eventlog.EventRecord, 0), nil
	}

	// Get events for the specific topic
	topicEvents := log.eventsByTopic[topic]
	if topicEvents == nil {
		// Topic doesn't exist, return empty slice
		return make([]eventlog.EventRecord, 0), nil
	}

	results := make([]eventlog.EventRecord, 0, maxCount)
	count := 0

	for _, event := range topicEvents {
		if event.Offset() >= startOffset {
			results = append(results, event)
			count++
			if count >= maxCount {
				break
			}
		}
	}

	return results, nil
}

// GetTopicEndOffset gets the current end offset for a specific topic (next append position).
func (log *InMemoryEventLog) GetTopicEndOffset(ctx context.Context, topic string) (int64, error) {
	// Check if context is cancelled
	select {
	case <-ctx.Done():
		return 0, ctx.Err()
	default:
	}

	log.mu.RLock()
	defer log.mu.RUnlock()

	// Return the next offset for the topic (0 if topic doesn't exist)
	return log.nextOffsetByTopic[topic], nil
}

// ReplayTopic replays events from a specific topic starting at a given offset via a channel.
// The channel will be closed when all events are sent or context is cancelled.
func (log *InMemoryEventLog) ReplayTopic(ctx context.Context, topic string, startOffset int64) (<-chan eventlog.EventRecord, <-chan error) {
	eventChan := make(chan eventlog.EventRecord)
	errChan := make(chan error, 1) // Buffered to prevent blocking

	go func() {
		defer close(eventChan)
		defer close(errChan)

		if startOffset < 0 {
			errChan <- ErrNegativeOffset
			return
		}

		log.mu.RLock()
		// Create a copy of relevant events from the specific topic to avoid holding the lock during iteration
		topicEvents := log.eventsByTopic[topic]
		var eventsToReplay []*eventlog.Record
		for _, event := range topicEvents {
			if event.Offset() >= startOffset {
				eventsToReplay = append(eventsToReplay, event)
			}
		}
		log.mu.RUnlock()

		// Send events through the channel
		for _, event := range eventsToReplay {
			select {
			case <-ctx.Done():
				errChan <- ctx.Err()
				return
			case eventChan <- event:
				// Event sent successfully
			}
		}
	}()

	return eventChan, errChan
}

// Compact performs log compaction or cleanup (future use).
func (log *InMemoryEventLog) Compact(ctx context.Context) error {
	// Check if context is cancelled
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	// No-op for in-memory implementation
	return nil
}

// Close closes the event log and clears all topics and events.
// This is equivalent to the C# Dispose method.
func (log *InMemoryEventLog) Close() error {
	log.mu.Lock()
	defer log.mu.Unlock()

	if log.closed {
		return nil // Already closed, idempotent
	}

	// Clear all topic data
	log.eventsByTopic = make(map[string][]*eventlog.Record)
	log.nextOffsetByTopic = make(map[string]int64)
	log.closed = true

	return nil
}

// Verify that InMemoryEventLog implements the EventLog interface at compile time
var _ eventlog.EventLog = (*InMemoryEventLog)(nil)