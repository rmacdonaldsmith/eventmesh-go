package meshnode

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rmacdonaldsmith/eventmesh-go/pkg/eventlog"
	"github.com/rmacdonaldsmith/eventmesh-go/pkg/routingtable"
)

func init() {
	slog.SetDefault(slog.New(slog.NewTextHandler(io.Discard, nil)))
}

func BenchmarkGRPCMeshNode_PublishOnly(b *testing.B) {
	node := newBenchmarkMeshNode(b, "bench-publish-only")
	defer func() { _ = node.Close() }()

	ctx := context.Background()
	publisher := NewTrustedClient("publisher")
	payload := []byte(`{"benchmark":true}`)

	b.ReportAllocs()
	b.SetBytes(int64(len(payload)))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := node.PublishEventWithResult(ctx, publisher, eventlog.NewEvent("bench.publish", payload)); err != nil {
			b.Fatalf("PublishEventWithResult failed: %v", err)
		}
	}
}

func BenchmarkGRPCMeshNode_LocalFanout100(b *testing.B) {
	benchmarkMeshNodeFanout(b, 100)
}

func BenchmarkGRPCMeshNode_LocalFanout1000(b *testing.B) {
	benchmarkMeshNodeFanout(b, 1000)
}

func BenchmarkGRPCMeshNode_ManySubscriptionsMixedTopics(b *testing.B) {
	node := newBenchmarkMeshNode(b, "bench-many-subscriptions")
	defer func() { _ = node.Close() }()

	ctx := context.Background()
	publisher := NewTrustedClient("publisher")
	const clientCount = 1000
	const topicCount = 100

	for i := 0; i < clientCount; i++ {
		client := newBenchmarkSubscriber(fmt.Sprintf("subscriber-%d", i))
		pattern := fmt.Sprintf("bench.topic.%d", i%topicCount)
		if i%5 == 0 {
			pattern = "bench.topic.*"
		}
		if err := node.Subscribe(ctx, client, pattern); err != nil {
			b.Fatalf("Subscribe failed: %v", err)
		}
	}

	payload := []byte(`{"mixed":true}`)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		topic := fmt.Sprintf("bench.topic.%d", i%topicCount)
		if _, err := node.PublishEventWithResult(ctx, publisher, eventlog.NewEvent(topic, payload)); err != nil {
			b.Fatalf("PublishEventWithResult failed: %v", err)
		}
	}
}

func TestGRPCMeshNode_MultiClientLoadSmoke(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping multi-client load smoke test in short mode")
	}

	node := newBenchmarkMeshNode(t, "load-smoke")
	defer func() { _ = node.Close() }()

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	const subscriberCount = 250
	const eventCount = 2000
	for i := 0; i < subscriberCount; i++ {
		client := newBenchmarkSubscriber(fmt.Sprintf("subscriber-%d", i))
		if err := node.Subscribe(ctx, client, "load.topic"); err != nil {
			t.Fatalf("Subscribe failed: %v", err)
		}
	}

	publisher := NewTrustedClient("publisher")
	start := time.Now()
	for i := 0; i < eventCount; i++ {
		if _, err := node.PublishEventWithResult(ctx, publisher, eventlog.NewEvent("load.topic", []byte(`{"load":true}`))); err != nil {
			t.Fatalf("PublishEventWithResult %d failed: %v", i, err)
		}
	}
	elapsed := time.Since(start)
	t.Logf("published=%d subscribers=%d local_deliveries=%d elapsed=%s events_per_second=%.0f deliveries_per_second=%.0f", eventCount, subscriberCount, eventCount*subscriberCount, elapsed, float64(eventCount)/elapsed.Seconds(), float64(eventCount*subscriberCount)/elapsed.Seconds())
}

func benchmarkMeshNodeFanout(b *testing.B, subscriberCount int) {
	node := newBenchmarkMeshNode(b, fmt.Sprintf("bench-fanout-%d", subscriberCount))
	defer func() { _ = node.Close() }()

	ctx := context.Background()
	for i := 0; i < subscriberCount; i++ {
		client := newBenchmarkSubscriber(fmt.Sprintf("subscriber-%d", i))
		if err := node.Subscribe(ctx, client, "bench.fanout"); err != nil {
			b.Fatalf("Subscribe failed: %v", err)
		}
	}

	publisher := NewTrustedClient("publisher")
	payload := []byte(`{"fanout":true}`)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := node.PublishEventWithResult(ctx, publisher, eventlog.NewEvent("bench.fanout", payload)); err != nil {
			b.Fatalf("PublishEventWithResult failed: %v", err)
		}
	}
}

type meshBenchmarkTest interface {
	Helper()
	Fatalf(format string, args ...any)
}

type benchmarkSubscriber struct {
	id        string
	delivered atomic.Int64
}

func newBenchmarkSubscriber(id string) *benchmarkSubscriber {
	return &benchmarkSubscriber{id: id}
}

func (s *benchmarkSubscriber) ID() string                         { return s.id }
func (s *benchmarkSubscriber) IsAuthenticated() bool              { return true }
func (s *benchmarkSubscriber) ConnectedAt() time.Time             { return time.Now() }
func (s *benchmarkSubscriber) Type() routingtable.SubscriberType  { return routingtable.LocalClient }
func (s *benchmarkSubscriber) DeliverEvent(*eventlog.Event) error { s.delivered.Add(1); return nil }

func newBenchmarkMeshNode(tb meshBenchmarkTest, nodeID string) *GRPCMeshNode {
	tb.Helper()

	node, err := NewGRPCMeshNode(NewConfig(nodeID, "127.0.0.1:0"))
	if err != nil {
		tb.Fatalf("NewGRPCMeshNode failed: %v", err)
	}
	if err := node.Start(context.Background()); err != nil {
		_ = node.Close()
		tb.Fatalf("Start failed: %v", err)
	}
	return node
}
