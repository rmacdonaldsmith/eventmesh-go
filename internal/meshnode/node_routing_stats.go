package meshnode

import (
	"context"
	"sort"
)

// RoutingStats is a point-in-time, lock-safe snapshot of local and peer routing
// interest. It is intended for admin/debug surfaces, not as a routing input.
type RoutingStats struct {
	LocalSubscriptions            int
	LocalClientsWithSubscriptions int
	LocalInterestTopics           []string
	PeerInterest                  map[string][]string
	InterestUpdatesSent           int64
	InterestUpdatesReceived       int64
	InterestSnapshotsSent         int64
	InterestSnapshotsReceived     int64
	EmptySnapshotsReceived        int64
	SnapshotTopicsReceived        int64
}

// GetRoutingStats returns a point-in-time snapshot of routing and peer-interest
// state for observability.
func (n *GRPCMeshNode) GetRoutingStats(ctx context.Context) RoutingStats {
	_ = ctx

	stats := RoutingStats{
		LocalInterestTopics:       n.localInterestSnapshot(),
		PeerInterest:              n.peerInterestSnapshot(),
		InterestUpdatesSent:       n.interestUpdatesSent.Load(),
		InterestUpdatesReceived:   n.interestUpdatesReceived.Load(),
		InterestSnapshotsSent:     n.interestSnapshotsSent.Load(),
		InterestSnapshotsReceived: n.interestSnapshotsReceived.Load(),
		EmptySnapshotsReceived:    n.emptySnapshotsReceived.Load(),
		SnapshotTopicsReceived:    n.snapshotTopicsReceived.Load(),
	}

	stats.LocalSubscriptions, stats.LocalClientsWithSubscriptions = n.localSubscriptionCounts()
	return stats
}

func (n *GRPCMeshNode) localSubscriptionCounts() (subscriptions int, clients int) {
	n.subscriptionsMu.RLock()
	defer n.subscriptionsMu.RUnlock()

	for _, clientSubscriptions := range n.subscriptions {
		if len(clientSubscriptions) == 0 {
			continue
		}
		clients++
		subscriptions += len(clientSubscriptions)
	}
	return subscriptions, clients
}

func (n *GRPCMeshNode) peerInterestSnapshot() map[string][]string {
	n.peerSubscriptionsMu.RLock()
	defer n.peerSubscriptionsMu.RUnlock()

	peers := make(map[string][]string, len(n.peerSubscriptions))
	for peerID, topics := range n.peerSubscriptions {
		if len(topics) == 0 {
			continue
		}
		peerTopics := make([]string, 0, len(topics))
		for topic, interested := range topics {
			if interested {
				peerTopics = append(peerTopics, topic)
			}
		}
		if len(peerTopics) == 0 {
			continue
		}
		sort.Strings(peerTopics)
		peers[peerID] = peerTopics
	}
	return peers
}
