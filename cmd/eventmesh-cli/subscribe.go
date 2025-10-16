package main

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
)

func newSubscribeCommand() *cobra.Command {
	var topic string

	cmd := &cobra.Command{
		Use:   "subscribe",
		Short: "Create an explicit subscription to a topic",
		Long: `Create an explicit subscription to a topic. This registers your interest
in the topic for management purposes. To actually receive events, use the 'stream' command.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSubscribe(topic)
		},
	}

	cmd.Flags().StringVar(&topic, "topic", "", "Topic pattern to subscribe to (required)")
	if err := cmd.MarkFlagRequired("topic"); err != nil {
		panic(fmt.Sprintf("Failed to mark topic as required: %v", err))
	}

	return cmd
}

func runSubscribe(topic string) error {
	if err := requireAuthentication(); err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	fmt.Printf("Creating subscription to topic '%s'...\n", topic)

	response, err := client.CreateSubscription(ctx, topic)
	if err != nil {
		return fmt.Errorf("failed to create subscription: %w", err)
	}

	fmt.Printf("✅ Subscription created successfully!\n")
	fmt.Printf("Subscription ID: %s\n", response.ID)
	fmt.Printf("Topic: %s\n", response.Topic)
	fmt.Printf("Client ID: %s\n", response.ClientID)
	fmt.Printf("Created At: %s\n", response.CreatedAt.Format("2006-01-02 15:04:05"))

	return nil
}