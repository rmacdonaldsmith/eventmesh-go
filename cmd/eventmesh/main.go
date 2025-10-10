package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/rmacdonaldsmith/eventmesh-go/internal/meshnode"
	"github.com/rmacdonaldsmith/eventmesh-go/internal/peerlink"
)

const (
	// Application info
	appName    = "EventMesh"
	appVersion = "0.1.0"
)

func main() {
	// Command-line flags
	var (
		nodeID       = flag.String("node-id", getDefaultNodeID(), "Unique node identifier")
		listenAddr   = flag.String("listen", ":8080", "Listen address for client connections")
		peerAddr     = flag.String("peer-listen", ":9090", "Listen address for peer connections")
		connectPeer  = flag.String("connect-peer", "", "Address of peer node to connect to (optional)")
		showVersion  = flag.Bool("version", false, "Show version and exit")
		showHealth   = flag.Bool("health", false, "Show health status and exit")
	)
	flag.Parse()

	// Handle version flag
	if *showVersion {
		fmt.Printf("%s v%s\n", appName, appVersion)
		os.Exit(0)
	}

	// Configure logging
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	log.Printf("🚀 Starting %s v%s", appName, appVersion)
	log.Printf("📋 Node ID: %s", *nodeID)
	log.Printf("🔌 Client Listen: %s", *listenAddr)
	log.Printf("🔗 Peer Listen: %s", *peerAddr)

	// Create PeerLink configuration
	peerLinkConfig := &peerlink.Config{
		NodeID:        *nodeID,
		ListenAddress: *peerAddr,
	}
	peerLinkConfig.SetDefaults()

	// Create MeshNode configuration
	config := meshnode.NewConfig(*nodeID, *listenAddr).
		WithPeerLinkConfig(peerLinkConfig)

	// Validate configuration
	if err := config.Validate(); err != nil {
		log.Fatalf("❌ Invalid configuration: %v", err)
	}

	// Create mesh node
	log.Printf("🔧 Creating mesh node...")
	node, err := meshnode.NewGRPCMeshNode(config)
	if err != nil {
		log.Fatalf("❌ Failed to create mesh node: %v", err)
	}
	defer func() {
		log.Printf("🛑 Closing mesh node...")
		if err := node.Close(); err != nil {
			log.Printf("⚠️  Error closing node: %v", err)
		}
	}()

	// Handle health check flag
	if *showHealth {
		showHealthStatus(node)
		return
	}

	// Start the mesh node
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	log.Printf("▶️  Starting mesh node...")
	if err := node.Start(ctx); err != nil {
		log.Fatalf("❌ Failed to start mesh node: %v", err)
	}

	// Connect to peer if specified
	if *connectPeer != "" {
		log.Printf("🤝 Connecting to peer: %s", *connectPeer)
		// Note: Peer connection will be implemented when PeerLink networking is ready
		log.Printf("⚠️  Peer connection not yet implemented (PeerLink networking in progress)")
	}

	// Show startup success and health
	showStartupInfo(node)

	// Set up graceful shutdown
	setupGracefulShutdown(ctx, cancel, node)

	log.Printf("✅ %s node %s started successfully!", appName, *nodeID)
	log.Printf("💡 Use Ctrl+C to shutdown gracefully")

	// Wait for shutdown signal
	<-ctx.Done()
	log.Printf("👋 %s node %s stopped", appName, *nodeID)
}

// getDefaultNodeID generates a default node ID based on hostname
func getDefaultNodeID() string {
	hostname, err := os.Hostname()
	if err != nil {
		return "eventmesh-node-1"
	}
	return fmt.Sprintf("eventmesh-%s", hostname)
}

// setupGracefulShutdown configures signal handling for graceful shutdown
func setupGracefulShutdown(ctx context.Context, cancel context.CancelFunc, node *meshnode.GRPCMeshNode) {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)

	go func() {
		sig := <-sigChan
		log.Printf("🛑 Received signal %v, shutting down gracefully...", sig)

		// Create shutdown timeout context
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer shutdownCancel()

		// Stop the mesh node
		if err := node.Stop(shutdownCtx); err != nil {
			log.Printf("⚠️  Error during graceful stop: %v", err)
		}

		// Cancel main context to exit
		cancel()
	}()
}

// showStartupInfo displays node information after successful startup
func showStartupInfo(node *meshnode.GRPCMeshNode) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Show health status
	health, err := node.GetHealth(ctx)
	if err != nil {
		log.Printf("⚠️  Could not get health status: %v", err)
		return
	}

	log.Printf("🏥 Health Status:")
	log.Printf("   Overall: %s", healthStatus(health.Healthy))
	log.Printf("   EventLog: %s", healthStatus(health.EventLogHealthy))
	log.Printf("   RoutingTable: %s", healthStatus(health.RoutingTableHealthy))
	log.Printf("   PeerLink: %s", healthStatus(health.PeerLinkHealthy))
	log.Printf("   Connected Clients: %d", health.ConnectedClients)
	log.Printf("   Connected Peers: %d", health.ConnectedPeers)

	if !health.Healthy {
		log.Printf("⚠️  Health issues: %s", health.Message)
	}

	// Show component access info
	log.Printf("🔧 Components:")
	log.Printf("   EventLog: %T", node.GetEventLog())
	log.Printf("   RoutingTable: %T", node.GetRoutingTable())
	log.Printf("   PeerLink: %T", node.GetPeerLink())
}

// showHealthStatus shows health and exits (for --health flag)
func showHealthStatus(node *meshnode.GRPCMeshNode) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	health, err := node.GetHealth(ctx)
	if err != nil {
		log.Fatalf("❌ Failed to get health status: %v", err)
	}

	fmt.Printf("EventMesh Node Health Status:\n")
	fmt.Printf("  Overall: %s\n", healthStatus(health.Healthy))
	fmt.Printf("  EventLog: %s\n", healthStatus(health.EventLogHealthy))
	fmt.Printf("  RoutingTable: %s\n", healthStatus(health.RoutingTableHealthy))
	fmt.Printf("  PeerLink: %s\n", healthStatus(health.PeerLinkHealthy))
	fmt.Printf("  Connected Clients: %d\n", health.ConnectedClients)
	fmt.Printf("  Connected Peers: %d\n", health.ConnectedPeers)
	fmt.Printf("  Message: %s\n", health.Message)

	if health.Healthy {
		os.Exit(0)
	} else {
		os.Exit(1)
	}
}

// healthStatus returns a colored health status string
func healthStatus(healthy bool) string {
	if healthy {
		return "✅ Healthy"
	}
	return "❌ Unhealthy"
}