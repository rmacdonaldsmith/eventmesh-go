package meshnode

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net"
	"strings"
	"sync"
	"testing"
	"time"

	internalpeerlink "github.com/rmacdonaldsmith/eventmesh-go/internal/peerlink"
	"github.com/rmacdonaldsmith/eventmesh-go/pkg/eventlog"
	peerlinkpkg "github.com/rmacdonaldsmith/eventmesh-go/pkg/peerlink"
)

func TestGRPCMeshNode_PeerFailureDoesNotBlockHealthyPeerForwarding(t *testing.T) {
	ctx := context.Background()
	topic := "orders.partition.partial"
	healthyPeerID := "healthy-peer"
	failedPeerID := "failed-peer"

	node, err := NewGRPCMeshNode(NewConfig("partition-publisher", "127.0.0.1:0"))
	if err != nil {
		t.Fatalf("Failed to create mesh node: %v", err)
	}
	peerLink := newPartitionPeerLink(
		&simplePeerNode{id: healthyPeerID, address: "healthy-peer", healthy: true},
		&simplePeerNode{id: failedPeerID, address: "failed-peer", healthy: true},
	)
	peerLink.failSendsTo(failedPeerID, errors.New("simulated partition"))
	node.setPeerLink(peerLink)
	defer node.Close()

	if err := node.Start(ctx); err != nil {
		t.Fatalf("Failed to start mesh node: %v", err)
	}

	node.processIncomingSubscriptionChange(ctx, &peerlinkpkg.SubscriptionChange{
		Action:   "subscribe",
		ClientId: "healthy-subscriber",
		Topic:    topic,
		NodeId:   healthyPeerID,
	})
	node.processIncomingSubscriptionChange(ctx, &peerlinkpkg.SubscriptionChange{
		Action:   "subscribe",
		ClientId: "failed-subscriber",
		Topic:    topic,
		NodeId:   failedPeerID,
	})

	publisher := NewTrustedClient("partition-publisher-client")
	payload := []byte(`{"partition":"partial","target":"healthy-peer"}`)
	_, err = node.PublishEventWithResult(ctx, publisher, eventlog.NewEvent(topic, payload))
	if err == nil {
		t.Fatal("Expected partial peer failure to be reported")
	}
	if !strings.Contains(err.Error(), "peer failures") {
		t.Fatalf("Expected peer failure error, got %v", err)
	}

	persistedEvents := requireEventLogCount(t, ctx, node, topic, 1)
	if string(persistedEvents[0].Payload) != string(payload) {
		t.Fatalf("Expected locally persisted payload %s, got %s", payload, persistedEvents[0].Payload)
	}

	healthyEvents := peerLink.sentEvents(healthyPeerID)
	if len(healthyEvents) != 1 {
		t.Fatalf("Expected healthy peer to receive 1 event despite failed peer, got %d", len(healthyEvents))
	}
	if string(healthyEvents[0].Payload) != string(payload) {
		t.Fatalf("Expected healthy peer payload %s, got %s", payload, healthyEvents[0].Payload)
	}

	failedEvents := peerLink.sentEvents(failedPeerID)
	if len(failedEvents) != 0 {
		t.Fatalf("Expected failed peer to receive no successfully sent events, got %d", len(failedEvents))
	}
}

func TestGRPCMeshNode_BackpressuredPeerReportsDropAfterLocalPersistence(t *testing.T) {
	restoreLogs := discardSlogForTest()
	defer restoreLogs()

	ctx := context.Background()
	topic := "orders.partition.backpressure"
	peerID := "backpressured-peer"

	unreachableAddress := reserveAndCloseLocalAddress(t)
	node, err := NewGRPCMeshNode(NewConfig("backpressure-publisher", "127.0.0.1:0").
		WithPeerLinkConfig(&internalpeerlink.Config{
			NodeID:          "backpressure-publisher",
			ListenAddress:   "127.0.0.1:0",
			SendQueueSize:   1,
			SendTimeout:     10 * time.Millisecond,
			MaxSendAttempts: 1,
		}))
	if err != nil {
		t.Fatalf("Failed to create mesh node: %v", err)
	}
	defer node.Close()

	if err := node.Start(ctx); err != nil {
		t.Fatalf("Failed to start mesh node: %v", err)
	}
	defer func() {
		stopCtx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		if err := node.Stop(stopCtx); err != nil {
			t.Fatalf("Failed to stop mesh node: %v", err)
		}
	}()

	if err := node.GetPeerLink().Connect(ctx, &simplePeerNode{
		id:      peerID,
		address: unreachableAddress,
		healthy: true,
	}); err != nil {
		t.Fatalf("Failed to connect unreachable peer: %v", err)
	}
	node.processIncomingSubscriptionChange(ctx, &peerlinkpkg.SubscriptionChange{
		Action:   "subscribe",
		ClientId: "backpressured-subscriber",
		Topic:    topic,
		NodeId:   peerID,
	})

	publisher := NewTrustedClient("backpressure-publisher-client")
	firstPayload := []byte(`{"backpressure":"queued"}`)
	if _, err := node.PublishEventWithResult(ctx, publisher, eventlog.NewEvent(topic, firstPayload)); err != nil {
		t.Fatalf("Expected first event to fill the peer queue without error, got %v", err)
	}

	secondPayload := []byte(`{"backpressure":"dropped"}`)
	_, err = node.PublishEventWithResult(ctx, publisher, eventlog.NewEvent(topic, secondPayload))
	if err == nil {
		t.Fatal("Expected saturated peer queue to report a publish delivery failure")
	}
	if !strings.Contains(err.Error(), "peer failures") {
		t.Fatalf("Expected peer failure error, got %v", err)
	}

	persistedEvents := requireEventLogCount(t, ctx, node, topic, 2)
	if string(persistedEvents[0].Payload) != string(firstPayload) {
		t.Fatalf("Expected first payload %s, got %s", firstPayload, persistedEvents[0].Payload)
	}
	if string(persistedEvents[1].Payload) != string(secondPayload) {
		t.Fatalf("Expected second payload %s, got %s", secondPayload, persistedEvents[1].Payload)
	}

	dropCounter, ok := node.GetPeerLink().(interface {
		GetDropsCount(peerID string) int64
	})
	if !ok {
		t.Fatal("Expected PeerLink to expose drop metrics")
	}
	if drops := dropCounter.GetDropsCount(peerID); drops != 1 {
		t.Fatalf("Expected 1 drop for saturated peer queue, got %d", drops)
	}
}

type partitionPeerLink struct {
	mu              sync.Mutex
	peers           []peerlinkpkg.PeerNode
	sendFailures    map[string]error
	sentByPeer      map[string][]*eventlog.Event
	events          chan *eventlog.Event
	eventErrs       chan error
	changes         chan *peerlinkpkg.SubscriptionChange
	changeErrs      chan error
	closeOnce       sync.Once
	heartbeatsStart int
	heartbeatsStop  int
}

func reserveAndCloseLocalAddress(t *testing.T) string {
	t.Helper()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to reserve local address: %v", err)
	}
	address := listener.Addr().String()
	if err := listener.Close(); err != nil {
		t.Fatalf("Failed to close reserved local address: %v", err)
	}
	return address
}

func discardSlogForTest() func() {
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(io.Discard, nil)))
	return func() {
		slog.SetDefault(previous)
	}
}

func newPartitionPeerLink(peers ...peerlinkpkg.PeerNode) *partitionPeerLink {
	return &partitionPeerLink{
		peers:        append([]peerlinkpkg.PeerNode{}, peers...),
		sendFailures: make(map[string]error),
		sentByPeer:   make(map[string][]*eventlog.Event),
		events:       make(chan *eventlog.Event),
		eventErrs:    make(chan error),
		changes:      make(chan *peerlinkpkg.SubscriptionChange),
		changeErrs:   make(chan error),
	}
}

func (p *partitionPeerLink) failSendsTo(peerID string, err error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.sendFailures[peerID] = err
}

func (p *partitionPeerLink) sentEvents(peerID string) []*eventlog.Event {
	p.mu.Lock()
	defer p.mu.Unlock()

	events := p.sentByPeer[peerID]
	copied := make([]*eventlog.Event, 0, len(events))
	for _, event := range events {
		copied = append(copied, event.Copy())
	}
	return copied
}

func (p *partitionPeerLink) SendEvent(ctx context.Context, peerID string, event *eventlog.Event) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if err := p.sendFailures[peerID]; err != nil {
		return err
	}
	p.sentByPeer[peerID] = append(p.sentByPeer[peerID], event.Copy())
	return nil
}

func (p *partitionPeerLink) ReceiveEvents(ctx context.Context) (<-chan *eventlog.Event, <-chan error) {
	return p.events, p.eventErrs
}

func (p *partitionPeerLink) SendSubscriptionChange(ctx context.Context, peerID string, change *peerlinkpkg.SubscriptionChange) error {
	return nil
}

func (p *partitionPeerLink) ReceiveSubscriptionChanges(ctx context.Context) (<-chan *peerlinkpkg.SubscriptionChange, <-chan error) {
	return p.changes, p.changeErrs
}

func (p *partitionPeerLink) SendHeartbeat(ctx context.Context, peerID string) error {
	return nil
}

func (p *partitionPeerLink) GetPeerHealth(ctx context.Context, peerID string) (peerlinkpkg.PeerHealthState, error) {
	return peerlinkpkg.PeerHealthy, nil
}

func (p *partitionPeerLink) StartHeartbeats(ctx context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.heartbeatsStart++
	return nil
}

func (p *partitionPeerLink) StopHeartbeats(ctx context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.heartbeatsStop++
	return nil
}

func (p *partitionPeerLink) Connect(ctx context.Context, peer peerlinkpkg.PeerNode) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.peers = append(p.peers, peer)
	return nil
}

func (p *partitionPeerLink) Disconnect(ctx context.Context, peerID string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	filtered := p.peers[:0]
	for _, peer := range p.peers {
		if peer.ID() != peerID {
			filtered = append(filtered, peer)
		}
	}
	p.peers = filtered
	return nil
}

func (p *partitionPeerLink) GetConnectedPeers(ctx context.Context) ([]peerlinkpkg.PeerNode, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return append([]peerlinkpkg.PeerNode{}, p.peers...), nil
}

func (p *partitionPeerLink) Close() error {
	p.closeOnce.Do(func() {
		close(p.events)
		close(p.eventErrs)
		close(p.changes)
		close(p.changeErrs)
	})
	return nil
}
