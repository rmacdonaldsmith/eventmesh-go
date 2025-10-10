package meshnode

import (
	"context"
	"fmt"
	"sync"

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
		config:       config,
		eventLog:     eventLog,
		routingTable: routingTable,
		peerLink:     peerLink,
		clients:      make(map[string]meshnode.Client),
		started:      false,
		closed:       false,
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
			// Cast to TrustedClient for delivery
			if trustedClient, ok := subscriber.(*TrustedClient); ok {
				trustedClient.DeliverEvent(persistedEvent)
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

	// TODO Phase 4.3: Propagate subscription to peer nodes via PeerLink

	return nil
}

// Unsubscribe removes a client's subscription to a topic pattern.
// Updates local routing table and notifies peer nodes.
// FOR MVP: This is a stub implementation that will be completed in Phase 4.3
func (n *GRPCMeshNode) Unsubscribe(ctx context.Context, client meshnode.Client, topic string) error {
	return fmt.Errorf("Unsubscribe not implemented yet - will be implemented in Phase 4.3")
}

// AuthenticateClient validates and authenticates a connecting client.
// FOR MVP: This is a stub implementation (REQ-MNODE-001 descoped from MVP)
func (n *GRPCMeshNode) AuthenticateClient(ctx context.Context, credentials interface{}) (meshnode.Client, error) {
	return nil, fmt.Errorf("AuthenticateClient not implemented yet - will be implemented in Phase 4.4")
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
// FOR MVP: This is a stub implementation that will be completed in Phase 4.5
func (n *GRPCMeshNode) GetHealth(ctx context.Context) (meshnode.HealthStatus, error) {
	n.mu.RLock()
	defer n.mu.RUnlock()

	// Basic health check - if we're not closed, we're healthy
	healthy := !n.closed

	// Get connected peers count
	peers, err := n.peerLink.GetConnectedPeers(ctx)
	if err != nil {
		// If we can't get peers, still report health but note the issue
		peers = []peerlinkpkg.PeerNode{}
	}

	return meshnode.HealthStatus{
		Healthy:              healthy,
		EventLogHealthy:      healthy,
		RoutingTableHealthy:  healthy,
		PeerLinkHealthy:      healthy && err == nil,
		ConnectedClients:     len(n.clients),
		ConnectedPeers:       len(peers),
		Message:              "Basic health check - detailed implementation in Phase 4.5",
	}, nil
}

// Verify that GRPCMeshNode implements the MeshNode interface at compile time
var _ meshnode.MeshNode = (*GRPCMeshNode)(nil)