package peerlink

import (
	"context"
	"testing"

	"github.com/rmacdonaldsmith/eventmesh-go/pkg/eventlog"
)

// Mock types for testing interfaces
type mockPeerNode struct {
	id      string
	address string
	healthy bool
}

func (m *mockPeerNode) ID() string      { return m.id }
func (m *mockPeerNode) Address() string { return m.address }
func (m *mockPeerNode) IsHealthy() bool { return m.healthy }

type mockDataPlanePeerLink struct{}

func (m *mockDataPlanePeerLink) SendEvent(ctx context.Context, peerID string, event *eventlog.Event) error {
	return nil
}

func (m *mockDataPlanePeerLink) ReceiveEvents(ctx context.Context) (<-chan *eventlog.Event, <-chan error) {
	eventChan := make(chan *eventlog.Event, 1)
	errChan := make(chan error, 1)
	return eventChan, errChan
}

type mockControlPlanePeerLink struct{}

func (m *mockControlPlanePeerLink) SendInterestUpdate(ctx context.Context, peerID string, update *InterestUpdate) error {
	return nil
}

func (m *mockControlPlanePeerLink) SendInterestSnapshot(ctx context.Context, peerID string, snapshot *InterestSnapshot) error {
	return nil
}

func (m *mockControlPlanePeerLink) ReceiveInterestMessages(ctx context.Context) (<-chan *InterestMessage, <-chan error) {
	messageChan := make(chan *InterestMessage, 1)
	errChan := make(chan error, 1)
	return messageChan, errChan
}

func (m *mockControlPlanePeerLink) SendHeartbeat(ctx context.Context, peerID string) error {
	return nil
}

func (m *mockControlPlanePeerLink) GetPeerHealth(ctx context.Context, peerID string) (PeerHealthState, error) {
	return PeerHealthy, nil
}

func (m *mockControlPlanePeerLink) StartHeartbeats(ctx context.Context) error {
	return nil
}

func (m *mockControlPlanePeerLink) StopHeartbeats(ctx context.Context) error {
	return nil
}

type mockPeerConnectionManager struct{}

func (m *mockPeerConnectionManager) Connect(ctx context.Context, peer PeerNode) error {
	return nil
}

func (m *mockPeerConnectionManager) Disconnect(ctx context.Context, peerID string) error {
	return nil
}

func (m *mockPeerConnectionManager) GetConnectedPeers(ctx context.Context) ([]PeerNode, error) {
	return []PeerNode{}, nil
}

func (m *mockPeerConnectionManager) Close() error {
	return nil
}

// TestDataPlanePeerLink_InterfaceCompliance verifies DataPlanePeerLink interface compliance
func TestDataPlanePeerLink_InterfaceCompliance(t *testing.T) {
	// This should compile if mockDataPlanePeerLink properly implements DataPlanePeerLink
	var _ DataPlanePeerLink = &mockDataPlanePeerLink{}
}

// TestControlPlanePeerLink_InterfaceCompliance verifies ControlPlanePeerLink interface compliance
func TestControlPlanePeerLink_InterfaceCompliance(t *testing.T) {
	// This should compile if mockControlPlanePeerLink properly implements ControlPlanePeerLink
	var _ ControlPlanePeerLink = &mockControlPlanePeerLink{}
}

// TestPeerConnectionManager_InterfaceCompliance verifies PeerConnectionManager interface compliance
func TestPeerConnectionManager_InterfaceCompliance(t *testing.T) {
	// This should compile if mockPeerConnectionManager properly implements PeerConnectionManager
	var _ PeerConnectionManager = &mockPeerConnectionManager{}
}

// TestDataPlanePeerLink_Functionality tests DataPlanePeerLink functionality
func TestDataPlanePeerLink_Functionality(t *testing.T) {
	ctx := context.Background()
	dataPlane := &mockDataPlanePeerLink{}

	// Test SendEvent
	event := &eventlog.Event{
		Topic:   "test.topic",
		Payload: []byte("test payload"),
		Offset:  1,
	}
	err := dataPlane.SendEvent(ctx, "peer-1", event)
	if err != nil {
		t.Errorf("Expected no error from SendEvent, got %v", err)
	}

	// Test ReceiveEvents
	eventChan, errChan := dataPlane.ReceiveEvents(ctx)
	if eventChan == nil {
		t.Error("Expected non-nil event channel")
	}
	if errChan == nil {
		t.Error("Expected non-nil error channel")
	}
}

// TestControlPlanePeerLink_Functionality tests ControlPlanePeerLink functionality
func TestControlPlanePeerLink_Functionality(t *testing.T) {
	ctx := context.Background()
	controlPlane := &mockControlPlanePeerLink{}

	// Test SendInterestUpdate
	update := &InterestUpdate{
		Action: "subscribe",
		Topic:  "orders.*",
		NodeId: "node-1",
	}
	err := controlPlane.SendInterestUpdate(ctx, "peer-1", update)
	if err != nil {
		t.Errorf("Expected no error from SendInterestUpdate, got %v", err)
	}

	// Test SendInterestSnapshot
	snapshot := &InterestSnapshot{NodeId: "node-1", Topics: []string{"orders.*"}}
	err = controlPlane.SendInterestSnapshot(ctx, "peer-1", snapshot)
	if err != nil {
		t.Errorf("Expected no error from SendInterestSnapshot, got %v", err)
	}

	// Test ReceiveInterestMessages
	messageChan, errChan := controlPlane.ReceiveInterestMessages(ctx)
	if messageChan == nil {
		t.Error("Expected non-nil interest message channel")
	}
	if errChan == nil {
		t.Error("Expected non-nil error channel")
	}

	// Test SendHeartbeat
	err = controlPlane.SendHeartbeat(ctx, "peer-1")
	if err != nil {
		t.Errorf("Expected no error from SendHeartbeat, got %v", err)
	}

	// Test GetPeerHealth
	health, err := controlPlane.GetPeerHealth(ctx, "peer-1")
	if err != nil {
		t.Errorf("Expected no error from GetPeerHealth, got %v", err)
	}
	if health != PeerHealthy {
		t.Errorf("Expected PeerHealthy, got %v", health)
	}

	// Test StartHeartbeats
	err = controlPlane.StartHeartbeats(ctx)
	if err != nil {
		t.Errorf("Expected no error from StartHeartbeats, got %v", err)
	}

	// Test StopHeartbeats
	err = controlPlane.StopHeartbeats(ctx)
	if err != nil {
		t.Errorf("Expected no error from StopHeartbeats, got %v", err)
	}
}

// TestPeerConnectionManager_Functionality tests PeerConnectionManager functionality
func TestPeerConnectionManager_Functionality(t *testing.T) {
	ctx := context.Background()
	connMgr := &mockPeerConnectionManager{}

	// Test Connect
	peer := &mockPeerNode{
		id:      "peer-1",
		address: "localhost:9090",
		healthy: true,
	}
	err := connMgr.Connect(ctx, peer)
	if err != nil {
		t.Errorf("Expected no error from Connect, got %v", err)
	}

	// Test GetConnectedPeers
	peers, err := connMgr.GetConnectedPeers(ctx)
	if err != nil {
		t.Errorf("Expected no error from GetConnectedPeers, got %v", err)
	}
	if peers == nil {
		t.Error("Expected non-nil peers slice")
	}

	// Test Disconnect
	err = connMgr.Disconnect(ctx, "peer-1")
	if err != nil {
		t.Errorf("Expected no error from Disconnect, got %v", err)
	}

	// Test Close
	err = connMgr.Close()
	if err != nil {
		t.Errorf("Expected no error from Close, got %v", err)
	}
}

// TestInterfaceSeparationPrinciples tests that interfaces are properly separated by concern
func TestInterfaceSeparationPrinciples(t *testing.T) {
	// Test that DataPlanePeerLink only handles user events
	dataPlane := &mockDataPlanePeerLink{}
	ctx := context.Background()

	// Should handle user events
	event := &eventlog.Event{Topic: "user.events", Payload: []byte("user data")}
	err := dataPlane.SendEvent(ctx, "peer-1", event)
	if err != nil {
		t.Error("DataPlanePeerLink should handle user events")
	}

	// Should receive user events
	eventChan, _ := dataPlane.ReceiveEvents(ctx)
	if eventChan == nil {
		t.Error("DataPlanePeerLink should provide user event channel")
	}

	// Test that ControlPlanePeerLink only handles control messages
	controlPlane := &mockControlPlanePeerLink{}

	// Should handle aggregate interest updates
	update := &InterestUpdate{Action: "subscribe", Topic: "control.topic", NodeId: "node-1"}
	err = controlPlane.SendInterestUpdate(ctx, "peer-1", update)
	if err != nil {
		t.Error("ControlPlanePeerLink should handle interest updates")
	}

	// Should handle heartbeats
	err = controlPlane.SendHeartbeat(ctx, "peer-1")
	if err != nil {
		t.Error("ControlPlanePeerLink should handle heartbeats")
	}

	// Should provide health monitoring
	health, err := controlPlane.GetPeerHealth(ctx, "peer-1")
	if err != nil || health != PeerHealthy {
		t.Error("ControlPlanePeerLink should provide health monitoring")
	}

	// Should handle heartbeat lifecycle
	err = controlPlane.StartHeartbeats(ctx)
	if err != nil {
		t.Error("ControlPlanePeerLink should handle heartbeat startup")
	}

	err = controlPlane.StopHeartbeats(ctx)
	if err != nil {
		t.Error("ControlPlanePeerLink should handle heartbeat shutdown")
	}

	// Test that PeerConnectionManager only handles connections
	connMgr := &mockPeerConnectionManager{}

	// Should handle connection lifecycle
	peer := &mockPeerNode{id: "peer-1", address: "localhost:9090"}
	err = connMgr.Connect(ctx, peer)
	if err != nil {
		t.Error("PeerConnectionManager should handle connections")
	}

	err = connMgr.Disconnect(ctx, "peer-1")
	if err != nil {
		t.Error("PeerConnectionManager should handle disconnections")
	}
}
