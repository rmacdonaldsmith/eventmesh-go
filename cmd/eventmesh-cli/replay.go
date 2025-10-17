package main

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
)

func newReplayCommand() *cobra.Command {
	var (
		topic        string
		offset       int64
		limit        int
		prettyFormat bool
	)

	cmd := &cobra.Command{
		Use:   "replay",
		Short: "Replay events from a topic starting at a specific offset",
		Long: `Replay events from a topic starting at a specific offset.
This command fetches historical events from the EventLog and displays them.
Unlike 'stream', this command fetches a batch of events and exits.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runReplay(topic, offset, limit, prettyFormat)
		},
	}

	cmd.Flags().StringVar(&topic, "topic", "", "Topic to replay events from (required)")
	cmd.Flags().Int64Var(&offset, "offset", 0, "Starting offset for event replay (default: 0)")
	cmd.Flags().IntVar(&limit, "limit", 100, "Maximum number of events to retrieve (default: 100, max: 1000)")
	cmd.Flags().BoolVar(&prettyFormat, "pretty", false, "Pretty print JSON payloads")

	// Mark topic as required
	if err := cmd.MarkFlagRequired("topic"); err != nil {
		panic(fmt.Sprintf("Failed to mark topic flag as required: %v", err))
	}

	return cmd
}

func runReplay(topic string, offset int64, limit int, prettyFormat bool) error {
	if err := requireAuthentication(); err != nil {
		return err
	}

	ctx := context.Background()

	fmt.Printf("🔄 Replaying events from topic '%s' (offset: %d, limit: %d)...\n", topic, offset, limit)

	// Read events from the topic
	response, err := client.ReadEvents(ctx, topic, offset, limit)
	if err != nil {
		return fmt.Errorf("failed to read events: %w", err)
	}

	// Display results summary
	fmt.Printf("📋 Found %d events (requested offset: %d, actual start: %d)\n\n",
		response.Count, offset, response.StartOffset)

	if len(response.Events) == 0 {
		fmt.Printf("🔍 No events found for topic '%s' starting from offset %d\n", topic, offset)
		return nil
	}

	// Display each event
	for i, event := range response.Events {
		printEvent(event, i+1, prettyFormat)
	}

	fmt.Printf("✅ Replay completed: %d events from topic '%s'\n", len(response.Events), topic)
	return nil
}
