package meshnode

import (
	"context"
	"fmt"

	eventlogpkg "github.com/rmacdonaldsmith/eventmesh-go/pkg/eventlog"
	"github.com/rmacdonaldsmith/eventmesh-go/pkg/meshnode"
	peerlinkpkg "github.com/rmacdonaldsmith/eventmesh-go/pkg/peerlink"
	routingtablepkg "github.com/rmacdonaldsmith/eventmesh-go/pkg/routingtable"
)

// AuthenticateClient validates and authenticates a connecting client.
// FOR MVP: Creates a TrustedClient (REQ-MNODE-001 descoped from MVP)
func (n *GRPCMeshNode) AuthenticateClient(ctx context.Context, credentials interface{}) (meshnode.Client, error) {
	n.mu.Lock()
	defer n.mu.Unlock()

	if n.closed {
		return nil, fmt.Errorf("cannot authenticate to closed mesh node")
	}

	if credentials == nil {
		return nil, fmt.Errorf("credentials cannot be nil")
	}

	// For MVP: credentials is expected to be a client ID string
	clientID, ok := credentials.(string)
	if !ok {
		return nil, fmt.Errorf("credentials must be a string client ID")
	}

	if clientID == "" {
		return nil, fmt.Errorf("client ID cannot be empty")
	}

	// Check if client already exists
	if existingClient, exists := n.clients[clientID]; exists {
		return existingClient, nil // Return existing client
	}

	// Create new TrustedClient
	client := NewTrustedClient(clientID)

	// Add to client tracking
	n.clients[clientID] = client

	return client, nil
}

// GetEventLog returns the node's event log interface.
func (n *GRPCMeshNode) GetEventLog() eventlogpkg.EventLog {
	n.mu.RLock()
	defer n.mu.RUnlock()
	return n.eventLog
}

// GetRoutingTable returns the node's routing table interface.
func (n *GRPCMeshNode) GetRoutingTable() routingtablepkg.RoutingTable {
	n.mu.RLock()
	defer n.mu.RUnlock()
	return n.routingTable
}

// GetPeerLink returns the node's peer link interface.
func (n *GRPCMeshNode) GetPeerLink() peerlinkpkg.CompletePeerLink {
	n.mu.RLock()
	defer n.mu.RUnlock()
	return n.completePeerLink()
}

// GetNodeID returns this node's unique identifier in the mesh.
func (n *GRPCMeshNode) GetNodeID() string {
	n.mu.RLock()
	defer n.mu.RUnlock()
	return n.config.NodeID
}

// GetConnectedClients returns clients currently known to this node.
func (n *GRPCMeshNode) GetConnectedClients(ctx context.Context) ([]meshnode.Client, error) {
	n.mu.RLock()
	defer n.mu.RUnlock()

	clients := make([]meshnode.Client, 0, len(n.clients))
	for _, client := range n.clients {
		clients = append(clients, client)
	}
	return clients, nil
}

// GetConnectedPeers returns all currently connected peer nodes.
func (n *GRPCMeshNode) GetConnectedPeers(ctx context.Context) ([]peerlinkpkg.PeerNode, error) {
	n.mu.RLock()
	defer n.mu.RUnlock()
	return n.peerConnections.GetConnectedPeers(ctx)
}
