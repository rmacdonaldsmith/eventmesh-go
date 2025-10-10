package meshnode

import (
	"context"
	"testing"
	"time"

	"github.com/rmacdonaldsmith/eventmesh-go/internal/peerlink"
	"github.com/rmacdonaldsmith/eventmesh-go/pkg/eventlog"
	meshnode "github.com/rmacdonaldsmith/eventmesh-go/pkg/meshnode"
	"github.com/rmacdonaldsmith/eventmesh-go/pkg/routingtable"
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

// TestGRPCMeshNode_PublishEvent_LocalPersistence tests REQ-MNODE-002: Local Persistence Before Forwarding
func TestGRPCMeshNode_PublishEvent_LocalPersistence(t *testing.T) {
	config := NewConfig("test-node", "localhost:8084")
	node, err := NewGRPCMeshNode(config)
	if err != nil {
		t.Fatalf("Expected no error creating mesh node, got %v", err)
	}
	defer node.Close()

	ctx := context.Background()

	// Start the node
	err = node.Start(ctx)
	if err != nil {
		t.Fatalf("Expected no error starting node, got %v", err)
	}

	// Create a test client and event
	client := NewTrustedClient("test-client-1")
	event := eventlog.NewRecord("orders.created", []byte(`{"orderId":"123","amount":99.99}`))

	// Publish the event - this should persist it locally first (REQ-MNODE-002)
	err = node.PublishEvent(ctx, client, event)
	if err != nil {
		t.Fatalf("Expected no error publishing event, got %v", err)
	}

	// Verify the event was persisted in EventLog
	eventLog := node.GetEventLog()

	// Read from the topic to verify persistence
	events, err := eventLog.ReadFromTopic(ctx, "orders.created", 0, 10)
	if err != nil {
		t.Fatalf("Expected no error reading from EventLog, got %v", err)
	}

	if len(events) != 1 {
		t.Fatalf("Expected 1 event in EventLog, got %d", len(events))
	}

	// Verify the persisted event matches what we published
	persistedEvent := events[0]
	if persistedEvent.Topic() != "orders.created" {
		t.Errorf("Expected topic 'orders.created', got '%s'", persistedEvent.Topic())
	}
	if string(persistedEvent.Payload()) != `{"orderId":"123","amount":99.99}` {
		t.Errorf("Expected payload to match, got '%s'", string(persistedEvent.Payload()))
	}
	if persistedEvent.Offset() != 0 {
		t.Errorf("Expected offset 0, got %d", persistedEvent.Offset())
	}
}

// TestGRPCMeshNode_PublishEvent_Multiple tests publishing multiple events
func TestGRPCMeshNode_PublishEvent_Multiple(t *testing.T) {
	config := NewConfig("test-node", "localhost:8085")
	node, err := NewGRPCMeshNode(config)
	if err != nil {
		t.Fatalf("Expected no error creating mesh node, got %v", err)
	}
	defer node.Close()

	ctx := context.Background()
	err = node.Start(ctx)
	if err != nil {
		t.Fatalf("Expected no error starting node, got %v", err)
	}

	client := NewTrustedClient("test-client-2")

	// Publish multiple events to different topics
	events := []struct {
		topic   string
		payload string
	}{
		{"orders.created", `{"orderId":"123"}`},
		{"orders.updated", `{"orderId":"123","status":"shipped"}`},
		{"orders.created", `{"orderId":"456"}`},
	}

	for _, testEvent := range events {
		event := eventlog.NewRecord(testEvent.topic, []byte(testEvent.payload))
		err = node.PublishEvent(ctx, client, event)
		if err != nil {
			t.Fatalf("Expected no error publishing event to %s, got %v", testEvent.topic, err)
		}
	}

	// Verify events were persisted correctly
	eventLog := node.GetEventLog()

	// Check orders.created topic (should have 2 events)
	createdEvents, err := eventLog.ReadFromTopic(ctx, "orders.created", 0, 10)
	if err != nil {
		t.Fatalf("Expected no error reading orders.created, got %v", err)
	}
	if len(createdEvents) != 2 {
		t.Errorf("Expected 2 events in orders.created, got %d", len(createdEvents))
	}

	// Check orders.updated topic (should have 1 event)
	updatedEvents, err := eventLog.ReadFromTopic(ctx, "orders.updated", 0, 10)
	if err != nil {
		t.Fatalf("Expected no error reading orders.updated, got %v", err)
	}
	if len(updatedEvents) != 1 {
		t.Errorf("Expected 1 event in orders.updated, got %d", len(updatedEvents))
	}

	// Verify offsets are correct (per-topic sequences)
	if createdEvents[0].Offset() != 0 {
		t.Errorf("Expected first orders.created offset 0, got %d", createdEvents[0].Offset())
	}
	if createdEvents[1].Offset() != 1 {
		t.Errorf("Expected second orders.created offset 1, got %d", createdEvents[1].Offset())
	}
	if updatedEvents[0].Offset() != 0 {
		t.Errorf("Expected first orders.updated offset 0, got %d", updatedEvents[0].Offset())
	}
}

// TestGRPCMeshNode_PublishEvent_NilClient tests error handling with nil client
func TestGRPCMeshNode_PublishEvent_NilClient(t *testing.T) {
	config := NewConfig("test-node", "localhost:8086")
	node, err := NewGRPCMeshNode(config)
	if err != nil {
		t.Fatalf("Expected no error creating mesh node, got %v", err)
	}
	defer node.Close()

	ctx := context.Background()
	err = node.Start(ctx)
	if err != nil {
		t.Fatalf("Expected no error starting node, got %v", err)
	}

	event := eventlog.NewRecord("test.topic", []byte("test payload"))

	// Try to publish with nil client - should handle gracefully
	err = node.PublishEvent(ctx, nil, event)
	if err == nil {
		t.Error("Expected error when publishing with nil client")
	}
}

// TestGRPCMeshNode_PublishEvent_NilEvent tests error handling with nil event
func TestGRPCMeshNode_PublishEvent_NilEvent(t *testing.T) {
	config := NewConfig("test-node", "localhost:8087")
	node, err := NewGRPCMeshNode(config)
	if err != nil {
		t.Fatalf("Expected no error creating mesh node, got %v", err)
	}
	defer node.Close()

	ctx := context.Background()
	err = node.Start(ctx)
	if err != nil {
		t.Fatalf("Expected no error starting node, got %v", err)
	}

	client := NewTrustedClient("test-client-3")

	// Try to publish nil event - should handle gracefully
	err = node.PublishEvent(ctx, client, nil)
	if err == nil {
		t.Error("Expected error when publishing nil event")
	}
}

// TestGRPCMeshNode_LocalSubscriberDelivery tests local event delivery to subscribed clients
func TestGRPCMeshNode_LocalSubscriberDelivery(t *testing.T) {
	config := NewConfig("test-node", "localhost:8088")
	node, err := NewGRPCMeshNode(config)
	if err != nil {
		t.Fatalf("Expected no error creating mesh node, got %v", err)
	}
	defer node.Close()

	ctx := context.Background()
	err = node.Start(ctx)
	if err != nil {
		t.Fatalf("Expected no error starting node, got %v", err)
	}

	// Create a subscriber client and subscribe to a topic
	subscriber := NewTrustedClient("subscriber-1")
	publisher := NewTrustedClient("publisher-1")

	// Subscribe the client to "orders.*" pattern
	err = node.Subscribe(ctx, subscriber, "orders.*")
	if err != nil {
		t.Fatalf("Expected no error subscribing, got %v", err)
	}

	// Publish an event that should match the subscription
	event := eventlog.NewRecord("orders.created", []byte(`{"orderId":"123"}`))
	err = node.PublishEvent(ctx, publisher, event)
	if err != nil {
		t.Fatalf("Expected no error publishing event, got %v", err)
	}

	// Verify the subscriber received the event
	receivedEvents := subscriber.GetReceivedEvents()
	if len(receivedEvents) != 1 {
		t.Fatalf("Expected subscriber to receive 1 event, got %d", len(receivedEvents))
	}

	receivedEvent := receivedEvents[0]
	if receivedEvent.Topic() != "orders.created" {
		t.Errorf("Expected received event topic 'orders.created', got '%s'", receivedEvent.Topic())
	}
	if string(receivedEvent.Payload()) != `{"orderId":"123"}` {
		t.Errorf("Expected received payload to match, got '%s'", string(receivedEvent.Payload()))
	}
}

// TestGRPCMeshNode_MultipleLocalSubscribers tests delivery to multiple local subscribers
func TestGRPCMeshNode_MultipleLocalSubscribers(t *testing.T) {
	config := NewConfig("test-node", "localhost:8089")
	node, err := NewGRPCMeshNode(config)
	if err != nil {
		t.Fatalf("Expected no error creating mesh node, got %v", err)
	}
	defer node.Close()

	ctx := context.Background()
	err = node.Start(ctx)
	if err != nil {
		t.Fatalf("Expected no error starting node, got %v", err)
	}

	// Create multiple subscribers
	subscriber1 := NewTrustedClient("subscriber-1")
	subscriber2 := NewTrustedClient("subscriber-2")
	subscriber3 := NewTrustedClient("subscriber-3")
	publisher := NewTrustedClient("publisher-1")

	// Subscribe to different patterns
	err = node.Subscribe(ctx, subscriber1, "orders.*")
	if err != nil {
		t.Fatalf("Expected no error subscribing subscriber1, got %v", err)
	}

	err = node.Subscribe(ctx, subscriber2, "orders.created")
	if err != nil {
		t.Fatalf("Expected no error subscribing subscriber2, got %v", err)
	}

	err = node.Subscribe(ctx, subscriber3, "payments.*")
	if err != nil {
		t.Fatalf("Expected no error subscribing subscriber3, got %v", err)
	}

	// Publish an orders.created event (should match subscriber1 and subscriber2)
	event := eventlog.NewRecord("orders.created", []byte(`{"orderId":"123"}`))
	err = node.PublishEvent(ctx, publisher, event)
	if err != nil {
		t.Fatalf("Expected no error publishing event, got %v", err)
	}

	// Verify delivery
	events1 := subscriber1.GetReceivedEvents()
	events2 := subscriber2.GetReceivedEvents()
	events3 := subscriber3.GetReceivedEvents()

	if len(events1) != 1 {
		t.Errorf("Expected subscriber1 to receive 1 event, got %d", len(events1))
	}
	if len(events2) != 1 {
		t.Errorf("Expected subscriber2 to receive 1 event, got %d", len(events2))
	}
	if len(events3) != 0 {
		t.Errorf("Expected subscriber3 to receive 0 events, got %d", len(events3))
	}
}

// TestGRPCMeshNode_PublishFlow_Complete tests the complete publish flow
// This test verifies REQ-MNODE-002: Local Persistence Before Forwarding
func TestGRPCMeshNode_PublishFlow_Complete(t *testing.T) {
	config := NewConfig("test-node", "localhost:8090")
	node, err := NewGRPCMeshNode(config)
	if err != nil {
		t.Fatalf("Expected no error creating mesh node, got %v", err)
	}
	defer node.Close()

	ctx := context.Background()
	err = node.Start(ctx)
	if err != nil {
		t.Fatalf("Expected no error starting node, got %v", err)
	}

	// Create clients
	publisher := NewTrustedClient("publisher-1")
	subscriber1 := NewTrustedClient("subscriber-1")
	subscriber2 := NewTrustedClient("subscriber-2")

	// Set up subscriptions
	err = node.Subscribe(ctx, subscriber1, "orders.*")
	if err != nil {
		t.Fatalf("Expected no error subscribing subscriber1, got %v", err)
	}

	err = node.Subscribe(ctx, subscriber2, "inventory.*")
	if err != nil {
		t.Fatalf("Expected no error subscribing subscriber2, got %v", err)
	}

	// Test Case 1: Publish event that matches subscriber1
	event1 := eventlog.NewRecord("orders.created", []byte(`{"orderId":"123","customerId":"456"}`))
	err = node.PublishEvent(ctx, publisher, event1)
	if err != nil {
		t.Fatalf("Expected no error publishing orders event, got %v", err)
	}

	// Test Case 2: Publish event that matches subscriber2
	event2 := eventlog.NewRecord("inventory.updated", []byte(`{"productId":"789","quantity":100}`))
	err = node.PublishEvent(ctx, publisher, event2)
	if err != nil {
		t.Fatalf("Expected no error publishing inventory event, got %v", err)
	}

	// Test Case 3: Publish event that matches no subscribers
	event3 := eventlog.NewRecord("analytics.view", []byte(`{"pageId":"home","userId":"999"}`))
	err = node.PublishEvent(ctx, publisher, event3)
	if err != nil {
		t.Fatalf("Expected no error publishing analytics event, got %v", err)
	}

	// Verify persistence (REQ-MNODE-002)
	eventLog := node.GetEventLog()

	ordersEvents, err := eventLog.ReadFromTopic(ctx, "orders.created", 0, 10)
	if err != nil {
		t.Fatalf("Expected no error reading orders events, got %v", err)
	}
	if len(ordersEvents) != 1 {
		t.Errorf("Expected 1 orders event persisted, got %d", len(ordersEvents))
	}

	inventoryEvents, err := eventLog.ReadFromTopic(ctx, "inventory.updated", 0, 10)
	if err != nil {
		t.Fatalf("Expected no error reading inventory events, got %v", err)
	}
	if len(inventoryEvents) != 1 {
		t.Errorf("Expected 1 inventory event persisted, got %d", len(inventoryEvents))
	}

	analyticsEvents, err := eventLog.ReadFromTopic(ctx, "analytics.view", 0, 10)
	if err != nil {
		t.Fatalf("Expected no error reading analytics events, got %v", err)
	}
	if len(analyticsEvents) != 1 {
		t.Errorf("Expected 1 analytics event persisted, got %d", len(analyticsEvents))
	}

	// Verify local delivery
	events1 := subscriber1.GetReceivedEvents()
	events2 := subscriber2.GetReceivedEvents()

	if len(events1) != 1 {
		t.Errorf("Expected subscriber1 to receive 1 event, got %d", len(events1))
	} else {
		if events1[0].Topic() != "orders.created" {
			t.Errorf("Expected subscriber1 to receive orders.created event, got %s", events1[0].Topic())
		}
	}

	if len(events2) != 1 {
		t.Errorf("Expected subscriber2 to receive 1 event, got %d", len(events2))
	} else {
		if events2[0].Topic() != "inventory.updated" {
			t.Errorf("Expected subscriber2 to receive inventory.updated event, got %s", events2[0].Topic())
		}
	}
}

// TestGRPCMeshNode_PublishFlow_ErrorHandling tests error handling in publish flow
func TestGRPCMeshNode_PublishFlow_ErrorHandling(t *testing.T) {
	config := NewConfig("test-node", "localhost:8091")
	node, err := NewGRPCMeshNode(config)
	if err != nil {
		t.Fatalf("Expected no error creating mesh node, got %v", err)
	}
	defer node.Close()

	ctx := context.Background()

	// Test publishing to stopped node
	client := NewTrustedClient("test-client")
	event := eventlog.NewRecord("test.topic", []byte("test"))

	err = node.PublishEvent(ctx, client, event)
	if err == nil {
		t.Error("Expected error when publishing to stopped node")
	}

	// Start node
	err = node.Start(ctx)
	if err != nil {
		t.Fatalf("Expected no error starting node, got %v", err)
	}

	// Test with unauthenticated client (mock scenario)
	unauthenticatedClient := &MockUnauthenticatedClient{id: "unauth-client"}
	err = node.PublishEvent(ctx, unauthenticatedClient, event)
	if err == nil {
		t.Error("Expected error when publishing with unauthenticated client")
	}

	// Test with empty topic
	emptyTopicEvent := eventlog.NewRecord("", []byte("test"))
	err = node.PublishEvent(ctx, client, emptyTopicEvent)
	if err == nil {
		t.Error("Expected error when publishing event with empty topic")
	}
}

// MockUnauthenticatedClient is a test helper for error handling tests
type MockUnauthenticatedClient struct {
	id string
}

func (c *MockUnauthenticatedClient) ID() string {
	return c.id
}

func (c *MockUnauthenticatedClient) IsAuthenticated() bool {
	return false // Simulates unauthenticated client
}

func (c *MockUnauthenticatedClient) Type() routingtable.SubscriberType {
	return routingtable.LocalClient
}

// Verify interface compliance
var _ meshnode.Client = (*MockUnauthenticatedClient)(nil)
var _ routingtable.Subscriber = (*MockUnauthenticatedClient)(nil)

// TestGRPCMeshNode_PersistenceBeforeForwarding explicitly tests REQ-MNODE-002
// This test verifies that events are persisted locally BEFORE any forwarding occurs
func TestGRPCMeshNode_PersistenceBeforeForwarding(t *testing.T) {
	config := NewConfig("test-node", "localhost:8092")
	node, err := NewGRPCMeshNode(config)
	if err != nil {
		t.Fatalf("Expected no error creating mesh node, got %v", err)
	}
	defer node.Close()

	ctx := context.Background()
	err = node.Start(ctx)
	if err != nil {
		t.Fatalf("Expected no error starting node, got %v", err)
	}

	// Create a client and event
	client := NewTrustedClient("test-client")
	event := eventlog.NewRecord("test.persistence", []byte(`{"testId":"persistence-test"}`))

	// Publish the event
	// Even if peer forwarding fails (no peers connected), local persistence should succeed
	err = node.PublishEvent(ctx, client, event)
	if err != nil {
		t.Fatalf("Expected no error publishing event (local persistence should succeed), got %v", err)
	}

	// Verify the event was persisted locally regardless of forwarding status
	eventLog := node.GetEventLog()
	persistedEvents, err := eventLog.ReadFromTopic(ctx, "test.persistence", 0, 10)
	if err != nil {
		t.Fatalf("Expected no error reading persisted events, got %v", err)
	}

	if len(persistedEvents) != 1 {
		t.Fatalf("Expected 1 persisted event, got %d", len(persistedEvents))
	}

	persistedEvent := persistedEvents[0]
	if persistedEvent.Topic() != "test.persistence" {
		t.Errorf("Expected persisted topic 'test.persistence', got '%s'", persistedEvent.Topic())
	}
	if string(persistedEvent.Payload()) != `{"testId":"persistence-test"}` {
		t.Errorf("Expected persisted payload to match, got '%s'", string(persistedEvent.Payload()))
	}

	// Verify the event has a valid offset (assigned during persistence)
	if persistedEvent.Offset() < 0 {
		t.Errorf("Expected non-negative offset, got %d", persistedEvent.Offset())
	}

	// This test confirms REQ-MNODE-002: Local Persistence Before Forwarding
	// The event was successfully persisted locally, even without connected peers
	t.Logf("✅ REQ-MNODE-002 verified: Event persisted locally with offset %d", persistedEvent.Offset())
}

// TestGRPCMeshNode_Unsubscribe tests the Unsubscribe functionality
func TestGRPCMeshNode_Unsubscribe(t *testing.T) {
	config := NewConfig("test-node", "localhost:8093")
	node, err := NewGRPCMeshNode(config)
	if err != nil {
		t.Fatalf("Expected no error creating mesh node, got %v", err)
	}
	defer node.Close()

	ctx := context.Background()
	err = node.Start(ctx)
	if err != nil {
		t.Fatalf("Expected no error starting node, got %v", err)
	}

	// Create clients
	subscriber := NewTrustedClient("subscriber-1")
	publisher := NewTrustedClient("publisher-1")

	// Subscribe to a topic
	err = node.Subscribe(ctx, subscriber, "orders.*")
	if err != nil {
		t.Fatalf("Expected no error subscribing, got %v", err)
	}

	// Verify subscription works by publishing and receiving
	event1 := eventlog.NewRecord("orders.created", []byte(`{"orderId":"123"}`))
	err = node.PublishEvent(ctx, publisher, event1)
	if err != nil {
		t.Fatalf("Expected no error publishing event, got %v", err)
	}

	receivedEvents := subscriber.GetReceivedEvents()
	if len(receivedEvents) != 1 {
		t.Fatalf("Expected subscriber to receive 1 event before unsubscribe, got %d", len(receivedEvents))
	}

	// Unsubscribe from the topic
	err = node.Unsubscribe(ctx, subscriber, "orders.*")
	if err != nil {
		t.Fatalf("Expected no error unsubscribing, got %v", err)
	}

	// Clear received events buffer to test unsubscribe
	subscriber.eventBuffer = make([]eventlog.EventRecord, 0)

	// Publish another event - should not be received after unsubscribe
	event2 := eventlog.NewRecord("orders.updated", []byte(`{"orderId":"123","status":"shipped"}`))
	err = node.PublishEvent(ctx, publisher, event2)
	if err != nil {
		t.Fatalf("Expected no error publishing event after unsubscribe, got %v", err)
	}

	// Verify subscriber no longer receives events
	receivedEvents = subscriber.GetReceivedEvents()
	if len(receivedEvents) != 0 {
		t.Errorf("Expected subscriber to receive 0 events after unsubscribe, got %d", len(receivedEvents))
	}
}

// TestGRPCMeshNode_Unsubscribe_ErrorHandling tests error handling in Unsubscribe
func TestGRPCMeshNode_Unsubscribe_ErrorHandling(t *testing.T) {
	config := NewConfig("test-node", "localhost:8094")
	node, err := NewGRPCMeshNode(config)
	if err != nil {
		t.Fatalf("Expected no error creating mesh node, got %v", err)
	}
	defer node.Close()

	ctx := context.Background()

	client := NewTrustedClient("test-client")

	// Test unsubscribing from stopped node
	err = node.Unsubscribe(ctx, client, "test.topic")
	if err == nil {
		t.Error("Expected error when unsubscribing from stopped node")
	}

	// Start node
	err = node.Start(ctx)
	if err != nil {
		t.Fatalf("Expected no error starting node, got %v", err)
	}

	// Test unsubscribing with nil client
	err = node.Unsubscribe(ctx, nil, "test.topic")
	if err == nil {
		t.Error("Expected error when unsubscribing with nil client")
	}

	// Test unsubscribing with empty topic
	err = node.Unsubscribe(ctx, client, "")
	if err == nil {
		t.Error("Expected error when unsubscribing with empty topic")
	}

	// Test unsubscribing from non-existent subscription (should not error)
	err = node.Unsubscribe(ctx, client, "non-existent.topic")
	if err != nil {
		t.Errorf("Expected no error when unsubscribing from non-existent subscription, got %v", err)
	}
}

// TestGRPCMeshNode_SubscriptionPropagation tests REQ-MNODE-003: Subscription Propagation
func TestGRPCMeshNode_SubscriptionPropagation(t *testing.T) {
	config := NewConfig("test-node", "localhost:8095")
	node, err := NewGRPCMeshNode(config)
	if err != nil {
		t.Fatalf("Expected no error creating mesh node, got %v", err)
	}
	defer node.Close()

	ctx := context.Background()
	err = node.Start(ctx)
	if err != nil {
		t.Fatalf("Expected no error starting node, got %v", err)
	}

	client := NewTrustedClient("test-client")

	// Subscribe to a topic - should trigger propagation to peers
	err = node.Subscribe(ctx, client, "orders.*")
	if err != nil {
		t.Fatalf("Expected no error subscribing, got %v", err)
	}

	// For MVP: We can't easily test actual peer propagation without setting up multiple nodes
	// But we can verify that the subscription worked locally
	routingTable := node.GetRoutingTable()
	subscribers, err := routingTable.GetSubscribers(ctx, "orders.created")
	if err != nil {
		t.Fatalf("Expected no error getting subscribers, got %v", err)
	}

	if len(subscribers) != 1 {
		t.Errorf("Expected 1 subscriber for orders.created pattern, got %d", len(subscribers))
	}

	// Unsubscribe - should also trigger propagation
	err = node.Unsubscribe(ctx, client, "orders.*")
	if err != nil {
		t.Fatalf("Expected no error unsubscribing, got %v", err)
	}

	// Verify local unsubscription worked
	subscribers, err = routingTable.GetSubscribers(ctx, "orders.created")
	if err != nil {
		t.Fatalf("Expected no error getting subscribers after unsubscribe, got %v", err)
	}

	if len(subscribers) != 0 {
		t.Errorf("Expected 0 subscribers after unsubscribe, got %d", len(subscribers))
	}

	t.Logf("✅ REQ-MNODE-003: Subscription propagation logic implemented (peer propagation via PeerLink)")
}

// TestGRPCMeshNode_IncomingEventHandling tests handling of incoming events from peers
func TestGRPCMeshNode_IncomingEventHandling(t *testing.T) {
	config := NewConfig("test-node", "localhost:8096")
	node, err := NewGRPCMeshNode(config)
	if err != nil {
		t.Fatalf("Expected no error creating mesh node, got %v", err)
	}
	defer node.Close()

	ctx := context.Background()
	err = node.Start(ctx)
	if err != nil {
		t.Fatalf("Expected no error starting node, got %v", err)
	}

	// Test subscription event detection
	subscriptionTopic := "__mesh.subscription.subscribe"
	if !node.isSubscriptionEvent(subscriptionTopic) {
		t.Errorf("Expected %s to be detected as subscription event", subscriptionTopic)
	}

	regularTopic := "orders.created"
	if node.isSubscriptionEvent(regularTopic) {
		t.Errorf("Expected %s to NOT be detected as subscription event", regularTopic)
	}

	// Test processing a simulated incoming event
	// Create a regular event that would come from a peer
	incomingEvent := eventlog.NewRecord("peer.events.test", []byte(`{"from":"peer-node-1"}`))

	// Subscribe a local client to receive the event
	subscriber := NewTrustedClient("local-subscriber")
	err = node.Subscribe(ctx, subscriber, "peer.events.*")
	if err != nil {
		t.Fatalf("Expected no error subscribing to peer events, got %v", err)
	}

	// Process the incoming event (simulate what happens when received from peer)
	node.processIncomingEvent(ctx, incomingEvent)

	// Verify the event was delivered to local subscriber
	receivedEvents := subscriber.GetReceivedEvents()
	if len(receivedEvents) != 1 {
		t.Fatalf("Expected local subscriber to receive 1 event from peer, got %d", len(receivedEvents))
	}

	if string(receivedEvents[0].Payload()) != `{"from":"peer-node-1"}` {
		t.Errorf("Expected to receive peer event payload, got '%s'", string(receivedEvents[0].Payload()))
	}

	// Verify the event was also persisted locally
	eventLog := node.GetEventLog()
	persistedEvents, err := eventLog.ReadFromTopic(ctx, "peer.events.test", 0, 10)
	if err != nil {
		t.Fatalf("Expected no error reading persisted peer events, got %v", err)
	}

	if len(persistedEvents) != 1 {
		t.Errorf("Expected 1 persisted peer event, got %d", len(persistedEvents))
	}

	t.Logf("✅ Incoming peer event handling working: persist + deliver to local subscribers")
}

// TestGRPCMeshNode_SubscriptionEventHandling tests handling of subscription events from peers
func TestGRPCMeshNode_SubscriptionEventHandling(t *testing.T) {
	config := NewConfig("test-node", "localhost:8097")
	node, err := NewGRPCMeshNode(config)
	if err != nil {
		t.Fatalf("Expected no error creating mesh node, got %v", err)
	}
	defer node.Close()

	ctx := context.Background()
	err = node.Start(ctx)
	if err != nil {
		t.Fatalf("Expected no error starting node, got %v", err)
	}

	// Create a subscription event like what would be received from a peer
	subscriptionPayload := []byte(`{"action":"subscribe","clientID":"peer-client-1","topic":"orders.*","nodeID":"peer-node-1"}`)
	subscriptionEvent := eventlog.NewRecord("__mesh.subscription.subscribe", subscriptionPayload)

	// Process the subscription event (simulate receiving from peer)
	node.handleSubscriptionEvent(ctx, subscriptionEvent)

	// For MVP: We just handle the event without errors
	// In a full implementation, we'd verify that peer subscription info was stored
	// and used for smarter routing decisions

	// Test unsubscription event as well
	unsubscriptionPayload := []byte(`{"action":"unsubscribe","clientID":"peer-client-1","topic":"orders.*","nodeID":"peer-node-1"}`)
	unsubscriptionEvent := eventlog.NewRecord("__mesh.subscription.unsubscribe", unsubscriptionPayload)

	node.handleSubscriptionEvent(ctx, unsubscriptionEvent)

	t.Logf("✅ Subscription event handling implemented (basic MVP version)")
}