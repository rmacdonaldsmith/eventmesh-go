# Examples

This directory contains example code demonstrating how to use the EventMesh EventLog.

## Basic Example

The `basic/` directory contains a comprehensive example showing:

- Creating an in-memory event log
- Appending events with topics and payloads
- Reading events from specific offsets
- Getting the current end offset
- Replaying events using Go channels
- Working with event headers

To run the basic example:

```bash
cd examples/basic
go run main.go
```

This will demonstrate all the core EventLog functionality with sample data.

## Usage Patterns

The examples demonstrate the typical usage patterns:

```go
// Create event log
eventLog := eventlog.NewInMemoryEventLog()
defer eventLog.Close()

// Append events
record := eventlog.NewRecord("topic", []byte("payload"))
appended, err := eventLog.Append(ctx, record)

// Read events
events, err := eventLog.ReadFrom(ctx, startOffset, maxCount)

// Stream events
eventChan, errChan := eventLog.Replay(ctx, startOffset)
for event := range eventChan {
    // Process event
}
```

See the Go documentation in `pkg/eventlog/doc.go` for complete API details.