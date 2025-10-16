package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/rmacdonaldsmith/eventmesh-go/pkg/httpclient"
	"github.com/spf13/cobra"
)

func newStreamCommand() *cobra.Command {
	var (
		topic        string
		bufferSize   int
		prettyFormat bool
	)

	cmd := &cobra.Command{
		Use:   "stream",
		Short: "Stream events from a topic in real-time",
		Long: `Stream events from a topic in real-time using Server-Sent Events.
This automatically subscribes to the topic and streams events as they are published.
Press Ctrl+C to stop streaming.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runStream(topic, bufferSize, prettyFormat)
		},
	}

	cmd.Flags().StringVar(&topic, "topic", "", "Topic to stream from (optional - streams all events if not specified)")
	cmd.Flags().IntVar(&bufferSize, "buffer-size", 100, "Event buffer size")
	cmd.Flags().BoolVar(&prettyFormat, "pretty", false, "Pretty print JSON payloads")

	return cmd
}

func runStream(topic string, bufferSize int, prettyFormat bool) error {
	if err := requireAuthentication(); err != nil {
		return err
	}

	// Create context that can be cancelled
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle Ctrl+C gracefully
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		fmt.Println("\n🛑 Stopping stream...")
		cancel()
	}()

	// Configure streaming
	config := httpclient.StreamConfig{
		Topic:                topic,
		BufferSize:           bufferSize,
		MaxReconnectAttempts: 0, // Infinite retries
	}

	fmt.Printf("🌊 Starting event stream from %s", serverURL)
	if topic != "" {
		fmt.Printf(" (topic: %s)", topic)
	} else {
		fmt.Printf(" (all topics)")
	}
	fmt.Println("...")
	fmt.Println("Press Ctrl+C to stop streaming")

	// Start streaming
	streamClient, err := client.Stream(ctx, config)
	if err != nil {
		return fmt.Errorf("failed to start streaming: %w", err)
	}
	defer func() {
		if err := streamClient.Close(); err != nil {
			fmt.Printf("Warning: failed to close stream client: %v\n", err)
		}
	}()

	// Process events and errors
	eventCount := 0
	for {
		select {
		case <-ctx.Done():
			fmt.Printf("\n✅ Stream stopped. Received %d events.\n", eventCount)
			return nil

		case event, ok := <-streamClient.Events():
			if !ok {
				fmt.Printf("\n🔌 Event stream closed. Received %d events.\n", eventCount)
				return nil
			}

			eventCount++
			printEvent(event, eventCount, prettyFormat)

		case err, ok := <-streamClient.Errors():
			if !ok {
				fmt.Printf("\n🔌 Error stream closed. Received %d events.\n", eventCount)
				return nil
			}

			fmt.Printf("❌ Stream error: %v\n", err)
			// Continue processing - errors are non-fatal for reconnection scenarios

		case <-streamClient.Done():
			fmt.Printf("\n🔌 Stream finished. Received %d events.\n", eventCount)
			return nil
		}
	}
}

func printEvent(event httpclient.EventStreamMessage, count int, pretty bool) {
	fmt.Printf("📨 Event #%d:\n", count)
	fmt.Printf("   ID: %s\n", event.EventID)
	fmt.Printf("   Topic: %s\n", event.Topic)
	fmt.Printf("   Offset: %d\n", event.Offset)
	fmt.Printf("   Time: %s\n", event.Timestamp.Format("2006-01-02 15:04:05.000"))

	if event.Payload != nil {
		fmt.Printf("   Payload: ")
		if pretty {
			jsonBytes, err := json.MarshalIndent(event.Payload, "            ", "  ")
			if err != nil {
				fmt.Printf("%v\n", event.Payload)
			} else {
				fmt.Printf("\n            %s\n", string(jsonBytes))
			}
		} else {
			jsonBytes, err := json.Marshal(event.Payload)
			if err != nil {
				fmt.Printf("%v\n", event.Payload)
			} else {
				fmt.Printf("%s\n", string(jsonBytes))
			}
		}
	} else {
		fmt.Printf("   Payload: null\n")
	}
	fmt.Println()
}