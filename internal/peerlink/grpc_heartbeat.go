package peerlink

import (
	"context"
	"errors"
	"time"

	"github.com/rmacdonaldsmith/eventmesh-go/pkg/peerlink"
)

// StartHeartbeats begins health monitoring for all connected peers
func (g *GRPCPeerLink) StartHeartbeats(ctx context.Context) error {
	if ctx == nil {
		return errors.New("context cannot be nil")
	}

	g.mu.Lock()
	if g.closed {
		g.mu.Unlock()
		return errors.New("PeerLink is closed")
	}
	if g.heartbeatsRunning {
		g.mu.Unlock()
		return nil
	}

	heartbeatCtx, cancel := context.WithCancel(ctx)
	token := &struct{}{}
	g.heartbeatCancel = cancel
	g.heartbeatToken = token
	g.heartbeatsRunning = true
	g.mu.Unlock()

	go g.runHeartbeatLoop(heartbeatCtx, token)
	return nil
}

// StopHeartbeats stops health monitoring
func (g *GRPCPeerLink) StopHeartbeats(ctx context.Context) error {
	g.mu.Lock()
	cancel := g.heartbeatCancel
	g.heartbeatCancel = nil
	g.heartbeatToken = nil
	g.heartbeatsRunning = false
	g.mu.Unlock()

	if cancel != nil {
		cancel()
	}
	return nil
}

func (g *GRPCPeerLink) runHeartbeatLoop(ctx context.Context, token *struct{}) {
	defer g.clearHeartbeatLifecycle(token)

	g.sendHeartbeatToConnectedPeers(ctx)

	ticker := time.NewTicker(g.config.HeartbeatInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			g.sendHeartbeatToConnectedPeers(ctx)
		}
	}
}

func (g *GRPCPeerLink) clearHeartbeatLifecycle(token *struct{}) {
	g.mu.Lock()
	defer g.mu.Unlock()

	if g.heartbeatToken == token {
		g.heartbeatCancel = nil
		g.heartbeatToken = nil
		g.heartbeatsRunning = false
	}
}

func (g *GRPCPeerLink) sendHeartbeatToConnectedPeers(ctx context.Context) {
	g.markSilentPeersUnhealthy(time.Now())

	peerIDs := g.connectedPeerIDs()
	for _, peerID := range peerIDs {
		if err := g.SendHeartbeat(ctx, peerID); err != nil {
			g.recordPlaneSendFailure(peerID, controlPlaneMessage)
			g.recordPeerSendFailure(peerID)
		}
	}
}

func (g *GRPCPeerLink) markSilentPeersUnhealthy(now time.Time) {
	g.mu.Lock()
	defer g.mu.Unlock()

	timeout := g.heartbeatTimeout()
	for _, metrics := range g.metrics {
		if metrics.healthState == peerlink.PeerDisconnected || metrics.lastSeenAt.IsZero() {
			continue
		}
		if now.Sub(metrics.lastSeenAt) >= timeout {
			metrics.healthState = peerlink.PeerUnhealthy
			metrics.missedHeartbeatCount = int(now.Sub(metrics.lastSeenAt) / g.config.HeartbeatInterval)
		}
	}
}

func (g *GRPCPeerLink) heartbeatTimeout() time.Duration {
	return g.config.HeartbeatInterval * time.Duration(g.config.MissedHeartbeatLimit)
}

func (g *GRPCPeerLink) connectedPeerIDs() []string {
	g.mu.RLock()
	defer g.mu.RUnlock()

	peerIDs := make([]string, 0, len(g.connections))
	for peerID := range g.connections {
		peerIDs = append(peerIDs, peerID)
	}
	return peerIDs
}
