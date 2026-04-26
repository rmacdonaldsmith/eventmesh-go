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
	_ = &peerlinkv1.ControlPlaneMessage{} // New control plane message type
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

// TestControlPlaneMessage verifies that we can create and use ControlPlaneMessage
func TestControlPlaneMessage(t *testing.T) {
	// Test creating a SubscriptionChange message (part of control plane)
	subscriptionChange := &peerlinkv1.SubscriptionChange{
		Action:   "subscribe",
		ClientId: "client-123",
		Topic:    "orders.*",
		NodeId:   "node-456",
	}

	if subscriptionChange.Action != "subscribe" {
		t.Errorf("Expected action 'subscribe', got %s", subscriptionChange.Action)
	}
	if subscriptionChange.ClientId != "client-123" {
		t.Errorf("Expected client_id 'client-123', got %s", subscriptionChange.ClientId)
	}
	if subscriptionChange.Topic != "orders.*" {
		t.Errorf("Expected topic 'orders.*', got %s", subscriptionChange.Topic)
	}
	if subscriptionChange.NodeId != "node-456" {
		t.Errorf("Expected node_id 'node-456', got %s", subscriptionChange.NodeId)
	}

	// Test creating a wrapped ControlPlaneMessage
	controlMsg := &peerlinkv1.ControlPlaneMessage{
		Kind: &peerlinkv1.ControlPlaneMessage_SubscriptionChange{
			SubscriptionChange: subscriptionChange,
		},
	}

	if controlMsg.GetSubscriptionChange() == nil {
		t.Error("Expected subscription change to be set")
	}

	// Verify we can access the nested data
	if controlMsg.GetSubscriptionChange().Action != "subscribe" {
		t.Error("Expected subscription change action to be accessible")
	}
}

// TestDataControlPlaneSeparation tests that data and control plane messages are clearly separated
func TestDataControlPlaneSeparation(t *testing.T) {
	// Create a data plane message (user event)
	userEvent := &peerlinkv1.Event{
		Topic:   "user.events",
		Offset:  1,
		Payload: []byte(`{"user_id": 123, "action": "login"}`),
		Headers: map[string]string{"source": "auth-service"},
	}

	dataPlaneMsg := &peerlinkv1.PeerMessage{
		Kind: &peerlinkv1.PeerMessage_Event{
			Event: userEvent,
		},
	}

	// Create a control plane message (subscription change)
	subscriptionChange := &peerlinkv1.SubscriptionChange{
		Action:   "unsubscribe",
		ClientId: "client-789",
		Topic:    "user.*",
		NodeId:   "node-abc",
	}

	controlPlaneMsg := &peerlinkv1.PeerMessage{
		Kind: &peerlinkv1.PeerMessage_ControlPlane{
			ControlPlane: &peerlinkv1.ControlPlaneMessage{
				Kind: &peerlinkv1.ControlPlaneMessage_SubscriptionChange{
					SubscriptionChange: subscriptionChange,
				},
			},
		},
	}

	// Test that data plane message only contains user events
	if dataPlaneMsg.GetEvent() == nil {
		t.Error("Data plane message should contain an event")
	}
	if dataPlaneMsg.GetControlPlane() != nil {
		t.Error("Data plane message should not contain control plane data")
	}

	// Verify user event content
	event := dataPlaneMsg.GetEvent()
	if event.Topic != "user.events" {
		t.Error("Data plane should handle user events with regular topics")
	}

	// Test that control plane message only contains control messages
	if controlPlaneMsg.GetControlPlane() == nil {
		t.Error("Control plane message should contain control plane data")
	}
	if controlPlaneMsg.GetEvent() != nil {
		t.Error("Control plane message should not contain user events")
	}

	// Verify subscription change content
	ctrlMsg := controlPlaneMsg.GetControlPlane()
	if ctrlMsg.GetSubscriptionChange() == nil {
		t.Error("Control plane message should contain subscription change")
	}

	subChange := ctrlMsg.GetSubscriptionChange()
	if subChange.Action != "unsubscribe" {
		t.Error("Control plane should handle subscription changes")
	}
	if subChange.Topic != "user.*" {
		t.Error("Control plane subscription changes should support topic patterns")
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
	if stream.StreamName == "DataStream" || stream.StreamName == "ControlStream" {
		t.Fatal("data/control planes should remain logical planes on the single physical stream")
	}
}

// TestSubscriptionChangeValidation tests validation of subscription change messages
func TestSubscriptionChangeValidation(t *testing.T) {
	testCases := []struct {
		name        string
		action      string
		clientId    string
		topic       string
		nodeId      string
		expectValid bool
	}{
		{
			name:        "valid subscribe",
			action:      "subscribe",
			clientId:    "client-1",
			topic:       "orders.*",
			nodeId:      "node-1",
			expectValid: true,
		},
		{
			name:        "valid unsubscribe",
			action:      "unsubscribe",
			clientId:    "client-2",
			topic:       "users.created",
			nodeId:      "node-2",
			expectValid: true,
		},
		{
			name:        "empty action",
			action:      "",
			clientId:    "client-3",
			topic:       "test",
			nodeId:      "node-3",
			expectValid: false,
		},
		{
			name:        "empty client_id",
			action:      "subscribe",
			clientId:    "",
			topic:       "test",
			nodeId:      "node-4",
			expectValid: false,
		},
		{
			name:        "empty topic",
			action:      "subscribe",
			clientId:    "client-5",
			topic:       "",
			nodeId:      "node-5",
			expectValid: false,
		},
		{
			name:        "empty node_id",
			action:      "subscribe",
			clientId:    "client-6",
			topic:       "test",
			nodeId:      "",
			expectValid: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			subChange := &peerlinkv1.SubscriptionChange{
				Action:   tc.action,
				ClientId: tc.clientId,
				Topic:    tc.topic,
				NodeId:   tc.nodeId,
			}

			// Basic field validation (protobuf will handle this)
			isEmpty := subChange.Action == "" || subChange.ClientId == "" ||
				subChange.Topic == "" || subChange.NodeId == ""

			if tc.expectValid && isEmpty {
				t.Error("Expected valid subscription change to have all fields populated")
			}
			if !tc.expectValid && !isEmpty {
				// This test case expects invalid, but if all fields are populated,
				// the issue might be with action validation (subscribe/unsubscribe)
				validActions := subChange.Action == "subscribe" || subChange.Action == "unsubscribe"
				if validActions {
					t.Error("Expected invalid subscription change, but appears valid")
				}
			}
		})
	}
}
