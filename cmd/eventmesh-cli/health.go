package main

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
)

func newHealthCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "health",
		Short: "Check server health",
		Long:  "Check the health status of the EventMesh server",
		RunE:  runHealth,
	}

	return cmd
}

func runHealth(cmd *cobra.Command, args []string) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	fmt.Printf("Checking health of %s...\n", serverURL)

	health, err := client.GetHealth(ctx)
	if err != nil {
		return fmt.Errorf("failed to check health: %w", err)
	}

	if health.Healthy {
		fmt.Printf("✅ Server is healthy!\n")
	} else {
		fmt.Printf("❌ Server is not healthy!\n")
	}
	fmt.Printf("EventLog: %t\n", health.EventLogHealthy)
	fmt.Printf("RoutingTable: %t\n", health.RoutingTableHealthy)
	fmt.Printf("PeerLink: %t\n", health.PeerLinkHealthy)
	fmt.Printf("Connected Clients: %d\n", health.ConnectedClients)
	fmt.Printf("Connected Peers: %d\n", health.ConnectedPeers)
	if health.Message != "" {
		fmt.Printf("Message: %s\n", health.Message)
	}

	return nil
}