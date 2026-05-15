package eventlog

import (
	"context"
	"errors"
	"fmt"
	"math"
	"sync"

	"github.com/cockroachdb/pebble"
	"github.com/rmacdonaldsmith/eventmesh-go/pkg/eventlog"
)

var ErrOffsetOverflow = errors.New("event log offset exceeds int64 range")

type PebbleEventLog struct {
	mu     sync.RWMutex
	db     *pebble.DB
	closed bool
}

func NewPebbleEventLog(path string) (*PebbleEventLog, error) {
	if path == "" {
		return nil, fmt.Errorf("path cannot be empty")
	}

	db, err := pebble.Open(path, &pebble.Options{})
	if err != nil {
		return nil, fmt.Errorf("failed to open pebble event log: %w", err)
	}

	return &PebbleEventLog{db: db}, nil
}

func (log *PebbleEventLog) AppendEvent(ctx context.Context, topic string, event *eventlog.Event) (*eventlog.Event, error) {
	if err := checkPebbleContext(ctx); err != nil {
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

	nextOffset, err := log.getTopicEndOffsetLocked(topic)
	if err != nil {
		return nil, err
	}
	if nextOffset > math.MaxInt64 {
		return nil, ErrOffsetOverflow
	}

	eventWithOffset := event.WithOffset(int64(nextOffset))
	encodedEvent, err := marshalPebbleStoredEvent(eventWithOffset)
	if err != nil {
		return nil, fmt.Errorf("failed to encode event: %w", err)
	}

	batch := log.db.NewBatch()
	defer func() { _ = batch.Close() }()

	if err := batch.Set(pebbleEventKey(topic, nextOffset), encodedEvent, nil); err != nil {
		return nil, fmt.Errorf("failed to stage event write: %w", err)
	}
	if err := batch.Set(pebbleOffsetKey(topic), encodePebbleOffset(nextOffset+1), nil); err != nil {
		return nil, fmt.Errorf("failed to stage offset write: %w", err)
	}
	if err := batch.Commit(pebble.Sync); err != nil {
		return nil, fmt.Errorf("failed to commit event append: %w", err)
	}

	return eventWithOffset, nil
}

func (log *PebbleEventLog) ReadEvents(ctx context.Context, topic string, startOffset int64, maxCount int) ([]*eventlog.Event, error) {
	if err := checkPebbleContext(ctx); err != nil {
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
	if maxCount == 0 {
		return []*eventlog.Event{}, nil
	}

	return log.readEventsLocked(ctx, topic, uint64(startOffset), maxCount)
}

func (log *PebbleEventLog) GetTopicEndOffset(ctx context.Context, topic string) (int64, error) {
	if err := checkPebbleContext(ctx); err != nil {
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

	offset, err := log.getTopicEndOffsetLocked(topic)
	if err != nil {
		return 0, err
	}
	if offset > math.MaxInt64 {
		return 0, ErrOffsetOverflow
	}
	return int64(offset), nil
}

func (log *PebbleEventLog) ReplayEvents(ctx context.Context, topic string, startOffset int64) (<-chan *eventlog.Event, <-chan error) {
	eventChan := make(chan *eventlog.Event, 10)
	errChan := make(chan error, 1)

	go func() {
		defer close(eventChan)
		defer close(errChan)

		if err := checkPebbleContext(ctx); err != nil {
			errChan <- err
			return
		}
		if topic == "" {
			errChan <- ErrEmptyTopic
			return
		}
		if startOffset < 0 {
			errChan <- ErrNegativeOffset
			return
		}

		log.mu.RLock()
		defer log.mu.RUnlock()

		if log.closed {
			errChan <- errors.New("event log is closed")
			return
		}

		iter, err := log.newTopicIterator(topic)
		if err != nil {
			errChan <- err
			return
		}
		defer func() { _ = iter.Close() }()

		for ok := iter.SeekGE(pebbleEventKey(topic, uint64(startOffset))); ok; ok = iter.Next() {
			if err := checkPebbleContext(ctx); err != nil {
				errChan <- err
				return
			}

			event, err := eventFromIteratorValue(iter)
			if err != nil {
				errChan <- err
				return
			}

			select {
			case <-ctx.Done():
				errChan <- ctx.Err()
				return
			case eventChan <- event:
			}
		}

		if err := iter.Error(); err != nil {
			errChan <- err
		}
	}()

	return eventChan, errChan
}

func (log *PebbleEventLog) Compact(ctx context.Context) error {
	if err := checkPebbleContext(ctx); err != nil {
		return err
	}

	log.mu.RLock()
	defer log.mu.RUnlock()

	if log.closed {
		return errors.New("event log is closed")
	}

	return nil
}

func (log *PebbleEventLog) GetStatistics(ctx context.Context) (eventlog.EventLogStatistics, error) {
	if err := checkPebbleContext(ctx); err != nil {
		return eventlog.EventLogStatistics{}, err
	}

	log.mu.RLock()
	defer log.mu.RUnlock()

	if log.closed {
		return eventlog.EventLogStatistics{}, errors.New("event log is closed")
	}

	iter, err := log.db.NewIter(&pebble.IterOptions{
		LowerBound: []byte{pebbleOffsetKeyPrefix},
		UpperBound: []byte{pebbleOffsetKeyPrefix + 1},
	})
	if err != nil {
		return eventlog.EventLogStatistics{}, err
	}
	defer func() { _ = iter.Close() }()

	stats := eventlog.EventLogStatistics{
		TopicCounts: make(map[string]int64),
	}

	for ok := iter.First(); ok; ok = iter.Next() {
		if err := checkPebbleContext(ctx); err != nil {
			return eventlog.EventLogStatistics{}, err
		}

		topic, ok := decodePebbleOffsetTopicKey(iter.Key())
		if !ok {
			continue
		}

		value, err := iter.ValueAndErr()
		if err != nil {
			return eventlog.EventLogStatistics{}, err
		}
		if len(value) != 8 {
			return eventlog.EventLogStatistics{}, fmt.Errorf("invalid offset value length %d", len(value))
		}
		count := decodePebbleOffset(value)
		if count > math.MaxInt64 {
			return eventlog.EventLogStatistics{}, ErrOffsetOverflow
		}

		stats.TopicCounts[topic] = int64(count)
		stats.TotalEvents += int64(count)
	}

	if err := iter.Error(); err != nil {
		return eventlog.EventLogStatistics{}, err
	}

	stats.TopicCount = len(stats.TopicCounts)
	return stats, nil
}

func (log *PebbleEventLog) Close() error {
	log.mu.Lock()
	defer log.mu.Unlock()

	if log.closed {
		return nil
	}

	log.closed = true
	return log.db.Close()
}

func (log *PebbleEventLog) getTopicEndOffsetLocked(topic string) (uint64, error) {
	value, closer, err := log.db.Get(pebbleOffsetKey(topic))
	if errors.Is(err, pebble.ErrNotFound) {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("failed to read topic end offset: %w", err)
	}
	defer func() { _ = closer.Close() }()

	if len(value) != 8 {
		return 0, fmt.Errorf("invalid offset value length %d", len(value))
	}
	return decodePebbleOffset(value), nil
}

func (log *PebbleEventLog) readEventsLocked(ctx context.Context, topic string, startOffset uint64, maxCount int) ([]*eventlog.Event, error) {
	iter, err := log.newTopicIterator(topic)
	if err != nil {
		return nil, err
	}
	defer func() { _ = iter.Close() }()

	events := make([]*eventlog.Event, 0, maxCount)
	for ok := iter.SeekGE(pebbleEventKey(topic, startOffset)); ok && len(events) < maxCount; ok = iter.Next() {
		if err := checkPebbleContext(ctx); err != nil {
			return nil, err
		}

		event, err := eventFromIteratorValue(iter)
		if err != nil {
			return nil, err
		}
		events = append(events, event)
	}

	if err := iter.Error(); err != nil {
		return nil, err
	}
	return events, nil
}

func (log *PebbleEventLog) newTopicIterator(topic string) (*pebble.Iterator, error) {
	prefix := pebbleEventTopicPrefix(topic)
	return log.db.NewIter(&pebble.IterOptions{
		LowerBound: prefix,
		UpperBound: pebblePrefixUpperBound(prefix),
	})
}

func eventFromIteratorValue(iter *pebble.Iterator) (*eventlog.Event, error) {
	value, err := iter.ValueAndErr()
	if err != nil {
		return nil, err
	}
	return unmarshalPebbleStoredEvent(value)
}

func checkPebbleContext(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
		return nil
	}
}
