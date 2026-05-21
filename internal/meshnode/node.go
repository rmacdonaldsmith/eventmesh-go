package meshnode

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/rmacdonaldsmith/eventmesh-go/internal/eventlog"
	"github.com/rmacdonaldsmith/eventmesh-go/internal/peerlink"
	"github.com/rmacdonaldsmith/eventmesh-go/internal/routingtable"
	eventlogpkg "github.com/rmacdonaldsmith/eventmesh-go/pkg/eventlog"
	"github.com/rmacdonaldsmith/eventmesh-go/pkg/meshnode"
	peerlinkpkg "github.com/rmacdonaldsmith/eventmesh-go/pkg/peerlink"
	routingtablepkg "github.com/rmacdonaldsmith/eventmesh-go/pkg/routingtable"
)

const (
	// MaxEventPayloadSize is the maximum allowed payload size in bytes (1MB)
	MaxEventPayloadSize = 1024 * 1024

	peerSubscriptionResyncInterval = 100 * time.Millisecond
)

// GRPCMeshNode implements the meshnode.MeshNode interface.
// It orchestrates EventLog, RoutingTable, and PeerLink components to provide
// distributed event streaming functionality.
type GRPCMeshNode struct {
	mu     sync.RWMutex
	config *Config

	// Core components
	eventLog             eventlogpkg.EventLog
	routingTable         routingtablepkg.RoutingTable
	dataPlanePeerLink    peerlinkpkg.DataPlanePeerLink
	controlPlanePeerLink peerlinkpkg.ControlPlanePeerLink
	peerConnections      peerlinkpkg.PeerConnectionManager

	// State management
	started         bool
	closed          bool
	lifecycleCancel context.CancelFunc

	// Client management (simplified for MVP - no authentication)
	clients map[string]meshnode.Client

	// Subscription management
	subscriptions   map[string]map[string]*meshnode.ClientSubscription // clientID -> subscriptionID -> subscription
	subscriptionsMu sync.RWMutex                                       // Protect subscriptions map

	// Peer subscription tracking for intelligent routing
	peerSubscriptions   map[string]map[string]bool // peerNodeID -> topic -> hasSubscriber
	peerSubscriptionsMu sync.RWMutex               // Protect peer subscriptions map

	interestUpdatesSent       atomic.Int64
	interestUpdatesReceived   atomic.Int64
	interestSnapshotsSent     atomic.Int64
	interestSnapshotsReceived atomic.Int64
	emptySnapshotsReceived    atomic.Int64
	snapshotTopicsReceived    atomic.Int64
}

func (n *GRPCMeshNode) setPeerLink(peerLink peerlinkpkg.CompletePeerLink) {
	n.dataPlanePeerLink = peerLink
	n.controlPlanePeerLink = peerLink
	n.peerConnections = peerLink
}

func (n *GRPCMeshNode) completePeerLink() peerlinkpkg.CompletePeerLink {
	return composedPeerLink{
		dataPlane:   n.dataPlanePeerLink,
		control:     n.controlPlanePeerLink,
		connections: n.peerConnections,
	}
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

	var nodeEventLog eventlogpkg.EventLog
	if config.EventLogFactory != nil {
		var err error
		nodeEventLog, err = config.EventLogFactory()
		if err != nil {
			return nil, fmt.Errorf("failed to create EventLog: %w", err)
		}
		if nodeEventLog == nil {
			return nil, fmt.Errorf("failed to create EventLog: factory returned nil")
		}
	} else {
		nodeEventLog = eventlog.NewInMemoryEventLog()
	}

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
		config:            config,
		eventLog:          nodeEventLog,
		routingTable:      routingTable,
		clients:           make(map[string]meshnode.Client),
		subscriptions:     make(map[string]map[string]*meshnode.ClientSubscription),
		peerSubscriptions: make(map[string]map[string]bool),
		started:           false,
		closed:            false,
	}
	node.setPeerLink(peerLink)

	return node, nil
}

// Verify that GRPCMeshNode implements the MeshNode interface at compile time
var _ meshnode.MeshNode = (*GRPCMeshNode)(nil)
