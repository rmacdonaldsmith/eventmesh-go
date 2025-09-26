// Package eventlog provides interfaces for append-only event storage.
//
// This package defines the core abstractions for the EventMesh event log component:
//   - EventRecord: Interface for individual events with offset, topic, payload, timestamp, and headers
//   - EventLog: Interface for append-only log operations (append, read, replay, compact)
//
// The interfaces use Go idioms:
//   - context.Context instead of CancellationToken for cancellation and timeouts
//   - Channels for async streaming (Replay method)
//   - io.Closer instead of IDisposable for resource cleanup
//   - Explicit error returns following Go conventions
//
// Example usage:
//
//	// Append an event
//	record, err := log.Append(ctx, event)
//	if err != nil {
//		return err
//	}
//
//	// Read events from offset 0, max 100 events
//	events, err := log.ReadFrom(ctx, 0, 100)
//	if err != nil {
//		return err
//	}
//
//	// Replay events starting from offset 10
//	eventChan, errChan := log.Replay(ctx, 10)
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
// This package is part of the EventMesh system for secure, distributed event routing.
// See the design.md file for complete architecture and requirements.
package eventlog