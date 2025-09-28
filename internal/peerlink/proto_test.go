package peerlink

import (
	"testing"

	peerlinkv1 "github.com/rmacdonaldsmith/eventmesh-go/proto/peerlink/v1"
)

// TestProtoImport verifies that generated protobuf code compiles and imports correctly
func TestProtoImport(t *testing.T) {
	// This test simply verifies that our generated protobuf code compiles
	// and that we can reference the generated types
	_ = &peerlinkv1.PeerMessage{}
	_ = &peerlinkv1.Handshake{}
	_ = &peerlinkv1.Event{}
	_ = &peerlinkv1.Ack{}
	_ = &peerlinkv1.Heartbeat{}
}

// TestMessageCreation verifies that we can create protobuf messages
func TestMessageCreation(t *testing.T) {
	// Test creating a Handshake message
	handshake := &peerlinkv1.Handshake{
		NodeId:          "test-node",
		ProtocolVersion: 100, // v1.0
		Features:        []string{"basic"},
	}
	if handshake.NodeId != "test-node" {
		t.Errorf("Expected NodeId 'test-node', got %s", handshake.NodeId)
	}

	// Test creating a wrapped PeerMessage
	peerMsg := &peerlinkv1.PeerMessage{
		Kind: &peerlinkv1.PeerMessage_Handshake{
			Handshake: handshake,
		},
	}
	if peerMsg.GetHandshake() == nil {
		t.Error("Expected handshake message to be set")
	}
}