package peerlink

import (
	"context"
	"io"

	"github.com/rmacdonaldsmith/eventmesh-go/pkg/eventlog"
)

// PeerHealthState represents this node's local view of a peer connection.
//
// State contract:
//   - Healthy means the peer has recently proven liveness through a heartbeat,
//     valid peer message, or successful reconnect handshake.
//   - Unhealthy means the peer is suspected but may recover; keep retry and
//     monitoring machinery alive, but avoid treating the peer as reliably
//     routable.
//   - Disconnected means this node has no active usable connection to the peer,
//     usually after explicit disconnect/close, stream close, or reconnect
//     exhaustion. A later successful reconnect handshake moves it to Healthy.
type PeerHealthState int

const (
	PeerHealthy PeerHealthState = iota
	PeerUnhealthy
	PeerDisconnected
)

func (s PeerHealthState) String() string {
	switch s {
	case PeerHealthy:
		return "Healthy"
	case PeerUnhealthy:
		return "Unhealthy"
	case PeerDisconnected:
		return "Disconnected"
	default:
		return "Unknown"
	}
}

// PeerNode represents a remote mesh node in the cluster
type PeerNode interface {
	// ID returns unique identifier for this peer node
	ID() string

	// Address returns the network address of the peer node
	Address() string

	// IsHealthy returns whether the peer node is currently reachable
	IsHealthy() bool
}

// SubscriptionChange represents a subscription change event for control plane communication
type SubscriptionChange struct {
	Action   string // "subscribe" or "unsubscribe"
	ClientId string // ID of the client making the change
	Topic    string // Topic pattern being subscribed/unsubscribed
	NodeId   string // ID of the node where the change occurred
}

// DataPlanePeerLink handles user event streaming between mesh nodes.
// Focused interface for data plane communication with independent flow control.
type DataPlanePeerLink interface {
	// SendEvent streams a user event to the specified peer node.
	// Handles backpressure and flow control automatically for data plane.
	SendEvent(ctx context.Context, peerID string, event *eventlog.Event) error

	// ReceiveEvents returns a channel for receiving user events from peer nodes.
	// Events are received from all connected peers on the data plane.
	ReceiveEvents(ctx context.Context) (<-chan *eventlog.Event, <-chan error)
}

// ControlPlanePeerLink handles subscription gossip and health monitoring between mesh nodes.
// Focused interface for control plane communication with independent QoS.
type ControlPlanePeerLink interface {
	// SendSubscriptionChange sends subscription change notifications to peers.
	// Used for subscription gossip propagation across the mesh.
	SendSubscriptionChange(ctx context.Context, peerID string, change *SubscriptionChange) error

	// ReceiveSubscriptionChanges returns a channel for receiving subscription changes.
	// Subscription changes are received from all connected peers.
	ReceiveSubscriptionChanges(ctx context.Context) (<-chan *SubscriptionChange, <-chan error)

	// SendHeartbeat sends a heartbeat message to the specified peer.
	// Implements health monitoring for control plane.
	SendHeartbeat(ctx context.Context, peerID string) error

	// GetPeerHealth returns health status for a specific peer node.
	GetPeerHealth(ctx context.Context, peerID string) (PeerHealthState, error)

	// StartHeartbeats begins health monitoring for all connected peers.
	// Implements REQ-PL-003: heartbeats and failure detection.
	StartHeartbeats(ctx context.Context) error

	// StopHeartbeats stops health monitoring.
	StopHeartbeats(ctx context.Context) error
}

// PeerConnectionManager handles peer connection lifecycle and management.
// Focused interface for connection establishment, cleanup, and peer discovery.
type PeerConnectionManager interface {
	io.Closer

	// Connect establishes a connection to the specified peer node.
	// The current implementation uses gRPC bidirectional streaming; authenticated
	// peer identity and mTLS are tracked as roadmap work.
	Connect(ctx context.Context, peer PeerNode) error

	// Disconnect closes the connection to the specified peer node.
	Disconnect(ctx context.Context, peerID string) error

	// GetConnectedPeers returns all currently connected peer nodes.
	GetConnectedPeers(ctx context.Context) ([]PeerNode, error)
}

// CompletePeerLink combines all three focused PeerLink interfaces
// into a single composite interface for components that need all functionality.
// This provides a clean, named interface instead of anonymous composition.
type CompletePeerLink interface {
	DataPlanePeerLink
	ControlPlanePeerLink
	PeerConnectionManager
}
