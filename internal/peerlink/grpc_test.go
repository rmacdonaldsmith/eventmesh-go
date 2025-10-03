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

	// Try to connect to a peer (should work but not actually connect for now)
	testPeer := &simplePeerNode{
		id:      "peer-1",
		address: "localhost:9999", // Non-existent address for testing
		healthy: false,
	}

	err = peerLink.Connect(ctx, testPeer)
	// For stub implementation, we expect this to succeed (not actually connect)
	if err != nil {
		t.Fatalf("Expected Connect to succeed in stub mode, got: %v", err)
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

	// Create a dummy event (we'll need to import eventlog package for this)
	// For now, let's just test that SendEvent doesn't crash with a nil event
	err = peerLink.SendEvent(ctx, "peer-1", nil)
	// For stub implementation, we expect this to succeed (not actually send)
	if err != nil {
		t.Fatalf("Expected SendEvent to succeed in stub mode, got: %v", err)
	}

	// Test sending to non-existent peer
	err = peerLink.SendEvent(ctx, "non-existent-peer", nil)
	// Should succeed in stub mode even for non-existent peers
	if err != nil {
		t.Fatalf("Expected SendEvent to succeed for non-existent peer in stub mode, got: %v", err)
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