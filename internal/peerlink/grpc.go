package peerlink

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"sync"
	"time"

	"github.com/rmacdonaldsmith/eventmesh-go/pkg/eventlog"
	"github.com/rmacdonaldsmith/eventmesh-go/pkg/peerlink"
	peerlinkv1 "github.com/rmacdonaldsmith/eventmesh-go/proto/peerlink/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
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
	dropsCount   int64                    // Number of dropped messages due to queue full
	healthState  peerlink.PeerHealthState // Current health state
	failureCount int                      // Number of consecutive failures
}

// receivedEventRecord implements eventlog.EventRecord for events received from peers
type receivedEventRecord struct {
	topic     string
	payload   []byte
	headers   map[string]string
	offset    int64
	timestamp time.Time
}

func (r *receivedEventRecord) Topic() string              { return r.topic }
func (r *receivedEventRecord) Payload() []byte            { return r.payload }
func (r *receivedEventRecord) Headers() map[string]string { return r.headers }
func (r *receivedEventRecord) Offset() int64              { return r.offset }
func (r *receivedEventRecord) Timestamp() time.Time       { return r.timestamp }

// GRPCPeerLink implements the PeerLink interface using gRPC for peer-to-peer communication
type GRPCPeerLink struct {
	peerlinkv1.UnimplementedPeerLinkServer
	config         *Config
	closed         bool
	mu             sync.RWMutex
	grpcServer     *grpc.Server
	listener       net.Listener
	started        bool
	connectedPeers map[string]peerlink.PeerNode       // peerID -> PeerNode
	sendQueues     map[string]chan queuedMessage      // peerID -> send queue
	metrics        map[string]*peerMetrics            // peerID -> metrics
	subscribers    map[chan eventlog.EventRecord]bool // active ReceiveEvents channels
	outboundConns  map[string]*grpc.ClientConn        // peerID -> outbound gRPC connection
	outboundCancel map[string]context.CancelFunc      // peerID -> cancel func for connection goroutine
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
		subscribers:    make(map[chan eventlog.EventRecord]bool),
		outboundConns:  make(map[string]*grpc.ClientConn),
		outboundCancel: make(map[string]context.CancelFunc),
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

	// Log the actual listening address
	slog.Info("peer server listening",
		"address", listener.Addr().String(),
		"node_id", g.config.NodeID)

	// Start serving in a goroutine
	go func() {
		if err := grpcServer.Serve(listener); err != nil {
			// Log error but don't panic - server shutdown is normal
			slog.Debug("peer server stopped",
				"node_id", g.config.NodeID,
				"error", err)
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

	// Handle bidirectional message flow
	// Start goroutine for receiving messages from peer
	recvDone := make(chan error, 1)
	go func() {
		for {
			msg, err := stream.Recv()
			if err != nil {
				recvDone <- err
				return
			}

			// Handle received event messages
			if eventMsg := msg.GetEvent(); eventMsg != nil {
				// Convert protobuf Event to EventRecord and distribute to subscribers
				g.mu.Lock()
				if !g.closed {
					// Create a simple EventRecord implementation for received events
					receivedEvent := &receivedEventRecord{
						topic:     eventMsg.Topic,
						payload:   eventMsg.Payload,
						headers:   eventMsg.Headers,
						offset:    eventMsg.Offset,
						timestamp: time.Now(), // Use current time since protobuf doesn't include timestamp
					}
					g.distributeEvent(receivedEvent)
				}
				g.mu.Unlock()
			}
		}
	}()

	// Handle outgoing message flow
	for {
		select {
		case <-ctx.Done():
			// Stream closed
			return ctx.Err()

		case err := <-recvDone:
			// Receive goroutine finished (error or stream closed)
			return err

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

	// Check if already connected
	if _, exists := g.outboundConns[peerID]; exists {
		slog.Debug("peer connection already exists",
			"peer_id", peerID,
			"peer_address", peer.Address())
		return nil // Already connected
	}

	slog.Info("connecting to peer",
		"peer_id", peerID,
		"peer_address", peer.Address(),
		"local_node_id", g.config.NodeID)

	// Add to connected peers map
	g.connectedPeers[peerID] = peer

	// Register peer (create queue and metrics)
	g.registerPeer(peerID)

	// Establish outbound gRPC connection
	conn, err := grpc.NewClient(peer.Address(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		slog.Error("failed to establish gRPC connection to peer",
			"peer_id", peerID,
			"peer_address", peer.Address(),
			"error", err)
		return err
	}

	// Store connection
	g.outboundConns[peerID] = conn

	// Create context for connection goroutine
	connCtx, cancel := context.WithCancel(context.Background())
	g.outboundCancel[peerID] = cancel

	// Start goroutine to consume from send queue and stream events
	go g.runOutboundConnection(connCtx, peerID, conn)

	slog.Info("peer connected successfully",
		"peer_id", peerID,
		"peer_address", peer.Address(),
		"local_node_id", g.config.NodeID)

	return nil
}

// runOutboundConnection handles the outbound gRPC connection to a peer
// Consumes from the peer's send queue and streams events over gRPC
func (g *GRPCPeerLink) runOutboundConnection(ctx context.Context, peerID string, conn *grpc.ClientConn) {
	defer func() {
		// Clean up connection on exit
		conn.Close()
		slog.Info("peer connection closed",
			"peer_id", peerID,
			"local_node_id", g.config.NodeID)
	}()

	// Create gRPC client
	client := peerlinkv1.NewPeerLinkClient(conn)

	// Establish EventStream
	stream, err := client.EventStream(ctx)
	if err != nil {
		slog.Error("failed to establish event stream with peer",
			"peer_id", peerID,
			"local_node_id", g.config.NodeID,
			"error", err)
		return
	}
	defer stream.CloseSend()

	// Send handshake
	handshake := &peerlinkv1.PeerMessage{
		Kind: &peerlinkv1.PeerMessage_Handshake{
			Handshake: &peerlinkv1.Handshake{
				NodeId:          g.config.NodeID,
				ProtocolVersion: ProtocolVersion,
			},
		},
	}

	if err := stream.Send(handshake); err != nil {
		slog.Error("failed to send handshake to peer",
			"peer_id", peerID,
			"local_node_id", g.config.NodeID,
			"error", err)
		return
	}

	// Receive handshake response
	_, err = stream.Recv()
	if err != nil {
		slog.Error("failed to receive handshake response from peer",
			"peer_id", peerID,
			"local_node_id", g.config.NodeID,
			"error", err)
		return
	}

	slog.Debug("peer handshake completed",
		"peer_id", peerID,
		"local_node_id", g.config.NodeID)

	// Get the send queue for this peer
	g.mu.RLock()
	sendQueue := g.sendQueues[peerID]
	g.mu.RUnlock()

	// Main loop: consume from queue and stream events
	for {
		select {
		case <-ctx.Done():
			// Connection cancelled
			return

		case queuedMsg, ok := <-sendQueue:
			if !ok {
				// Send queue closed
				return
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
				// TODO: Add proper error handling/reconnection
				return
			}
		}
	}
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

	// Cancel outbound connection goroutine
	if cancel, exists := g.outboundCancel[peerID]; exists {
		cancel()
		delete(g.outboundCancel, peerID)
	}

	// Clean up outbound connection (will be closed by goroutine defer)
	delete(g.outboundConns, peerID)

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
	g.mu.Lock()
	defer g.mu.Unlock()

	if g.closed {
		errChan := make(chan error, 1)
		errChan <- errors.New("PeerLink is closed")
		return nil, errChan
	}

	// Create channels for this subscriber
	eventChan := make(chan eventlog.EventRecord, 10) // Small buffer to prevent blocking
	errChan := make(chan error, 1)

	// Register this channel as a subscriber
	g.registerSubscriber(eventChan)

	// Handle cleanup when context is cancelled
	go func() {
		<-ctx.Done()
		g.mu.Lock()
		g.unregisterSubscriber(eventChan)
		close(eventChan)
		g.mu.Unlock()
	}()

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

// registerSubscriber adds a channel to the subscriber registry
// Must be called with mutex held
func (g *GRPCPeerLink) registerSubscriber(ch chan eventlog.EventRecord) {
	g.subscribers[ch] = true
}

// unregisterSubscriber removes a channel from the subscriber registry
// Must be called with mutex held
func (g *GRPCPeerLink) unregisterSubscriber(ch chan eventlog.EventRecord) {
	delete(g.subscribers, ch)
}

// distributeEvent sends an event to all registered subscribers (non-blocking)
// Must be called with mutex held
func (g *GRPCPeerLink) distributeEvent(event eventlog.EventRecord) {
	for subscriber := range g.subscribers {
		select {
		case subscriber <- event:
			// Event sent successfully
		default:
			// Subscriber channel is full - skip this subscriber to avoid blocking
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

	// Cancel all outbound connections
	for _, cancel := range g.outboundCancel {
		cancel()
	}
	g.outboundCancel = nil
	g.outboundConns = nil

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

// GetListeningAddress returns the actual address this PeerLink is listening on
// Returns empty string if not started or no listener
func (g *GRPCPeerLink) GetListeningAddress() string {
	g.mu.RLock()
	defer g.mu.RUnlock()

	if g.listener != nil {
		return g.listener.Addr().String()
	}
	return ""
}

// GetConnectedPeerSummary returns a formatted list of connected peers for logging
// Returns slice of strings in format "node-id@address"
func (g *GRPCPeerLink) GetConnectedPeerSummary(ctx context.Context) ([]string, error) {
	peers, err := g.GetConnectedPeers(ctx)
	if err != nil {
		return nil, err
	}

	summary := make([]string, len(peers))
	for i, peer := range peers {
		summary[i] = peer.ID() + "@" + peer.Address()
	}
	return summary, nil
}

// PeerMetrics represents metrics for a single peer
type PeerMetrics struct {
	PeerID       string                   `json:"peer_id"`
	QueueDepth   int                      `json:"queue_depth"`
	DropsCount   int64                    `json:"drops_count"`
	HealthState  peerlink.PeerHealthState `json:"health_state"`
	FailureCount int                      `json:"failure_count"`
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
