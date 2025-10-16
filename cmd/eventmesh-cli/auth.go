package main

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
)

func newAuthCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "auth",
		Short: "Authenticate with EventMesh server",
		Long: `Authenticate with the EventMesh server using your client ID.
This will generate a JWT token that can be used for subsequent requests.`,
		RunE: runAuth,
	}

	return cmd
}

func runAuth(cmd *cobra.Command, args []string) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	fmt.Printf("Authenticating with server %s as client %s...\n", serverURL, clientID)

	err := client.Authenticate(ctx)
	if err != nil {
		return fmt.Errorf("authentication failed: %w", err)
	}

	token := client.GetToken()
	fmt.Printf("✅ Authentication successful!\n")
	fmt.Printf("Token: %s\n", token)
	fmt.Printf("\nYou can now use other commands or save this token for future use:\n")
	fmt.Printf("  export EVENTMESH_TOKEN=\"%s\"\n", token)
	fmt.Printf("  eventmesh-cli --token \"$EVENTMESH_TOKEN\" publish --topic test.events --payload '{\"msg\":\"hello\"}'\n")

	return nil
}