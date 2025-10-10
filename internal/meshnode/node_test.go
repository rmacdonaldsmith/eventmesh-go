package meshnode

import (
	"context"
	"testing"
	"time"

	"github.com/rmacdonaldsmith/eventmesh-go/internal/peerlink"
)

// TestNewGRPCMeshNode tests creating a new GRPCMeshNode
func TestNewGRPCMeshNode(t *testing.T) {
	config := NewConfig("test-node", "localhost:8080")

	node, err := NewGRPCMeshNode(config)
	if err != nil {
		t.Fatalf("Expected no error creating mesh node, got %v", err)
	}

	if node == nil {
		t.Fatal("Expected non-nil mesh node")
	}

	// Verify configuration is stored
	if node.config != config {
		t.Error("Expected config to be stored in node")
	}

	// Verify components are initialized
	if node.eventLog == nil {
		t.Error("Expected EventLog to be initialized")
	}
	if node.routingTable == nil {
		t.Error("Expected RoutingTable to be initialized")
	}
	if node.peerLink == nil {
		t.Error("Expected PeerLink to be initialized")
	}

	// Verify initial state
	if node.started {
		t.Error("Expected node to not be started initially")
	}
	if node.closed {
		t.Error("Expected node to not be closed initially")
	}

	// Verify clients map is initialized
	if node.clients == nil {
		t.Error("Expected clients map to be initialized")
	}
	if len(node.clients) != 0 {
		t.Error("Expected clients map to be empty initially")
	}
}

// TestNewGRPCMeshNode_NilConfig tests creating node with nil config
func TestNewGRPCMeshNode_NilConfig(t *testing.T) {
	node, err := NewGRPCMeshNode(nil)
	if err == nil {
		t.Error("Expected error when creating node with nil config")
	}
	if node != nil {
		t.Error("Expected nil node when config is nil")
	}
}

// TestNewGRPCMeshNode_InvalidConfig tests creating node with invalid config
func TestNewGRPCMeshNode_InvalidConfig(t *testing.T) {
	invalidConfig := NewConfig("", "localhost:8080") // Empty node ID

	node, err := NewGRPCMeshNode(invalidConfig)
	if err == nil {
		t.Error("Expected error when creating node with invalid config")
	}
	if node != nil {
		t.Error("Expected nil node when config is invalid")
	}
}

// TestNewGRPCMeshNode_WithPeerLinkConfig tests creating node with custom PeerLink config
func TestNewGRPCMeshNode_WithPeerLinkConfig(t *testing.T) {
	peerLinkConfig := &peerlink.Config{
		NodeID:        "test-node",
		ListenAddress: "localhost:9090",
		SendQueueSize: 200, // Custom value
	}
	peerLinkConfig.SetDefaults()

	config := NewConfig("test-node", "localhost:8080").WithPeerLinkConfig(peerLinkConfig)

	node, err := NewGRPCMeshNode(config)
	if err != nil {
		t.Fatalf("Expected no error creating mesh node with custom PeerLink config, got %v", err)
	}

	if node == nil {
		t.Fatal("Expected non-nil mesh node")
	}

	// The actual PeerLink config verification would be internal to PeerLink component
	// We just verify the node was created successfully
}

// TestGRPCMeshNode_InterfaceCompliance tests that GRPCMeshNode implements MeshNode interface
func TestGRPCMeshNode_InterfaceCompliance(t *testing.T) {
	config := NewConfig("test-node", "localhost:8080")
	node, err := NewGRPCMeshNode(config)
	if err != nil {
		t.Fatalf("Expected no error creating mesh node, got %v", err)
	}
	defer node.Close()

	ctx := context.Background()

	// Test GetNodeID
	nodeID := node.GetNodeID()
	if nodeID != "test-node" {
		t.Errorf("Expected node ID 'test-node', got '%s'", nodeID)
	}

	// Test GetEventLog
	eventLog := node.GetEventLog()
	if eventLog == nil {
		t.Error("Expected non-nil EventLog from GetEventLog()")
	}

	// Test GetRoutingTable
	routingTable := node.GetRoutingTable()
	if routingTable == nil {
		t.Error("Expected non-nil RoutingTable from GetRoutingTable()")
	}

	// Test GetPeerLink
	peerLink := node.GetPeerLink()
	if peerLink == nil {
		t.Error("Expected non-nil PeerLink from GetPeerLink()")
	}

	// Test GetConnectedClients (stub implementation)
	clients, err := node.GetConnectedClients(ctx)
	if err != nil {
		t.Errorf("Expected no error from GetConnectedClients(), got %v", err)
	}
	if clients == nil {
		t.Error("Expected non-nil clients slice")
	}
	if len(clients) != 0 {
		t.Errorf("Expected 0 connected clients initially, got %d", len(clients))
	}

	// Test GetConnectedPeers
	peers, err := node.GetConnectedPeers(ctx)
	if err != nil {
		t.Errorf("Expected no error from GetConnectedPeers(), got %v", err)
	}
	if peers == nil {
		t.Error("Expected non-nil peers slice")
	}

	// Test GetHealth (stub implementation)
	health, err := node.GetHealth(ctx)
	if err != nil {
		t.Errorf("Expected no error from GetHealth(), got %v", err)
	}
	if !health.Healthy {
		t.Error("Expected node to be healthy initially")
	}
}

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
		t.Error("Expected node to not be closed after Stop() (only stopped)")
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

	// Close the node
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

// TestGRPCMeshNode_StubMethods tests the stub implementations
func TestGRPCMeshNode_StubMethods(t *testing.T) {
	config := NewConfig("test-node", "localhost:8083")
	node, err := NewGRPCMeshNode(config)
	if err != nil {
		t.Fatalf("Expected no error creating mesh node, got %v", err)
	}
	defer node.Close()

	ctx := context.Background()

	// Test PublishEvent stub
	err = node.PublishEvent(ctx, nil, nil)
	if err == nil {
		t.Error("Expected error from PublishEvent stub")
	}

	// Test Subscribe stub
	err = node.Subscribe(ctx, nil, "test-topic")
	if err == nil {
		t.Error("Expected error from Subscribe stub")
	}

	// Test Unsubscribe stub
	err = node.Unsubscribe(ctx, nil, "test-topic")
	if err == nil {
		t.Error("Expected error from Unsubscribe stub")
	}

	// Test AuthenticateClient stub
	client, err := node.AuthenticateClient(ctx, nil)
	if err == nil {
		t.Error("Expected error from AuthenticateClient stub")
	}
	if client != nil {
		t.Error("Expected nil client from AuthenticateClient stub")
	}
}