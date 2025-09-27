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

// InMemoryEventLog implements the eventlog.EventLog interface using in-memory storage.
// This is equivalent to the C# InMemoryEventLog class.
// It is safe for concurrent use.
type InMemoryEventLog struct {
	mu         sync.RWMutex
	events     []*Record
	nextOffset int64
	closed     bool
}

// NewInMemoryEventLog creates a new in-memory event log.
func NewInMemoryEventLog() *InMemoryEventLog {
	return &InMemoryEventLog{
		events:     make([]*Record, 0),
		nextOffset: 0,
		closed:     false,
	}
}

// Append appends a new event to the log.
// The Offset will be assigned by the log and set on the returned record.
func (log *InMemoryEventLog) Append(ctx context.Context, record eventlog.EventRecord) (eventlog.EventRecord, error) {
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

	// Note: Allow operations after close (like C# version), but they work with empty state

	// Create a new Record with the assigned offset
	// We need to ensure we have a concrete Record type to store
	var storedRecord *Record
	if r, ok := record.(*Record); ok {
		// If it's already our Record type, create a copy with the new offset
		storedRecord = &Record{
			offset:    log.nextOffset,
			topic:     r.topic,
			payload:   r.payload,
			timestamp: r.timestamp,
			headers:   r.headers,
		}
	} else {
		// If it's a different implementation, extract the data
		storedRecord = &Record{
			offset:    log.nextOffset,
			topic:     record.Topic(),
			payload:   record.Payload(),
			timestamp: record.Timestamp(),
			headers:   record.Headers(),
		}
	}

	// Ensure headers are never nil
	if storedRecord.headers == nil {
		storedRecord.headers = make(map[string]string)
	}

	log.events = append(log.events, storedRecord)
	log.nextOffset++

	return storedRecord, nil
}

// ReadFrom reads events starting at a given offset, up to a max count.
func (log *InMemoryEventLog) ReadFrom(ctx context.Context, startOffset int64, maxCount int) ([]eventlog.EventRecord, error) {
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

	results := make([]eventlog.EventRecord, 0, maxCount)
	count := 0

	for _, event := range log.events {
		if event.offset >= startOffset {
			results = append(results, event)
			count++
			if count >= maxCount {
				break
			}
		}
	}

	return results, nil
}

// GetEndOffset gets the current end offset (next append position).
func (log *InMemoryEventLog) GetEndOffset(ctx context.Context) (int64, error) {
	// Check if context is cancelled
	select {
	case <-ctx.Done():
		return 0, ctx.Err()
	default:
	}

	log.mu.RLock()
	defer log.mu.RUnlock()

	return log.nextOffset, nil
}

// Replay replays events from a given offset via a channel.
// The channel will be closed when all events are sent or context is cancelled.
func (log *InMemoryEventLog) Replay(ctx context.Context, startOffset int64) (<-chan eventlog.EventRecord, <-chan error) {
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
		// Create a copy of relevant events to avoid holding the lock during iteration
		var eventsToReplay []*Record
		for _, event := range log.events {
			if event.offset >= startOffset {
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

// Close closes the event log and clears all events.
// This is equivalent to the C# Dispose method.
func (log *InMemoryEventLog) Close() error {
	log.mu.Lock()
	defer log.mu.Unlock()

	if log.closed {
		return nil // Already closed, idempotent
	}

	log.events = log.events[:0] // Clear slice but keep capacity
	log.nextOffset = 0
	log.closed = true

	return nil
}

// Verify that InMemoryEventLog implements the EventLog interface at compile time
var _ eventlog.EventLog = (*InMemoryEventLog)(nil)