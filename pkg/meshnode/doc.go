// Package meshnode provides interfaces for the main mesh node orchestrator.
//
// This package defines the core abstractions for the EventMesh node component:
//   - Client: Interface representing connected publishers/subscribers
//   - MeshNode: Main orchestrator interface that coordinates all mesh components
//   - HealthStatus: Health monitoring and status reporting
//
// The mesh node owns client identity, local persistence, routing decisions, and
// peer forwarding. The current implementation is used by the HTTP API and CLI.
//
// The MeshNode is the main entry point that orchestrates:
//   - EventLog: For local event persistence and replay
//   - RoutingTable: For topic-to-subscriber mapping and routing decisions
//   - PeerLink: For gRPC communication with other mesh nodes
//   - Client connections: For handling publisher/subscriber clients
//
// Architecture:
//  1. Clients connect to MeshNode and authenticate
//  2. Clients publish events or subscribe to topics
//  3. MeshNode persists events locally first (REQ-MNODE-002)
//  4. MeshNode uses RoutingTable to find interested subscribers
//  5. MeshNode uses PeerLink to forward events to remote subscribers
//  6. MeshNode propagates subscription changes via gossip (REQ-MNODE-003)
//
// The interfaces use Go idioms:
//   - context.Context for cancellation and timeouts
//   - Explicit error returns following Go conventions
//   - io.Closer for resource cleanup
//   - Health monitoring for observability
//
// Example usage:
//
//	// Start a mesh node supplied by the concrete implementation.
//	var node MeshNode = configuredNode
//	err := node.Start(ctx)
//	if err != nil {
//		return err
//	}
//	defer node.Close()
//
//	// Authenticate a connecting client
//	client, err := node.AuthenticateClient(ctx, credentials)
//	if err != nil {
//		return err
//	}
//
//	// Handle client subscription
//	err = node.Subscribe(ctx, client, "orders.*")
//	if err != nil {
//		return err
//	}
//
//	// Handle client publishing
//	event := &eventlog.Event{Payload: eventData}
//	err = node.PublishEvent(ctx, client, event)
//	if err != nil {
//		return err
//	}
//
//	// Monitor node health
//	health, err := node.GetHealth(ctx)
//	if err != nil {
//		return err
//	}
//	if !health.Healthy {
//		log.Printf("Node unhealthy: %s", health.Message)
//	}
//
// This package is part of EventMesh's orchestration boundary.
// See docs/design.md for current architecture and roadmap notes.
package meshnode
