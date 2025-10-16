package main

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
)

func newAdminCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "admin",
		Short: "Admin commands (requires admin privileges)",
		Long:  "Administrative commands for monitoring the EventMesh system",
	}

	cmd.AddCommand(newAdminClientsCommand())
	cmd.AddCommand(newAdminSubscriptionsCommand())
	cmd.AddCommand(newAdminStatsCommand())

	return cmd
}

func newAdminClientsCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "clients",
		Short: "List all connected clients",
		Long:  "List all clients currently connected to the EventMesh server",
		RunE:  runAdminClients,
	}

	return cmd
}

func newAdminSubscriptionsCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "subscriptions",
		Short: "List all subscriptions across all clients",
		Long:  "List all subscriptions in the system (admin view)",
		RunE:  runAdminSubscriptions,
	}

	return cmd
}

func newAdminStatsCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "stats",
		Short: "Show system statistics",
		Long:  "Display EventMesh system statistics and metrics",
		RunE:  runAdminStats,
	}

	return cmd
}

func runAdminClients(cmd *cobra.Command, args []string) error {
	if err := requireAuthentication(); err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	fmt.Println("Fetching connected clients...")

	response, err := client.AdminListClients(ctx)
	if err != nil {
		return fmt.Errorf("failed to list clients: %w", err)
	}

	if len(response.Clients) == 0 {
		fmt.Println("No clients currently connected")
		return nil
	}

	fmt.Printf("\nFound %d connected client(s):\n\n", len(response.Clients))
	for i, clientInfo := range response.Clients {
		fmt.Printf("%d. Client ID: %s\n", i+1, clientInfo.ID)
		fmt.Printf("   Authenticated: %t\n", clientInfo.Authenticated)
		fmt.Printf("   Connected At: %s\n", clientInfo.ConnectedAt.Format("2006-01-02 15:04:05"))
		fmt.Printf("   Subscriptions: %d topics\n", len(clientInfo.Subscriptions))
		if len(clientInfo.Subscriptions) > 0 {
			fmt.Printf("   Topics: %v\n", clientInfo.Subscriptions)
		}
		if i < len(response.Clients)-1 {
			fmt.Println()
		}
	}

	return nil
}

func runAdminSubscriptions(cmd *cobra.Command, args []string) error {
	if err := requireAuthentication(); err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	fmt.Println("Fetching all subscriptions...")

	response, err := client.AdminListSubscriptions(ctx)
	if err != nil {
		return fmt.Errorf("failed to list subscriptions: %w", err)
	}

	if len(response.Subscriptions) == 0 {
		fmt.Println("No subscriptions found in the system")
		return nil
	}

	fmt.Printf("\nFound %d subscription(s) across all clients:\n\n", len(response.Subscriptions))
	for i, sub := range response.Subscriptions {
		fmt.Printf("%d. Subscription ID: %s\n", i+1, sub.ID)
		fmt.Printf("   Topic: %s\n", sub.Topic)
		fmt.Printf("   Client ID: %s\n", sub.ClientID)
		fmt.Printf("   Created At: %s\n", sub.CreatedAt.Format("2006-01-02 15:04:05"))
		if i < len(response.Subscriptions)-1 {
			fmt.Println()
		}
	}

	return nil
}

func runAdminStats(cmd *cobra.Command, args []string) error {
	if err := requireAuthentication(); err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	fmt.Println("Fetching system statistics...")

	response, err := client.AdminGetStats(ctx)
	if err != nil {
		return fmt.Errorf("failed to get stats: %w", err)
	}

	fmt.Printf("\n📊 EventMesh System Statistics:\n\n")
	fmt.Printf("Connected Clients: %d\n", response.ConnectedClients)
	fmt.Printf("Total Subscriptions: %d\n", response.TotalSubscriptions)
	fmt.Printf("Total Topics: %d\n", response.TotalTopics)
	fmt.Printf("Events Published: %d\n", response.EventsPublished)

	return nil
}