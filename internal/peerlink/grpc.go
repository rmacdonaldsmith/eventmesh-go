package peerlink

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"strings"
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

// PeerMessage interface represents a message that can be sent to peers
type PeerMessage interface {
	// PeerID returns the target peer ID for this message
	PeerID() string
	// SentAt returns when this message was queued for sending
	SentAt() time.Time
	// SetSentAt updates the sent timestamp
	SetSentAt(time.Time)
	// SendAttempts returns how many failed send attempts this message has seen.
	SendAttempts() int
	// IncrementSendAttempts increments and returns the failed send attempt count.
	IncrementSendAttempts() int
}

// DataPlaneMessage represents a user event message
type DataPlaneMessage struct {
	peerID       string
	event        *eventlog.Event
	sentAt       time.Time
	sendAttempts int
}

func (m *DataPlaneMessage) PeerID() string        { return m.peerID }
func (m *DataPlaneMessage) SentAt() time.Time     { return m.sentAt }
func (m *DataPlaneMessage) SetSentAt(t time.Time) { m.sentAt = t }
func (m *DataPlaneMessage) SendAttempts() int     { return m.sendAttempts }
func (m *DataPlaneMessage) IncrementSendAttempts() int {
	m.sendAttempts++
	return m.sendAttempts
}
func (m *DataPlaneMessage) Event() *eventlog.Event { return m.event }

// ControlPlaneMsg represents a subscription change message
type ControlPlaneMsg struct {
	peerID             string
	subscriptionChange *peerlink.SubscriptionChange
	sentAt             time.Time
	sendAttempts       int
}

func (m *ControlPlaneMsg) PeerID() string        { return m.peerID }
func (m *ControlPlaneMsg) SentAt() time.Time     { return m.sentAt }
func (m *ControlPlaneMsg) SetSentAt(t time.Time) { m.sentAt = t }
func (m *ControlPlaneMsg) SendAttempts() int     { return m.sendAttempts }
func (m *ControlPlaneMsg) IncrementSendAttempts() int {
	m.sendAttempts++
	return m.sendAttempts
}
func (m *ControlPlaneMsg) SubscriptionChange() *peerlink.SubscriptionChange {
	return m.subscriptionChange
}

// HeartbeatMsg represents a control-plane heartbeat message.
type HeartbeatMsg struct {
	peerID       string
	sentAt       time.Time
	sendAttempts int
}

func (m *HeartbeatMsg) PeerID() string        { return m.peerID }
func (m *HeartbeatMsg) SentAt() time.Time     { return m.sentAt }
func (m *HeartbeatMsg) SetSentAt(t time.Time) { m.sentAt = t }
func (m *HeartbeatMsg) SendAttempts() int     { return m.sendAttempts }
func (m *HeartbeatMsg) IncrementSendAttempts() int {
	m.sendAttempts++
	return m.sendAttempts
}

type peerMessagePlane int

const (
	dataPlaneMessage peerMessagePlane = iota
	controlPlaneMessage
)

// peerMetrics tracks basic metrics per peer
type peerMetrics struct {
	dropsCount               int64                    // Number of dropped messages due to queue full
	dataPlaneDropsCount      int64                    // Number of dropped data-plane messages
	controlPlaneDropsCount   int64                    // Number of dropped control-plane messages
	dataPlaneQueuedCount     int64                    // Number of accepted data-plane messages
	controlPlaneQueuedCount  int64                    // Number of accepted control-plane messages
	dataPlaneFailureCount    int64                    // Number of data-plane send failures
	controlPlaneFailureCount int64                    // Number of control-plane send failures
	healthState              peerlink.PeerHealthState // Current health state
	failureCount             int                      // Number of consecutive failures
	missedHeartbeatCount     int                      // Number of consecutive missed heartbeat windows
	lastSeenAt               time.Time                // Last heartbeat, valid peer message, or handshake
}

// inboundPeer represents a peer that connected to us (inbound connection)
type inboundPeer struct {
	id      string
	address string
}

func (p *inboundPeer) ID() string      { return p.id }
func (p *inboundPeer) Address() string { return p.address }
func (p *inboundPeer) IsHealthy() bool { return true } // Assume healthy if connected

// connectionInfo represents the connection state for a peer
type connectionInfo struct {
	node   peerlink.PeerNode
	stream grpc.BidiStreamingServer[peerlinkv1.PeerMessage, peerlinkv1.PeerMessage] // nil for outbound, non-nil for inbound
}

// GRPCPeerLink implements the PeerLink interface using gRPC for peer-to-peer communication
type GRPCPeerLink struct {
	peerlinkv1.UnimplementedPeerLinkServer
	config                  *Config
	closed                  bool
	mu                      sync.RWMutex
	grpcServer              *grpc.Server
	listener                net.Listener
	started                 bool
	connections             map[string]*connectionInfo                 // peerID -> connection info
	sendQueues              map[string]chan PeerMessage                // peerID -> data-plane send queue
	controlQueues           map[string]chan PeerMessage                // peerID -> control-plane send queue
	metrics                 map[string]*peerMetrics                    // peerID -> metrics
	subscribers             map[chan *eventlog.Event]bool              // active ReceiveEvents channels
	subscriptionSubscribers map[chan *peerlink.SubscriptionChange]bool // active ReceiveSubscriptionChanges channels
	outboundConns           map[string]*grpc.ClientConn                // peerID -> outbound gRPC connection
	outboundCancel          map[string]context.CancelFunc              // peerID -> cancel func for connection goroutine
	heartbeatCancel         context.CancelFunc
	heartbeatToken          *struct{}
	heartbeatsRunning       bool
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
		config:                  &configCopy,
		closed:                  false,
		started:                 false,
		connections:             make(map[string]*connectionInfo),
		sendQueues:              make(map[string]chan PeerMessage),
		controlQueues:           make(map[string]chan PeerMessage),
		metrics:                 make(map[string]*peerMetrics),
		subscribers:             make(map[chan *eventlog.Event]bool),
		subscriptionSubscribers: make(map[chan *peerlink.SubscriptionChange]bool),
		outboundConns:           make(map[string]*grpc.ClientConn),
		outboundCancel:          make(map[string]context.CancelFunc),
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
		"address", formatAddress(listener.Addr().String()),
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
	if !g.started || g.grpcServer == nil {
		g.mu.Unlock()
		return nil // Not started or already stopped
	}

	grpcServer := g.grpcServer
	g.grpcServer = nil
	g.listener = nil
	g.started = false
	g.mu.Unlock()

	// Graceful shutdown with context
	done := make(chan struct{})
	go func() {
		grpcServer.GracefulStop()
		close(done)
	}()

	select {
	case <-done:
		// Graceful shutdown completed
	case <-ctx.Done():
		// Context timeout - force shutdown
		grpcServer.Stop()
	}

	return nil
}

// EventStream implements the PeerLink gRPC service handler
func (g *GRPCPeerLink) EventStream(stream grpc.BidiStreamingServer[peerlinkv1.PeerMessage, peerlinkv1.PeerMessage]) error {
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

	// Create a simple peer node representation for inbound connections
	// We don't know the actual address, so use a placeholder
	peerNode := &inboundPeer{
		id:      peerID,
		address: "inbound-connection",
	}

	// Store connection info
	g.connections[peerID] = &connectionInfo{
		node:   peerNode,
		stream: stream,
	}

	dataQueue := g.sendQueues[peerID]
	controlQueue := g.controlQueues[peerID]
	slog.Debug("EventStream registered peer for bidirectional communication",
		"peer_id", peerID,
		"local_node_id", g.config.NodeID)
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
		if conn, exists := g.connections[peerID]; exists && conn.stream == stream {
			delete(g.connections, peerID)
		}
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

			// Handle received messages
			if eventMsg := msg.GetEvent(); eventMsg != nil {
				g.markPeerSeen(peerID)
				// Data plane: Convert protobuf Event to EventRecord and distribute to subscribers
				g.mu.Lock()
				if !g.closed {
					g.distributeEvent(eventFromProto(eventMsg))
				}
				g.mu.Unlock()
			} else if controlMsg := msg.GetControlPlane(); controlMsg != nil {
				g.markPeerSeen(peerID)
				// Control plane: Handle subscription changes and other control messages
				if subChange := controlMsg.GetSubscriptionChange(); subChange != nil {
					g.mu.Lock()
					if !g.closed {
						receivedChange := &peerlink.SubscriptionChange{
							Action:   subChange.Action,
							ClientId: subChange.ClientId,
							Topic:    subChange.Topic,
							NodeId:   subChange.NodeId,
						}
						g.distributeSubscriptionChange(receivedChange)
					}
					g.mu.Unlock()
				}
			} else if msg.GetHeartbeat() != nil {
				g.markPeerSeen(peerID)
			} else {
				slog.Debug("EventStream received unhandled message type", "node_id", g.config.NodeID)
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

		case queuedMsg, ok := <-controlQueue:
			if !ok {
				// Send queue closed
				return nil
			}

			if err := g.sendServerStreamMessage(peerID, queuedMsg, stream); err != nil {
				return err
			}

		default:
			select {
			case <-ctx.Done():
				// Stream closed
				return ctx.Err()

			case err := <-recvDone:
				// Receive goroutine finished (error or stream closed)
				return err

			case queuedMsg, ok := <-controlQueue:
				if !ok {
					// Send queue closed
					return nil
				}

				if err := g.sendServerStreamMessage(peerID, queuedMsg, stream); err != nil {
					return err
				}

			case queuedMsg, ok := <-dataQueue:
				if !ok {
					// Send queue closed
					return nil
				}

				if err := g.sendServerStreamMessage(peerID, queuedMsg, stream); err != nil {
					return err
				}
			}
		}
	}
}

func (g *GRPCPeerLink) sendServerStreamMessage(peerID string, queuedMsg PeerMessage, stream grpc.BidiStreamingServer[peerlinkv1.PeerMessage, peerlinkv1.PeerMessage]) error {
	peerMsg, err := g.serializeMessage(queuedMsg)
	if err != nil {
		slog.Error("EventStream failed to serialize message",
			"peer_id", peerID,
			"error", err)
		return nil
	}

	if err := stream.Send(peerMsg); err != nil {
		switch m := queuedMsg.(type) {
		case *DataPlaneMessage:
			slog.Error("EventStream failed to send user event",
				"topic", m.Event().Topic,
				"peer_id", peerID,
				"error", err)
		case *ControlPlaneMsg:
			slog.Error("EventStream failed to send subscription change",
				"action", m.SubscriptionChange().Action,
				"topic", m.SubscriptionChange().Topic,
				"peer_id", peerID,
				"error", err)
		case *HeartbeatMsg:
			slog.Error("EventStream failed to send heartbeat",
				"peer_id", peerID,
				"error", err)
		}
		return err
	}

	return nil
}

// Connect establishes a secure connection to the specified peer node
func (g *GRPCPeerLink) Connect(ctx context.Context, peer peerlink.PeerNode) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	if g.closed {
		return errors.New("PeerLink is closed")
	}

	peerID := peer.ID()

	// Check if already connected (from either direction)
	if _, exists := g.connections[peerID]; exists {
		slog.Debug("peer already connected",
			"peer_id", peerID,
			"peer_address", peer.Address())
		return nil // Already connected
	}

	slog.Info("connecting to peer",
		"peer_id", peerID,
		"peer_address", peer.Address(),
		"local_node_id", g.config.NodeID)

	// Store connection info (outbound connection - no stream yet)
	g.connections[peerID] = &connectionInfo{
		node:   peer,
		stream: nil, // Will be set during stream establishment
	}

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

// runOutboundConnection handles the outbound gRPC connection to a peer with automatic retry
// Consumes from the peer's send queue and streams events over gRPC
func (g *GRPCPeerLink) runOutboundConnection(ctx context.Context, peerID string, conn *grpc.ClientConn) {
	defer func() {
		// Clean up connection on exit
		if err := conn.Close(); err != nil {
			slog.Warn("failed to close peer connection", "error", err, "peer_id", peerID)
		}
		slog.Info("peer connection closed",
			"peer_id", peerID,
			"local_node_id", g.config.NodeID)
	}()

	// Retry loop for connection resilience
	retryCount := 0
	maxRetries := 5
	baseDelay := 1 * time.Second

	for {
		select {
		case <-ctx.Done():
			// Context cancelled, stop trying
			return
		default:
			// Continue with connection attempt
		}

		// Check if PeerLink is closed before attempting connection
		g.mu.RLock()
		closed := g.closed
		g.mu.RUnlock()
		if closed {
			return
		}

		// Attempt to establish stream connection
		success := g.attemptStreamConnection(ctx, peerID, conn)
		if success {
			// Reset retry count on successful connection
			retryCount = 0
			continue
		}

		// Connection failed, implement exponential backoff
		retryCount++
		if retryCount > maxRetries {
			slog.Error("max retries exceeded for peer connection",
				"peer_id", peerID,
				"local_node_id", g.config.NodeID,
				"max_retries", maxRetries)

			// Mark peer as unhealthy
			g.SetPeerHealth(peerID, peerlink.PeerUnhealthy)
			return
		}

		// Exponential backoff with jitter
		delay := time.Duration(retryCount) * baseDelay
		slog.Warn("retrying peer connection",
			"peer_id", peerID,
			"local_node_id", g.config.NodeID,
			"retry_count", retryCount,
			"delay_seconds", delay.Seconds())

		select {
		case <-ctx.Done():
			return
		case <-time.After(delay):
			// Continue to retry
		}
	}
}

// establishStreamWithHandshake establishes a gRPC stream and performs the PeerLink handshake protocol
// Returns the established stream on success, or an error on failure
func (g *GRPCPeerLink) establishStreamWithHandshake(ctx context.Context, peerID string, conn *grpc.ClientConn) (grpc.BidiStreamingClient[peerlinkv1.PeerMessage, peerlinkv1.PeerMessage], error) {
	// Create gRPC client
	client := peerlinkv1.NewPeerLinkClient(conn)

	// Establish EventStream
	stream, err := client.EventStream(ctx)
	if err != nil {
		slog.Error("failed to establish event stream with peer",
			"peer_id", peerID,
			"local_node_id", g.config.NodeID,
			"error", err)
		return nil, err
	}

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
		_ = stream.CloseSend() // Best effort close
		return nil, err
	}

	// Receive handshake response
	_, err = stream.Recv()
	if err != nil {
		slog.Error("failed to receive handshake response from peer",
			"peer_id", peerID,
			"local_node_id", g.config.NodeID,
			"error", err)
		_ = stream.CloseSend() // Best effort close
		return nil, err
	}

	slog.Debug("peer handshake completed",
		"peer_id", peerID,
		"local_node_id", g.config.NodeID)

	// Mark peer as healthy after successful handshake.
	g.markPeerSeen(peerID)

	return stream, nil
}

// startReceiveGoroutine starts a goroutine to receive messages from the peer stream
// Returns a channel that will receive an error when the goroutine finishes
func (g *GRPCPeerLink) startReceiveGoroutine(ctx context.Context, peerID string, stream grpc.BidiStreamingClient[peerlinkv1.PeerMessage, peerlinkv1.PeerMessage]) <-chan error {
	recvDone := make(chan error, 1)
	go func() {
		for {
			msg, err := stream.Recv()
			if err != nil {
				recvDone <- err
				return
			}

			// Handle received messages
			if eventMsg := msg.GetEvent(); eventMsg != nil {
				g.markPeerSeen(peerID)
				// Data plane: Convert protobuf Event to EventRecord and distribute to subscribers
				g.mu.Lock()
				if !g.closed {
					g.distributeEvent(eventFromProto(eventMsg))
				}
				g.mu.Unlock()
			} else if controlMsg := msg.GetControlPlane(); controlMsg != nil {
				g.markPeerSeen(peerID)
				// Control plane: Handle subscription changes and other control messages
				if subChange := controlMsg.GetSubscriptionChange(); subChange != nil {
					g.mu.Lock()
					if !g.closed {
						receivedChange := &peerlink.SubscriptionChange{
							Action:   subChange.Action,
							ClientId: subChange.ClientId,
							Topic:    subChange.Topic,
							NodeId:   subChange.NodeId,
						}
						g.distributeSubscriptionChange(receivedChange)
					}
					g.mu.Unlock()
				}
			} else if msg.GetHeartbeat() != nil {
				g.markPeerSeen(peerID)
			} else {
				slog.Debug("startReceiveGoroutine received unhandled message type",
					"peer_id", peerID,
					"local_node_id", g.config.NodeID)
			}
		}
	}()
	return recvDone
}

func eventFromProto(eventMsg *peerlinkv1.Event) *eventlog.Event {
	payload := append([]byte(nil), eventMsg.Payload...)
	headers := make(map[string]string, len(eventMsg.Headers))
	for key, value := range eventMsg.Headers {
		headers[key] = value
	}

	return &eventlog.Event{
		Topic:     eventMsg.Topic,
		Payload:   payload,
		Headers:   headers,
		Offset:    eventMsg.Offset,
		Timestamp: time.Now(),
	}
}

// serializeMessage converts a PeerMessage to a protobuf PeerMessage
func (g *GRPCPeerLink) serializeMessage(msg PeerMessage) (*peerlinkv1.PeerMessage, error) {
	switch m := msg.(type) {
	case *DataPlaneMessage:
		// Data plane: user event
		event := &peerlinkv1.Event{
			Topic:   m.Event().Topic,
			Payload: m.Event().Payload,
			Headers: m.Event().Headers,
			Offset:  m.Event().Offset,
		}

		return &peerlinkv1.PeerMessage{
			Kind: &peerlinkv1.PeerMessage_Event{
				Event: event,
			},
		}, nil

	case *ControlPlaneMsg:
		// Control plane: subscription change
		subChange := &peerlinkv1.SubscriptionChange{
			Action:   m.SubscriptionChange().Action,
			ClientId: m.SubscriptionChange().ClientId,
			Topic:    m.SubscriptionChange().Topic,
			NodeId:   m.SubscriptionChange().NodeId,
		}

		controlMsg := &peerlinkv1.ControlPlaneMessage{
			Kind: &peerlinkv1.ControlPlaneMessage_SubscriptionChange{
				SubscriptionChange: subChange,
			},
		}

		return &peerlinkv1.PeerMessage{
			Kind: &peerlinkv1.PeerMessage_ControlPlane{
				ControlPlane: controlMsg,
			},
		}, nil

	case *HeartbeatMsg:
		return &peerlinkv1.PeerMessage{
			Kind: &peerlinkv1.PeerMessage_Heartbeat{
				Heartbeat: &peerlinkv1.Heartbeat{
					Timestamp: m.SentAt().UnixMilli(),
				},
			},
		}, nil

	default:
		return nil, fmt.Errorf("unsupported message type: %T", msg)
	}
}

// processOutboundMessage converts and sends a queued message over the stream
// Returns nil on success, error on failure
func (g *GRPCPeerLink) processOutboundMessage(peerID string, msg PeerMessage, stream grpc.BidiStreamingClient[peerlinkv1.PeerMessage, peerlinkv1.PeerMessage]) error {
	if stream == nil {
		return errors.New("stream is nil")
	}

	peerMsg, err := g.serializeMessage(msg)
	if err != nil {
		return err
	}

	// Send over gRPC stream
	if err := stream.Send(peerMsg); err != nil {
		switch m := msg.(type) {
		case *DataPlaneMessage:
			slog.Error("failed to send event to peer",
				"peer_id", peerID,
				"local_node_id", g.config.NodeID,
				"topic", m.Event().Topic,
				"event_offset", m.Event().Offset,
				"error", err)
		case *ControlPlaneMsg:
			slog.Error("failed to send subscription change to peer",
				"peer_id", peerID,
				"local_node_id", g.config.NodeID,
				"action", m.SubscriptionChange().Action,
				"topic", m.SubscriptionChange().Topic,
				"error", err)
		case *HeartbeatMsg:
			slog.Error("failed to send heartbeat to peer",
				"peer_id", peerID,
				"local_node_id", g.config.NodeID,
				"error", err)
		}
		return err
	}

	return nil
}

// handleSendFailure handles message send failures by attempting to requeue or dropping the message
func (g *GRPCPeerLink) handleSendFailure(peerID string, msg PeerMessage, sendQueue chan PeerMessage, err error) {
	g.recordPlaneSendFailure(peerID, planeForMessage(msg))

	attempts := msg.IncrementSendAttempts()
	if attempts > g.config.MaxSendAttempts {
		g.dropFailedMessage(peerID, msg, "max send attempts exceeded")
		g.recordPeerSendFailure(peerID)
		return
	}

	// Put the message back in the queue if possible (bounded queue, might drop)
	select {
	case sendQueue <- msg:
		slog.Debug("requeued failed message",
			"peer_id", peerID,
			"topic", messageTopic(msg),
			"attempts", attempts,
			"max_attempts", g.config.MaxSendAttempts)
	default:
		g.dropFailedMessage(peerID, msg, "queue full on retry")
	}

	// Mark peer unhealthy on send failure
	g.recordPeerSendFailure(peerID)
}

func (g *GRPCPeerLink) dropFailedMessage(peerID string, msg PeerMessage, reason string) {
	g.recordPlaneDrop(peerID, planeForMessage(msg))

	slog.Warn("dropped failed message",
		"peer_id", peerID,
		"topic", messageTopic(msg),
		"attempts", msg.SendAttempts(),
		"max_attempts", g.config.MaxSendAttempts,
		"reason", reason)
}

func messageTopic(msg PeerMessage) string {
	switch m := msg.(type) {
	case *DataPlaneMessage:
		if m.Event() == nil {
			return "unknown"
		}
		return m.Event().Topic
	case *ControlPlaneMsg:
		if m.SubscriptionChange() == nil {
			return "unknown"
		}
		return m.SubscriptionChange().Topic
	case *HeartbeatMsg:
		return "heartbeat"
	default:
		return "unknown"
	}
}

func planeForMessage(msg PeerMessage) peerMessagePlane {
	switch msg.(type) {
	case *DataPlaneMessage:
		return dataPlaneMessage
	case *ControlPlaneMsg, *HeartbeatMsg:
		return controlPlaneMessage
	default:
		return controlPlaneMessage
	}
}

// getSendQueue safely retrieves the send queue for a peer
func (g *GRPCPeerLink) getSendQueue(peerID string) chan PeerMessage {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.sendQueues[peerID]
}

// getControlQueue safely retrieves the control queue for a peer
func (g *GRPCPeerLink) getControlQueue(peerID string) chan PeerMessage {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.controlQueues[peerID]
}

type queuedOutboundMessage struct {
	msg   PeerMessage
	queue chan PeerMessage
}

func (g *GRPCPeerLink) nextOutboundMessage(ctx context.Context, peerID string) (PeerMessage, bool) {
	queued, ok, _ := g.nextOutboundMessageOrDone(ctx, peerID, nil)
	if !ok {
		return nil, false
	}
	return queued.msg, true
}

func (g *GRPCPeerLink) nextOutboundMessageOrDone(ctx context.Context, peerID string, recvDone <-chan error) (queuedOutboundMessage, bool, error) {
	dataQueue := g.getSendQueue(peerID)
	controlQueue := g.getControlQueue(peerID)

	select {
	case msg, ok := <-controlQueue:
		return queuedOutboundMessage{msg: msg, queue: controlQueue}, ok, nil
	default:
	}

	select {
	case <-ctx.Done():
		return queuedOutboundMessage{}, false, ctx.Err()
	case err := <-recvDone:
		return queuedOutboundMessage{}, false, err
	case msg, ok := <-controlQueue:
		return queuedOutboundMessage{msg: msg, queue: controlQueue}, ok, nil
	case msg, ok := <-dataQueue:
		return queuedOutboundMessage{msg: msg, queue: dataQueue}, ok, nil
	}
}

// runBidirectionalStreamLoop runs the main event processing loop for a bidirectional stream
// Returns true if the connection should be retried, false if it should terminate
func (g *GRPCPeerLink) runBidirectionalStreamLoop(ctx context.Context, peerID string, stream grpc.BidiStreamingClient[peerlinkv1.PeerMessage, peerlinkv1.PeerMessage], sendQueue chan PeerMessage) bool {
	// Start receive goroutine for bidirectional communication
	recvDone := g.startReceiveGoroutine(ctx, peerID, stream)

	// Main loop: consume from queue and stream events
	for {
		queued, ok, err := g.nextOutboundMessageOrDone(ctx, peerID, recvDone)
		if err != nil {
			if ctx.Err() != nil {
				// Connection cancelled
				return false // Don't retry
			}
			// Receive goroutine finished (error or stream closed)
			slog.Warn("runBidirectionalStreamLoop receive goroutine finished",
				"peer_id", peerID,
				"local_node_id", g.config.NodeID,
				"error", err)
			return true // Retry connection
		}
		if !ok {
			// Send queue closed
			return false // Don't retry
		}

		// Process outbound message
		if err := g.processOutboundMessage(peerID, queued.msg, stream); err != nil {
			// Handle send failure
			g.handleSendFailure(peerID, queued.msg, queued.queue, err)
			return true // Retry connection
		}
	}
}

// attemptStreamConnection attempts to establish and maintain a single stream connection
// Returns true if the connection should be retried, false if it should terminate
func (g *GRPCPeerLink) attemptStreamConnection(ctx context.Context, peerID string, conn *grpc.ClientConn) bool {
	// Establish stream and perform handshake
	stream, err := g.establishStreamWithHandshake(ctx, peerID, conn)
	if err != nil {
		return true // Retry on establishment/handshake failure
	}
	defer func() {
		if err := stream.CloseSend(); err != nil {
			slog.Debug("failed to close send stream", "error", err)
		}
	}()

	// Get the send queue for this peer
	sendQueue := g.getSendQueue(peerID)

	// Run the bidirectional stream loop
	return g.runBidirectionalStreamLoop(ctx, peerID, stream, sendQueue)
}

// Disconnect closes the connection to the specified peer node
func (g *GRPCPeerLink) Disconnect(ctx context.Context, peerID string) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	if g.closed {
		return errors.New("PeerLink is closed")
	}

	// Remove from connections map
	delete(g.connections, peerID)

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
	if queue, exists := g.controlQueues[peerID]; exists {
		close(queue)
		delete(g.controlQueues, peerID)
	}

	// Set health state to Disconnected and keep metrics for observability
	if metrics, exists := g.metrics[peerID]; exists {
		if metrics.healthState != peerlink.PeerDisconnected {
			metrics.healthState = peerlink.PeerDisconnected
		}
	}

	return nil
}

// SendEvent streams an event to the specified peer node
func (g *GRPCPeerLink) SendEvent(ctx context.Context, peerID string, event *eventlog.Event) error {
	g.mu.RLock()
	if g.closed {
		g.mu.RUnlock()
		return errors.New("PeerLink is closed")
	}

	// Get the send queue for this peer
	queue, queueExists := g.sendQueues[peerID]
	g.mu.RUnlock()

	if !queueExists {
		return errors.New("peer not connected")
	}

	// Create data plane message
	msg := &DataPlaneMessage{
		peerID: peerID,
		event:  event,
		sentAt: time.Now(),
	}

	// Try to send to queue with timeout (bounded queue behavior)
	timeoutCtx, cancel := context.WithTimeout(ctx, g.config.SendTimeout)
	defer cancel()

	select {
	case queue <- msg:
		g.recordPlaneQueued(peerID, dataPlaneMessage)
		return nil
	case <-timeoutCtx.Done():
		g.recordPlaneDrop(peerID, dataPlaneMessage)
		return errors.New("send queue full - message dropped")
	}
}

// SendSubscriptionChange sends subscription change notifications to peers
func (g *GRPCPeerLink) SendSubscriptionChange(ctx context.Context, peerID string, change *peerlink.SubscriptionChange) error {
	g.mu.RLock()
	if g.closed {
		g.mu.RUnlock()
		return errors.New("PeerLink is closed")
	}

	// Get the control queue for this peer
	queue, queueExists := g.controlQueues[peerID]
	g.mu.RUnlock()

	if !queueExists {
		return errors.New("peer not connected")
	}

	// Create control plane message
	msg := &ControlPlaneMsg{
		peerID:             peerID,
		subscriptionChange: change,
		sentAt:             time.Now(),
	}

	// Try to send to queue with timeout (bounded queue behavior)
	timeoutCtx, cancel := context.WithTimeout(ctx, g.config.SendTimeout)
	defer cancel()

	select {
	case queue <- msg:
		g.recordPlaneQueued(peerID, controlPlaneMessage)
		return nil
	case <-timeoutCtx.Done():
		g.recordPlaneDrop(peerID, controlPlaneMessage)
		return errors.New("send queue full - subscription change dropped")
	}
}

// ReceiveEvents returns a channel for receiving events from peer nodes
func (g *GRPCPeerLink) ReceiveEvents(ctx context.Context) (<-chan *eventlog.Event, <-chan error) {
	g.mu.Lock()
	defer g.mu.Unlock()

	if g.closed {
		errChan := make(chan error, 1)
		errChan <- errors.New("PeerLink is closed")
		return nil, errChan
	}

	// Create channels for this subscriber
	eventChan := make(chan *eventlog.Event, 10) // Small buffer to prevent blocking
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

// ReceiveSubscriptionChanges returns a channel for receiving subscription changes from peer nodes
func (g *GRPCPeerLink) ReceiveSubscriptionChanges(ctx context.Context) (<-chan *peerlink.SubscriptionChange, <-chan error) {
	g.mu.Lock()
	defer g.mu.Unlock()

	if g.closed {
		errChan := make(chan error, 1)
		errChan <- errors.New("PeerLink is closed")
		return nil, errChan
	}

	// Create channels for this subscriber
	changeChan := make(chan *peerlink.SubscriptionChange, 10) // Small buffer to prevent blocking
	errChan := make(chan error, 1)

	// Register this channel as a subscriber
	g.registerSubscriptionSubscriber(changeChan)

	// Handle cleanup when context is cancelled
	go func() {
		<-ctx.Done()
		g.mu.Lock()
		g.unregisterSubscriptionSubscriber(changeChan)
		close(changeChan)
		g.mu.Unlock()
	}()

	return changeChan, errChan
}

// SendHeartbeat sends a heartbeat message to the specified peer
func (g *GRPCPeerLink) SendHeartbeat(ctx context.Context, peerID string) error {
	g.mu.RLock()
	if g.closed {
		g.mu.RUnlock()
		return errors.New("PeerLink is closed")
	}

	queue, queueExists := g.controlQueues[peerID]
	g.mu.RUnlock()

	if !queueExists {
		return errors.New("peer not connected")
	}

	msg := &HeartbeatMsg{
		peerID: peerID,
		sentAt: time.Now(),
	}

	timeoutCtx, cancel := context.WithTimeout(ctx, g.config.SendTimeout)
	defer cancel()

	select {
	case queue <- msg:
		g.recordPlaneQueued(peerID, controlPlaneMessage)
		return nil
	case <-timeoutCtx.Done():
		g.recordPlaneDrop(peerID, controlPlaneMessage)
		return errors.New("send queue full - heartbeat dropped")
	}
}

// GetConnectedPeers returns all currently connected peer nodes
func (g *GRPCPeerLink) GetConnectedPeers(ctx context.Context) ([]peerlink.PeerNode, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()

	if g.closed {
		return nil, errors.New("PeerLink is closed")
	}

	// Convert map to slice
	peers := make([]peerlink.PeerNode, 0, len(g.connections))
	for _, conn := range g.connections {
		peers = append(peers, conn.node)
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
		g.sendQueues[peerID] = make(chan PeerMessage, g.config.SendQueueSize)
	}
	if _, exists := g.controlQueues[peerID]; !exists {
		g.controlQueues[peerID] = make(chan PeerMessage, g.config.SendQueueSize)
	}
	if _, exists := g.metrics[peerID]; !exists {
		g.metrics[peerID] = &peerMetrics{
			dropsCount:           0,
			healthState:          peerlink.PeerHealthy,
			failureCount:         0,
			missedHeartbeatCount: 0,
			lastSeenAt:           time.Now(),
		}
	}
}

// registerSubscriber adds a channel to the subscriber registry
// Must be called with mutex held
func (g *GRPCPeerLink) registerSubscriber(ch chan *eventlog.Event) {
	g.subscribers[ch] = true
}

// unregisterSubscriber removes a channel from the subscriber registry
// Must be called with mutex held
func (g *GRPCPeerLink) unregisterSubscriber(ch chan *eventlog.Event) {
	delete(g.subscribers, ch)
}

// registerSubscriptionSubscriber adds a channel to the subscription change subscriber registry
// Must be called with mutex held
func (g *GRPCPeerLink) registerSubscriptionSubscriber(ch chan *peerlink.SubscriptionChange) {
	g.subscriptionSubscribers[ch] = true
}

// unregisterSubscriptionSubscriber removes a channel from the subscription change subscriber registry
// Must be called with mutex held
func (g *GRPCPeerLink) unregisterSubscriptionSubscriber(ch chan *peerlink.SubscriptionChange) {
	delete(g.subscriptionSubscribers, ch)
}

// distributeEvent sends an event to all registered subscribers (non-blocking)
// Must be called with mutex held
func (g *GRPCPeerLink) distributeEvent(event *eventlog.Event) {
	for subscriber := range g.subscribers {
		select {
		case subscriber <- event:
			// Event sent successfully
		default:
			// Subscriber channel is full - skip this subscriber to avoid blocking
		}
	}
}

// distributeSubscriptionChange sends a subscription change to all registered subscribers (non-blocking)
// Must be called with mutex held
func (g *GRPCPeerLink) distributeSubscriptionChange(change *peerlink.SubscriptionChange) {
	for subscriber := range g.subscriptionSubscribers {
		select {
		case subscriber <- change:
			// Subscription change sent successfully
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
		if metrics.healthState != newState {
			metrics.healthState = newState
		}
	}
}

func (g *GRPCPeerLink) markPeerSeen(peerID string) {
	g.mu.Lock()
	defer g.mu.Unlock()

	if metrics, exists := g.metrics[peerID]; exists {
		metrics.healthState = peerlink.PeerHealthy
		metrics.failureCount = 0
		metrics.missedHeartbeatCount = 0
		metrics.lastSeenAt = time.Now()
	}
}

func (g *GRPCPeerLink) recordPlaneQueued(peerID string, plane peerMessagePlane) {
	g.mu.Lock()
	defer g.mu.Unlock()

	if metrics, exists := g.metrics[peerID]; exists {
		switch plane {
		case dataPlaneMessage:
			metrics.dataPlaneQueuedCount++
		case controlPlaneMessage:
			metrics.controlPlaneQueuedCount++
		}
	}
}

func (g *GRPCPeerLink) recordPlaneDrop(peerID string, plane peerMessagePlane) {
	g.mu.Lock()
	defer g.mu.Unlock()

	if metrics, exists := g.metrics[peerID]; exists {
		metrics.dropsCount++
		switch plane {
		case dataPlaneMessage:
			metrics.dataPlaneDropsCount++
		case controlPlaneMessage:
			metrics.controlPlaneDropsCount++
		}
	}
}

func (g *GRPCPeerLink) recordPlaneSendFailure(peerID string, plane peerMessagePlane) {
	g.mu.Lock()
	defer g.mu.Unlock()

	if metrics, exists := g.metrics[peerID]; exists {
		switch plane {
		case dataPlaneMessage:
			metrics.dataPlaneFailureCount++
		case controlPlaneMessage:
			metrics.controlPlaneFailureCount++
		}
	}
}

func (g *GRPCPeerLink) recordPeerSendFailure(peerID string) {
	g.mu.Lock()
	defer g.mu.Unlock()

	if metrics, exists := g.metrics[peerID]; exists {
		metrics.failureCount++
		metrics.healthState = peerlink.PeerUnhealthy
	}
}

// StartHeartbeats begins health monitoring for all connected peers
func (g *GRPCPeerLink) StartHeartbeats(ctx context.Context) error {
	if ctx == nil {
		return errors.New("context cannot be nil")
	}

	g.mu.Lock()
	if g.closed {
		g.mu.Unlock()
		return errors.New("PeerLink is closed")
	}
	if g.heartbeatsRunning {
		g.mu.Unlock()
		return nil
	}

	heartbeatCtx, cancel := context.WithCancel(ctx)
	token := &struct{}{}
	g.heartbeatCancel = cancel
	g.heartbeatToken = token
	g.heartbeatsRunning = true
	g.mu.Unlock()

	go g.runHeartbeatLoop(heartbeatCtx, token)
	return nil
}

// StopHeartbeats stops health monitoring
func (g *GRPCPeerLink) StopHeartbeats(ctx context.Context) error {
	g.mu.Lock()
	cancel := g.heartbeatCancel
	g.heartbeatCancel = nil
	g.heartbeatToken = nil
	g.heartbeatsRunning = false
	g.mu.Unlock()

	if cancel != nil {
		cancel()
	}
	return nil
}

func (g *GRPCPeerLink) runHeartbeatLoop(ctx context.Context, token *struct{}) {
	defer g.clearHeartbeatLifecycle(token)

	g.sendHeartbeatToConnectedPeers(ctx)

	ticker := time.NewTicker(g.config.HeartbeatInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			g.sendHeartbeatToConnectedPeers(ctx)
		}
	}
}

func (g *GRPCPeerLink) clearHeartbeatLifecycle(token *struct{}) {
	g.mu.Lock()
	defer g.mu.Unlock()

	if g.heartbeatToken == token {
		g.heartbeatCancel = nil
		g.heartbeatToken = nil
		g.heartbeatsRunning = false
	}
}

func (g *GRPCPeerLink) sendHeartbeatToConnectedPeers(ctx context.Context) {
	g.markSilentPeersUnhealthy(time.Now())

	peerIDs := g.connectedPeerIDs()
	for _, peerID := range peerIDs {
		if err := g.SendHeartbeat(ctx, peerID); err != nil {
			g.recordPlaneSendFailure(peerID, controlPlaneMessage)
			g.recordPeerSendFailure(peerID)
		}
	}
}

func (g *GRPCPeerLink) markSilentPeersUnhealthy(now time.Time) {
	g.mu.Lock()
	defer g.mu.Unlock()

	timeout := g.heartbeatTimeout()
	for _, metrics := range g.metrics {
		if metrics.healthState == peerlink.PeerDisconnected || metrics.lastSeenAt.IsZero() {
			continue
		}
		if now.Sub(metrics.lastSeenAt) >= timeout {
			metrics.healthState = peerlink.PeerUnhealthy
			metrics.missedHeartbeatCount = int(now.Sub(metrics.lastSeenAt) / g.config.HeartbeatInterval)
		}
	}
}

func (g *GRPCPeerLink) heartbeatTimeout() time.Duration {
	return g.config.HeartbeatInterval * time.Duration(g.config.MissedHeartbeatLimit)
}

func (g *GRPCPeerLink) connectedPeerIDs() []string {
	g.mu.RLock()
	defer g.mu.RUnlock()

	peerIDs := make([]string, 0, len(g.connections))
	for peerID := range g.connections {
		peerIDs = append(peerIDs, peerID)
	}
	return peerIDs
}

// Close closes the PeerLink and cleans up resources
func (g *GRPCPeerLink) Close() error {
	g.mu.Lock()
	defer g.mu.Unlock()

	if g.closed {
		return nil // Already closed, safe to call multiple times
	}

	if g.heartbeatCancel != nil {
		g.heartbeatCancel()
		g.heartbeatCancel = nil
		g.heartbeatToken = nil
		g.heartbeatsRunning = false
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
		depth := len(queue)
		if controlQueue, exists := g.controlQueues[peerID]; exists {
			depth += len(controlQueue)
		}
		return depth
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
	PeerID                   string                   `json:"peer_id"`
	QueueDepth               int                      `json:"queue_depth"`
	DataPlaneQueueDepth      int                      `json:"data_plane_queue_depth"`
	ControlPlaneQueueDepth   int                      `json:"control_plane_queue_depth"`
	DropsCount               int64                    `json:"drops_count"`
	DataPlaneDropsCount      int64                    `json:"data_plane_drops_count"`
	ControlPlaneDropsCount   int64                    `json:"control_plane_drops_count"`
	DataPlaneQueuedCount     int64                    `json:"data_plane_queued_count"`
	ControlPlaneQueuedCount  int64                    `json:"control_plane_queued_count"`
	DataPlaneFailureCount    int64                    `json:"data_plane_failure_count"`
	ControlPlaneFailureCount int64                    `json:"control_plane_failure_count"`
	HealthState              peerlink.PeerHealthState `json:"health_state"`
	FailureCount             int                      `json:"failure_count"`
	MissedHeartbeatCount     int                      `json:"missed_heartbeat_count"`
	LastSeenAt               time.Time                `json:"last_seen_at"`
}

// GetAllPeerMetrics returns metrics for all peers
func (g *GRPCPeerLink) GetAllPeerMetrics() []PeerMetrics {
	g.mu.RLock()
	defer g.mu.RUnlock()

	result := make([]PeerMetrics, 0, len(g.metrics))
	for peerID, metrics := range g.metrics {
		dataPlaneQueueDepth := 0
		if queue, exists := g.sendQueues[peerID]; exists {
			dataPlaneQueueDepth = len(queue)
		}
		controlPlaneQueueDepth := 0
		if queue, exists := g.controlQueues[peerID]; exists {
			controlPlaneQueueDepth = len(queue)
		}
		queueDepth := dataPlaneQueueDepth + controlPlaneQueueDepth

		result = append(result, PeerMetrics{
			PeerID:                   peerID,
			QueueDepth:               queueDepth,
			DataPlaneQueueDepth:      dataPlaneQueueDepth,
			ControlPlaneQueueDepth:   controlPlaneQueueDepth,
			DropsCount:               metrics.dropsCount,
			DataPlaneDropsCount:      metrics.dataPlaneDropsCount,
			ControlPlaneDropsCount:   metrics.controlPlaneDropsCount,
			DataPlaneQueuedCount:     metrics.dataPlaneQueuedCount,
			ControlPlaneQueuedCount:  metrics.controlPlaneQueuedCount,
			DataPlaneFailureCount:    metrics.dataPlaneFailureCount,
			ControlPlaneFailureCount: metrics.controlPlaneFailureCount,
			HealthState:              metrics.healthState,
			FailureCount:             metrics.failureCount,
			MissedHeartbeatCount:     metrics.missedHeartbeatCount,
			LastSeenAt:               metrics.lastSeenAt,
		})
	}
	return result
}

// formatAddress converts IPv6 addresses to more readable format
func formatAddress(addr string) string {
	if addr == "" {
		return ""
	}

	// Convert [::]:8080 to localhost:8080 for readability
	if strings.HasPrefix(addr, "[::]:") {
		port := strings.TrimPrefix(addr, "[::]:")
		return fmt.Sprintf("localhost:%s", port)
	}

	// Return as-is for other formats (IPv4, etc.)
	return addr
}
