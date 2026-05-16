package peerlink

import (
	"context"
	"errors"
	"fmt"
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

// ControlPlaneMsg represents an aggregate peer-interest control message.
type ControlPlaneMsg struct {
	peerID          string
	interestMessage *peerlink.InterestMessage
	sentAt          time.Time
	sendAttempts    int
}

func (m *ControlPlaneMsg) PeerID() string        { return m.peerID }
func (m *ControlPlaneMsg) SentAt() time.Time     { return m.sentAt }
func (m *ControlPlaneMsg) SetSentAt(t time.Time) { m.sentAt = t }
func (m *ControlPlaneMsg) SendAttempts() int     { return m.sendAttempts }
func (m *ControlPlaneMsg) IncrementSendAttempts() int {
	m.sendAttempts++
	return m.sendAttempts
}
func (m *ControlPlaneMsg) InterestMessage() *peerlink.InterestMessage {
	return m.interestMessage
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
	config              *Config
	closed              bool
	mu                  sync.RWMutex
	grpcServer          *grpc.Server
	listener            net.Listener
	started             bool
	connections         map[string]*connectionInfo              // peerID -> connection info
	sendQueues          map[string]chan PeerMessage             // peerID -> data-plane send queue
	controlQueues       map[string]chan PeerMessage             // peerID -> control-plane send queue
	metrics             map[string]*peerMetrics                 // peerID -> metrics
	subscribers         map[chan *eventlog.Event]bool           // active ReceiveEvents channels
	interestSubscribers map[chan *peerlink.InterestMessage]bool // active ReceiveInterestMessages channels
	outboundConns       map[string]*grpc.ClientConn             // peerID -> outbound gRPC connection
	outboundCancel      map[string]context.CancelFunc           // peerID -> cancel func for connection goroutine
	heartbeatCancel     context.CancelFunc
	heartbeatToken      *struct{}
	heartbeatsRunning   bool
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
		config:              &configCopy,
		closed:              false,
		started:             false,
		connections:         make(map[string]*connectionInfo),
		sendQueues:          make(map[string]chan PeerMessage),
		controlQueues:       make(map[string]chan PeerMessage),
		metrics:             make(map[string]*peerMetrics),
		subscribers:         make(map[chan *eventlog.Event]bool),
		interestSubscribers: make(map[chan *peerlink.InterestMessage]bool),
		outboundConns:       make(map[string]*grpc.ClientConn),
		outboundCancel:      make(map[string]context.CancelFunc),
	}, nil
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
				if interestMessage := interestMessageFromProto(controlMsg); interestMessage != nil {
					g.mu.Lock()
					if !g.closed {
						g.distributeInterestMessage(interestMessage)
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
			slog.Error("EventStream failed to send interest message",
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
				if interestMessage := interestMessageFromProto(controlMsg); interestMessage != nil {
					g.mu.Lock()
					if !g.closed {
						g.distributeInterestMessage(interestMessage)
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

func interestMessageFromProto(controlMsg *peerlinkv1.ControlPlaneMessage) *peerlink.InterestMessage {
	if update := controlMsg.GetInterestUpdate(); update != nil {
		return &peerlink.InterestMessage{Update: &peerlink.InterestUpdate{
			NodeId: update.NodeId,
			Action: update.Action,
			Topic:  update.Topic,
		}}
	}
	if snapshot := controlMsg.GetInterestSnapshot(); snapshot != nil {
		return &peerlink.InterestMessage{Snapshot: &peerlink.InterestSnapshot{
			NodeId: snapshot.NodeId,
			Topics: append([]string(nil), snapshot.Topics...),
		}}
	}
	return nil
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
		controlMsg, err := controlMessageToProto(m.InterestMessage())
		if err != nil {
			return nil, err
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
			slog.Error("failed to send interest message to peer",
				"peer_id", peerID,
				"local_node_id", g.config.NodeID,
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

func controlMessageToProto(message *peerlink.InterestMessage) (*peerlinkv1.ControlPlaneMessage, error) {
	if message == nil {
		return nil, errors.New("interest message is nil")
	}
	if message.Update != nil {
		return &peerlinkv1.ControlPlaneMessage{
			Kind: &peerlinkv1.ControlPlaneMessage_InterestUpdate{
				InterestUpdate: &peerlinkv1.InterestUpdate{
					NodeId: message.Update.NodeId,
					Action: message.Update.Action,
					Topic:  message.Update.Topic,
				},
			},
		}, nil
	}
	if message.Snapshot != nil {
		return &peerlinkv1.ControlPlaneMessage{
			Kind: &peerlinkv1.ControlPlaneMessage_InterestSnapshot{
				InterestSnapshot: &peerlinkv1.InterestSnapshot{
					NodeId: message.Snapshot.NodeId,
					Topics: append([]string(nil), message.Snapshot.Topics...),
				},
			},
		}, nil
	}
	return nil, errors.New("interest message has no update or snapshot")
}

func messageTopic(msg PeerMessage) string {
	switch m := msg.(type) {
	case *DataPlaneMessage:
		if m.Event() == nil {
			return "unknown"
		}
		return m.Event().Topic
	case *ControlPlaneMsg:
		message := m.InterestMessage()
		if message == nil {
			return "unknown"
		}
		if message.Update != nil {
			return message.Update.Topic
		}
		if message.Snapshot != nil {
			return "interest-snapshot"
		}
		return "unknown"
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

func (g *GRPCPeerLink) sendInterestMessage(ctx context.Context, peerID string, message *peerlink.InterestMessage) error {
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
		peerID:          peerID,
		interestMessage: message,
		sentAt:          time.Now(),
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
		return errors.New("send queue full - interest message dropped")
	}
}

// SendInterestUpdate sends an aggregate topic-interest delta to a peer.
func (g *GRPCPeerLink) SendInterestUpdate(ctx context.Context, peerID string, update *peerlink.InterestUpdate) error {
	return g.sendInterestMessage(ctx, peerID, &peerlink.InterestMessage{Update: update})
}

// SendInterestSnapshot sends a complete aggregate topic-interest snapshot to a peer.
func (g *GRPCPeerLink) SendInterestSnapshot(ctx context.Context, peerID string, snapshot *peerlink.InterestSnapshot) error {
	return g.sendInterestMessage(ctx, peerID, &peerlink.InterestMessage{Snapshot: snapshot})
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

// ReceiveInterestMessages returns a channel for receiving peer-interest messages.
func (g *GRPCPeerLink) ReceiveInterestMessages(ctx context.Context) (<-chan *peerlink.InterestMessage, <-chan error) {
	g.mu.Lock()
	defer g.mu.Unlock()

	if g.closed {
		errChan := make(chan error, 1)
		errChan <- errors.New("PeerLink is closed")
		return nil, errChan
	}

	// Create channels for this subscriber
	messageChan := make(chan *peerlink.InterestMessage, 10) // Small buffer to prevent blocking
	errChan := make(chan error, 1)

	// Register this channel as a subscriber
	g.registerInterestSubscriber(messageChan)

	// Handle cleanup when context is cancelled
	go func() {
		<-ctx.Done()
		g.mu.Lock()
		g.unregisterInterestSubscriber(messageChan)
		close(messageChan)
		g.mu.Unlock()
	}()

	return messageChan, errChan
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

// registerInterestSubscriber adds a channel to the peer-interest subscriber registry.
// Must be called with mutex held
func (g *GRPCPeerLink) registerInterestSubscriber(ch chan *peerlink.InterestMessage) {
	g.interestSubscribers[ch] = true
}

// unregisterInterestSubscriber removes a channel from the peer-interest subscriber registry.
// Must be called with mutex held
func (g *GRPCPeerLink) unregisterInterestSubscriber(ch chan *peerlink.InterestMessage) {
	delete(g.interestSubscribers, ch)
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

// distributeInterestMessage sends a peer-interest message to all registered subscribers (non-blocking).
// Must be called with mutex held
func (g *GRPCPeerLink) distributeInterestMessage(message *peerlink.InterestMessage) {
	for subscriber := range g.interestSubscribers {
		select {
		case subscriber <- message:
			// Interest message sent successfully
		default:
			// Subscriber channel is full - skip this subscriber to avoid blocking
		}
	}
}

// SetPeerHealth sets the health state for a specific peer with logging
