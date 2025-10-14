// Package peerlink provides interfaces for secure peer-to-peer communication.
//
// This package defines the core abstractions for EventMesh peer link component:
//   - PeerNode: Interface representing a remote mesh node
//   - PeerLink: Interface for managing connections and streaming between mesh nodes
//
// The peer link implements the following requirements from design.md:
//   - REQ-PL-001: Secure Connection Lifecycle - mTLS connections with proper lifecycle management
//   - REQ-PL-002: Backpressure and Flow Control - handles streaming backpressure
//   - REQ-PL-003: Heartbeats and Failure Detection - monitors peer health
//
// Key features:
//   - gRPC over HTTP/2 with mTLS for secure, efficient streaming
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
//	event := eventlog.NewRecord("orders.created", eventData)
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
// This package is part of the EventMesh system for secure, distributed event routing.
// See the design.md file for complete architecture and requirements.
package peerlink
