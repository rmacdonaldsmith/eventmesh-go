package peerlink

import (
	"testing"

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