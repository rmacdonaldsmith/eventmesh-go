package discovery

import (
	"context"
	"testing"
)

// TestDiscoveryInterface_FindPeers tests the discovery interface contract
func TestDiscoveryInterface_FindPeers(t *testing.T) {
	// Test that we can create a discovery service
	seedNodes := []string{"node1:8080", "node2:8080"}
	discovery := NewStaticDiscovery(seedNodes)

	if discovery == nil {
		t.Fatal("Expected discovery service to be created, got nil")
	}

	// Test FindPeers method
	ctx := context.Background()
	peers, err := discovery.FindPeers(ctx)

	if err != nil {
		t.Errorf("Expected no error from FindPeers, got %v", err)
	}

	if len(peers) != 2 {
		t.Errorf("Expected 2 peers, got %d", len(peers))
	}

	// Verify first peer
	if peers[0].ID() != "node1:8080" {
		t.Errorf("Expected first peer ID 'node1:8080', got '%s'", peers[0].ID())
	}
	if peers[0].Address() != "node1:8080" {
		t.Errorf("Expected first peer address 'node1:8080', got '%s'", peers[0].Address())
	}

	// Verify second peer
	if peers[1].ID() != "node2:8080" {
		t.Errorf("Expected second peer ID 'node2:8080', got '%s'", peers[1].ID())
	}
	if peers[1].Address() != "node2:8080" {
		t.Errorf("Expected second peer address 'node2:8080', got '%s'", peers[1].Address())
	}
}

// TestDiscoveryInterface_EmptySeedNodes tests discovery with empty seed nodes
func TestDiscoveryInterface_EmptySeedNodes(t *testing.T) {
	discovery := NewStaticDiscovery([]string{})

	ctx := context.Background()
	peers, err := discovery.FindPeers(ctx)

	if err != nil {
		t.Errorf("Expected no error from FindPeers with empty seeds, got %v", err)
	}

	if len(peers) != 0 {
		t.Errorf("Expected 0 peers with empty seed nodes, got %d", len(peers))
	}
}

// TestDiscoveryInterface_InterfaceCompliance tests that StaticDiscovery implements Discovery
func TestDiscoveryInterface_InterfaceCompliance(t *testing.T) {
	var _ Discovery = (*StaticDiscovery)(nil)
}
