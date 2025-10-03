package peerlink

import (
	"context"
	"testing"
	"time"

	"github.com/rmacdonaldsmith/eventmesh-go/pkg/peerlink"
)

// TestGRPCPeerLink_InterfaceCompliance verifies that GRPCPeerLink implements the PeerLink interface
func TestGRPCPeerLink_InterfaceCompliance(t *testing.T) {
	// This should compile if GRPCPeerLink properly implements peerlink.PeerLink
	var _ peerlink.PeerLink = &GRPCPeerLink{}
}

// TestNewGRPCPeerLink tests the constructor
func TestNewGRPCPeerLink(t *testing.T) {
	config := &Config{
		NodeID:        "test-node",
		ListenAddress: "localhost:9090",
	}
	config.SetDefaults()

	peerLink, err := NewGRPCPeerLink(config)
	if err != nil {
		t.Fatalf("Expected no error creating GRPCPeerLink, got: %v", err)
	}
	if peerLink == nil {
		t.Fatal("Expected non-nil GRPCPeerLink")
	}

	// Should be able to close without error
	err = peerLink.Close()
	if err != nil {
		t.Errorf("Expected no error closing GRPCPeerLink, got: %v", err)
	}
}

// TestNewGRPCPeerLink_InvalidConfig tests constructor with invalid config
func TestNewGRPCPeerLink_InvalidConfig(t *testing.T) {
	config := &Config{
		NodeID: "", // invalid - empty node ID
	}

	_, err := NewGRPCPeerLink(config)
	if err == nil {
		t.Fatal("Expected error with invalid config, got nil")
	}
}

// TestGRPCPeerLink_Close tests the Close method works without panics
func TestGRPCPeerLink_Close(t *testing.T) {
	config := &Config{
		NodeID:        "test-node",
		ListenAddress: "localhost:9090",
	}
	config.SetDefaults()

	peerLink, err := NewGRPCPeerLink(config)
	if err != nil {
		t.Fatalf("Expected no error creating GRPCPeerLink, got: %v", err)
	}

	// Close should work without error
	err = peerLink.Close()
	if err != nil {
		t.Errorf("Expected no error closing GRPCPeerLink, got: %v", err)
	}

	// Multiple closes should be safe
	err = peerLink.Close()
	if err != nil {
		t.Errorf("Expected no error on second close, got: %v", err)
	}
}

// TestGRPCPeerLink_StartStop tests the Start and Stop methods
func TestGRPCPeerLink_StartStop(t *testing.T) {
	config := &Config{
		NodeID:        "test-node",
		ListenAddress: "localhost:0", // Use port 0 to let OS assign an available port
	}
	config.SetDefaults()

	peerLink, err := NewGRPCPeerLink(config)
	if err != nil {
		t.Fatalf("Expected no error creating GRPCPeerLink, got: %v", err)
	}
	defer peerLink.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Test Start
	err = peerLink.Start(ctx)
	if err != nil {
		t.Fatalf("Expected no error starting PeerLink, got: %v", err)
	}

	// Test Stop
	err = peerLink.Stop(ctx)
	if err != nil {
		t.Fatalf("Expected no error stopping PeerLink, got: %v", err)
	}
}

// TestGRPCPeerLink_StartStopMultiple tests multiple Start/Stop calls are safe
func TestGRPCPeerLink_StartStopMultiple(t *testing.T) {
	config := &Config{
		NodeID:        "test-node",
		ListenAddress: "localhost:0",
	}
	config.SetDefaults()

	peerLink, err := NewGRPCPeerLink(config)
	if err != nil {
		t.Fatalf("Expected no error creating GRPCPeerLink, got: %v", err)
	}
	defer peerLink.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Start, Stop, Start, Stop should all work
	if err := peerLink.Start(ctx); err != nil {
		t.Fatalf("First Start failed: %v", err)
	}
	if err := peerLink.Stop(ctx); err != nil {
		t.Fatalf("First Stop failed: %v", err)
	}
	if err := peerLink.Start(ctx); err != nil {
		t.Fatalf("Second Start failed: %v", err)
	}
	if err := peerLink.Stop(ctx); err != nil {
		t.Fatalf("Second Stop failed: %v", err)
	}

	// Multiple stops should be safe
	if err := peerLink.Stop(ctx); err != nil {
		t.Errorf("Multiple Stop calls should be safe, got: %v", err)
	}
}

// simplePeerNode is a test implementation of peerlink.PeerNode
type simplePeerNode struct {
	id      string
	address string
	healthy bool
}

func (p *simplePeerNode) ID() string      { return p.id }
func (p *simplePeerNode) Address() string { return p.address }
func (p *simplePeerNode) IsHealthy() bool { return p.healthy }

// testConfig returns a standard test configuration
func testConfig() *Config {
	config := &Config{
		NodeID:        "test-node",
		ListenAddress: "localhost:0",
	}
	config.SetDefaults()
	return config
}

// TestGRPCPeerLink_ConnectGetPeers tests basic connection management
func TestGRPCPeerLink_ConnectGetPeers(t *testing.T) {
	config := &Config{
		NodeID:        "test-node",
		ListenAddress: "localhost:0",
	}
	config.SetDefaults()

	peerLink, err := NewGRPCPeerLink(config)
	if err != nil {
		t.Fatalf("Expected no error creating GRPCPeerLink, got: %v", err)
	}
	defer peerLink.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Initially no connected peers
	peers, err := peerLink.GetConnectedPeers(ctx)
	if err != nil {
		t.Fatalf("Expected no error getting connected peers, got: %v", err)
	}
	if len(peers) != 0 {
		t.Errorf("Expected 0 connected peers initially, got %d", len(peers))
	}

	// Connect a test peer (tracks in memory without actual gRPC connection)
	testPeer := &simplePeerNode{
		id:      "peer-1",
		address: "localhost:9999", // Non-existent address for testing
		healthy: false,
	}

	err = peerLink.Connect(ctx, testPeer)
	// Connect should succeed and track peer in connected peers map
	if err != nil {
		t.Fatalf("Expected Connect to succeed, got: %v", err)
	}

	// After connecting, should show up in GetConnectedPeers
	peers, err = peerLink.GetConnectedPeers(ctx)
	if err != nil {
		t.Fatalf("Expected no error getting connected peers after connect, got: %v", err)
	}
	if len(peers) != 1 {
		t.Errorf("Expected 1 connected peer after connect, got %d", len(peers))
	}
	if len(peers) > 0 && peers[0].ID() != "peer-1" {
		t.Errorf("Expected connected peer ID to be 'peer-1', got '%s'", peers[0].ID())
	}

	// Test disconnect
	err = peerLink.Disconnect(ctx, "peer-1")
	if err != nil {
		t.Fatalf("Expected no error disconnecting peer, got: %v", err)
	}

	// After disconnect, should have no connected peers
	peers, err = peerLink.GetConnectedPeers(ctx)
	if err != nil {
		t.Fatalf("Expected no error getting connected peers after disconnect, got: %v", err)
	}
	if len(peers) != 0 {
		t.Errorf("Expected 0 connected peers after disconnect, got %d", len(peers))
	}
}

// TestGRPCPeerLink_SendEvent tests stub event routing functionality
func TestGRPCPeerLink_SendEvent(t *testing.T) {
	config := &Config{
		NodeID:        "test-node",
		ListenAddress: "localhost:0",
	}
	config.SetDefaults()

	peerLink, err := NewGRPCPeerLink(config)
	if err != nil {
		t.Fatalf("Expected no error creating GRPCPeerLink, got: %v", err)
	}
	defer peerLink.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Connect a test peer first
	testPeer := &simplePeerNode{
		id:      "peer-1",
		address: "localhost:9999",
		healthy: false,
	}
	err = peerLink.Connect(ctx, testPeer)
	if err != nil {
		t.Fatalf("Expected no error connecting peer, got: %v", err)
	}

	// Test sending event (queues message without actual gRPC transmission)
	err = peerLink.SendEvent(ctx, "peer-1", nil)
	// SendEvent should succeed and queue message for connected peer
	if err != nil {
		t.Fatalf("Expected SendEvent to succeed for connected peer, got: %v", err)
	}

	// Test sending to non-existent peer
	err = peerLink.SendEvent(ctx, "non-existent-peer", nil)
	// Should fail for non-connected peers (improved behavior in Phase 3.3)
	if err == nil {
		t.Error("Expected error when sending to non-connected peer, got nil")
	}
}

// TestGRPCPeerLink_ReceiveEvents tests stub event receiving functionality
func TestGRPCPeerLink_ReceiveEvents(t *testing.T) {
	config := &Config{
		NodeID:        "test-node",
		ListenAddress: "localhost:0",
	}
	config.SetDefaults()

	peerLink, err := NewGRPCPeerLink(config)
	if err != nil {
		t.Fatalf("Expected no error creating GRPCPeerLink, got: %v", err)
	}
	defer peerLink.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	// Test ReceiveEvents returns channels
	eventChan, errChan := peerLink.ReceiveEvents(ctx)
	if eventChan == nil {
		t.Fatal("Expected non-nil event channel")
	}
	if errChan == nil {
		t.Fatal("Expected non-nil error channel")
	}

	// For stub implementation, should get an error indicating not implemented
	// The eventChan is closed in the stub, so we should receive both a closed channel and an error
	select {
	case err := <-errChan:
		if err == nil {
			t.Error("Expected error from stub ReceiveEvents, got nil")
		}
		// Expected - stub implementation returns error
	case event, ok := <-eventChan:
		if ok {
			// If we get an event, it should be the zero value since it's a closed channel
			t.Errorf("Expected closed channel from stub ReceiveEvents, but got event: %v", event)
		}
		// Expected - channel is closed in stub implementation
	case <-ctx.Done():
		t.Error("Test timeout - expected immediate response from stub")
	}
}

// TestGRPCPeerLink_BoundedQueue tests bounded send queue functionality
func TestGRPCPeerLink_BoundedQueue(t *testing.T) {
	config := testConfig()
	config.SendQueueSize = 2                      // Small queue for testing
	config.SendTimeout = 100 * time.Millisecond  // Fast timeout for testing

	peerLink, err := NewGRPCPeerLink(config)
	if err != nil {
		t.Fatalf("Expected no error creating GRPCPeerLink, got: %v", err)
	}
	defer peerLink.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Connect a test peer first
	testPeer := &simplePeerNode{
		id:      "peer-1",
		address: "localhost:9999",
		healthy: false,
	}
	err = peerLink.Connect(ctx, testPeer)
	if err != nil {
		t.Fatalf("Expected no error connecting peer, got: %v", err)
	}

	// Test queue depth starts at 0
	depth := peerLink.GetQueueDepth("peer-1")
	if depth != 0 {
		t.Errorf("Expected initial queue depth 0, got %d", depth)
	}

	// Send events to fill the queue
	for i := 0; i < 2; i++ {
		err = peerLink.SendEvent(ctx, "peer-1", nil)
		if err != nil {
			t.Fatalf("Expected no error sending event %d, got: %v", i, err)
		}
	}

	// Queue should now be full
	depth = peerLink.GetQueueDepth("peer-1")
	if depth != 2 {
		t.Errorf("Expected queue depth 2 after sending 2 events, got %d", depth)
	}

	// Sending another event should timeout and return error (queue full)
	// Use shorter timeout than config to ensure we hit timeout before queue space opens up
	shortCtx, shortCancel := context.WithTimeout(ctx, 50*time.Millisecond)
	defer shortCancel()

	err = peerLink.SendEvent(shortCtx, "peer-1", nil)
	if err == nil {
		t.Error("Expected error when queue is full and timeout expires, got nil")
	}

	// Check drops counter
	drops := peerLink.GetDropsCount("peer-1")
	if drops != 1 {
		t.Errorf("Expected 1 drop after timeout, got %d", drops)
	}

	// Test metrics for non-existent peer should return zero values
	depth = peerLink.GetQueueDepth("non-existent")
	if depth != 0 {
		t.Errorf("Expected queue depth 0 for non-existent peer, got %d", depth)
	}
	drops = peerLink.GetDropsCount("non-existent")
	if drops != 0 {
		t.Errorf("Expected drops count 0 for non-existent peer, got %d", drops)
	}
}

// TestGRPCPeerLink_HealthState tests peer health state management
func TestGRPCPeerLink_HealthState(t *testing.T) {
	config := testConfig()

	peerLink, err := NewGRPCPeerLink(config)
	if err != nil {
		t.Fatalf("Expected no error creating GRPCPeerLink, got: %v", err)
	}
	defer peerLink.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Test health state for non-existent peer
	healthState := peerLink.GetPeerHealthState("non-existent")
	if healthState != PeerDisconnected {
		t.Errorf("Expected PeerDisconnected for non-existent peer, got %s", healthState)
	}

	// Connect a test peer
	testPeer := &simplePeerNode{
		id:      "peer-1",
		address: "localhost:9999",
		healthy: false,
	}
	err = peerLink.Connect(ctx, testPeer)
	if err != nil {
		t.Fatalf("Expected no error connecting peer, got: %v", err)
	}

	// Newly connected peer should be Healthy
	healthState = peerLink.GetPeerHealthState("peer-1")
	if healthState != PeerHealthy {
		t.Errorf("Expected PeerHealthy for newly connected peer, got %s", healthState)
	}

	// Test GetPeerHealth interface method
	isHealthy, err := peerLink.GetPeerHealth(ctx, "peer-1")
	if err != nil {
		t.Fatalf("Expected no error getting peer health, got: %v", err)
	}
	if !isHealthy {
		t.Error("Expected peer to be healthy, got false")
	}

	// Test setting health state to Unhealthy
	peerLink.SetPeerHealth("peer-1", PeerUnhealthy)
	healthState = peerLink.GetPeerHealthState("peer-1")
	if healthState != PeerUnhealthy {
		t.Errorf("Expected PeerUnhealthy after SetPeerHealth, got %s", healthState)
	}

	// GetPeerHealth should now return false
	isHealthy, err = peerLink.GetPeerHealth(ctx, "peer-1")
	if err != nil {
		t.Fatalf("Expected no error getting peer health, got: %v", err)
	}
	if isHealthy {
		t.Error("Expected peer to be unhealthy, got true")
	}

	// Test Disconnect sets health to Disconnected
	err = peerLink.Disconnect(ctx, "peer-1")
	if err != nil {
		t.Fatalf("Expected no error disconnecting peer, got: %v", err)
	}

	healthState = peerLink.GetPeerHealthState("peer-1")
	if healthState != PeerDisconnected {
		t.Errorf("Expected PeerDisconnected after disconnect, got %s", healthState)
	}

	// Test GetPeerHealth for disconnected peer still works (uses metrics)
	isHealthy, err = peerLink.GetPeerHealth(ctx, "peer-1")
	if err != nil {
		t.Fatalf("Expected no error getting health for disconnected peer, got: %v", err)
	}
	if isHealthy {
		t.Error("Expected disconnected peer to be unhealthy, got true")
	}
}

// TestPeerHealthState_String tests the string representation of health states
func TestPeerHealthState_String(t *testing.T) {
	tests := []struct {
		state    PeerHealthState
		expected string
	}{
		{PeerHealthy, "Healthy"},
		{PeerUnhealthy, "Unhealthy"},
		{PeerDisconnected, "Disconnected"},
		{PeerHealthState(999), "Unknown"},
	}

	for _, test := range tests {
		result := test.state.String()
		if result != test.expected {
			t.Errorf("Expected %s.String() = %s, got %s", test.state, test.expected, result)
		}
	}
}