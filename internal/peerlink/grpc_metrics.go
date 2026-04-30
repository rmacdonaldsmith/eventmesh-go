package peerlink

import (
	"context"
	"time"

	"github.com/rmacdonaldsmith/eventmesh-go/pkg/peerlink"
)

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
