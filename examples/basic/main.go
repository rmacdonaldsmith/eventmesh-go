package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/rmacdonaldsmith/eventmesh-go/internal/eventlog"
	eventlogpkg "github.com/rmacdonaldsmith/eventmesh-go/pkg/eventlog"
)

func main() {
	fmt.Println("EventMesh EventLog Basic Example")
	fmt.Println("================================")

	// Create a new in-memory event log
	eventLog := eventlog.NewInMemoryEventLog()
	defer eventLog.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Example 1: Append events to different topics
	fmt.Println("\n1. Appending events to topics:")
	events := []struct {
		topic   string
		payload string
	}{
		{"orders", "Order #1234 created"},
		{"payments", "Payment processed for order #1234"},
		{"inventory", "Stock updated for product ABC123"},
		{"orders", "Order #1234 completed"},
	}

	var appendedRecords []eventlogpkg.EventRecord
	for _, event := range events {
		record := eventlogpkg.NewRecord(event.topic, []byte(event.payload))
		appended, err := eventLog.AppendToTopic(ctx, event.topic, record)
		if err != nil {
			log.Fatalf("Failed to append event: %v", err)
		}
		appendedRecords = append(appendedRecords, appended)
		fmt.Printf("  Topic %s, Offset %d: %s\n", appended.Topic(), appended.Offset(), string(appended.Payload()))
	}

	// Example 2: Read events from a specific topic
	fmt.Println("\n2. Reading events from 'orders' topic starting at offset 0:")
	readEvents, err := eventLog.ReadFromTopic(ctx, "orders", 0, 10)
	if err != nil {
		log.Fatalf("Failed to read events: %v", err)
	}

	for _, event := range readEvents {
		fmt.Printf("  Topic %s, Offset %d: %s\n", event.Topic(), event.Offset(), string(event.Payload()))
	}

	// Example 3: Get end offset for specific topics
	fmt.Println("\n3. Topic end offsets:")
	for _, topic := range []string{"orders", "payments", "inventory"} {
		endOffset, err := eventLog.GetTopicEndOffset(ctx, topic)
		if err != nil {
			log.Fatalf("Failed to get end offset for topic %s: %v", topic, err)
		}
		fmt.Printf("  Topic '%s' end offset: %d\n", topic, endOffset)
	}

	// Example 4: Replay events from a specific topic
	fmt.Println("\n4. Replaying 'orders' topic from offset 0:")
	eventChan, errChan := eventLog.ReplayTopic(ctx, "orders", 0)

	for {
		select {
		case event, ok := <-eventChan:
			if !ok {
				fmt.Println("  Replay completed!")
				goto done
			}
			fmt.Printf("  Replayed Topic %s, Offset %d: %s\n", event.Topic(), event.Offset(), string(event.Payload()))
		case err := <-errChan:
			if err != nil {
				log.Fatalf("Replay error: %v", err)
			}
		case <-ctx.Done():
			log.Fatalf("Context timeout: %v", ctx.Err())
		}
	}

done:
	// Example 5: Demonstrate headers
	fmt.Println("\n5. Event with headers:")
	headers := map[string]string{
		"user-id":      "user123",
		"request-id":   "req-456",
		"content-type": "application/json",
	}
	recordWithHeaders := eventlogpkg.NewRecordWithHeaders("user-events", []byte(`{"action": "login", "timestamp": "2023-01-01T12:00:00Z"}`), headers)
	appended, err := eventLog.AppendToTopic(ctx, "user-events", recordWithHeaders)
	if err != nil {
		log.Fatalf("Failed to append event with headers: %v", err)
	}

	fmt.Printf("  Topic %s, Offset %d: %s\n", appended.Topic(), appended.Offset(), string(appended.Payload()))
	fmt.Println("  Headers:")
	for key, value := range appended.Headers() {
		fmt.Printf("    %s: %s\n", key, value)
	}

	fmt.Println("\nExample completed successfully!")
}