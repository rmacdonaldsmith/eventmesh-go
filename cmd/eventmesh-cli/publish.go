package main

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
)

func newPublishCommand() *cobra.Command {
	var (
		topic   string
		payload string
	)

	cmd := &cobra.Command{
		Use:   "publish",
		Short: "Publish an event to a topic",
		Long: `Publish an event to a topic. The payload should be valid JSON.
If no payload is provided, an empty event will be published.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runPublish(topic, payload)
		},
	}

	cmd.Flags().StringVar(&topic, "topic", "", "Topic to publish to (required)")
	cmd.Flags().StringVar(&payload, "payload", "{}", "Event payload as JSON")
	if err := cmd.MarkFlagRequired("topic"); err != nil {
		panic(fmt.Sprintf("Failed to mark topic as required: %v", err))
	}

	return cmd
}

func runPublish(topic, payloadStr string) error {
	if err := requireAuthentication(); err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	// Parse payload JSON
	var payload interface{}
	if payloadStr != "" {
		err := json.Unmarshal([]byte(payloadStr), &payload)
		if err != nil {
			return fmt.Errorf("invalid JSON payload: %w", err)
		}
	}

	fmt.Printf("Publishing event to topic '%s'...\n", topic)

	response, err := client.PublishEvent(ctx, topic, payload)
	if err != nil {
		return fmt.Errorf("failed to publish event: %w", err)
	}

	fmt.Printf("✅ Event published successfully!\n")
	fmt.Printf("Event ID: %s\n", response.EventID)
	fmt.Printf("Offset: %d\n", response.Offset)
	fmt.Printf("Timestamp: %s\n", response.Timestamp.Format("2006-01-02 15:04:05"))

	return nil
}