package peerlink

import (
	"testing"

	peerlinkv1 "github.com/rmacdonaldsmith/eventmesh-go/proto/peerlink/v1"
)

// TestProtoPackage verifies that the protobuf package is correctly generated
func TestProtoPackage(t *testing.T) {
	_ = &peerlinkv1.Event{}
	_ = &peerlinkv1.Handshake{}
	_ = &peerlinkv1.PeerMessage{}
	_ = &peerlinkv1.ControlPlaneMessage{}
	_ = &peerlinkv1.InterestUpdate{}
	_ = &peerlinkv1.InterestSnapshot{}
}

func TestEventMessage(t *testing.T) {
	event := &peerlinkv1.Event{
		Topic:   "test-topic",
		Offset:  42,
		Payload: []byte("test-payload"),
		Headers: map[string]string{"key": "value"},
	}
	if event.Topic != "test-topic" {
		t.Errorf("Expected topic 'test-topic', got %s", event.Topic)
	}
	if event.Offset != 42 {
		t.Errorf("Expected offset 42, got %d", event.Offset)
	}
	if string(event.Payload) != "test-payload" {
		t.Errorf("Expected payload 'test-payload', got %s", string(event.Payload))
	}
	if event.Headers["key"] != "value" {
		t.Errorf("Expected header key=value, got %s", event.Headers["key"])
	}
}

func TestAckMessage(t *testing.T) {
	ack := &peerlinkv1.Ack{Topic: "test-topic", Offset: 42}
	if ack.Topic != "test-topic" {
		t.Errorf("Expected topic 'test-topic', got %s", ack.Topic)
	}
	if ack.Offset != 42 {
		t.Errorf("Expected offset 42, got %d", ack.Offset)
	}
}

func TestHandshakeMessage(t *testing.T) {
	handshake := &peerlinkv1.Handshake{
		NodeId:          "node-123",
		ProtocolVersion: 100,
		Features:        []string{"feature1", "feature2"},
	}
	if handshake.NodeId != "node-123" {
		t.Errorf("Expected node_id 'node-123', got %s", handshake.NodeId)
	}
	if handshake.ProtocolVersion != 100 {
		t.Errorf("Expected protocol version 100, got %d", handshake.ProtocolVersion)
	}
	if len(handshake.Features) != 2 {
		t.Errorf("Expected 2 features, got %d", len(handshake.Features))
	}
}

func TestPeerMessageOneof(t *testing.T) {
	event := &peerlinkv1.Event{Topic: "test", Payload: []byte("data")}
	eventMsg := &peerlinkv1.PeerMessage{Kind: &peerlinkv1.PeerMessage_Event{Event: event}}
	if eventMsg.GetEvent() == nil {
		t.Error("Expected event message to be set")
	}

	ack := &peerlinkv1.Ack{Topic: "test", Offset: 1}
	ackMsg := &peerlinkv1.PeerMessage{Kind: &peerlinkv1.PeerMessage_Ack{Ack: ack}}
	if ackMsg.GetAck() == nil {
		t.Error("Expected ack message to be set")
	}

	handshake := &peerlinkv1.Handshake{NodeId: "node-1"}
	handshakeMsg := &peerlinkv1.PeerMessage{Kind: &peerlinkv1.PeerMessage_Handshake{Handshake: handshake}}
	if handshakeMsg.GetHandshake() == nil {
		t.Error("Expected handshake message to be set")
	}
}

func TestControlPlaneMessage(t *testing.T) {
	update := &peerlinkv1.InterestUpdate{Action: "subscribe", Topic: "orders.*", NodeId: "node-456"}
	controlUpdate := &peerlinkv1.ControlPlaneMessage{Kind: &peerlinkv1.ControlPlaneMessage_InterestUpdate{InterestUpdate: update}}
	if controlUpdate.GetInterestUpdate() == nil {
		t.Fatal("Expected interest update to be set")
	}
	if controlUpdate.GetInterestUpdate().Action != "subscribe" {
		t.Fatal("Expected interest update action to be accessible")
	}

	snapshot := &peerlinkv1.InterestSnapshot{NodeId: "node-456", Topics: []string{"orders.*", "payments.*"}}
	controlSnapshot := &peerlinkv1.ControlPlaneMessage{Kind: &peerlinkv1.ControlPlaneMessage_InterestSnapshot{InterestSnapshot: snapshot}}
	if controlSnapshot.GetInterestSnapshot() == nil {
		t.Fatal("Expected interest snapshot to be set")
	}
	if len(controlSnapshot.GetInterestSnapshot().Topics) != 2 {
		t.Fatal("Expected snapshot topics to be accessible")
	}
}

func TestDataControlPlaneSeparation(t *testing.T) {
	userEvent := &peerlinkv1.Event{Topic: "user.events", Offset: 1, Payload: []byte(`{"user_id": 123}`)}
	dataPlaneMsg := &peerlinkv1.PeerMessage{Kind: &peerlinkv1.PeerMessage_Event{Event: userEvent}}

	interestUpdate := &peerlinkv1.InterestUpdate{Action: "unsubscribe", Topic: "user.*", NodeId: "node-abc"}
	controlPlaneMsg := &peerlinkv1.PeerMessage{Kind: &peerlinkv1.PeerMessage_ControlPlane{ControlPlane: &peerlinkv1.ControlPlaneMessage{Kind: &peerlinkv1.ControlPlaneMessage_InterestUpdate{InterestUpdate: interestUpdate}}}}

	if dataPlaneMsg.GetEvent() == nil {
		t.Error("Data plane message should contain an event")
	}
	if dataPlaneMsg.GetControlPlane() != nil {
		t.Error("Data plane message should not contain control plane data")
	}
	if controlPlaneMsg.GetControlPlane() == nil {
		t.Error("Control plane message should contain control plane data")
	}
	if controlPlaneMsg.GetEvent() != nil {
		t.Error("Control plane message should not contain user events")
	}
	if controlPlaneMsg.GetControlPlane().GetInterestUpdate().Topic != "user.*" {
		t.Error("Control plane interest updates should support topic patterns")
	}
}

func TestPeerLinkServiceUsesSingleBidirectionalStream(t *testing.T) {
	service := peerlinkv1.PeerLink_ServiceDesc
	if service.ServiceName != "peerlink.v1.PeerLink" {
		t.Fatalf("expected PeerLink service name, got %q", service.ServiceName)
	}
	if len(service.Methods) != 0 {
		t.Fatalf("expected no unary PeerLink RPCs, got %d", len(service.Methods))
	}
	if len(service.Streams) != 1 {
		t.Fatalf("expected one physical PeerLink stream, got %d", len(service.Streams))
	}
	stream := service.Streams[0]
	if stream.StreamName != "EventStream" {
		t.Fatalf("expected EventStream to be the single physical stream, got %q", stream.StreamName)
	}
	if !stream.ClientStreams || !stream.ServerStreams {
		t.Fatalf("expected EventStream to be bidirectional, got client=%t server=%t", stream.ClientStreams, stream.ServerStreams)
	}
}

func TestInterestUpdateValidation(t *testing.T) {
	testCases := []struct {
		name        string
		action      string
		topic       string
		nodeID      string
		expectValid bool
	}{
		{name: "valid subscribe", action: "subscribe", topic: "orders.*", nodeID: "node-1", expectValid: true},
		{name: "valid unsubscribe", action: "unsubscribe", topic: "users.created", nodeID: "node-2", expectValid: true},
		{name: "empty action", action: "", topic: "test", nodeID: "node-3", expectValid: false},
		{name: "empty topic", action: "subscribe", topic: "", nodeID: "node-5", expectValid: false},
		{name: "empty node_id", action: "subscribe", topic: "test", nodeID: "", expectValid: false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			update := &peerlinkv1.InterestUpdate{Action: tc.action, Topic: tc.topic, NodeId: tc.nodeID}
			validActions := update.Action == "subscribe" || update.Action == "unsubscribe"
			isValid := validActions && update.Topic != "" && update.NodeId != ""
			if isValid != tc.expectValid {
				t.Fatalf("Expected valid=%t, got %t for %#v", tc.expectValid, isValid, update)
			}
		})
	}
}
