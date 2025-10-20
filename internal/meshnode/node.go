package meshnode

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/rmacdonaldsmith/eventmesh-go/internal/discovery"
	"github.com/rmacdonaldsmith/eventmesh-go/internal/eventlog"
	"github.com/rmacdonaldsmith/eventmesh-go/internal/peerlink"
	"github.com/rmacdonaldsmith/eventmesh-go/internal/routingtable"
	eventlogpkg "github.com/rmacdonaldsmith/eventmesh-go/pkg/eventlog"
	"github.com/rmacdonaldsmith/eventmesh-go/pkg/meshnode"
	peerlinkpkg "github.com/rmacdonaldsmith/eventmesh-go/pkg/peerlink"
	routingtablepkg "github.com/rmacdonaldsmith/eventmesh-go/pkg/routingtable"
)

// GRPCMeshNode implements the meshnode.MeshNode interface.
// It orchestrates EventLog, RoutingTable, and PeerLink components to provide
// distributed event streaming functionality.
//
// This implementation focuses on:
// - REQ-MNODE-002: Local Persistence Before Forwarding
// - REQ-MNODE-003: Subscription Propagation
// - Component orchestration and lifecycle management
type GRPCMeshNode struct {
	mu     sync.RWMutex
	config *Config

	// Core components
	eventLog     eventlogpkg.EventLog
	routingTable routingtablepkg.RoutingTable
	peerLink     peerlinkpkg.PeerLink

	// State management
	started bool
	closed  bool

	// Client management (simplified for MVP - no authentication)
	clients map[string]meshnode.Client

	// Subscription management
	subscriptions   map[string]map[string]*meshnode.ClientSubscription // clientID -> subscriptionID -> subscription
	subscriptionsMu sync.RWMutex                                       // Protect subscriptions map
}

// NewGRPCMeshNode creates a new gRPC-based mesh node with the given configuration.
// It initializes the core components (EventLog, RoutingTable, PeerLink) but does not start them.
// Call Start() to begin operation.
func NewGRPCMeshNode(config *Config) (*GRPCMeshNode, error) {
	if config == nil {
		return nil, fmt.Errorf("config cannot be nil")
	}

	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	// Create EventLog component
	// For MVP, use in-memory implementation
	eventLog := eventlog.NewInMemoryEventLog()

	// Create RoutingTable component
	// For MVP, use in-memory implementation
	routingTable := routingtable.NewInMemoryRoutingTable()

	// Create PeerLink component
	var peerLinkConfig *peerlink.Config
	if config.PeerLinkConfig != nil {
		peerLinkConfig = config.PeerLinkConfig
	} else {
		// Create default PeerLink config
		peerLinkConfig = &peerlink.Config{
			NodeID:        config.NodeID,
			ListenAddress: config.ListenAddress,
		}
		peerLinkConfig.SetDefaults()
	}

	peerLink, err := peerlink.NewGRPCPeerLink(peerLinkConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create PeerLink: %w", err)
	}

	node := &GRPCMeshNode{
		config:        config,
		eventLog:      eventLog,
		routingTable:  routingTable,
		peerLink:      peerLink,
		clients:       make(map[string]meshnode.Client),
		subscriptions: make(map[string]map[string]*meshnode.ClientSubscription),
		started:       false,
		closed:        false,
	}

	return node, nil
}

// Start initializes and starts the mesh node services.
// This starts the EventLog, RoutingTable, and PeerLink components.
func (n *GRPCMeshNode) Start(ctx context.Context) error {
	n.mu.Lock()
	defer n.mu.Unlock()

	if n.closed {
		return fmt.Errorf("cannot start closed mesh node")
	}

	if n.started {
		return nil // Already started, idempotent
	}

	// For MVP: PeerLink doesn't have explicit Start/Stop in interface
	// The components manage their own lifecycle
	// In future phases, we'll add explicit lifecycle management

	// Run discovery if bootstrap configuration is provided
	if n.config.BootstrapConfig != nil && len(n.config.BootstrapConfig.SeedNodes) > 0 {
		if err := n.runDiscovery(ctx); err != nil {
			// Log error but don't fail startup - discovery is best effort
			fmt.Printf("⚠️ Discovery failed: %v\n", err)
		}
	}

	// Start listening for incoming peer events (including subscription events)
	// This implements REQ-MNODE-003: Handle incoming peer subscription notifications
	go n.handleIncomingPeerEvents(ctx)

	n.started = true
	return nil
}

// Stop gracefully shuts down the mesh node and all its services.
func (n *GRPCMeshNode) Stop(ctx context.Context) error {
	n.mu.Lock()
	defer n.mu.Unlock()

	if !n.started {
		return nil // Not started, idempotent
	}

	// For MVP: Components manage their own lifecycle
	// In future phases, we'll add explicit lifecycle coordination

	n.started = false
	return nil
}

// Close closes the mesh node and releases all resources.
// This is equivalent to Stop() but also marks the node as permanently closed.
func (n *GRPCMeshNode) Close() error {
	n.mu.Lock()
	defer n.mu.Unlock()

	if n.closed {
		return nil // Already closed, idempotent
	}

	// Stop if still running
	if n.started {
		// For MVP: Components manage their own lifecycle
		// In future phases, we'll add explicit stop coordination
	}

	// Close components
	if err := n.eventLog.Close(); err != nil {
		return fmt.Errorf("failed to close EventLog: %w", err)
	}

	if err := n.routingTable.Close(); err != nil {
		return fmt.Errorf("failed to close RoutingTable: %w", err)
	}

	if err := n.peerLink.Close(); err != nil {
		return fmt.Errorf("failed to close PeerLink: %w", err)
	}

	n.started = false
	n.closed = true
	return nil
}

// PublishEvent accepts an event from a local client and handles routing.
// Implements REQ-MNODE-002: persists locally before forwarding to peers.
//
// Event Flow (REQ-MNODE-002):
// 1. Validate client and event
// 2. Persist event to local EventLog FIRST (durability)
// 3. Find local subscribers via RoutingTable
// 4. Forward to remote peers via PeerLink (for remote subscribers)
func (n *GRPCMeshNode) PublishEvent(ctx context.Context, client meshnode.Client, event eventlogpkg.EventRecord) error {
	n.mu.RLock()
	defer n.mu.RUnlock()

	if n.closed {
		return fmt.Errorf("cannot publish to closed mesh node")
	}

	if !n.started {
		return fmt.Errorf("cannot publish to stopped mesh node")
	}

	// Validate inputs
	if client == nil {
		return fmt.Errorf("client cannot be nil")
	}
	if event == nil {
		return fmt.Errorf("event cannot be nil")
	}
	if !client.IsAuthenticated() {
		return fmt.Errorf("client is not authenticated")
	}

	// Step 1: PERSIST LOCALLY FIRST (REQ-MNODE-002)
	// This ensures durability before any forwarding occurs
	persistedEvent, err := n.eventLog.AppendToTopic(ctx, event.Topic(), event)
	if err != nil {
		return fmt.Errorf("failed to persist event locally: %w", err)
	}

	// Step 2: Find local subscribers and deliver events
	localSubscribers, err := n.routingTable.GetSubscribers(ctx, event.Topic())
	if err != nil {
		return fmt.Errorf("failed to get local subscribers: %w", err)
	}

	// Deliver to local subscribers
	for _, subscriber := range localSubscribers {
		if subscriber.Type() == routingtablepkg.LocalClient {
			// Try generic event delivery interface first (works for both TrustedClient and HTTPClient)
			if eventReceiver, ok := subscriber.(interface{ DeliverEvent(eventlogpkg.EventRecord) }); ok {
				eventReceiver.DeliverEvent(persistedEvent)
			}
			// Note: In production, we'd handle delivery failures, queuing, etc.
		}
	}

	// Step 3: Forward to remote peers for distributed routing
	// For MVP: Forward to all connected peers (simple approach)
	// In Phase 4.3, we'll implement smarter routing based on peer subscriptions
	connectedPeers, err := n.peerLink.GetConnectedPeers(ctx)
	if err != nil {
		// Log error but don't fail the publish (local delivery succeeded)
		// In production, we'd use structured logging
		_ = err // Failed to get peers, but local delivery succeeded
	} else {
		// Forward event to all connected peers
		for _, peer := range connectedPeers {
			err := n.peerLink.SendEvent(ctx, peer.ID(), persistedEvent)
			if err != nil {
				// Log error but continue with other peers
				// In production, we'd use structured logging and retry logic
				_ = err // Failed to send to this peer, continue with others
			}
		}
	}

	// Success: Event was persisted locally and forwarded to peers
	return nil
}

// Subscribe registers a client's interest in a topic pattern.
// Implements REQ-MNODE-003: propagates subscription to peer nodes.
//
// For Phase 4.2: Implements local subscription only
// For Phase 4.3: Will add peer propagation
func (n *GRPCMeshNode) Subscribe(ctx context.Context, client meshnode.Client, topic string) error {
	n.mu.Lock()
	defer n.mu.Unlock()

	if n.closed {
		return fmt.Errorf("cannot subscribe to closed mesh node")
	}

	if !n.started {
		return fmt.Errorf("cannot subscribe to stopped mesh node")
	}

	// Validate inputs
	if client == nil {
		return fmt.Errorf("client cannot be nil")
	}
	if topic == "" {
		return fmt.Errorf("topic cannot be empty")
	}
	if !client.IsAuthenticated() {
		return fmt.Errorf("client is not authenticated")
	}

	// Cast client to subscriber (TrustedClient implements both interfaces)
	subscriber, ok := client.(routingtablepkg.Subscriber)
	if !ok {
		return fmt.Errorf("client does not implement Subscriber interface")
	}

	// Add to local routing table
	err := n.routingTable.Subscribe(ctx, topic, subscriber)
	if err != nil {
		return fmt.Errorf("failed to add local subscription: %w", err)
	}

	// Add to local client tracking
	n.clients[client.ID()] = client

	// Store subscription metadata for HTTP API
	// Generate a simple subscription ID for this client/topic pair
	subscriptionID := fmt.Sprintf("sub-%s-%d", client.ID(), time.Now().UnixNano())
	n.addSubscriptionMetadata(client.ID(), subscriptionID, topic)

	// Propagate subscription to peer nodes (REQ-MNODE-003)
	// For MVP: Use special subscription events via existing PeerLink infrastructure
	err = n.propagateSubscriptionChange(ctx, "subscribe", client.ID(), topic)
	if err != nil {
		// Log error but don't fail the local subscription
		// In production, we'd use structured logging
		_ = err // Failed to propagate, but local subscription succeeded
	}

	return nil
}

// Unsubscribe removes a client's subscription to a topic pattern.
// Updates local routing table and notifies peer nodes.
//
// For Phase 4.3: Implements local unsubscription
// Future: Will add peer propagation in subsequent tasks
func (n *GRPCMeshNode) Unsubscribe(ctx context.Context, client meshnode.Client, topic string) error {
	n.mu.Lock()
	defer n.mu.Unlock()

	if n.closed {
		return fmt.Errorf("cannot unsubscribe from closed mesh node")
	}

	if !n.started {
		return fmt.Errorf("cannot unsubscribe from stopped mesh node")
	}

	// Validate inputs
	if client == nil {
		return fmt.Errorf("client cannot be nil")
	}
	if topic == "" {
		return fmt.Errorf("topic cannot be empty")
	}
	if !client.IsAuthenticated() {
		return fmt.Errorf("client is not authenticated")
	}

	// Remove from local routing table
	err := n.routingTable.Unsubscribe(ctx, topic, client.ID())
	if err != nil {
		return fmt.Errorf("failed to remove local subscription: %w", err)
	}

	// Remove from local client tracking if this was the last subscription
	// For MVP: Keep client in tracking (simple approach)
	// In production, we'd track subscription counts and remove if zero

	// Propagate unsubscription to peer nodes (REQ-MNODE-003)
	// For MVP: Use special subscription events via existing PeerLink infrastructure
	err = n.propagateSubscriptionChange(ctx, "unsubscribe", client.ID(), topic)
	if err != nil {
		// Log error but don't fail the local unsubscription
		// In production, we'd use structured logging
		_ = err // Failed to propagate, but local unsubscription succeeded
	}

	return nil
}

// AuthenticateClient validates and authenticates a connecting client.
// FOR MVP: Creates a TrustedClient (REQ-MNODE-001 descoped from MVP)
func (n *GRPCMeshNode) AuthenticateClient(ctx context.Context, credentials interface{}) (meshnode.Client, error) {
	n.mu.Lock()
	defer n.mu.Unlock()

	if n.closed {
		return nil, fmt.Errorf("cannot authenticate to closed mesh node")
	}

	if credentials == nil {
		return nil, fmt.Errorf("credentials cannot be nil")
	}

	// For MVP: credentials is expected to be a client ID string
	clientID, ok := credentials.(string)
	if !ok {
		return nil, fmt.Errorf("credentials must be a string client ID")
	}

	if clientID == "" {
		return nil, fmt.Errorf("client ID cannot be empty")
	}

	// Check if client already exists
	if existingClient, exists := n.clients[clientID]; exists {
		return existingClient, nil // Return existing client
	}

	// Create new TrustedClient
	client := NewTrustedClient(clientID)

	// Add to client tracking
	n.clients[clientID] = client

	return client, nil
}

// GetEventLog returns the node's event log interface.
func (n *GRPCMeshNode) GetEventLog() eventlogpkg.EventLog {
	n.mu.RLock()
	defer n.mu.RUnlock()
	return n.eventLog
}

// GetRoutingTable returns the node's routing table interface.
func (n *GRPCMeshNode) GetRoutingTable() routingtablepkg.RoutingTable {
	n.mu.RLock()
	defer n.mu.RUnlock()
	return n.routingTable
}

// GetPeerLink returns the node's peer link interface.
func (n *GRPCMeshNode) GetPeerLink() peerlinkpkg.PeerLink {
	n.mu.RLock()
	defer n.mu.RUnlock()
	return n.peerLink
}

// GetNodeID returns this node's unique identifier in the mesh.
func (n *GRPCMeshNode) GetNodeID() string {
	n.mu.RLock()
	defer n.mu.RUnlock()
	return n.config.NodeID
}

// GetConnectedClients returns all currently connected clients.
// FOR MVP: This is a stub implementation that will be completed in Phase 4.4
func (n *GRPCMeshNode) GetConnectedClients(ctx context.Context) ([]meshnode.Client, error) {
	n.mu.RLock()
	defer n.mu.RUnlock()

	clients := make([]meshnode.Client, 0, len(n.clients))
	for _, client := range n.clients {
		clients = append(clients, client)
	}
	return clients, nil
}

// GetConnectedPeers returns all currently connected peer nodes.
func (n *GRPCMeshNode) GetConnectedPeers(ctx context.Context) ([]peerlinkpkg.PeerNode, error) {
	n.mu.RLock()
	defer n.mu.RUnlock()
	return n.peerLink.GetConnectedPeers(ctx)
}

// GetHealth returns the overall health status of this mesh node.
// Performs comprehensive health checks on all components and provides detailed status.
func (n *GRPCMeshNode) GetHealth(ctx context.Context) (meshnode.HealthStatus, error) {
	n.mu.RLock()
	defer n.mu.RUnlock()

	var healthMessages []string

	// Check node lifecycle state
	nodeHealthy := !n.closed
	if n.closed {
		healthMessages = append(healthMessages, "node is closed")
	}
	if !n.started {
		healthMessages = append(healthMessages, "node is not started")
	}

	// Check EventLog health - test basic operations
	eventLogHealthy := true
	if n.closed {
		// If node is closed, components are closed and unhealthy
		eventLogHealthy = false
		healthMessages = append(healthMessages, "EventLog unhealthy: node is closed")
	} else if n.eventLog != nil {
		// Test EventLog by checking if we can create a simple operation
		testEvent := eventlogpkg.NewRecord("__health.check", []byte("health-check"))
		_, err := n.eventLog.AppendToTopic(ctx, "__health.check", testEvent)
		if err != nil {
			eventLogHealthy = false
			healthMessages = append(healthMessages, fmt.Sprintf("EventLog unhealthy: %v", err))
		}
	} else {
		eventLogHealthy = false
		healthMessages = append(healthMessages, "EventLog is nil")
	}

	// Check RoutingTable health - test basic operations
	routingTableHealthy := true
	if n.closed {
		// If node is closed, components are closed and unhealthy
		routingTableHealthy = false
		healthMessages = append(healthMessages, "RoutingTable unhealthy: node is closed")
	} else if n.routingTable != nil {
		// Test RoutingTable by checking if we can perform a lookup
		_, err := n.routingTable.GetSubscribers(ctx, "__health.check")
		if err != nil {
			routingTableHealthy = false
			healthMessages = append(healthMessages, fmt.Sprintf("RoutingTable unhealthy: %v", err))
		}
	} else {
		routingTableHealthy = false
		healthMessages = append(healthMessages, "RoutingTable is nil")
	}

	// Check PeerLink health - test connectivity
	peerLinkHealthy := true
	var peers []peerlinkpkg.PeerNode
	if n.closed {
		// If node is closed, components are closed and unhealthy
		peerLinkHealthy = false
		healthMessages = append(healthMessages, "PeerLink unhealthy: node is closed")
		peers = []peerlinkpkg.PeerNode{}
	} else if n.peerLink != nil {
		var err error
		peers, err = n.peerLink.GetConnectedPeers(ctx)
		if err != nil {
			peerLinkHealthy = false
			healthMessages = append(healthMessages, fmt.Sprintf("PeerLink unhealthy: %v", err))
			peers = []peerlinkpkg.PeerNode{} // Ensure we have a valid slice
		}
	} else {
		peerLinkHealthy = false
		healthMessages = append(healthMessages, "PeerLink is nil")
		peers = []peerlinkpkg.PeerNode{}
	}

	// Overall health: all components must be healthy
	overallHealthy := nodeHealthy && eventLogHealthy && routingTableHealthy && peerLinkHealthy

	// Construct health message
	var message string
	if len(healthMessages) == 0 {
		message = "All components healthy"
	} else {
		message = "Issues detected: " + fmt.Sprintf("%v", healthMessages)
	}

	return meshnode.HealthStatus{
		Healthy:             overallHealthy,
		EventLogHealthy:     eventLogHealthy,
		RoutingTableHealthy: routingTableHealthy,
		PeerLinkHealthy:     peerLinkHealthy,
		ConnectedClients:    len(n.clients),
		ConnectedPeers:      len(peers),
		Message:             message,
	}, nil
}

// propagateSubscriptionChange sends subscription changes to all connected peers
// For MVP: Uses special subscription events via existing PeerLink infrastructure
// This implements REQ-MNODE-003: Subscription Propagation
func (n *GRPCMeshNode) propagateSubscriptionChange(ctx context.Context, action, clientID, topic string) error {
	// Get connected peers
	connectedPeers, err := n.peerLink.GetConnectedPeers(ctx)
	if err != nil {
		return fmt.Errorf("failed to get connected peers: %w", err)
	}

	if len(connectedPeers) == 0 {
		// No peers to notify, not an error
		return nil
	}

	// Create subscription change event
	// For MVP: Use JSON payload with subscription information
	// In production, we'd use structured protobuf messages
	payloadBytes := []byte(fmt.Sprintf(`{"action":"%s","clientID":"%s","topic":"%s","nodeID":"%s"}`,
		action, clientID, topic, n.config.NodeID))

	// Create subscription event with special topic prefix
	subscriptionEvent := eventlogpkg.NewRecord(
		fmt.Sprintf("__mesh.subscription.%s", action), // Special topic for subscription changes
		payloadBytes,
	)

	// Send to all connected peers
	for _, peer := range connectedPeers {
		err := n.peerLink.SendEvent(ctx, peer.ID(), subscriptionEvent)
		if err != nil {
			// Log error but continue with other peers
			// In production, we'd use structured logging and retry logic
			_ = err // Failed to send to this peer, continue with others
		}
	}

	return nil
}

// handleIncomingPeerEvents processes events received from peer nodes
// This handles both regular events and subscription events (REQ-MNODE-003)
func (n *GRPCMeshNode) handleIncomingPeerEvents(ctx context.Context) {
	// Get event channel from PeerLink
	eventChan, errChan := n.peerLink.ReceiveEvents(ctx)

	for {
		select {
		case <-ctx.Done():
			// Context cancelled, stop processing
			return

		case event, ok := <-eventChan:
			if !ok {
				// Event channel closed, stop processing
				return
			}

			// Process the incoming event
			n.processIncomingEvent(ctx, event)

		case err, ok := <-errChan:
			if !ok {
				// Error channel closed, stop processing
				return
			}
			if err != nil {
				// Log error but continue processing
				// In production, we'd use structured logging
				_ = err // Error receiving events from peers
			}
		}
	}
}

// processIncomingEvent handles a single incoming event from a peer
func (n *GRPCMeshNode) processIncomingEvent(ctx context.Context, event eventlogpkg.EventRecord) {
	topic := event.Topic()

	// Check if this is a subscription event
	if n.isSubscriptionEvent(topic) {
		n.handleSubscriptionEvent(ctx, event)
		return
	}

	// This is a regular event - persist locally and deliver to local subscribers
	n.mu.RLock()
	if n.closed || !n.started {
		n.mu.RUnlock()
		return // Node is not running, ignore event
	}
	n.mu.RUnlock()

	// Persist the event locally (we received it from a peer)
	persistedEvent, err := n.eventLog.AppendToTopic(ctx, topic, event)
	if err != nil {
		// Log error but continue
		// In production, we'd use structured logging
		_ = err // Failed to persist incoming event
		return
	}

	// Deliver to local subscribers
	n.mu.RLock()
	defer n.mu.RUnlock()

	localSubscribers, err := n.routingTable.GetSubscribers(ctx, topic)
	if err != nil {
		// Log error but continue
		_ = err // Failed to get local subscribers
		return
	}

	for _, subscriber := range localSubscribers {
		if subscriber.Type() == routingtablepkg.LocalClient {
			// Try generic event delivery interface (works for both TrustedClient and HTTPClient)
			if eventReceiver, ok := subscriber.(interface{ DeliverEvent(eventlogpkg.EventRecord) }); ok {
				eventReceiver.DeliverEvent(persistedEvent)
			}
		}
	}
}

// isSubscriptionEvent checks if an event is a subscription management event
func (n *GRPCMeshNode) isSubscriptionEvent(topic string) bool {
	return topic == "__mesh.subscription.subscribe" || topic == "__mesh.subscription.unsubscribe"
}

// handleSubscriptionEvent processes subscription events from peers
// This implements REQ-MNODE-003: Handle incoming peer subscription notifications
func (n *GRPCMeshNode) handleSubscriptionEvent(ctx context.Context, event eventlogpkg.EventRecord) {
	// For MVP: Just log that we received a subscription event
	// In a full implementation, we'd:
	// 1. Parse the JSON payload to extract subscription info
	// 2. Update our knowledge of peer subscriptions
	// 3. Use this info for smarter routing decisions

	// Simple implementation: extract basic info for logging
	payload := string(event.Payload())
	_ = payload // Subscription event received from peer

	// In production, we'd:
	// - Parse JSON to get action, clientID, topic, nodeID
	// - Update peer subscription tracking
	// - Use for intelligent routing instead of broadcasting to all peers
}

// Helper methods for subscription metadata management

// addSubscriptionMetadata adds a subscription to the metadata store
func (n *GRPCMeshNode) addSubscriptionMetadata(clientID, subscriptionID, topic string) {
	n.subscriptionsMu.Lock()
	defer n.subscriptionsMu.Unlock()

	// Initialize client map if it doesn't exist
	if n.subscriptions[clientID] == nil {
		n.subscriptions[clientID] = make(map[string]*meshnode.ClientSubscription)
	}

	// Create and store subscription
	subscription := &meshnode.ClientSubscription{
		ID:        subscriptionID,
		Topic:     topic,
		ClientID:  clientID,
		CreatedAt: time.Now(),
	}

	n.subscriptions[clientID][subscriptionID] = subscription
}

// GetClientSubscriptions returns all subscriptions for a specific client
func (n *GRPCMeshNode) GetClientSubscriptions(ctx context.Context, clientID string) ([]meshnode.ClientSubscription, error) {
	n.subscriptionsMu.RLock()
	defer n.subscriptionsMu.RUnlock()

	clientSubs := n.subscriptions[clientID]
	if clientSubs == nil {
		return []meshnode.ClientSubscription{}, nil
	}

	// Convert to slice with value copies (not pointers)
	var result []meshnode.ClientSubscription
	for _, sub := range clientSubs {
		result = append(result, *sub)
	}

	return result, nil
}

// UnsubscribeByID removes a client's subscription by subscription ID
func (n *GRPCMeshNode) UnsubscribeByID(ctx context.Context, clientID, subscriptionID string) error {
	n.subscriptionsMu.RLock()
	// Find the subscription first to get the topic
	clientSubs := n.subscriptions[clientID]
	if clientSubs == nil {
		n.subscriptionsMu.RUnlock()
		return fmt.Errorf("no subscriptions found for client %s", clientID)
	}

	subscription, exists := clientSubs[subscriptionID]
	if !exists {
		n.subscriptionsMu.RUnlock()
		return fmt.Errorf("subscription %s not found for client %s", subscriptionID, clientID)
	}

	topic := subscription.Topic
	n.subscriptionsMu.RUnlock()

	// Get the client to call the regular Unsubscribe method
	n.mu.RLock()
	client := n.clients[clientID]
	n.mu.RUnlock()

	if client == nil {
		return fmt.Errorf("client %s not found", clientID)
	}

	// Call the regular Unsubscribe method to handle routing table and peer propagation
	err := n.Unsubscribe(ctx, client, topic)
	if err != nil {
		return fmt.Errorf("failed to unsubscribe: %w", err)
	}

	// Remove subscription metadata
	return n.removeSubscriptionMetadata(clientID, subscriptionID)
}

// removeSubscriptionMetadata removes a subscription from the metadata store
func (n *GRPCMeshNode) removeSubscriptionMetadata(clientID, subscriptionID string) error {
	n.subscriptionsMu.Lock()
	defer n.subscriptionsMu.Unlock()

	clientSubs := n.subscriptions[clientID]
	if clientSubs == nil {
		return fmt.Errorf("no subscriptions found for client %s", clientID)
	}

	_, exists := clientSubs[subscriptionID]
	if !exists {
		return fmt.Errorf("subscription %s not found for client %s", subscriptionID, clientID)
	}

	delete(clientSubs, subscriptionID)

	// Clean up empty client map
	if len(clientSubs) == 0 {
		delete(n.subscriptions, clientID)
	}

	return nil
}

// runDiscovery performs node discovery and connects to discovered peers
func (n *GRPCMeshNode) runDiscovery(ctx context.Context) error {
	// Create static discovery service from bootstrap config
	discoveryService := discovery.NewStaticDiscovery(n.config.BootstrapConfig.SeedNodes)

	// Find peer nodes
	peers, err := discoveryService.FindPeers(ctx)
	if err != nil {
		return fmt.Errorf("discovery failed: %w", err)
	}

	fmt.Printf("🔍 Discovery found %d peers\n", len(peers))

	// Connect to discovered peers
	connectedCount := 0
	for _, peer := range peers {
		fmt.Printf("   Connecting to peer: %s\n", peer.Address())

		// Attempt to connect to the peer
		err := n.peerLink.Connect(ctx, peer)
		if err != nil {
			// Log error but continue with other peers
			fmt.Printf("   ⚠️ Failed to connect to %s: %v\n", peer.Address(), err)
			continue
		}

		connectedCount++
		fmt.Printf("   ✅ Connected to peer: %s\n", peer.Address())
	}

	fmt.Printf("🌐 Discovery complete: %d/%d peers connected\n", connectedCount, len(peers))
	return nil
}

// Verify that GRPCMeshNode implements the MeshNode interface at compile time
var _ meshnode.MeshNode = (*GRPCMeshNode)(nil)
