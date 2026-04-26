package peerlink

import (
	"context"
	"testing"
	"time"

	"github.com/rmacdonaldsmith/eventmesh-go/pkg/peerlink"
)

func TestGRPCPeerLink_SendHeartbeatMarksReceiverHealthy(t *testing.T) {
	server, client := newConnectedHeartbeatPeerLinks(t, 5*time.Second)
	defer server.Close()
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	waitForPeerHealth(t, ctx, server, "client-node", peerlink.PeerHealthy)
	server.SetPeerHealth("client-node", peerlink.PeerUnhealthy)
	waitForPeerHealth(t, ctx, server, "client-node", peerlink.PeerUnhealthy)

	if err := client.SendHeartbeat(ctx, "server-node"); err != nil {
		t.Fatalf("SendHeartbeat failed: %v", err)
	}

	waitForPeerHealth(t, ctx, server, "client-node", peerlink.PeerHealthy)
}

func TestGRPCPeerLink_StartHeartbeatsSendsPeriodicHeartbeats(t *testing.T) {
	server, client := newConnectedHeartbeatPeerLinks(t, 20*time.Millisecond)
	defer server.Close()
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	waitForPeerHealth(t, ctx, server, "client-node", peerlink.PeerHealthy)
	server.SetPeerHealth("client-node", peerlink.PeerUnhealthy)
	waitForPeerHealth(t, ctx, server, "client-node", peerlink.PeerUnhealthy)

	if err := client.StartHeartbeats(ctx); err != nil {
		t.Fatalf("StartHeartbeats failed: %v", err)
	}
	defer client.StopHeartbeats(context.Background())

	waitForPeerHealth(t, ctx, server, "client-node", peerlink.PeerHealthy)

	if err := client.StartHeartbeats(ctx); err != nil {
		t.Fatalf("second StartHeartbeats should be idempotent: %v", err)
	}
	if err := client.StopHeartbeats(ctx); err != nil {
		t.Fatalf("StopHeartbeats failed: %v", err)
	}
	if err := client.StopHeartbeats(ctx); err != nil {
		t.Fatalf("second StopHeartbeats should be idempotent: %v", err)
	}
}

func TestGRPCPeerLink_StartHeartbeatsMarksSilentPeerUnhealthy(t *testing.T) {
	peerLink, err := NewGRPCPeerLink(&Config{
		NodeID:               "monitor-node",
		ListenAddress:        "localhost:0",
		SendQueueSize:        100,
		HeartbeatInterval:    20 * time.Millisecond,
		MissedHeartbeatLimit: 2,
	})
	if err != nil {
		t.Fatalf("failed to create PeerLink: %v", err)
	}
	defer peerLink.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	peerLink.mu.Lock()
	peerLink.registerPeer("silent-peer")
	peerLink.connections["silent-peer"] = &connectionInfo{
		node: &simplePeerNode{id: "silent-peer", address: "localhost:0", healthy: true},
	}
	peerLink.mu.Unlock()

	waitForPeerHealth(t, ctx, peerLink, "silent-peer", peerlink.PeerHealthy)

	if err := peerLink.StartHeartbeats(ctx); err != nil {
		t.Fatalf("StartHeartbeats failed: %v", err)
	}
	defer peerLink.StopHeartbeats(context.Background())

	waitForPeerHealth(t, ctx, peerLink, "silent-peer", peerlink.PeerUnhealthy)
}

func TestGRPCPeerLink_HeartbeatRecoveryResetsHealthCounters(t *testing.T) {
	server, client := newConnectedHeartbeatPeerLinks(t, 5*time.Second)
	defer server.Close()
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	waitForPeerHealth(t, ctx, server, "client-node", peerlink.PeerHealthy)

	server.mu.Lock()
	metrics := server.metrics["client-node"]
	metrics.healthState = peerlink.PeerUnhealthy
	metrics.failureCount = 2
	metrics.missedHeartbeatCount = 3
	metrics.lastSeenAt = time.Now().Add(-time.Minute)
	server.mu.Unlock()

	if err := client.SendHeartbeat(ctx, "server-node"); err != nil {
		t.Fatalf("SendHeartbeat failed: %v", err)
	}

	waitForPeerHealth(t, ctx, server, "client-node", peerlink.PeerHealthy)

	server.mu.RLock()
	defer server.mu.RUnlock()
	metrics = server.metrics["client-node"]
	if metrics.failureCount != 0 {
		t.Errorf("expected failureCount reset to 0, got %d", metrics.failureCount)
	}
	if metrics.missedHeartbeatCount != 0 {
		t.Errorf("expected missedHeartbeatCount reset to 0, got %d", metrics.missedHeartbeatCount)
	}
	if time.Since(metrics.lastSeenAt) > time.Second {
		t.Errorf("expected lastSeenAt to be refreshed recently, got %v", metrics.lastSeenAt)
	}
}

func newConnectedHeartbeatPeerLinks(t *testing.T, heartbeatInterval time.Duration) (*GRPCPeerLink, *GRPCPeerLink) {
	t.Helper()

	server, err := NewGRPCPeerLink(&Config{
		NodeID:            "server-node",
		ListenAddress:     "localhost:0",
		HeartbeatInterval: heartbeatInterval,
	})
	if err != nil {
		t.Fatalf("failed to create server PeerLink: %v", err)
	}

	client, err := NewGRPCPeerLink(&Config{
		NodeID:            "client-node",
		ListenAddress:     "localhost:0",
		HeartbeatInterval: heartbeatInterval,
	})
	if err != nil {
		server.Close()
		t.Fatalf("failed to create client PeerLink: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Cleanup(cancel)

	if err := server.Start(ctx); err != nil {
		client.Close()
		server.Close()
		t.Fatalf("failed to start server PeerLink: %v", err)
	}
	if err := client.Start(ctx); err != nil {
		client.Close()
		server.Close()
		t.Fatalf("failed to start client PeerLink: %v", err)
	}

	if err := client.Connect(ctx, &simplePeerNode{
		id:      "server-node",
		address: server.GetListeningAddress(),
		healthy: true,
	}); err != nil {
		client.Close()
		server.Close()
		t.Fatalf("failed to connect client to server: %v", err)
	}

	waitForPeerHealth(t, ctx, client, "server-node", peerlink.PeerHealthy)
	return server, client
}

func waitForPeerHealth(t *testing.T, ctx context.Context, link *GRPCPeerLink, peerID string, expected peerlink.PeerHealthState) {
	t.Helper()

	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()

	for {
		health, _ := link.GetPeerHealth(ctx, peerID)
		if health == expected {
			return
		}

		select {
		case <-ctx.Done():
			t.Fatalf("timed out waiting for %s health to become %s", peerID, expected)
		case <-ticker.C:
		}
	}
}
