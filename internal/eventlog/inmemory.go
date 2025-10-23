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
	// ErrNilEvent is returned when a nil event is provided
	ErrNilEvent = errors.New("event cannot be nil")
	// ErrEmptyTopic is returned when an empty topic is provided
	ErrEmptyTopic = errors.New("topic cannot be empty")
)

// InMemoryEventLog implements the eventlog.EventLog interface using in-memory topic-partitioned storage.
// Each topic has its own independent event sequence and offset counter starting from 0.
// It is safe for concurrent use.
type InMemoryEventLog struct {
	mu                sync.RWMutex
	eventsByTopic     map[string][]*eventlog.Event // topic -> events
	nextOffsetByTopic map[string]int64             // topic -> nextOffset
	closed            bool
}

// NewInMemoryEventLog creates a new in-memory topic-partitioned event log.
func NewInMemoryEventLog() *InMemoryEventLog {
	return &InMemoryEventLog{
		eventsByTopic:     make(map[string][]*eventlog.Event),
		nextOffsetByTopic: make(map[string]int64),
		closed:            false,
	}
}

// AppendEvent appends a new event to a specific topic.
func (log *InMemoryEventLog) AppendEvent(ctx context.Context, topic string, event *eventlog.Event) (*eventlog.Event, error) {
	if err := log.checkContext(ctx); err != nil {
		return nil, err
	}

	if topic == "" {
		return nil, ErrEmptyTopic
	}

	if event == nil {
		return nil, ErrNilEvent
	}

	log.mu.Lock()
	defer log.mu.Unlock()

	if log.closed {
		return nil, errors.New("event log is closed")
	}

	// Get the next offset for this topic
	nextOffset := log.nextOffsetByTopic[topic]
	log.nextOffsetByTopic[topic] = nextOffset + 1

	// Create new event with assigned offset
	eventWithOffset := event.WithOffset(nextOffset)

	// Append to topic
	log.eventsByTopic[topic] = append(log.eventsByTopic[topic], eventWithOffset)

	return eventWithOffset, nil
}

// ReadEvents reads events from a specific topic starting at a given offset, up to a max count.
func (log *InMemoryEventLog) ReadEvents(ctx context.Context, topic string, startOffset int64, maxCount int) ([]*eventlog.Event, error) {
	if err := log.checkContext(ctx); err != nil {
		return nil, err
	}

	if topic == "" {
		return nil, ErrEmptyTopic
	}

	if startOffset < 0 {
		return nil, ErrNegativeOffset
	}

	if maxCount < 0 {
		return nil, ErrNegativeMaxCount
	}

	log.mu.RLock()
	defer log.mu.RUnlock()

	if log.closed {
		return nil, errors.New("event log is closed")
	}

	events, exists := log.eventsByTopic[topic]
	if !exists || len(events) == 0 {
		return []*eventlog.Event{}, nil
	}

	// Find starting position
	startIdx := int(startOffset)
	if startIdx >= len(events) {
		return []*eventlog.Event{}, nil
	}

	// Calculate end position
	endIdx := startIdx + maxCount
	if endIdx > len(events) {
		endIdx = len(events)
	}

	// Return slice of events
	result := make([]*eventlog.Event, endIdx-startIdx)
	copy(result, events[startIdx:endIdx])

	return result, nil
}

// GetTopicEndOffset gets the current end offset for a specific topic.
func (log *InMemoryEventLog) GetTopicEndOffset(ctx context.Context, topic string) (int64, error) {
	if err := log.checkContext(ctx); err != nil {
		return 0, err
	}

	if topic == "" {
		return 0, ErrEmptyTopic
	}

	log.mu.RLock()
	defer log.mu.RUnlock()

	if log.closed {
		return 0, errors.New("event log is closed")
	}

	return log.nextOffsetByTopic[topic], nil
}

// ReplayEvents replays events from a specific topic starting at a given offset via a channel.
func (log *InMemoryEventLog) ReplayEvents(ctx context.Context, topic string, startOffset int64) (<-chan *eventlog.Event, <-chan error) {
	eventChan := make(chan *eventlog.Event, 10)
	errChan := make(chan error, 1)

	go func() {
		defer close(eventChan)
		defer close(errChan)

		if topic == "" {
			errChan <- ErrEmptyTopic
			return
		}

		if startOffset < 0 {
			errChan <- ErrNegativeOffset
			return
		}

		log.mu.RLock()
		events, exists := log.eventsByTopic[topic]
		log.mu.RUnlock()

		if !exists || len(events) == 0 {
			return
		}

		startIdx := int(startOffset)
		if startIdx >= len(events) {
			return
		}

		// Send events one by one
		for i := startIdx; i < len(events); i++ {
			select {
			case <-ctx.Done():
				errChan <- ctx.Err()
				return
			case eventChan <- events[i]:
			}
		}
	}()

	return eventChan, errChan
}

// Compact performs log compaction (no-op for in-memory implementation).
func (log *InMemoryEventLog) Compact(ctx context.Context) error {
	// No-op for in-memory implementation
	return nil
}

// GetStatistics returns overall statistics about the event log.
func (log *InMemoryEventLog) GetStatistics(ctx context.Context) (eventlog.EventLogStatistics, error) {
	if err := log.checkContext(ctx); err != nil {
		return eventlog.EventLogStatistics{}, err
	}

	log.mu.RLock()
	defer log.mu.RUnlock()

	if log.closed {
		return eventlog.EventLogStatistics{}, errors.New("event log is closed")
	}

	totalEvents := int64(0)
	topicCounts := make(map[string]int64)

	for topic, events := range log.eventsByTopic {
		count := int64(len(events))
		topicCounts[topic] = count
		totalEvents += count
	}

	return eventlog.EventLogStatistics{
		TotalEvents: totalEvents,
		TopicCounts: topicCounts,
		TopicCount:  len(topicCounts),
	}, nil
}

// Close closes the event log.
func (log *InMemoryEventLog) Close() error {
	log.mu.Lock()
	defer log.mu.Unlock()

	log.closed = true
	return nil
}

// checkContext checks if the context is cancelled and returns the context error if so
func (log *InMemoryEventLog) checkContext(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
		return nil
	}
}
