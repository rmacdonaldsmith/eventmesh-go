package peerlink

import (
	"context"
	"errors"
	"net"
	"sync"

	"github.com/rmacdonaldsmith/eventmesh-go/pkg/eventlog"
	"github.com/rmacdonaldsmith/eventmesh-go/pkg/peerlink"
	peerlinkv1 "github.com/rmacdonaldsmith/eventmesh-go/proto/peerlink/v1"
	"google.golang.org/grpc"
)

// GRPCPeerLink implements the PeerLink interface using gRPC for peer-to-peer communication
type GRPCPeerLink struct {
	peerlinkv1.UnimplementedPeerLinkServer
	config          *Config
	closed          bool
	mu              sync.RWMutex
	grpcServer      *grpc.Server
	listener        net.Listener
	started         bool
	connectedPeers  map[string]peerlink.PeerNode // peerID -> PeerNode
}

// NewGRPCPeerLink creates a new GRPCPeerLink with the given configuration
func NewGRPCPeerLink(config *Config) (*GRPCPeerLink, error) {
	if err := config.Validate(); err != nil {
		return nil, err
	}

	// Make a copy and set defaults
	configCopy := *config
	configCopy.SetDefaults()

	return &GRPCPeerLink{
		config:         &configCopy,
		closed:         false,
		started:        false,
		connectedPeers: make(map[string]peerlink.PeerNode),
	}, nil
}

// Start initializes the gRPC server and begins listening for connections
func (g *GRPCPeerLink) Start(ctx context.Context) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	if g.closed {
		return errors.New("PeerLink is closed")
	}

	if g.started {
		return nil // Already started, safe to call multiple times
	}

	// Create listener
	listener, err := net.Listen("tcp", g.config.ListenAddress)
	if err != nil {
		return err
	}

	// Create gRPC server
	grpcServer := grpc.NewServer()

	// Register the PeerLink service (we'll implement the service handler later)
	peerlinkv1.RegisterPeerLinkServer(grpcServer, g)

	// Store server references
	g.listener = listener
	g.grpcServer = grpcServer
	g.started = true

	// Start serving in a goroutine
	go func() {
		if err := grpcServer.Serve(listener); err != nil {
			// Log error but don't panic - server shutdown is normal
			// TODO: Add proper logging when we have a logger
		}
	}()

	return nil
}

// Stop gracefully shuts down the gRPC server
func (g *GRPCPeerLink) Stop(ctx context.Context) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	if !g.started || g.grpcServer == nil {
		return nil // Not started or already stopped
	}

	// Graceful shutdown with context
	done := make(chan struct{})
	go func() {
		g.grpcServer.GracefulStop()
		close(done)
	}()

	select {
	case <-done:
		// Graceful shutdown completed
	case <-ctx.Done():
		// Context timeout - force shutdown
		g.grpcServer.Stop()
	}

	// Clean up references
	g.grpcServer = nil
	g.listener = nil
	g.started = false

	return nil
}

// EventStream implements the PeerLink gRPC service handler
func (g *GRPCPeerLink) EventStream(stream grpc.BidiStreamingServer[peerlinkv1.PeerMessage, peerlinkv1.PeerMessage]) error {
	// TODO: Implement full protocol logic in later phases
	// For now, just return "not implemented" to satisfy the interface
	return errors.New("EventStream not fully implemented yet")
}

// Connect establishes a secure connection to the specified peer node
func (g *GRPCPeerLink) Connect(ctx context.Context, peer peerlink.PeerNode) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	if g.closed {
		return errors.New("PeerLink is closed")
	}

	// For Phase 3.2 stub: just add to connected peers map without actual connection
	// Real connection logic will be implemented in later phases
	g.connectedPeers[peer.ID()] = peer

	return nil
}

// Disconnect closes the connection to the specified peer node
func (g *GRPCPeerLink) Disconnect(ctx context.Context, peerID string) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	if g.closed {
		return errors.New("PeerLink is closed")
	}

	// For Phase 3.2 stub: just remove from connected peers map
	// Real disconnection logic will be implemented in later phases
	delete(g.connectedPeers, peerID)

	return nil
}

// SendEvent streams an event to the specified peer node
func (g *GRPCPeerLink) SendEvent(ctx context.Context, peerID string, event eventlog.EventRecord) error {
	g.mu.RLock()
	defer g.mu.RUnlock()

	if g.closed {
		return errors.New("PeerLink is closed")
	}

	// For Phase 3.2 stub: just return success without actually sending
	// Real event sending logic will be implemented in later phases
	// This validates that the peer exists in our connected peers map
	if _, exists := g.connectedPeers[peerID]; !exists {
		// For stub mode, we'll be lenient and allow sending to non-connected peers
		// Real implementation would return an error here
	}

	return nil
}

// ReceiveEvents returns a channel for receiving events from peer nodes
func (g *GRPCPeerLink) ReceiveEvents(ctx context.Context) (<-chan eventlog.EventRecord, <-chan error) {
	eventChan := make(chan eventlog.EventRecord)
	errChan := make(chan error, 1)
	close(eventChan)
	errChan <- errors.New("not implemented")
	return eventChan, errChan
}

// GetConnectedPeers returns all currently connected peer nodes
func (g *GRPCPeerLink) GetConnectedPeers(ctx context.Context) ([]peerlink.PeerNode, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()

	if g.closed {
		return nil, errors.New("PeerLink is closed")
	}

	// Convert map to slice
	peers := make([]peerlink.PeerNode, 0, len(g.connectedPeers))
	for _, peer := range g.connectedPeers {
		peers = append(peers, peer)
	}

	return peers, nil
}

// GetPeerHealth returns health status for a specific peer node
func (g *GRPCPeerLink) GetPeerHealth(ctx context.Context, peerID string) (bool, error) {
	return false, errors.New("not implemented")
}

// StartHeartbeats begins health monitoring for all connected peers
func (g *GRPCPeerLink) StartHeartbeats(ctx context.Context) error {
	return errors.New("not implemented")
}

// StopHeartbeats stops health monitoring
func (g *GRPCPeerLink) StopHeartbeats(ctx context.Context) error {
	return errors.New("not implemented")
}

// Close closes the PeerLink and cleans up resources
func (g *GRPCPeerLink) Close() error {
	g.mu.Lock()
	defer g.mu.Unlock()

	if g.closed {
		return nil // Already closed, safe to call multiple times
	}

	// Stop the server if it's running
	if g.started && g.grpcServer != nil {
		g.grpcServer.Stop() // Force stop since we don't have context
		g.grpcServer = nil
		g.listener = nil
		g.started = false
	}

	g.closed = true
	return nil
}