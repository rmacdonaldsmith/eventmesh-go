package peerlink

import (
	"context"
	"os"
	"runtime"
	"testing"
	"time"

	"github.com/rmacdonaldsmith/eventmesh-go/pkg/eventlog"
	peerlinkpkg "github.com/rmacdonaldsmith/eventmesh-go/pkg/peerlink"
)

func BenchmarkGRPCPeerLink_ReceiveEvents(b *testing.B) {
	link, events, cancel := newDirectReceiveEventsBenchmark(b, b.N)
	defer cancel()

	payload := []byte(`{"benchmark":true}`)
	event := eventlog.NewEvent("bench.peerlink", payload)
	done := drainBenchmarkEvents(b, events, b.N)

	b.ReportAllocs()
	b.SetBytes(int64(len(payload)))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		link.mu.Lock()
		link.distributeEvent(event)
		link.mu.Unlock()
	}
	<-done
}

func TestGRPCPeerLink_ReceiveEventsSustainedResourceSmoke(t *testing.T) {
	if testing.Short() || os.Getenv("EVENTMESH_PERF_SMOKE") != "1" {
		t.Skip("Skipping sustained PeerLink resource smoke test; set EVENTMESH_PERF_SMOKE=1 to run")
	}

	server, client := newPeerLinkBenchmarkPair(t)
	defer func() { _ = client.Close() }()
	defer func() { _ = server.Close() }()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	events, errs := server.ReceiveEvents(ctx)
	const totalEvents = 1000
	payload := []byte(`{"sustained":true}`)

	var before runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&before)
	goroutinesBefore := runtime.NumGoroutine()

	received := make(chan int, 1)
	go func() {
		count := 0
		for count < totalEvents {
			select {
			case <-events:
				count++
			case err := <-errs:
				t.Errorf("ReceiveEvents failed: %v", err)
				received <- count
				return
			case <-ctx.Done():
				t.Errorf("timed out receiving events: %v", ctx.Err())
				received <- count
				return
			}
		}
		received <- count
	}()

	for i := 0; i < totalEvents; i++ {
		if err := client.SendEvent(ctx, "server-node", eventlog.NewEvent("bench.sustained", payload)); err != nil {
			t.Fatalf("SendEvent %d failed: %v", i, err)
		}
	}

	if got := <-received; got != totalEvents {
		t.Fatalf("Expected %d events, got %d", totalEvents, got)
	}

	runtime.GC()
	var after runtime.MemStats
	runtime.ReadMemStats(&after)
	t.Logf("delivered=%d goroutines_before=%d goroutines_after=%d heap_alloc_before=%d heap_alloc_after=%d", totalEvents, goroutinesBefore, runtime.NumGoroutine(), before.HeapAlloc, after.HeapAlloc)
}

func newDirectReceiveEventsBenchmark(b *testing.B, bufferSize int) (*GRPCPeerLink, <-chan *eventlog.Event, context.CancelFunc) {
	b.Helper()

	link, err := NewGRPCPeerLink(&Config{NodeID: "receive-benchmark", ListenAddress: "127.0.0.1:0"})
	if err != nil {
		b.Fatalf("NewGRPCPeerLink failed: %v", err)
	}
	events := make(chan *eventlog.Event, bufferSize)
	link.mu.Lock()
	link.registerSubscriber(events)
	link.mu.Unlock()
	return link, events, func() {
		link.mu.Lock()
		link.unregisterSubscriber(events)
		close(events)
		link.mu.Unlock()
		_ = link.Close()
	}
}

func drainBenchmarkEvents(b *testing.B, events <-chan *eventlog.Event, count int) <-chan struct{} {
	b.Helper()

	done := make(chan struct{})
	go func() {
		defer close(done)
		for i := 0; i < count; i++ {
			<-events
		}
	}()
	return done
}

type peerLinkTest interface {
	Helper()
	Fatalf(format string, args ...any)
}

func newPeerLinkBenchmarkPair(tb peerLinkTest) (*GRPCPeerLink, *GRPCPeerLink) {
	tb.Helper()

	server, err := NewGRPCPeerLink(&Config{NodeID: "server-node", ListenAddress: "127.0.0.1:0", SendQueueSize: 50000})
	if err != nil {
		tb.Fatalf("Failed to create server PeerLink: %v", err)
	}
	client, err := NewGRPCPeerLink(&Config{NodeID: "client-node", ListenAddress: "127.0.0.1:0", SendQueueSize: 50000})
	if err != nil {
		_ = server.Close()
		tb.Fatalf("Failed to create client PeerLink: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := server.Start(ctx); err != nil {
		_ = server.Close()
		_ = client.Close()
		tb.Fatalf("Failed to start server PeerLink: %v", err)
	}
	if err := client.Start(ctx); err != nil {
		_ = server.Close()
		_ = client.Close()
		tb.Fatalf("Failed to start client PeerLink: %v", err)
	}
	if err := client.Connect(ctx, &benchmarkPeerNode{id: "server-node", address: server.GetListeningAddress(), healthy: true}); err != nil {
		_ = server.Close()
		_ = client.Close()
		tb.Fatalf("Failed to connect client to server: %v", err)
	}
	waitForBenchmarkPeerHealth(tb, ctx, client, "server-node", peerlinkpkg.PeerHealthy)
	return server, client
}

func waitForBenchmarkPeerHealth(tb peerLinkTest, ctx context.Context, link *GRPCPeerLink, peerID string, expected peerlinkpkg.PeerHealthState) {
	tb.Helper()

	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		state, err := link.GetPeerHealth(context.Background(), peerID)
		if err == nil && state == expected {
			return
		}
		select {
		case <-ctx.Done():
			tb.Fatalf("timed out waiting for %s health: %v", peerID, ctx.Err())
		case <-time.After(10 * time.Millisecond):
		}
	}
	state, err := link.GetPeerHealth(context.Background(), peerID)
	tb.Fatalf("timed out waiting for %s health to become %s: last=%s err=%v", peerID, expected, state, err)
}

type benchmarkPeerNode struct {
	id      string
	address string
	healthy bool
}

func (p *benchmarkPeerNode) ID() string      { return p.id }
func (p *benchmarkPeerNode) Address() string { return p.address }
func (p *benchmarkPeerNode) IsHealthy() bool { return p.healthy }

func BenchmarkGRPCPeerLink_ReceiveEventsParallelSenders(b *testing.B) {
	link, events, cancel := newDirectReceiveEventsBenchmark(b, b.N)
	defer cancel()

	payload := []byte(`{"parallel":true}`)
	event := eventlog.NewEvent("bench.peerlink", payload)
	done := drainBenchmarkEvents(b, events, b.N)

	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			link.mu.Lock()
			link.distributeEvent(event)
			link.mu.Unlock()
		}
	})
	<-done
}
