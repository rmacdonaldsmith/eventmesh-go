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

	// Test creating an Event message with headers (new schema)
	event := &peerlinkv1.Event{
		Topic:   "test-topic",
		Offset:  42,
		Payload: []byte("test-payload"),
		Headers: map[string]string{
			"source": "test",
			"type":   "example",
		},
	}
	if event.Topic != "test-topic" {
		t.Errorf("Expected topic 'test-topic', got %s", event.Topic)
	}
	if event.Offset != 42 {
		t.Errorf("Expected offset 42, got %d", event.Offset)
	}
	if len(event.Headers) != 2 {
		t.Errorf("Expected 2 headers, got %d", len(event.Headers))
	}

	// Test creating an Ack message (topic-scoped)
	ack := &peerlinkv1.Ack{
		Topic:  "test-topic",
		Offset: 41,
	}
	if ack.Topic != "test-topic" {
		t.Errorf("Expected topic 'test-topic', got %s", ack.Topic)
	}

	// Test creating a Heartbeat message (updated field name)
	heartbeat := &peerlinkv1.Heartbeat{
		Timestamp: 1633024800000,
	}
	if heartbeat.Timestamp != 1633024800000 {
		t.Errorf("Expected timestamp 1633024800000, got %d", heartbeat.Timestamp)
	}

	// Test creating wrapped PeerMessages
	eventMsg := &peerlinkv1.PeerMessage{
		Kind: &peerlinkv1.PeerMessage_Event{
			Event: event,
		},
	}
	if eventMsg.GetEvent() == nil {
		t.Error("Expected event message to be set")
	}

	ackMsg := &peerlinkv1.PeerMessage{
		Kind: &peerlinkv1.PeerMessage_Ack{
			Ack: ack,
		},
	}
	if ackMsg.GetAck() == nil {
		t.Error("Expected ack message to be set")
	}

	heartbeatMsg := &peerlinkv1.PeerMessage{
		Kind: &peerlinkv1.PeerMessage_Heartbeat{
			Heartbeat: heartbeat,
		},
	}
	if heartbeatMsg.GetHeartbeat() == nil {
		t.Error("Expected heartbeat message to be set")
	}

	handshakeMsg := &peerlinkv1.PeerMessage{
		Kind: &peerlinkv1.PeerMessage_Handshake{
			Handshake: handshake,
		},
	}
	if handshakeMsg.GetHandshake() == nil {
		t.Error("Expected handshake message to be set")
	}
}