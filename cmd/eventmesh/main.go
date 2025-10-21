package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/rmacdonaldsmith/eventmesh-go/internal/httpapi"
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
		nodeID      = flag.String("node-id", getDefaultNodeID(), "Unique node identifier")
		listenAddr  = flag.String("listen", ":8080", "Listen address for client connections")
		peerAddr    = flag.String("peer-listen", ":9090", "Listen address for peer connections")
		connectPeer = flag.String("connect-peer", "", "Address of peer node to connect to (optional)")
		enableHTTP  = flag.Bool("http", false, "Enable HTTP API server")
		httpPort    = flag.String("http-port", "8081", "Port for HTTP API server")
		noAuth      = flag.Bool("no-auth", false, "Disable authentication for development (INSECURE - development only)")
		showVersion = flag.Bool("version", false, "Show version and exit")
		showHealth  = flag.Bool("health", false, "Show health status and exit")
		seedNodes   = flag.String("seed-nodes", "", "Comma-separated list of seed node addresses (e.g., \"node1:8080,node2:8080\")")
		logLevel    = flag.String("log-level", "info", "Log level (debug, info, warn, error)")
	)
	flag.Parse()

	// Handle version flag
	if *showVersion {
		fmt.Printf("%s v%s\n", appName, appVersion)
		os.Exit(0)
	}

	// Configure structured logging
	setupLogging(*logLevel)

	slog.Info("starting EventMesh",
		"app", appName,
		"version", appVersion,
		"node_id", *nodeID,
		"log_level", *logLevel)
	slog.Info("server configuration",
		"client_listen", *listenAddr,
		"peer_listen", *peerAddr,
		"http_enabled", *enableHTTP,
		"http_port", *httpPort,
		"no_auth", *noAuth)

	// Create PeerLink configuration
	peerLinkConfig := &peerlink.Config{
		NodeID:        *nodeID,
		ListenAddress: *peerAddr,
	}
	peerLinkConfig.SetDefaults()

	// Parse seed nodes if provided
	var bootstrapConfig *meshnode.BootstrapConfig
	if *seedNodes != "" {
		seedList := strings.Split(strings.TrimSpace(*seedNodes), ",")
		// Clean up whitespace from each seed node address
		for i, seed := range seedList {
			seedList[i] = strings.TrimSpace(seed)
		}
		bootstrapConfig = meshnode.NewBootstrapConfig(seedList)
		slog.Info("bootstrap configuration",
			"seed_node_count", len(seedList),
			"seed_nodes", seedList)
	}

	// Create MeshNode configuration
	config := meshnode.NewConfig(*nodeID, *listenAddr).
		WithPeerLinkConfig(peerLinkConfig).
		WithBootstrapConfig(bootstrapConfig)

	// Validate configuration
	if err := config.Validate(); err != nil {
		log.Fatalf("❌ Invalid configuration: %v", err)
	}

	// Create mesh node
	slog.Debug("creating mesh node", "node_id", *nodeID)
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

	// Create HTTP API server if enabled
	var httpServer *httpapi.Server
	if *enableHTTP {
		slog.Debug("creating HTTP API server", "http_port", *httpPort)

		// Safety warning for no-auth mode
		if *noAuth {
			slog.Warn("running in NO-AUTH mode - authentication is DISABLED",
				"security_warning", "INSECURE - development/testing only",
				"admin_endpoints_note", "admin endpoints still require valid JWT tokens")
		}

		// Get JWT secret from environment variable
		jwtSecret := os.Getenv("EVENTMESH_JWT_SECRET")
		if jwtSecret == "" {
			jwtSecret = "eventmesh-mvp-secret-key-change-in-production"
			slog.Warn("using default JWT secret",
				"security_warning", "set EVENTMESH_JWT_SECRET environment variable for production")
		}

		httpConfig := httpapi.Config{
			Port:      *httpPort,
			SecretKey: jwtSecret,
			NoAuth:    *noAuth,
		}
		httpServer = httpapi.NewServer(node, httpConfig)
	}

	// Handle health check flag
	if *showHealth {
		showHealthStatus(node)
		return
	}

	// Start the mesh node
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	slog.Debug("starting mesh node", "node_id", *nodeID)
	if err := node.Start(ctx); err != nil {
		log.Fatalf("❌ Failed to start mesh node: %v", err)
	}

	// Start HTTP API server if enabled
	if httpServer != nil {
		slog.Debug("starting HTTP API server", "port", *httpPort)
		go func() {
			if err := httpServer.Start(); err != nil {
				log.Printf("❌ HTTP API server error: %v", err)
			}
		}()
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
	setupGracefulShutdown(ctx, cancel, node, httpServer)

	slog.Info("node started successfully",
		"app", appName,
		"node_id", *nodeID,
		"shutdown_instruction", "use Ctrl+C to shutdown gracefully")

	// Wait for shutdown signal
	<-ctx.Done()
	slog.Info("node stopped",
		"app", appName,
		"node_id", *nodeID)
}

// setupLogging configures the global slog logger with the specified level
func setupLogging(levelStr string) {
	var level slog.Level

	switch strings.ToLower(levelStr) {
	case "debug":
		level = slog.LevelDebug
	case "info":
		level = slog.LevelInfo
	case "warn", "warning":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	default:
		// Default to INFO for unknown levels
		level = slog.LevelInfo
		fmt.Fprintf(os.Stderr, "Warning: unknown log level '%s', using 'info'\n", levelStr)
	}

	// Create structured text handler for human-readable output
	opts := &slog.HandlerOptions{
		Level: level,
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			// Simplify time format for better readability
			if a.Key == slog.TimeKey {
				return slog.String("time", a.Value.Time().Format("15:04:05.000"))
			}
			return a
		},
	}

	handler := slog.NewTextHandler(os.Stdout, opts)
	slog.SetDefault(slog.New(handler))
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
func setupGracefulShutdown(ctx context.Context, cancel context.CancelFunc, node *meshnode.GRPCMeshNode, httpServer *httpapi.Server) {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)

	go func() {
		sig := <-sigChan
		slog.Info("received shutdown signal",
			"signal", sig,
			"action", "shutting down gracefully")

		// Create shutdown timeout context
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer shutdownCancel()

		// Stop the HTTP server first (if running)
		if httpServer != nil {
			slog.Info("stopping HTTP API server")
			if err := httpServer.Stop(shutdownCtx); err != nil {
				slog.Error("error stopping HTTP server", "error", err)
			}
		}

		// Stop the mesh node
		slog.Info("stopping mesh node")
		if err := node.Stop(shutdownCtx); err != nil {
			slog.Error("error during graceful stop", "error", err)
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
