package eventlogtest

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"testing"

	"github.com/rmacdonaldsmith/eventmesh-go/pkg/eventlog"
)

// Factory creates a fresh EventLog for a single contract subtest.
type Factory func(t testing.TB) eventlog.EventLog

// DurableFactory creates an EventLog that can be closed and reopened against the
// same backing store. Durable implementations should use this in addition to
// RunContract.
type DurableFactory func(t testing.TB) (log eventlog.EventLog, reopen func(t testing.TB) eventlog.EventLog)

// RunContract verifies the behavior every EventLog implementation must provide.
func RunContract(t *testing.T, name string, factory Factory) {
	t.Helper()

	t.Run(name+"/append_assigns_per_topic_offsets", func(t *testing.T) {
		log := factory(t)
		defer closeLog(t, log)

		ctx := context.Background()
		first := appendEvent(t, log, ctx, "orders.created", "order-1")
		second := appendEvent(t, log, ctx, "orders.created", "order-2")
		otherTopic := appendEvent(t, log, ctx, "users.created", "user-1")

		if first.Offset != 0 {
			t.Fatalf("Expected first orders offset 0, got %d", first.Offset)
		}
		if second.Offset != 1 {
			t.Fatalf("Expected second orders offset 1, got %d", second.Offset)
		}
		if otherTopic.Offset != 0 {
			t.Fatalf("Expected first users offset 0, got %d", otherTopic.Offset)
		}
	})

	t.Run(name+"/read_events_by_offset_and_limit", func(t *testing.T) {
		log := factory(t)
		defer closeLog(t, log)

		ctx := context.Background()
		for i := 0; i < 5; i++ {
			appendEvent(t, log, ctx, "orders.created", fmt.Sprintf("order-%d", i))
		}

		events, err := log.ReadEvents(ctx, "orders.created", 2, 2)
		if err != nil {
			t.Fatalf("ReadEvents failed: %v", err)
		}
		assertOffsets(t, events, []int64{2, 3})

		events, err = log.ReadEvents(ctx, "orders.created", 10, 5)
		if err != nil {
			t.Fatalf("ReadEvents beyond end failed: %v", err)
		}
		if len(events) != 0 {
			t.Fatalf("Expected no events beyond end, got %d", len(events))
		}

		events, err = log.ReadEvents(ctx, "orders.created", 0, 0)
		if err != nil {
			t.Fatalf("ReadEvents with zero max count failed: %v", err)
		}
		if len(events) != 0 {
			t.Fatalf("Expected zero max count to return no events, got %d", len(events))
		}
	})

	t.Run(name+"/replay_events_from_offset", func(t *testing.T) {
		log := factory(t)
		defer closeLog(t, log)

		ctx := context.Background()
		for i := 0; i < 4; i++ {
			appendEvent(t, log, ctx, "orders.created", fmt.Sprintf("order-%d", i))
		}

		events := collectReplay(t, log, ctx, "orders.created", 1)
		assertOffsets(t, events, []int64{1, 2, 3})
	})

	t.Run(name+"/statistics_report_topic_counts", func(t *testing.T) {
		log := factory(t)
		defer closeLog(t, log)

		ctx := context.Background()
		appendEvent(t, log, ctx, "orders.created", "order-1")
		appendEvent(t, log, ctx, "orders.created", "order-2")
		appendEvent(t, log, ctx, "users.created", "user-1")

		stats, err := log.GetStatistics(ctx)
		if err != nil {
			t.Fatalf("GetStatistics failed: %v", err)
		}
		if stats.TotalEvents != 3 {
			t.Fatalf("Expected 3 total events, got %d", stats.TotalEvents)
		}
		if stats.TopicCount != 2 {
			t.Fatalf("Expected 2 topics, got %d", stats.TopicCount)
		}
		if stats.TopicCounts["orders.created"] != 2 {
			t.Fatalf("Expected 2 orders.created events, got %d", stats.TopicCounts["orders.created"])
		}
		if stats.TopicCounts["users.created"] != 1 {
			t.Fatalf("Expected 1 users.created event, got %d", stats.TopicCounts["users.created"])
		}
	})

	t.Run(name+"/invalid_inputs_return_errors", func(t *testing.T) {
		log := factory(t)
		defer closeLog(t, log)

		ctx := context.Background()
		validEvent := eventlog.NewEvent("orders.created", []byte("order-1"))

		if _, err := log.AppendEvent(ctx, "", validEvent); err == nil {
			t.Fatal("Expected AppendEvent to reject empty topic")
		}
		if _, err := log.AppendEvent(ctx, "orders.created", nil); err == nil {
			t.Fatal("Expected AppendEvent to reject nil event")
		}
		if _, err := log.ReadEvents(ctx, "", 0, 1); err == nil {
			t.Fatal("Expected ReadEvents to reject empty topic")
		}
		if _, err := log.ReadEvents(ctx, "orders.created", -1, 1); err == nil {
			t.Fatal("Expected ReadEvents to reject negative offset")
		}
		if _, err := log.ReadEvents(ctx, "orders.created", 0, -1); err == nil {
			t.Fatal("Expected ReadEvents to reject negative max count")
		}
		if _, err := log.GetTopicEndOffset(ctx, ""); err == nil {
			t.Fatal("Expected GetTopicEndOffset to reject empty topic")
		}
		if err := firstReplayError(log, ctx, "", 0); err == nil {
			t.Fatal("Expected ReplayEvents to reject empty topic")
		}
		if err := firstReplayError(log, ctx, "orders.created", -1); err == nil {
			t.Fatal("Expected ReplayEvents to reject negative offset")
		}
	})

	t.Run(name+"/context_cancellation_returns_error", func(t *testing.T) {
		log := factory(t)
		defer closeLog(t, log)

		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		if _, err := log.AppendEvent(ctx, "orders.created", eventlog.NewEvent("orders.created", []byte("order-1"))); !errors.Is(err, context.Canceled) {
			t.Fatalf("Expected AppendEvent canceled error, got %v", err)
		}
		if _, err := log.ReadEvents(ctx, "orders.created", 0, 1); !errors.Is(err, context.Canceled) {
			t.Fatalf("Expected ReadEvents canceled error, got %v", err)
		}
		if _, err := log.GetTopicEndOffset(ctx, "orders.created"); !errors.Is(err, context.Canceled) {
			t.Fatalf("Expected GetTopicEndOffset canceled error, got %v", err)
		}
		if _, err := log.GetStatistics(ctx); !errors.Is(err, context.Canceled) {
			t.Fatalf("Expected GetStatistics canceled error, got %v", err)
		}
	})

	t.Run(name+"/concurrent_appends_keep_unique_offsets", func(t *testing.T) {
		log := factory(t)
		defer closeLog(t, log)

		ctx := context.Background()
		const totalEvents = 200
		errs := make(chan error, totalEvents)

		for i := 0; i < totalEvents; i++ {
			i := i
			go func() {
				_, err := log.AppendEvent(ctx, "orders.created", eventlog.NewEvent("orders.created", []byte(fmt.Sprintf("order-%d", i))))
				errs <- err
			}()
		}

		for i := 0; i < totalEvents; i++ {
			if err := <-errs; err != nil {
				t.Fatalf("AppendEvent %d failed: %v", i, err)
			}
		}

		events, err := log.ReadEvents(ctx, "orders.created", 0, totalEvents)
		if err != nil {
			t.Fatalf("ReadEvents failed: %v", err)
		}
		if len(events) != totalEvents {
			t.Fatalf("Expected %d events, got %d", totalEvents, len(events))
		}

		offsets := make([]int64, 0, len(events))
		for _, event := range events {
			offsets = append(offsets, event.Offset)
		}
		sort.Slice(offsets, func(i, j int) bool { return offsets[i] < offsets[j] })
		for i, offset := range offsets {
			if offset != int64(i) {
				t.Fatalf("Expected offset %d, got %d", i, offset)
			}
		}
	})

	t.Run(name+"/closed_log_rejects_operations", func(t *testing.T) {
		log := factory(t)
		ctx := context.Background()
		appendEvent(t, log, ctx, "orders.created", "order-1")

		closeLog(t, log)

		if _, err := log.AppendEvent(ctx, "orders.created", eventlog.NewEvent("orders.created", []byte("order-2"))); err == nil {
			t.Fatal("Expected AppendEvent to fail after Close")
		}
		if _, err := log.ReadEvents(ctx, "orders.created", 0, 1); err == nil {
			t.Fatal("Expected ReadEvents to fail after Close")
		}
		if _, err := log.GetTopicEndOffset(ctx, "orders.created"); err == nil {
			t.Fatal("Expected GetTopicEndOffset to fail after Close")
		}
		if _, err := log.GetStatistics(ctx); err == nil {
			t.Fatal("Expected GetStatistics to fail after Close")
		}
		if err := firstReplayError(log, ctx, "orders.created", 0); err == nil {
			t.Fatal("Expected ReplayEvents to fail after Close")
		}
		if err := log.Compact(ctx); err == nil {
			t.Fatal("Expected Compact to fail after Close")
		}
	})
}

// RunDurabilityContract verifies persistence behavior expected from durable logs.
func RunDurabilityContract(t *testing.T, name string, factory DurableFactory) {
	t.Helper()

	t.Run(name+"/reopen_preserves_events_and_offsets", func(t *testing.T) {
		log, reopen := factory(t)
		ctx := context.Background()

		appendEvent(t, log, ctx, "orders.created", "order-1")
		appendEvent(t, log, ctx, "orders.created", "order-2")
		appendEvent(t, log, ctx, "users.created", "user-1")
		closeLog(t, log)

		reopened := reopen(t)
		defer closeLog(t, reopened)

		events, err := reopened.ReadEvents(ctx, "orders.created", 0, 10)
		if err != nil {
			t.Fatalf("ReadEvents after reopen failed: %v", err)
		}
		assertOffsets(t, events, []int64{0, 1})

		next := appendEvent(t, reopened, ctx, "orders.created", "order-3")
		if next.Offset != 2 {
			t.Fatalf("Expected reopened log to continue at offset 2, got %d", next.Offset)
		}

		userEvents, err := reopened.ReadEvents(ctx, "users.created", 0, 10)
		if err != nil {
			t.Fatalf("ReadEvents for users after reopen failed: %v", err)
		}
		assertOffsets(t, userEvents, []int64{0})
	})
}

func appendEvent(t testing.TB, log eventlog.EventLog, ctx context.Context, topic, payload string) *eventlog.Event {
	t.Helper()

	event, err := log.AppendEvent(ctx, topic, eventlog.NewEvent(topic, []byte(payload)))
	if err != nil {
		t.Fatalf("AppendEvent failed: %v", err)
	}
	return event
}

func collectReplay(t testing.TB, log eventlog.EventLog, ctx context.Context, topic string, startOffset int64) []*eventlog.Event {
	t.Helper()

	eventChan, errChan := log.ReplayEvents(ctx, topic, startOffset)
	var events []*eventlog.Event
	for event := range eventChan {
		events = append(events, event)
	}

	if err := <-errChan; err != nil {
		t.Fatalf("ReplayEvents returned error: %v", err)
	}
	return events
}

func firstReplayError(log eventlog.EventLog, ctx context.Context, topic string, startOffset int64) error {
	eventChan, errChan := log.ReplayEvents(ctx, topic, startOffset)
	for range eventChan {
	}
	return <-errChan
}

func assertOffsets(t testing.TB, events []*eventlog.Event, want []int64) {
	t.Helper()

	if len(events) != len(want) {
		t.Fatalf("Expected %d events, got %d", len(want), len(events))
	}
	for i, event := range events {
		if event.Offset != want[i] {
			t.Fatalf("Expected event %d offset %d, got %d", i, want[i], event.Offset)
		}
	}
}

func closeLog(t testing.TB, log eventlog.EventLog) {
	t.Helper()

	if err := log.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}
}
