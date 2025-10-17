package main

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
)

func newTopicsCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "topics",
		Short: "Manage and inspect topics",
		Long:  `Commands for managing topics and inspecting topic metadata.`,
	}

	// Add subcommands
	cmd.AddCommand(newTopicsInfoCommand())

	return cmd
}

func newTopicsInfoCommand() *cobra.Command {
	var topic string

	cmd := &cobra.Command{
		Use:   "info",
		Short: "Show topic metadata and statistics",
		Long: `Display information about a topic including current end offset,
event count, and other metadata. This helps determine appropriate
offset values for replay and streaming operations.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runTopicsInfo(topic)
		},
	}

	cmd.Flags().StringVar(&topic, "topic", "", "Topic to inspect (required)")

	// Mark topic as required
	if err := cmd.MarkFlagRequired("topic"); err != nil {
		panic(fmt.Sprintf("Failed to mark topic flag as required: %v", err))
	}

	return cmd
}

func runTopicsInfo(topic string) error {
	if err := requireAuthentication(); err != nil {
		return err
	}

	ctx := context.Background()

	fmt.Printf("🔍 Inspecting topic '%s'...\n\n", topic)

	// Read a reasonable batch to get topic info (simplified approach)
	response, err := client.ReadEvents(ctx, topic, 0, 100)
	if err != nil {
		return fmt.Errorf("failed to inspect topic: %w", err)
	}

	if len(response.Events) == 0 {
		fmt.Printf("📭 Topic '%s' is empty or does not exist\n", topic)
		fmt.Printf("   Start offset: 0\n")
		fmt.Printf("   Event count: 0\n")
		return nil
	}

	// Get first and last events from our sample
	firstEvent := response.Events[0]
	lastEvent := response.Events[len(response.Events)-1]

	fmt.Printf("📊 Topic Information:\n")
	fmt.Printf("   Topic: %s\n", topic)
	fmt.Printf("   First available offset: %d\n", firstEvent.Offset)
	fmt.Printf("   First event timestamp: %s\n", firstEvent.Timestamp.Format("2006-01-02 15:04:05.000"))
	fmt.Printf("   Sample shows %d events (offset %d to %d)\n", len(response.Events), firstEvent.Offset, lastEvent.Offset)
	fmt.Printf("   Latest sampled timestamp: %s\n", lastEvent.Timestamp.Format("2006-01-02 15:04:05.000"))

	if len(response.Events) == 100 {
		fmt.Printf("   (Topic may contain more events - showing first 100)\n")
	}

	fmt.Println()
	fmt.Printf("💡 Usage Examples:\n")
	fmt.Printf("   Replay from beginning:     eventmesh-cli replay --topic %s --offset 0\n", topic)
	fmt.Printf("   Replay recent events:      eventmesh-cli replay --topic %s --offset %d --limit 10\n",
		topic, maxInt64(0, lastEvent.Offset-9))
	fmt.Printf("   Stream from beginning:     eventmesh-cli stream --topic %s --offset 0\n", topic)
	fmt.Printf("   Stream real-time only:     eventmesh-cli stream --topic %s\n", topic)

	return nil
}

func maxInt64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}
