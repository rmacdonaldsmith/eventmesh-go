package meshnode

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/rmacdonaldsmith/eventmesh-go/internal/peerlink"
)

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

	slog.Info("starting mesh node",
		"node_id", n.config.NodeID,
		"listen_address", n.config.ListenAddress)

	// Start PeerLink to listen for incoming peer connections
	if grpcPeerLink, ok := n.peerConnections.(*peerlink.GRPCPeerLink); ok {
		if err := grpcPeerLink.Start(ctx); err != nil {
			return fmt.Errorf("failed to start PeerLink: %w", err)
		}
	}

	// Run discovery if bootstrap configuration is provided
	if n.config.BootstrapConfig != nil && len(n.config.BootstrapConfig.SeedNodes) > 0 {
		if err := n.runDiscovery(ctx); err != nil {
			slog.Warn("discovery failed",
				"node_id", n.config.NodeID,
				"error", err)
		}
	}

	lifecycleCtx, lifecycleCancel := context.WithCancel(context.Background())

	if err := n.controlPlanePeerLink.StartHeartbeats(lifecycleCtx); err != nil {
		lifecycleCancel()
		return fmt.Errorf("failed to start peer heartbeats: %w", err)
	}

	// Start listening for incoming peer events (including subscription events)
	// This implements REQ-MNODE-003: Handle incoming peer subscription notifications
	go n.handleIncomingPeerEvents(lifecycleCtx)
	go n.resyncSubscriptionsToNewPeers(lifecycleCtx)

	n.lifecycleCancel = lifecycleCancel
	n.started = true
	slog.Info("mesh node ready",
		"node_id", n.config.NodeID,
		"listen_address", n.config.ListenAddress)
	return nil
}

// Stop gracefully shuts down the mesh node and all its services.
func (n *GRPCMeshNode) Stop(ctx context.Context) error {
	n.mu.Lock()
	if !n.started {
		n.mu.Unlock()
		return nil // Not started, idempotent
	}

	controlPlane := n.controlPlanePeerLink
	peerConnections := n.peerConnections
	cancel := n.lifecycleCancel
	n.lifecycleCancel = nil
	n.started = false
	n.mu.Unlock()

	if cancel != nil {
		cancel()
	}

	if err := controlPlane.StopHeartbeats(ctx); err != nil {
		return fmt.Errorf("failed to stop peer heartbeats: %w", err)
	}

	if stopper, ok := peerConnections.(interface {
		Stop(context.Context) error
	}); ok {
		if err := stopper.Stop(ctx); err != nil {
			return fmt.Errorf("failed to stop PeerLink: %w", err)
		}
	}

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

	// Stop if still running (empty branch is intentional - components manage lifecycle)
	_ = n.started // For future use in explicit stop coordination

	// Close components
	if err := n.eventLog.Close(); err != nil {
		return fmt.Errorf("failed to close EventLog: %w", err)
	}

	if err := n.routingTable.Close(); err != nil {
		return fmt.Errorf("failed to close RoutingTable: %w", err)
	}

	if err := n.peerConnections.Close(); err != nil {
		return fmt.Errorf("failed to close PeerLink: %w", err)
	}

	n.started = false
	n.closed = true
	return nil
}
