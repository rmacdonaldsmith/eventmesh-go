package peerlink

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/rmacdonaldsmith/eventmesh-go/pkg/eventlog"
	"github.com/rmacdonaldsmith/eventmesh-go/pkg/peerlink"
	peerlinkv1 "github.com/rmacdonaldsmith/eventmesh-go/proto/peerlink/v1"
	"google.golang.org/grpc"
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
	healthState, err := peerLink.GetPeerHealth(ctx, "non-existent")
	if err == nil {
		t.Error("Expected error for non-existent peer")
	}
	if healthState != peerlink.PeerDisconnected {
		t.Errorf("Expected peerlink.PeerDisconnected for non-existent peer, got %s", healthState)
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
	healthState, err = peerLink.GetPeerHealth(ctx, "peer-1")
	if err != nil {
		t.Fatalf("Expected no error getting peer health, got: %v", err)
	}
	if healthState != peerlink.PeerHealthy {
		t.Errorf("Expected peerlink.PeerHealthy for newly connected peer, got %s", healthState)
	}

	// Test setting health state to Unhealthy
	peerLink.SetPeerHealth("peer-1", peerlink.PeerUnhealthy)
	healthState, err = peerLink.GetPeerHealth(ctx, "peer-1")
	if err != nil {
		t.Fatalf("Expected no error getting peer health, got: %v", err)
	}
	if healthState != peerlink.PeerUnhealthy {
		t.Errorf("Expected peerlink.PeerUnhealthy after SetPeerHealth, got %s", healthState)
	}

	// Test Disconnect sets health to Disconnected
	err = peerLink.Disconnect(ctx, "peer-1")
	if err != nil {
		t.Fatalf("Expected no error disconnecting peer, got: %v", err)
	}

	healthState, err = peerLink.GetPeerHealth(ctx, "peer-1")
	if err != nil {
		t.Fatalf("Expected no error getting health for disconnected peer, got: %v", err)
	}
	if healthState != peerlink.PeerDisconnected {
		t.Errorf("Expected peerlink.PeerDisconnected after disconnect, got %s", healthState)
	}
}

// TestPeerHealthState_String tests the string representation of health states
func TestPeerHealthState_String(t *testing.T) {
	tests := []struct {
		state    peerlink.PeerHealthState
		expected string
	}{
		{peerlink.PeerHealthy, "Healthy"},
		{peerlink.PeerUnhealthy, "Unhealthy"},
		{peerlink.PeerDisconnected, "Disconnected"},
		{peerlink.PeerHealthState(999), "Unknown"},
	}

	for _, test := range tests {
		result := test.state.String()
		if result != test.expected {
			t.Errorf("Expected %s.String() = %s, got %s", test.state, test.expected, result)
		}
	}
}

// TestGRPCPeerLink_Metrics tests the metrics collection functionality
func TestGRPCPeerLink_Metrics(t *testing.T) {
	config := &Config{
		NodeID:        "test-node",
		ListenAddress: "localhost:0",
		SendQueueSize: 2, // Small queue for testing drops
		SendTimeout:   100 * time.Millisecond,
	}
	peerLink, err := NewGRPCPeerLink(config)
	if err != nil {
		t.Fatalf("Failed to create GRPCPeerLink: %v", err)
	}

	// Test metrics for non-existent peer
	if peerLink.GetQueueDepth("nonexistent") != 0 {
		t.Error("Expected queue depth 0 for non-existent peer")
	}
	if peerLink.GetDropsCount("nonexistent") != 0 {
		t.Error("Expected drops count 0 for non-existent peer")
	}

	// Connect a peer to create metrics
	peer := &simplePeerNode{id: "test-peer", address: "localhost:8080", healthy: true}
	err = peerLink.Connect(context.Background(), peer)
	if err != nil {
		t.Fatalf("Failed to connect peer: %v", err)
	}

	// Test initial metrics
	if peerLink.GetQueueDepth("test-peer") != 0 {
		t.Error("Expected initial queue depth to be 0")
	}
	if peerLink.GetDropsCount("test-peer") != 0 {
		t.Error("Expected initial drops count to be 0")
	}

	// Test queue depth by sending messages
	event1 := eventlog.NewRecord("test", []byte("test1"))
	event2 := eventlog.NewRecord("test", []byte("test2"))

	err = peerLink.SendEvent(context.Background(), "test-peer", event1)
	if err != nil {
		t.Fatalf("Failed to send event1: %v", err)
	}
	if peerLink.GetQueueDepth("test-peer") != 1 {
		t.Errorf("Expected queue depth 1, got %d", peerLink.GetQueueDepth("test-peer"))
	}

	err = peerLink.SendEvent(context.Background(), "test-peer", event2)
	if err != nil {
		t.Fatalf("Failed to send event2: %v", err)
	}
	if peerLink.GetQueueDepth("test-peer") != 2 {
		t.Errorf("Expected queue depth 2, got %d", peerLink.GetQueueDepth("test-peer"))
	}

	// Test drops count by filling queue and causing drops
	event3 := eventlog.NewRecord("test", []byte("test3"))
	err = peerLink.SendEvent(context.Background(), "test-peer", event3)

	// This should cause a drop since queue is full (size 2) and timeout is short
	if err == nil {
		t.Error("Expected error when queue is full, but got none")
	}
	if peerLink.GetDropsCount("test-peer") != 1 {
		t.Errorf("Expected drops count 1, got %d", peerLink.GetDropsCount("test-peer"))
	}
}

// TestGRPCPeerLink_GetAllPeerMetrics tests the aggregate metrics functionality
func TestGRPCPeerLink_GetAllPeerMetrics(t *testing.T) {
	config := &Config{
		NodeID:        "test-node",
		ListenAddress: "localhost:0",
		SendQueueSize: 10,
		SendTimeout:   1 * time.Second,
	}
	peerLink, err := NewGRPCPeerLink(config)
	if err != nil {
		t.Fatalf("Failed to create GRPCPeerLink: %v", err)
	}

	// Test empty metrics
	metrics := peerLink.GetAllPeerMetrics()
	if len(metrics) != 0 {
		t.Errorf("Expected 0 metrics, got %d", len(metrics))
	}

	// Connect multiple peers
	peer1 := &simplePeerNode{id: "peer1", address: "localhost:8081", healthy: true}
	peer2 := &simplePeerNode{id: "peer2", address: "localhost:8082", healthy: true}

	err = peerLink.Connect(context.Background(), peer1)
	if err != nil {
		t.Fatalf("Failed to connect peer1: %v", err)
	}
	err = peerLink.Connect(context.Background(), peer2)
	if err != nil {
		t.Fatalf("Failed to connect peer2: %v", err)
	}

	// Send events to create different queue depths
	event := eventlog.NewRecord("test", []byte("test"))
	err = peerLink.SendEvent(context.Background(), "peer1", event)
	if err != nil {
		t.Fatalf("Failed to send event to peer1: %v", err)
	}

	// Set different health states
	peerLink.SetPeerHealth("peer2", peerlink.PeerUnhealthy)

	// Get all metrics
	metrics = peerLink.GetAllPeerMetrics()
	if len(metrics) != 2 {
		t.Fatalf("Expected 2 metrics, got %d", len(metrics))
	}

	// Find metrics for each peer
	var peer1Metrics, peer2Metrics *PeerMetrics
	for i := range metrics {
		if metrics[i].PeerID == "peer1" {
			peer1Metrics = &metrics[i]
		} else if metrics[i].PeerID == "peer2" {
			peer2Metrics = &metrics[i]
		}
	}

	// Verify peer1 metrics
	if peer1Metrics == nil {
		t.Fatal("peer1 metrics not found")
	}
	if peer1Metrics.QueueDepth != 1 {
		t.Errorf("Expected peer1 queue depth 1, got %d", peer1Metrics.QueueDepth)
	}
	if peer1Metrics.HealthState != peerlink.PeerHealthy {
		t.Errorf("Expected peer1 health state Healthy, got %v", peer1Metrics.HealthState)
	}
	if peer1Metrics.DropsCount != 0 {
		t.Errorf("Expected peer1 drops count 0, got %d", peer1Metrics.DropsCount)
	}

	// Verify peer2 metrics
	if peer2Metrics == nil {
		t.Fatal("peer2 metrics not found")
	}
	if peer2Metrics.QueueDepth != 0 {
		t.Errorf("Expected peer2 queue depth 0, got %d", peer2Metrics.QueueDepth)
	}
	if peer2Metrics.HealthState != peerlink.PeerUnhealthy {
		t.Errorf("Expected peer2 health state Unhealthy, got %v", peer2Metrics.HealthState)
	}
	if peer2Metrics.DropsCount != 0 {
		t.Errorf("Expected peer2 drops count 0, got %d", peer2Metrics.DropsCount)
	}
}

// TestGRPCPeerLink_EventStreamBasic tests basic EventStream functionality
func TestGRPCPeerLink_EventStreamBasic(t *testing.T) {
	config := &Config{
		NodeID:        "test-node",
		ListenAddress: "localhost:0",
		SendQueueSize: 10,
		SendTimeout:   1 * time.Second,
	}

	peerLink, err := NewGRPCPeerLink(config)
	if err != nil {
		t.Fatalf("Failed to create GRPCPeerLink: %v", err)
	}
	defer peerLink.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Start the gRPC server
	err = peerLink.Start(ctx)
	if err != nil {
		t.Fatalf("Failed to start PeerLink: %v", err)
	}
	defer peerLink.Stop(ctx)

	// Connect a test peer to create the send queue
	testPeer := &simplePeerNode{id: "peer1", address: "localhost:8080", healthy: true}
	err = peerLink.Connect(ctx, testPeer)
	if err != nil {
		t.Fatalf("Failed to connect peer: %v", err)
	}

	// Send an event - this should queue the message
	testEvent := eventlog.NewRecord("test-topic", []byte("test-payload"))
	err = peerLink.SendEvent(ctx, "peer1", testEvent)
	if err != nil {
		t.Fatalf("Failed to send event: %v", err)
	}

	// Verify the event was queued
	queueDepth := peerLink.GetQueueDepth("peer1")
	if queueDepth != 1 {
		t.Errorf("Expected queue depth 1, got %d", queueDepth)
	}

	// TODO: This test will fail until we implement EventStream properly
	// The EventStream should consume from the queue and send over gRPC

	// Try to create a gRPC client connection to test EventStream
	// This will fail because EventStream is not implemented yet
	conn, err := grpc.Dial(peerLink.listener.Addr().String(), grpc.WithInsecure())
	if err != nil {
		t.Fatalf("Failed to create gRPC client: %v", err)
	}
	defer conn.Close()

	client := peerlinkv1.NewPeerLinkClient(conn)

	// Try to establish EventStream - this should eventually work but will fail now
	stream, err := client.EventStream(ctx)
	if err != nil {
		t.Fatalf("Failed to create EventStream: %v", err)
	}
	defer stream.CloseSend()

	// Send a handshake message
	handshake := &peerlinkv1.PeerMessage{
		Kind: &peerlinkv1.PeerMessage_Handshake{
			Handshake: &peerlinkv1.Handshake{
				NodeId:          "test-client",
				ProtocolVersion: 100,
			},
		},
	}

	err = stream.Send(handshake)
	if err != nil {
		t.Fatalf("Failed to send handshake: %v", err)
	}

	// Try to receive handshake response - this should now work with basic EventStream
	response, err := stream.Recv()
	if err != nil {
		t.Fatalf("Failed to receive handshake response: %v", err)
	}

	// Verify handshake response
	responseHandshake := response.GetHandshake()
	if responseHandshake == nil {
		t.Error("Expected handshake response")
	} else {
		if responseHandshake.NodeId != "test-node" {
			t.Errorf("Expected node ID 'test-node', got '%s'", responseHandshake.NodeId)
		}
		if responseHandshake.ProtocolVersion != 100 {
			t.Errorf("Expected protocol version 100, got %d", responseHandshake.ProtocolVersion)
		}
		t.Logf("Handshake successful: node=%s, version=%d",
			responseHandshake.NodeId, responseHandshake.ProtocolVersion)
	}

	// Now test that the EventStream consumes from send queues
	// First, verify the peer was registered during handshake
	if peerLink.GetQueueDepth("test-client") != 0 {
		t.Errorf("Expected queue depth 0 after handshake, got %d", peerLink.GetQueueDepth("test-client"))
	}

	// Send an event to the peer
	testEvent2 := eventlog.NewRecord("test-topic", []byte("test-payload"))
	err2 := peerLink.SendEvent(ctx, "test-client", testEvent2)
	if err2 != nil {
		t.Fatalf("Failed to send event: %v", err2)
	}

	// The EventStream should consume the message and send it over gRPC quickly
	eventMsg, err := stream.Recv()
	if err != nil {
		t.Fatalf("Failed to receive event message: %v", err)
	}

	// Verify it's an event message
	event := eventMsg.GetEvent()
	if event == nil {
		t.Error("Expected event message")
	} else {
		if event.Topic != "test-topic" {
			t.Errorf("Expected topic 'test-topic', got '%s'", event.Topic)
		}
		if string(event.Payload) != "test-payload" {
			t.Errorf("Expected payload 'test-payload', got '%s'", string(event.Payload))
		}
		t.Logf("Event received: topic=%s, payload=%s", event.Topic, string(event.Payload))
	}

	// Verify the message was consumed from the queue (should be 0 now)
	if peerLink.GetQueueDepth("test-client") != 0 {
		t.Errorf("Expected queue depth 0 after consumption, got %d", peerLink.GetQueueDepth("test-client"))
	}

	// Close the stream properly to end the test
	if err := stream.CloseSend(); err != nil {
		t.Logf("Error closing send stream: %v", err)
	}

	// Cancel the context to terminate the EventStream goroutine
	cancel()
}

// TestGRPCPeerLink_EventStreamHandshakeValidation tests handshake validation errors
func TestGRPCPeerLink_EventStreamHandshakeValidation(t *testing.T) {
	config := &Config{
		NodeID:        "test-node",
		ListenAddress: "localhost:0",
	}

	peerLink, err := NewGRPCPeerLink(config)
	if err != nil {
		t.Fatalf("Failed to create GRPCPeerLink: %v", err)
	}
	defer peerLink.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	// Start the gRPC server
	err = peerLink.Start(ctx)
	if err != nil {
		t.Fatalf("Failed to start PeerLink: %v", err)
	}
	defer peerLink.Stop(ctx)

	// Create gRPC client
	conn, err := grpc.Dial(peerLink.listener.Addr().String(), grpc.WithInsecure())
	if err != nil {
		t.Fatalf("Failed to create gRPC client: %v", err)
	}
	defer conn.Close()

	client := peerlinkv1.NewPeerLinkClient(conn)

	t.Run("missing handshake message", func(t *testing.T) {
		stream, err := client.EventStream(ctx)
		if err != nil {
			t.Fatalf("Failed to create EventStream: %v", err)
		}
		defer stream.CloseSend()

		// Send non-handshake message first
		eventMsg := &peerlinkv1.PeerMessage{
			Kind: &peerlinkv1.PeerMessage_Event{
				Event: &peerlinkv1.Event{
					Topic:   "test",
					Payload: []byte("should fail"),
				},
			},
		}

		err = stream.Send(eventMsg)
		if err != nil {
			t.Fatalf("Failed to send event message: %v", err)
		}

		// Should get error response
		_, err = stream.Recv()
		if err == nil {
			t.Error("Expected error for missing handshake, got none")
		}
		if !strings.Contains(err.Error(), "expected handshake message") {
			t.Errorf("Expected 'expected handshake message' error, got: %v", err)
		}
	})

	t.Run("empty peer ID", func(t *testing.T) {
		stream, err := client.EventStream(ctx)
		if err != nil {
			t.Fatalf("Failed to create EventStream: %v", err)
		}
		defer stream.CloseSend()

		// Send handshake with empty node ID
		handshake := &peerlinkv1.PeerMessage{
			Kind: &peerlinkv1.PeerMessage_Handshake{
				Handshake: &peerlinkv1.Handshake{
					NodeId:          "", // Empty node ID
					ProtocolVersion: ProtocolVersion,
				},
			},
		}

		err = stream.Send(handshake)
		if err != nil {
			t.Fatalf("Failed to send handshake: %v", err)
		}

		// Should get error response
		_, err = stream.Recv()
		if err == nil {
			t.Error("Expected error for empty peer ID, got none")
		}
		if !strings.Contains(err.Error(), "peer ID cannot be empty") {
			t.Errorf("Expected 'peer ID cannot be empty' error, got: %v", err)
		}
	})
}

// TestGRPCPeerLink_EventStreamContextCancellation tests proper cleanup on context cancellation
func TestGRPCPeerLink_EventStreamContextCancellation(t *testing.T) {
	config := &Config{
		NodeID:        "test-node",
		ListenAddress: "localhost:0",
	}

	peerLink, err := NewGRPCPeerLink(config)
	if err != nil {
		t.Fatalf("Failed to create GRPCPeerLink: %v", err)
	}
	defer peerLink.Close()

	// Use a short-lived context that we can cancel
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// Start the gRPC server
	err = peerLink.Start(ctx)
	if err != nil {
		t.Fatalf("Failed to start PeerLink: %v", err)
	}
	defer peerLink.Stop(ctx)

	// Create gRPC client
	conn, err := grpc.Dial(peerLink.listener.Addr().String(), grpc.WithInsecure())
	if err != nil {
		t.Fatalf("Failed to create gRPC client: %v", err)
	}
	defer conn.Close()

	client := peerlinkv1.NewPeerLinkClient(conn)

	// Establish EventStream
	stream, err := client.EventStream(ctx)
	if err != nil {
		t.Fatalf("Failed to create EventStream: %v", err)
	}
	defer stream.CloseSend()

	// Send handshake
	handshake := &peerlinkv1.PeerMessage{
		Kind: &peerlinkv1.PeerMessage_Handshake{
			Handshake: &peerlinkv1.Handshake{
				NodeId:          "test-client",
				ProtocolVersion: ProtocolVersion,
			},
		},
	}

	err = stream.Send(handshake)
	if err != nil {
		t.Fatalf("Failed to send handshake: %v", err)
	}

	// Receive handshake response
	_, err = stream.Recv()
	if err != nil {
		t.Fatalf("Failed to receive handshake response: %v", err)
	}

	// Verify peer is registered and healthy
	healthState, err := peerLink.GetPeerHealth(ctx, "test-client")
	if err != nil {
		t.Fatalf("Failed to get peer health: %v", err)
	}
	if healthState != peerlink.PeerHealthy {
		t.Errorf("Expected healthy peer, got %v", healthState)
	}

	// Cancel the context - this should cause EventStream to exit gracefully
	cancel()

	// Give the EventStream a moment to process the cancellation
	time.Sleep(100 * time.Millisecond)

	// Verify peer health was updated to Disconnected
	// We need a new context since the old one was cancelled
	newCtx := context.Background()
	healthState, err = peerLink.GetPeerHealth(newCtx, "test-client")
	if err != nil {
		t.Fatalf("Failed to get peer health after cancellation: %v", err)
	}
	if healthState != peerlink.PeerDisconnected {
		t.Errorf("Expected disconnected peer after context cancellation, got %v", healthState)
	}

	t.Logf("Context cancellation handled correctly - peer marked as disconnected")
}