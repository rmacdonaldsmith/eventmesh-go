// Package peerlink provides interfaces for peer-to-peer mesh communication.
//
// This package defines the core abstractions for EventMesh peer link component:
//   - PeerNode: Interface representing a remote mesh node
//   - PeerLink: Interface for managing connections and streaming between mesh nodes
//
// The peer link is the transport boundary between mesh nodes. The current
// implementation uses gRPC bidirectional streams for events, subscription
// changes, and heartbeats. mTLS and stronger peer identity are roadmap work,
// not current implementation guarantees.
//
// Key features:
//   - gRPC over HTTP/2 for bidirectional peer streaming
//   - Bi-directional event streaming between mesh nodes
//   - Automatic connection lifecycle management
//   - Built-in backpressure and flow control
//   - Health monitoring with heartbeats and failure detection
//   - Connection pooling and reconnection logic
//
// The interfaces use Go idioms:
//   - context.Context for cancellation and timeouts
//   - Channels for streaming event data
//   - Explicit error returns following Go conventions
//   - io.Closer for resource cleanup
//
// Example usage:
//
//	// Create and connect to a peer
//	peer := &RemotePeer{id: "node-2", address: "node-2.cluster.local:9090"}
//	err := peerLink.Connect(ctx, peer)
//	if err != nil {
//		return err
//	}
//
//	// Send an event to the peer
//	event := &eventlog.Event{Topic: "orders.created", Payload: eventData}
//	err = peerLink.SendEvent(ctx, "node-2", event)
//	if err != nil {
//		return err
//	}
//
//	// Receive events from all connected peers
//	eventChan, errChan := peerLink.ReceiveEvents(ctx)
//	for {
//		select {
//		case event := <-eventChan:
//			processEvent(event)
//		case err := <-errChan:
//			handleError(err)
//		case <-ctx.Done():
//			return ctx.Err()
//		}
//	}
//
//	// Monitor peer health
//	err = peerLink.StartHeartbeats(ctx)
//	if err != nil {
//		return err
//	}
//
// This package is part of the EventMesh system for distributed event routing.
// See docs/design.md for current architecture and roadmap notes.
package peerlink
