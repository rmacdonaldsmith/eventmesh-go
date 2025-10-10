package peerlink

import (
	"context"
	"errors"
	"net"
	"sync"
	"time"

	"github.com/rmacdonaldsmith/eventmesh-go/pkg/eventlog"
	"github.com/rmacdonaldsmith/eventmesh-go/pkg/peerlink"
	peerlinkv1 "github.com/rmacdonaldsmith/eventmesh-go/proto/peerlink/v1"
	"google.golang.org/grpc"
)

const (
	// ProtocolVersion represents the current PeerLink protocol version (v1.0)
	ProtocolVersion uint32 = 100
)

// queuedMessage represents a message in the send queue
type queuedMessage struct {
	peerID string
	event  eventlog.EventRecord
	sentAt time.Time
}

// peerMetrics tracks basic metrics per peer
type peerMetrics struct {
	dropsCount     int64                      // Number of dropped messages due to queue full
	healthState    peerlink.PeerHealthState   // Current health state
	failureCount   int                        // Number of consecutive failures
}

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
	sendQueues      map[string]chan queuedMessage // peerID -> send queue
	metrics         map[string]*peerMetrics       // peerID -> metrics
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
		sendQueues:     make(map[string]chan queuedMessage),
		metrics:        make(map[string]*peerMetrics),
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
	// Basic EventStream implementation for MVP
	ctx := stream.Context()

	// Wait for handshake message
	msg, err := stream.Recv()
	if err != nil {
		return err
	}

	// Validate handshake
	handshake := msg.GetHandshake()
	if handshake == nil {
		return errors.New("expected handshake message")
	}

	peerID := handshake.NodeId
	if peerID == "" {
		return errors.New("peer ID cannot be empty")
	}

	// Register this peer as connected (create send queue if needed)
	g.mu.Lock()
	g.registerPeer(peerID)
	sendQueue := g.sendQueues[peerID]
	g.mu.Unlock()

	// Send handshake response
	response := &peerlinkv1.PeerMessage{
		Kind: &peerlinkv1.PeerMessage_Handshake{
			Handshake: &peerlinkv1.Handshake{
				NodeId:          g.config.NodeID,
				ProtocolVersion: ProtocolVersion,
			},
		},
	}

	if err := stream.Send(response); err != nil {
		return err
	}

	// Ensure cleanup on stream close
	defer func() {
		g.mu.Lock()
		if metrics, exists := g.metrics[peerID]; exists {
			metrics.healthState = peerlink.PeerDisconnected
		}
		g.mu.Unlock()
	}()

	// Handle ongoing message flow
	for {
		select {
		case <-ctx.Done():
			// Stream closed
			return ctx.Err()

		case queuedMsg, ok := <-sendQueue:
			if !ok {
				// Send queue closed
				return nil
			}

			// Convert EventRecord to protobuf Event message
			event := &peerlinkv1.Event{
				Topic:   queuedMsg.event.Topic(),
				Payload: queuedMsg.event.Payload(),
				Headers: queuedMsg.event.Headers(),
				Offset:  queuedMsg.event.Offset(),
			}

			// Wrap in PeerMessage
			peerMsg := &peerlinkv1.PeerMessage{
				Kind: &peerlinkv1.PeerMessage_Event{
					Event: event,
				},
			}

			// Send over gRPC stream
			if err := stream.Send(peerMsg); err != nil {
				// Failed to send - put message back in queue if possible
				// For MVP, just return the error
				return err
			}
		}
	}
}

// Connect establishes a secure connection to the specified peer node
func (g *GRPCPeerLink) Connect(ctx context.Context, peer peerlink.PeerNode) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	if g.closed {
		return errors.New("PeerLink is closed")
	}

	peerID := peer.ID()

	// Add to connected peers map
	g.connectedPeers[peerID] = peer

	// Register peer (create queue and metrics)
	g.registerPeer(peerID)

	return nil
}

// Disconnect closes the connection to the specified peer node
func (g *GRPCPeerLink) Disconnect(ctx context.Context, peerID string) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	if g.closed {
		return errors.New("PeerLink is closed")
	}

	// Remove from connected peers map
	delete(g.connectedPeers, peerID)

	// Close and remove send queue
	if queue, exists := g.sendQueues[peerID]; exists {
		close(queue)
		delete(g.sendQueues, peerID)
	}

	// Set health state to Disconnected and keep metrics for observability
	if metrics, exists := g.metrics[peerID]; exists {
		oldState := metrics.healthState
		if oldState != peerlink.PeerDisconnected {
			metrics.healthState = peerlink.PeerDisconnected
			// Basic logging of state transitions
			// TODO: Replace with proper logger when available
			// fmt.Printf("Peer %s health transition: %s -> %s\n", peerID, oldState, PeerDisconnected)
		}
	}

	return nil
}

// SendEvent streams an event to the specified peer node
func (g *GRPCPeerLink) SendEvent(ctx context.Context, peerID string, event eventlog.EventRecord) error {
	g.mu.RLock()
	if g.closed {
		g.mu.RUnlock()
		return errors.New("PeerLink is closed")
	}

	// Get the send queue for this peer
	queue, queueExists := g.sendQueues[peerID]
	metrics, metricsExist := g.metrics[peerID]
	g.mu.RUnlock()

	if !queueExists {
		return errors.New("peer not connected")
	}

	// Create queued message
	msg := queuedMessage{
		peerID: peerID,
		event:  event,
		sentAt: time.Now(),
	}

	// Try to send to queue with timeout (bounded queue behavior)
	timeoutCtx, cancel := context.WithTimeout(ctx, g.config.SendTimeout)
	defer cancel()

	select {
	case queue <- msg:
		// Successfully queued
		return nil
	case <-timeoutCtx.Done():
		// Queue is full and timeout expired - increment drops counter
		if metricsExist {
			g.mu.Lock()
			metrics.dropsCount++
			g.mu.Unlock()
		}
		return errors.New("send queue full - message dropped")
	}
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
func (g *GRPCPeerLink) GetPeerHealth(ctx context.Context, peerID string) (peerlink.PeerHealthState, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()

	if metrics, exists := g.metrics[peerID]; exists {
		return metrics.healthState, nil
	}
	return peerlink.PeerDisconnected, errors.New("peer not found")
}

// registerPeer creates send queue and metrics for a peer if they don't exist
// Must be called with mutex held
func (g *GRPCPeerLink) registerPeer(peerID string) {
	if _, exists := g.sendQueues[peerID]; !exists {
		g.sendQueues[peerID] = make(chan queuedMessage, g.config.SendQueueSize)
	}
	if _, exists := g.metrics[peerID]; !exists {
		g.metrics[peerID] = &peerMetrics{
			dropsCount:   0,
			healthState:  peerlink.PeerHealthy,
			failureCount: 0,
		}
	}
}

// SetPeerHealth sets the health state for a specific peer with logging
func (g *GRPCPeerLink) SetPeerHealth(peerID string, newState peerlink.PeerHealthState) {
	g.mu.Lock()
	defer g.mu.Unlock()

	if metrics, exists := g.metrics[peerID]; exists {
		oldState := metrics.healthState
		if oldState != newState {
			metrics.healthState = newState
			// Basic logging of state transitions
			// TODO: Replace with proper logger when available
			// fmt.Printf("Peer %s health transition: %s -> %s\n", peerID, oldState, newState)
		}
	}
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

// GetQueueDepth returns the current depth of the send queue for a peer
func (g *GRPCPeerLink) GetQueueDepth(peerID string) int {
	g.mu.RLock()
	defer g.mu.RUnlock()

	if queue, exists := g.sendQueues[peerID]; exists {
		return len(queue)
	}
	return 0
}

// GetDropsCount returns the number of dropped messages for a peer
func (g *GRPCPeerLink) GetDropsCount(peerID string) int64 {
	g.mu.RLock()
	defer g.mu.RUnlock()

	if metrics, exists := g.metrics[peerID]; exists {
		return metrics.dropsCount
	}
	return 0
}

// PeerMetrics represents metrics for a single peer
type PeerMetrics struct {
	PeerID       string          `json:"peer_id"`
	QueueDepth   int             `json:"queue_depth"`
	DropsCount   int64           `json:"drops_count"`
	HealthState  peerlink.PeerHealthState `json:"health_state"`
	FailureCount int             `json:"failure_count"`
}

// GetAllPeerMetrics returns metrics for all peers
func (g *GRPCPeerLink) GetAllPeerMetrics() []PeerMetrics {
	g.mu.RLock()
	defer g.mu.RUnlock()

	result := make([]PeerMetrics, 0, len(g.metrics))
	for peerID, metrics := range g.metrics {
		queueDepth := 0
		if queue, exists := g.sendQueues[peerID]; exists {
			queueDepth = len(queue)
		}

		result = append(result, PeerMetrics{
			PeerID:       peerID,
			QueueDepth:   queueDepth,
			DropsCount:   metrics.dropsCount,
			HealthState:  metrics.healthState,
			FailureCount: metrics.failureCount,
		})
	}
	return result
}