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

	// Example 1: Append some events
	fmt.Println("\n1. Appending events:")
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
		record := eventlog.NewRecord(event.topic, []byte(event.payload))
		appended, err := eventLog.Append(ctx, record)
		if err != nil {
			log.Fatalf("Failed to append event: %v", err)
		}
		appendedRecords = append(appendedRecords, appended)
		fmt.Printf("  Offset %d: [%s] %s\n", appended.Offset(), appended.Topic(), string(appended.Payload()))
	}

	// Example 2: Read events from a specific offset
	fmt.Println("\n2. Reading events from offset 1:")
	readEvents, err := eventLog.ReadFrom(ctx, 1, 10)
	if err != nil {
		log.Fatalf("Failed to read events: %v", err)
	}

	for _, event := range readEvents {
		fmt.Printf("  Offset %d: [%s] %s\n", event.Offset(), event.Topic(), string(event.Payload()))
	}

	// Example 3: Get current end offset
	fmt.Println("\n3. Current end offset:")
	endOffset, err := eventLog.GetEndOffset(ctx)
	if err != nil {
		log.Fatalf("Failed to get end offset: %v", err)
	}
	fmt.Printf("  End offset: %d (next append position)\n", endOffset)

	// Example 4: Replay all events using channels
	fmt.Println("\n4. Replaying all events from offset 0:")
	eventChan, errChan := eventLog.Replay(ctx, 0)

	for {
		select {
		case event, ok := <-eventChan:
			if !ok {
				fmt.Println("  Replay completed!")
				goto done
			}
			fmt.Printf("  Replayed offset %d: [%s] %s\n", event.Offset(), event.Topic(), string(event.Payload()))
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
	recordWithHeaders := eventlog.NewRecordWithHeaders("user-events", []byte(`{"action": "login", "timestamp": "2023-01-01T12:00:00Z"}`), headers)
	appended, err := eventLog.Append(ctx, recordWithHeaders)
	if err != nil {
		log.Fatalf("Failed to append event with headers: %v", err)
	}

	fmt.Printf("  Offset %d: [%s] %s\n", appended.Offset(), appended.Topic(), string(appended.Payload()))
	fmt.Println("  Headers:")
	for key, value := range appended.Headers() {
		fmt.Printf("    %s: %s\n", key, value)
	}

	fmt.Println("\nExample completed successfully!")
}