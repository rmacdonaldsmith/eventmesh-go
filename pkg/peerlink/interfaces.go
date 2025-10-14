package peerlink

import (
	"context"
	"io"

	"github.com/rmacdonaldsmith/eventmesh-go/pkg/eventlog"
)

// PeerHealthState represents the health state of a peer
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

// PeerLink manages secure streaming connections between mesh nodes.
// This implements the requirements from design.md for peer-to-peer communication.
//
// Requirements implemented:
// - REQ-PL-001: Secure Connection Lifecycle - mTLS connections with proper lifecycle management
// - REQ-PL-002: Backpressure and Flow Control - handles streaming backpressure
// - REQ-PL-003: Heartbeats and Failure Detection - monitors peer health
type PeerLink interface {
	io.Closer

	// Connect establishes a secure connection to the specified peer node.
	// Uses gRPC with mTLS for secure, efficient bi-directional streaming.
	Connect(ctx context.Context, peer PeerNode) error

	// Disconnect closes the connection to the specified peer node.
	Disconnect(ctx context.Context, peerID string) error

	// SendEvent streams an event to the specified peer node.
	// Handles backpressure and flow control automatically.
	SendEvent(ctx context.Context, peerID string, event eventlog.EventRecord) error

	// ReceiveEvents returns a channel for receiving events from peer nodes.
	// Events are received from all connected peers.
	ReceiveEvents(ctx context.Context) (<-chan eventlog.EventRecord, <-chan error)

	// GetConnectedPeers returns all currently connected peer nodes.
	GetConnectedPeers(ctx context.Context) ([]PeerNode, error)

	// GetPeerHealth returns health status for a specific peer node.
	GetPeerHealth(ctx context.Context, peerID string) (PeerHealthState, error)

	// StartHeartbeats begins health monitoring for all connected peers.
	// Implements REQ-PL-003: heartbeats and failure detection.
	StartHeartbeats(ctx context.Context) error

	// StopHeartbeats stops health monitoring.
	StopHeartbeats(ctx context.Context) error
}
