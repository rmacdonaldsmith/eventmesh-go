package meshnode

import (
	"context"
	"testing"

	"github.com/rmacdonaldsmith/eventmesh-go/internal/peerlink"
)

// TestNewGRPCMeshNode tests creating a new GRPCMeshNode
func TestNewGRPCMeshNode(t *testing.T) {
	config := NewConfig("test-node", "localhost:8080")
	node, err := NewGRPCMeshNode(config)
	if err != nil {
		t.Fatalf("Expected no error creating mesh node, got %v", err)
	}
	defer node.Close()

	// Verify the node was created with correct configuration
	if node.config.NodeID != "test-node" {
		t.Errorf("Expected NodeID 'test-node', got '%s'", node.config.NodeID)
	}
	if node.config.ListenAddress != "localhost:8080" {
		t.Errorf("Expected ListenAddress 'localhost:8080', got '%s'", node.config.ListenAddress)
	}

	// Verify components were created
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
	if node.clients == nil {
		t.Error("Expected clients map to be initialized")
	}
	if len(node.clients) != 0 {
		t.Errorf("Expected empty clients map initially, got %d clients", len(node.clients))
	}
}

// TestNewGRPCMeshNode_NilConfig tests creating mesh node with nil config
func TestNewGRPCMeshNode_NilConfig(t *testing.T) {
	_, err := NewGRPCMeshNode(nil)
	if err == nil {
		t.Error("Expected error when creating mesh node with nil config")
	}
}

// TestNewGRPCMeshNode_InvalidConfig tests creating mesh node with invalid config
func TestNewGRPCMeshNode_InvalidConfig(t *testing.T) {
	// Empty NodeID should fail validation
	config := NewConfig("", "localhost:8080")
	_, err := NewGRPCMeshNode(config)
	if err == nil {
		t.Error("Expected error when creating mesh node with empty NodeID")
	}
}

// TestNewGRPCMeshNode_WithPeerLinkConfig tests creating mesh node with PeerLink config
func TestNewGRPCMeshNode_WithPeerLinkConfig(t *testing.T) {
	// Create config with custom PeerLink configuration
	peerLinkConfig := &peerlink.Config{
		NodeID:        "test-peer-node",
		ListenAddress: "localhost:9090",
	}
	peerLinkConfig.SetDefaults()

	config := NewConfig("test-node", "localhost:8080").WithPeerLinkConfig(peerLinkConfig)
	node, err := NewGRPCMeshNode(config)
	if err != nil {
		t.Fatalf("Expected no error creating mesh node with PeerLink config, got %v", err)
	}
	defer node.Close()

	// Verify the PeerLink was created with custom config
	if node.peerLink == nil {
		t.Error("Expected PeerLink to be initialized with custom config")
	}
}

// TestGRPCMeshNode_InterfaceCompliance tests that GRPCMeshNode implements required interfaces
func TestGRPCMeshNode_InterfaceCompliance(t *testing.T) {
	config := NewConfig("interface-test-node", "localhost:8080")
	node, err := NewGRPCMeshNode(config)
	if err != nil {
		t.Fatalf("Expected no error creating mesh node, got %v", err)
	}
	defer node.Close()

	ctx := context.Background()

	// Test MeshNode interface methods exist and can be called
	_ = node.GetNodeID()
	_ = node.GetEventLog()
	_ = node.GetRoutingTable()
	_ = node.GetPeerLink()

	_, err = node.GetConnectedClients(ctx)
	if err != nil {
		t.Errorf("Expected no error from GetConnectedClients(), got %v", err)
	}

	peers, err := node.GetConnectedPeers(ctx)
	if err != nil {
		t.Errorf("Expected no error from GetConnectedPeers(), got %v", err)
	}
	if peers == nil {
		t.Error("Expected non-nil peers slice")
	}

	// Test GetHealth (implementation)
	health, err := node.GetHealth(ctx)
	if err != nil {
		t.Errorf("Expected no error from GetHealth(), got %v", err)
	}
	if !health.Healthy {
		t.Error("Expected node to be healthy initially")
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

	// Note: AuthenticateClient is no longer a stub - tested separately
}

// TestGRPCMeshNode_AuthenticateClient tests the AuthenticateClient implementation
func TestGRPCMeshNode_AuthenticateClient(t *testing.T) {
	config := NewConfig("test-node", "localhost:8083")
	node, err := NewGRPCMeshNode(config)
	if err != nil {
		t.Fatalf("Expected no error creating mesh node, got %v", err)
	}
	defer node.Close()

	ctx := context.Background()

	// Test valid client authentication
	client1, err := node.AuthenticateClient(ctx, "client-1")
	if err != nil {
		t.Errorf("Expected no error for valid client ID, got %v", err)
	}
	if client1 == nil {
		t.Error("Expected non-nil client for valid authentication")
	}
	if client1.ID() != "client-1" {
		t.Errorf("Expected client ID 'client-1', got '%s'", client1.ID())
	}
	if !client1.IsAuthenticated() {
		t.Error("Expected client to be authenticated")
	}

	// Test client is added to tracking
	clients, err := node.GetConnectedClients(ctx)
	if err != nil {
		t.Errorf("Expected no error getting connected clients, got %v", err)
	}
	if len(clients) != 1 {
		t.Errorf("Expected 1 connected client, got %d", len(clients))
	}

	// Test authenticating same client returns existing instance
	client1Again, err := node.AuthenticateClient(ctx, "client-1")
	if err != nil {
		t.Errorf("Expected no error for duplicate client ID, got %v", err)
	}
	if client1Again != client1 {
		t.Error("Expected same client instance for duplicate authentication")
	}

	// Test second unique client
	client2, err := node.AuthenticateClient(ctx, "client-2")
	if err != nil {
		t.Errorf("Expected no error for second valid client ID, got %v", err)
	}
	if client2 == nil {
		t.Error("Expected non-nil client for valid authentication")
	}
	if client2.ID() != "client-2" {
		t.Errorf("Expected client ID 'client-2', got '%s'", client2.ID())
	}

	// Verify both clients are tracked
	clients, err = node.GetConnectedClients(ctx)
	if err != nil {
		t.Errorf("Expected no error getting connected clients, got %v", err)
	}
	if len(clients) != 2 {
		t.Errorf("Expected 2 connected clients, got %d", len(clients))
	}

	// Test nil credentials
	_, err = node.AuthenticateClient(ctx, nil)
	if err == nil {
		t.Error("Expected error for nil credentials")
	}

	// Test empty client ID
	_, err = node.AuthenticateClient(ctx, "")
	if err == nil {
		t.Error("Expected error for empty client ID")
	}

	// Test non-string credentials
	_, err = node.AuthenticateClient(ctx, 123)
	if err == nil {
		t.Error("Expected error for non-string credentials")
	}

	// Test authentication on closed node
	node.Close()
	_, err = node.AuthenticateClient(ctx, "client-3")
	if err == nil {
		t.Error("Expected error for authentication on closed node")
	}
}
