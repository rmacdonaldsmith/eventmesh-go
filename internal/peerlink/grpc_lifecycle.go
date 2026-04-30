package peerlink

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"strings"

	peerlinkv1 "github.com/rmacdonaldsmith/eventmesh-go/proto/peerlink/v1"
	"google.golang.org/grpc"
)

// Start initializes the gRPC server and begins listening for connections
func (g *GRPCPeerLink) Start(ctx context.Context) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	if g.closed {
		return errors.New("PeerLink is closed")
	}

	if g.started {
		return nil // Already started, safe to call multiple times
	}

	// Create listener
	listener, err := net.Listen("tcp", g.config.ListenAddress)
	if err != nil {
		return err
	}

	// Create gRPC server
	grpcServer := grpc.NewServer()

	// Register the PeerLink service (we'll implement the service handler later)
	peerlinkv1.RegisterPeerLinkServer(grpcServer, g)

	// Store server references
	g.listener = listener
	g.grpcServer = grpcServer
	g.started = true

	// Log the actual listening address
	slog.Info("peer server listening",
		"address", formatAddress(listener.Addr().String()),
		"node_id", g.config.NodeID)

	// Start serving in a goroutine
	go func() {
		if err := grpcServer.Serve(listener); err != nil {
			// Log error but don't panic - server shutdown is normal
			slog.Debug("peer server stopped",
				"node_id", g.config.NodeID,
				"error", err)
		}
	}()

	return nil
}

// Stop gracefully shuts down the gRPC server
func (g *GRPCPeerLink) Stop(ctx context.Context) error {
	g.mu.Lock()
	if !g.started || g.grpcServer == nil {
		g.mu.Unlock()
		return nil // Not started or already stopped
	}

	grpcServer := g.grpcServer
	g.grpcServer = nil
	g.listener = nil
	g.started = false
	g.mu.Unlock()

	// Graceful shutdown with context
	done := make(chan struct{})
	go func() {
		grpcServer.GracefulStop()
		close(done)
	}()

	select {
	case <-done:
		// Graceful shutdown completed
	case <-ctx.Done():
		// Context timeout - force shutdown
		grpcServer.Stop()
	}

	return nil
}

// Close closes the PeerLink and cleans up resources
func (g *GRPCPeerLink) Close() error {
	g.mu.Lock()
	defer g.mu.Unlock()

	if g.closed {
		return nil // Already closed, safe to call multiple times
	}

	if g.heartbeatCancel != nil {
		g.heartbeatCancel()
		g.heartbeatCancel = nil
		g.heartbeatToken = nil
		g.heartbeatsRunning = false
	}

	// Stop the server if it's running
	if g.started && g.grpcServer != nil {
		g.grpcServer.Stop() // Force stop since we don't have context
		g.grpcServer = nil
		g.listener = nil
		g.started = false
	}

	// Cancel all outbound connections
	for _, cancel := range g.outboundCancel {
		cancel()
	}
	g.outboundCancel = nil
	g.outboundConns = nil

	g.closed = true
	return nil
}

// GetListeningAddress returns the actual address this PeerLink is listening on
// Returns empty string if not started or no listener
func (g *GRPCPeerLink) GetListeningAddress() string {
	g.mu.RLock()
	defer g.mu.RUnlock()

	if g.listener != nil {
		return g.listener.Addr().String()
	}
	return ""
}

// formatAddress converts IPv6 addresses to more readable format
func formatAddress(addr string) string {
	if addr == "" {
		return ""
	}

	// Convert [::]:8080 to localhost:8080 for readability
	if strings.HasPrefix(addr, "[::]:") {
		port := strings.TrimPrefix(addr, "[::]:")
		return fmt.Sprintf("localhost:%s", port)
	}

	// Return as-is for other formats (IPv4, etc.)
	return addr
}
