package main

import (
	"fmt"
	"os"
	"time"

	"github.com/rmacdonaldsmith/eventmesh-go/pkg/httpclient"
	"github.com/spf13/cobra"
)

var (
	// Global flags
	serverURL string
	clientID  string
	token     string
	timeout   time.Duration

	// Global client instance
	client *httpclient.Client
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "eventmesh-cli",
		Short: "EventMesh HTTP API command line interface",
		Long: `eventmesh-cli is a command line interface for the EventMesh HTTP API.
It provides commands for authentication, event publishing, subscription management,
and real-time event streaming.`,
		PersistentPreRunE: initializeClient,
	}

	// Add global flags
	rootCmd.PersistentFlags().StringVar(&serverURL, "server", "http://localhost:8081", "EventMesh server URL")
	rootCmd.PersistentFlags().StringVar(&clientID, "client-id", "", "Client ID for authentication")
	rootCmd.PersistentFlags().StringVar(&token, "token", "", "JWT token (if already authenticated)")
	rootCmd.PersistentFlags().DurationVar(&timeout, "timeout", 30*time.Second, "Request timeout")

	// Mark required flags
	rootCmd.MarkPersistentFlagRequired("client-id")

	// Add subcommands
	rootCmd.AddCommand(newAuthCommand())
	rootCmd.AddCommand(newPublishCommand())
	rootCmd.AddCommand(newSubscribeCommand())
	rootCmd.AddCommand(newSubscriptionsCommand())
	rootCmd.AddCommand(newStreamCommand())
	rootCmd.AddCommand(newAdminCommand())
	rootCmd.AddCommand(newHealthCommand())

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

// initializeClient sets up the HTTP client with global configuration
func initializeClient(cmd *cobra.Command, args []string) error {
	// Skip client initialization for help commands
	if cmd.Name() == "help" || cmd.Parent() == nil {
		return nil
	}

	config := httpclient.Config{
		ServerURL: serverURL,
		ClientID:  clientID,
		Timeout:   timeout,
	}

	var err error
	client, err = httpclient.NewClient(config)
	if err != nil {
		return fmt.Errorf("failed to create client: %w", err)
	}

	// Set token if provided
	if token != "" {
		client.SetToken(token)
	}

	return nil
}

// requireAuthentication checks if the client is authenticated
func requireAuthentication() error {
	if client == nil {
		return fmt.Errorf("client not initialized")
	}
	if !client.IsAuthenticated() {
		return fmt.Errorf("not authenticated - run 'eventmesh-cli auth' first or provide --token")
	}
	return nil
}