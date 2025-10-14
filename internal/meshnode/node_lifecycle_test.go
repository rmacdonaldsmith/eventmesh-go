package meshnode

import (
	"context"
	"strings"
	"testing"
	"time"
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
