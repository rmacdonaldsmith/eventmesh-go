package meshnode

import (
	"context"
	"fmt"

	"github.com/rmacdonaldsmith/eventmesh-go/pkg/meshnode"
	peerlinkpkg "github.com/rmacdonaldsmith/eventmesh-go/pkg/peerlink"
)

// GetHealth returns the overall health status of this mesh node.
// Performs comprehensive health checks on all components and provides detailed status.
func (n *GRPCMeshNode) GetHealth(ctx context.Context) (meshnode.HealthStatus, error) {
	n.mu.RLock()
	defer n.mu.RUnlock()

	var healthMessages []string

	// Check node lifecycle state
	nodeHealthy := !n.closed
	if n.closed {
		healthMessages = append(healthMessages, "node is closed")
	}
	if !n.started {
		healthMessages = append(healthMessages, "node is not started")
	}

	// Check EventLog health with a read-only operation.
	eventLogHealthy := true
	if n.closed {
		// If node is closed, components are closed and unhealthy
		eventLogHealthy = false
		healthMessages = append(healthMessages, "EventLog unhealthy: node is closed")
	} else if n.eventLog != nil {
		_, err := n.eventLog.GetStatistics(ctx)
		if err != nil {
			eventLogHealthy = false
			healthMessages = append(healthMessages, fmt.Sprintf("EventLog unhealthy: %v", err))
		}
	} else {
		eventLogHealthy = false
		healthMessages = append(healthMessages, "EventLog is nil")
	}

	// Check RoutingTable health - test basic operations
	routingTableHealthy := true
	if n.closed {
		// If node is closed, components are closed and unhealthy
		routingTableHealthy = false
		healthMessages = append(healthMessages, "RoutingTable unhealthy: node is closed")
	} else if n.routingTable != nil {
		// Test RoutingTable by checking if we can perform a lookup
		_, err := n.routingTable.GetSubscribers(ctx, "__health.check")
		if err != nil {
			routingTableHealthy = false
			healthMessages = append(healthMessages, fmt.Sprintf("RoutingTable unhealthy: %v", err))
		}
	} else {
		routingTableHealthy = false
		healthMessages = append(healthMessages, "RoutingTable is nil")
	}

	// Check PeerLink health - test connectivity
	peerLinkHealthy := true
	var peers []peerlinkpkg.PeerNode
	if n.closed {
		// If node is closed, components are closed and unhealthy
		peerLinkHealthy = false
		healthMessages = append(healthMessages, "PeerLink unhealthy: node is closed")
		peers = []peerlinkpkg.PeerNode{}
	} else if n.peerConnections != nil {
		var err error
		peers, err = n.peerConnections.GetConnectedPeers(ctx)
		if err != nil {
			peerLinkHealthy = false
			healthMessages = append(healthMessages, fmt.Sprintf("PeerLink unhealthy: %v", err))
			peers = []peerlinkpkg.PeerNode{} // Ensure we have a valid slice
		}
	} else {
		peerLinkHealthy = false
		healthMessages = append(healthMessages, "PeerLink is nil")
		peers = []peerlinkpkg.PeerNode{}
	}

	// Overall health: all components must be healthy
	overallHealthy := nodeHealthy && eventLogHealthy && routingTableHealthy && peerLinkHealthy

	// Construct health message
	var message string
	if len(healthMessages) == 0 {
		message = "All components healthy"
	} else {
		message = "Issues detected: " + fmt.Sprintf("%v", healthMessages)
	}

	return meshnode.HealthStatus{
		Healthy:             overallHealthy,
		EventLogHealthy:     eventLogHealthy,
		RoutingTableHealthy: routingTableHealthy,
		PeerLinkHealthy:     peerLinkHealthy,
		ConnectedClients:    len(n.clients),
		ConnectedPeers:      len(peers),
		Message:             message,
	}, nil
}
