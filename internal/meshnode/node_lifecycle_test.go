package meshnode

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	eventlogpkg "github.com/rmacdonaldsmith/eventmesh-go/pkg/eventlog"
	peerlinkpkg "github.com/rmacdonaldsmith/eventmesh-go/pkg/peerlink"
)

// TestGRPCMeshNode_StartStopClose tests the lifecycle methods
func TestGRPCMeshNode_StartStopClose(t *testing.T) {
	config := NewConfig("test-node", "localhost:8081") // Different port to avoid conflicts
	node, err := NewGRPCMeshNode(config)
	if err != nil {
		t.Fatalf("Expected no error creating mesh node, got %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Test Start
	err = node.Start(ctx)
	if err != nil {
		t.Fatalf("Expected no error starting node, got %v", err)
	}

	// Verify node is started
	node.mu.RLock()
	started := node.started
	closed := node.closed
	node.mu.RUnlock()

	if !started {
		t.Error("Expected node to be started after Start()")
	}
	if closed {
		t.Error("Expected node to not be closed after Start()")
	}

	// Test idempotent Start
	err = node.Start(ctx)
	if err != nil {
		t.Errorf("Expected no error from idempotent Start(), got %v", err)
	}

	// Test Stop
	err = node.Stop(ctx)
	if err != nil {
		t.Fatalf("Expected no error stopping node, got %v", err)
	}

	// Verify node is stopped
	node.mu.RLock()
	started = node.started
	closed = node.closed
	node.mu.RUnlock()

	if started {
		t.Error("Expected node to not be started after Stop()")
	}
	if closed {
		t.Error("Expected node to not be closed after Stop() (should still be closeable)")
	}

	// Test idempotent Stop
	err = node.Stop(ctx)
	if err != nil {
		t.Errorf("Expected no error from idempotent Stop(), got %v", err)
	}

	// Test Close
	err = node.Close()
	if err != nil {
		t.Fatalf("Expected no error closing node, got %v", err)
	}

	// Verify node is closed
	node.mu.RLock()
	started = node.started
	closed = node.closed
	node.mu.RUnlock()

	if started {
		t.Error("Expected node to not be started after Close()")
	}
	if !closed {
		t.Error("Expected node to be closed after Close()")
	}

	// Test idempotent Close
	err = node.Close()
	if err != nil {
		t.Errorf("Expected no error from idempotent Close(), got %v", err)
	}
}

// TestGRPCMeshNode_StartAfterClose tests starting a closed node
func TestGRPCMeshNode_StartAfterClose(t *testing.T) {
	config := NewConfig("test-node", "localhost:8082")
	node, err := NewGRPCMeshNode(config)
	if err != nil {
		t.Fatalf("Expected no error creating mesh node, got %v", err)
	}

	ctx := context.Background()

	// Close the node first
	err = node.Close()
	if err != nil {
		t.Fatalf("Expected no error closing node, got %v", err)
	}

	// Try to start after close - should fail
	err = node.Start(ctx)
	if err == nil {
		t.Error("Expected error when starting closed node")
	}
}

func TestGRPCMeshNode_StartOwnsHeartbeatLifecycle(t *testing.T) {
	config := NewConfig("heartbeat-owner-node", "localhost:0")
	node, err := NewGRPCMeshNode(config)
	if err != nil {
		t.Fatalf("Expected no error creating mesh node, got %v", err)
	}
	defer node.Close()

	peerLink := newLifecycleSpyPeerLink()
	node.peerLink = peerLink

	ctx := context.Background()
	if err := node.Start(ctx); err != nil {
		t.Fatalf("Expected no error starting node, got %v", err)
	}

	if peerLink.startHeartbeatsCalls != 1 {
		t.Fatalf("Expected Start to start heartbeats once, got %d", peerLink.startHeartbeatsCalls)
	}

	if err := node.Start(ctx); err != nil {
		t.Fatalf("Expected idempotent Start to succeed, got %v", err)
	}
	if peerLink.startHeartbeatsCalls != 1 {
		t.Fatalf("Expected idempotent Start not to start heartbeats again, got %d", peerLink.startHeartbeatsCalls)
	}
}

func TestGRPCMeshNode_StopOwnsPeerLifecycle(t *testing.T) {
	config := NewConfig("stop-owner-node", "localhost:0")
	node, err := NewGRPCMeshNode(config)
	if err != nil {
		t.Fatalf("Expected no error creating mesh node, got %v", err)
	}
	defer node.Close()

	peerLink := newLifecycleSpyPeerLink()
	node.peerLink = peerLink

	ctx := context.Background()
	if err := node.Start(ctx); err != nil {
		t.Fatalf("Expected no error starting node, got %v", err)
	}

	eventCtx := peerLink.waitForEventContext(t)
	changeCtx := peerLink.waitForChangeContext(t)

	if err := node.Stop(ctx); err != nil {
		t.Fatalf("Expected no error stopping node, got %v", err)
	}

	if peerLink.stopHeartbeatsCalls != 1 {
		t.Fatalf("Expected Stop to stop heartbeats once, got %d", peerLink.stopHeartbeatsCalls)
	}
	if peerLink.stopCalls != 1 {
		t.Fatalf("Expected Stop to stop PeerLink server once, got %d", peerLink.stopCalls)
	}

	assertContextCanceled(t, eventCtx)
	assertContextCanceled(t, changeCtx)
}

// TestGRPCMeshNode_ComprehensiveHealthMonitoring tests the enhanced GetHealth implementation
func TestGRPCMeshNode_ComprehensiveHealthMonitoring(t *testing.T) {
	config := NewConfig("health-test-node", "localhost:9090")
	node, err := NewGRPCMeshNode(config)
	if err != nil {
		t.Fatalf("Expected no error creating mesh node, got %v", err)
	}
	defer node.Close()

	ctx := context.Background()

	// Test health when node is created but not started
	health, err := node.GetHealth(ctx)
	if err != nil {
		t.Errorf("Expected no error from GetHealth(), got %v", err)
	}

	// Node should be healthy (components working) but note that it's not started
	if !health.EventLogHealthy {
		t.Error("Expected EventLog to be healthy")
	}
	if !health.RoutingTableHealthy {
		t.Error("Expected RoutingTable to be healthy")
	}
	if !health.PeerLinkHealthy {
		t.Error("Expected PeerLink to be healthy")
	}
	if health.ConnectedClients != 0 {
		t.Errorf("Expected 0 connected clients, got %d", health.ConnectedClients)
	}
	if health.ConnectedPeers != 0 {
		t.Errorf("Expected 0 connected peers, got %d", health.ConnectedPeers)
	}
	if health.Message != "Issues detected: [node is not started]" {
		t.Errorf("Expected message about not started, got '%s'", health.Message)
	}

	// Start the node
	err = node.Start(ctx)
	if err != nil {
		t.Errorf("Expected no error starting node, got %v", err)
	}

	// Test health when node is started - should be fully healthy
	health, err = node.GetHealth(ctx)
	if err != nil {
		t.Errorf("Expected no error from GetHealth(), got %v", err)
	}

	if !health.Healthy {
		t.Error("Expected node to be healthy when started")
	}
	if !health.EventLogHealthy {
		t.Error("Expected EventLog to be healthy")
	}
	if !health.RoutingTableHealthy {
		t.Error("Expected RoutingTable to be healthy")
	}
	if !health.PeerLinkHealthy {
		t.Error("Expected PeerLink to be healthy")
	}
	if health.Message != "All components healthy" {
		t.Errorf("Expected 'All components healthy', got '%s'", health.Message)
	}

	// Add a client to test client counting
	client, err := node.AuthenticateClient(ctx, "health-test-client")
	if err != nil {
		t.Errorf("Expected no error authenticating client, got %v", err)
	}

	// Subscribe the client to test routing table interaction
	err = node.Subscribe(ctx, client, "test.health")
	if err != nil {
		t.Errorf("Expected no error subscribing client, got %v", err)
	}

	// Check health includes connected client
	health, err = node.GetHealth(ctx)
	if err != nil {
		t.Errorf("Expected no error from GetHealth(), got %v", err)
	}

	if health.ConnectedClients != 1 {
		t.Errorf("Expected 1 connected client, got %d", health.ConnectedClients)
	}
	if !health.Healthy {
		t.Error("Expected node to remain healthy with connected client")
	}

	// Test health when node is closed
	err = node.Close()
	if err != nil {
		t.Errorf("Expected no error closing node, got %v", err)
	}

	health, err = node.GetHealth(ctx)
	if err != nil {
		t.Errorf("Expected no error from GetHealth() on closed node, got %v", err)
	}

	if health.Healthy {
		t.Error("Expected node to be unhealthy when closed")
	}
	if health.EventLogHealthy {
		t.Error("Expected EventLog to be unhealthy when closed")
	}
	if health.RoutingTableHealthy {
		t.Error("Expected RoutingTable to be unhealthy when closed")
	}
	if health.PeerLinkHealthy {
		t.Error("Expected PeerLink to be unhealthy when closed")
	}

	// Message should indicate node is closed
	if !strings.Contains(health.Message, "node is closed") {
		t.Errorf("Expected health message to mention node is closed, got '%s'", health.Message)
	}

	t.Logf("✅ Phase 4.5: Comprehensive health monitoring implemented and tested")
}

type lifecycleSpyPeerLink struct {
	startHeartbeatsCalls int
	stopHeartbeatsCalls  int
	stopCalls            int
	events               chan *eventlogpkg.Event
	eventErrs            chan error
	changes              chan *peerlinkpkg.SubscriptionChange
	changeErrs           chan error
	eventCtxs            chan context.Context
	changeCtxs           chan context.Context
	closeOnce            sync.Once
}

func newLifecycleSpyPeerLink() *lifecycleSpyPeerLink {
	return &lifecycleSpyPeerLink{
		events:     make(chan *eventlogpkg.Event),
		eventErrs:  make(chan error),
		changes:    make(chan *peerlinkpkg.SubscriptionChange),
		changeErrs: make(chan error),
		eventCtxs:  make(chan context.Context, 1),
		changeCtxs: make(chan context.Context, 1),
	}
}

func (l *lifecycleSpyPeerLink) SendEvent(ctx context.Context, peerID string, event *eventlogpkg.Event) error {
	return nil
}

func (l *lifecycleSpyPeerLink) ReceiveEvents(ctx context.Context) (<-chan *eventlogpkg.Event, <-chan error) {
	l.eventCtxs <- ctx
	return l.events, l.eventErrs
}

func (l *lifecycleSpyPeerLink) SendSubscriptionChange(ctx context.Context, peerID string, change *peerlinkpkg.SubscriptionChange) error {
	return nil
}

func (l *lifecycleSpyPeerLink) ReceiveSubscriptionChanges(ctx context.Context) (<-chan *peerlinkpkg.SubscriptionChange, <-chan error) {
	l.changeCtxs <- ctx
	return l.changes, l.changeErrs
}

func (l *lifecycleSpyPeerLink) SendHeartbeat(ctx context.Context, peerID string) error {
	return nil
}

func (l *lifecycleSpyPeerLink) GetPeerHealth(ctx context.Context, peerID string) (peerlinkpkg.PeerHealthState, error) {
	return peerlinkpkg.PeerHealthy, nil
}

func (l *lifecycleSpyPeerLink) StartHeartbeats(ctx context.Context) error {
	l.startHeartbeatsCalls++
	return nil
}

func (l *lifecycleSpyPeerLink) StopHeartbeats(ctx context.Context) error {
	l.stopHeartbeatsCalls++
	return nil
}

func (l *lifecycleSpyPeerLink) Stop(ctx context.Context) error {
	l.stopCalls++
	return nil
}

func (l *lifecycleSpyPeerLink) Connect(ctx context.Context, peer peerlinkpkg.PeerNode) error {
	return nil
}

func (l *lifecycleSpyPeerLink) Disconnect(ctx context.Context, peerID string) error {
	return nil
}

func (l *lifecycleSpyPeerLink) GetConnectedPeers(ctx context.Context) ([]peerlinkpkg.PeerNode, error) {
	return nil, nil
}

func (l *lifecycleSpyPeerLink) Close() error {
	l.closeOnce.Do(func() {
		close(l.events)
		close(l.eventErrs)
		close(l.changes)
		close(l.changeErrs)
	})
	return nil
}

func (l *lifecycleSpyPeerLink) waitForEventContext(t *testing.T) context.Context {
	t.Helper()

	select {
	case ctx := <-l.eventCtxs:
		return ctx
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for ReceiveEvents context")
		return nil
	}
}

func (l *lifecycleSpyPeerLink) waitForChangeContext(t *testing.T) context.Context {
	t.Helper()

	select {
	case ctx := <-l.changeCtxs:
		return ctx
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for ReceiveSubscriptionChanges context")
		return nil
	}
}

func assertContextCanceled(t *testing.T, ctx context.Context) {
	t.Helper()

	select {
	case <-ctx.Done():
	case <-time.After(time.Second):
		t.Fatal("expected lifecycle context to be canceled")
	}
}

func TestGRPCMeshNode_GetHealthDoesNotMutateEventLog(t *testing.T) {
	config := NewConfig("health-readonly-node", "localhost:0")
	node, err := NewGRPCMeshNode(config)
	if err != nil {
		t.Fatalf("Expected no error creating mesh node, got %v", err)
	}
	defer node.Close()

	ctx := context.Background()
	eventLog := node.GetEventLog()

	before, err := eventLog.GetStatistics(ctx)
	if err != nil {
		t.Fatalf("Expected no error getting initial event log statistics, got %v", err)
	}

	for i := 0; i < 3; i++ {
		if _, err := node.GetHealth(ctx); err != nil {
			t.Fatalf("Expected no error from GetHealth(), got %v", err)
		}
	}

	after, err := eventLog.GetStatistics(ctx)
	if err != nil {
		t.Fatalf("Expected no error getting final event log statistics, got %v", err)
	}

	if after.TotalEvents != before.TotalEvents {
		t.Errorf("Expected GetHealth not to change total events, before=%d after=%d", before.TotalEvents, after.TotalEvents)
	}
	if after.TopicCount != before.TopicCount {
		t.Errorf("Expected GetHealth not to change topic count, before=%d after=%d", before.TopicCount, after.TopicCount)
	}
	if _, exists := after.TopicCounts["__health.check"]; exists {
		t.Error("Expected GetHealth not to create __health.check topic")
	}
}
