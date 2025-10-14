// Package routingtable provides interfaces for topic-to-subscriber routing.
//
// This package defines the core abstractions for the EventMesh routing table component:
//   - Subscriber: Interface for entities that can receive events (clients, peer nodes)
//   - Subscription: Represents a topic subscription with wildcard support
//   - RoutingTable: Interface for managing topic-to-subscriber mappings
//
// The routing table implements the following requirements from design.md:
//   - REQ-RT-001: Wildcard Matching - supports wildcard topic patterns
//   - REQ-RT-002: Gossip-based Rebuild - can be rebuilt from gossip data
//   - REQ-RT-003: Efficient Lookup - optimized for fast topic matching
//
// The interfaces use Go idioms:
//   - context.Context for cancellation and timeouts
//   - Explicit error returns following Go conventions
//   - io.Closer for resource cleanup
//   - Slice returns for multiple results
//
// Example usage:
//
//	// Subscribe a local client to a topic
//	subscriber := &LocalSubscriber{id: "client-123"}
//	err := routingTable.Subscribe(ctx, "orders.*", subscriber)
//	if err != nil {
//		return err
//	}
//
//	// Find all subscribers for a specific topic
//	subscribers, err := routingTable.GetSubscribers(ctx, "orders.created")
//	if err != nil {
//		return err
//	}
//
//	// Process each subscriber that matches the topic
//	for _, sub := range subscribers {
//		deliverEvent(sub, event)
//	}
//
//	// Rebuild from gossip data after node restart
//	gossipSubscriptions := []Subscription{...}
//	err = routingTable.RebuildFromGossip(ctx, gossipSubscriptions)
//	if err != nil {
//		return err
//	}
//
// Wildcard Patterns:
//   - "*" matches any single topic segment
//   - "orders.*" matches "orders.created", "orders.updated", etc.
//   - "*.urgent" matches "orders.urgent", "payments.urgent", etc.
//   - Future: "orders.**" for multi-level wildcards
//
// This package is part of the EventMesh system for secure, distributed event routing.
// See the design.md file for complete architecture and requirements.
package routingtable
