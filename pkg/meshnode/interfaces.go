package meshnode

import (
	"context"
	"io"

	"github.com/rmacdonaldsmith/eventmesh-go/pkg/eventlog"
	"github.com/rmacdonaldsmith/eventmesh-go/pkg/peerlink"
	"github.com/rmacdonaldsmith/eventmesh-go/pkg/routingtable"
)

// Client represents a connected client (publisher/subscriber)
type Client interface {
	// ID returns unique identifier for this client
	ID() string

	// IsAuthenticated returns whether the client is properly authenticated
	IsAuthenticated() bool
}

// MeshNode represents a single node in the event mesh.
// It orchestrates the event log, routing table, and peer links.
//
// Requirements implemented:
// - REQ-MNODE-001: Authentication of Clients
// - REQ-MNODE-002: Local Persistence Before Forwarding
// - REQ-MNODE-003: Subscription Propagation
type MeshNode interface {
	io.Closer

	// Start initializes and starts the mesh node services.
	// This includes starting the event log, routing table, and peer links.
	Start(ctx context.Context) error

	// Stop gracefully shuts down the mesh node and all its services.
	Stop(ctx context.Context) error

	// PublishEvent accepts an event from a local client and handles routing.
	// Implements REQ-MNODE-002: persists locally before forwarding to peers.
	PublishEvent(ctx context.Context, client Client, event eventlog.EventRecord) error

	// Subscribe registers a client's interest in a topic pattern.
	// Implements REQ-MNODE-003: propagates subscription to peer nodes.
	Subscribe(ctx context.Context, client Client, topic string) error

	// Unsubscribe removes a client's subscription to a topic pattern.
	// Updates local routing table and notifies peer nodes.
	Unsubscribe(ctx context.Context, client Client, topic string) error

	// AuthenticateClient validates and authenticates a connecting client.
	// Implements REQ-MNODE-001: authentication of clients.
	AuthenticateClient(ctx context.Context, credentials interface{}) (Client, error)

	// GetEventLog returns the node's event log interface.
	// Used for direct access to event storage and replay.
	GetEventLog() eventlog.EventLog

	// GetRoutingTable returns the node's routing table interface.
	// Used for direct access to subscription management.
	GetRoutingTable() routingtable.RoutingTable

	// GetPeerLink returns the node's peer link interface.
	// Used for direct access to peer communication.
	GetPeerLink() peerlink.PeerLink

	// GetNodeID returns this node's unique identifier in the mesh.
	GetNodeID() string

	// GetConnectedClients returns all currently connected clients.
	GetConnectedClients(ctx context.Context) ([]Client, error)

	// GetConnectedPeers returns all currently connected peer nodes.
	GetConnectedPeers(ctx context.Context) ([]peerlink.PeerNode, error)

	// GetHealth returns the overall health status of this mesh node.
	GetHealth(ctx context.Context) (HealthStatus, error)
}

// HealthStatus represents the overall health of a mesh node
type HealthStatus struct {
	// Healthy indicates if the node is functioning properly
	Healthy bool

	// EventLogHealthy indicates if the event log is operational
	EventLogHealthy bool

	// RoutingTableHealthy indicates if the routing table is operational
	RoutingTableHealthy bool

	// PeerLinkHealthy indicates if peer links are operational
	PeerLinkHealthy bool

	// ConnectedClients is the number of connected local clients
	ConnectedClients int

	// ConnectedPeers is the number of connected peer nodes
	ConnectedPeers int

	// Message provides additional health information
	Message string
}