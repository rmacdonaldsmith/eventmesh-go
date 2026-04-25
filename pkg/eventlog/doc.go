// Package eventlog provides interfaces for topic-scoped append-only event storage.
//
// This package defines the core abstractions for the EventMesh event log component:
//   - Event: A topic event with offset, payload, timestamp, and headers
//   - EventLog: Interface for appending, reading, replaying, compacting, and reporting statistics
//
// The current in-repository implementation is in-memory. Offsets are assigned
// independently per topic and start at 0. Durable storage is roadmap work.
//
// The interfaces use Go idioms:
//   - context.Context instead of CancellationToken for cancellation and timeouts
//   - Channels for async streaming (Replay method)
//   - io.Closer instead of IDisposable for resource cleanup
//   - Explicit error returns following Go conventions
//
// Example usage:
//
//	// Append an event to a topic
//	stored, err := log.AppendEvent(ctx, "orders.created", event)
//	if err != nil {
//		return err
//	}
//
//	// Read events from offset 0, max 100 events
//	events, err := log.ReadEvents(ctx, "orders.created", 0, 100)
//	if err != nil {
//		return err
//	}
//
//	// Replay events for one topic starting from offset 10
//	eventChan, errChan := log.ReplayEvents(ctx, "orders.created", 10)
//	for {
//		select {
//		case event, ok := <-eventChan:
//			if !ok {
//				return // Channel closed, all events processed
//			}
//			processEvent(event)
//		case err := <-errChan:
//			if err != nil {
//				return err
//			}
//		case <-ctx.Done():
//			return ctx.Err()
//		}
//	}
//
//	_ = stored
//	_ = events
//
// This package is part of EventMesh's local persistence and replay boundary.
// See docs/design.md for current architecture and roadmap notes.
package eventlog
