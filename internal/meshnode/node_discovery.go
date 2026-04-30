package meshnode

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/rmacdonaldsmith/eventmesh-go/internal/discovery"
)

// runDiscovery performs node discovery and connects to discovered peers
func (n *GRPCMeshNode) runDiscovery(ctx context.Context) error {
	seedNodes := n.config.BootstrapConfig.SeedNodes
	slog.Info("starting peer discovery",
		"node_id", n.config.NodeID,
		"seed_nodes", len(seedNodes))

	// Create static discovery service from bootstrap config
	discoveryService := discovery.NewStaticDiscovery(seedNodes)

	// Find peer nodes
	peers, err := discoveryService.FindPeers(ctx)
	if err != nil {
		return fmt.Errorf("discovery failed: %w", err)
	}

	slog.Info("peers discovered",
		"node_id", n.config.NodeID,
		"peers_found", len(peers))

	// Connect to discovered peers
	connectedCount := 0
	for _, peer := range peers {
		slog.Debug("attempting peer connection",
			"node_id", n.config.NodeID,
			"peer_address", peer.Address(),
			"peer_id", peer.ID())

		// Attempt to connect to the peer
		err := n.peerConnections.Connect(ctx, peer)
		if err != nil {
			slog.Warn("failed to connect to discovered peer",
				"node_id", n.config.NodeID,
				"peer_address", peer.Address(),
				"peer_id", peer.ID(),
				"error", err)
			continue
		}

		connectedCount++
	}

	slog.Info("discovery complete",
		"node_id", n.config.NodeID,
		"peers_found", len(peers),
		"peers_connected", connectedCount)
	return nil
}
