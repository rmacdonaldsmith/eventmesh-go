package routingtable

import (
	"context"
	"io"
)

// Subscriber represents a local client or peer node that subscribes to topics
type Subscriber interface {
	// ID returns unique identifier for this subscriber
	ID() string

	// Type returns the type of subscriber (local client, peer node, etc.)
	Type() SubscriberType
}

// SubscriberType represents different types of subscribers
type SubscriberType int

const (
	// LocalClient represents a local client connection
	LocalClient SubscriberType = iota

	// PeerNode represents a remote peer node in the mesh
	PeerNode
)

// Subscription represents a topic subscription with optional wildcard patterns
type Subscription struct {
	// Topic is the topic pattern to subscribe to (may include wildcards)
	Topic string

	// Subscriber is the entity that wants to receive events for this topic
	Subscriber Subscriber
}

// RoutingTable manages topic-to-subscriber mappings for event routing.
// This is equivalent to the "Routing Table" component in the design document.
//
// Requirements implemented:
// - REQ-RT-001: Wildcard Matching - supports wildcard topic patterns
// - REQ-RT-002: Gossip-based Rebuild - can be rebuilt from gossip data
// - REQ-RT-003: Efficient Lookup - optimized for fast topic matching
type RoutingTable interface {
	io.Closer

	// Subscribe adds a subscription for a topic pattern to a subscriber.
	// Supports wildcard patterns for flexible topic matching.
	Subscribe(ctx context.Context, topic string, subscriber Subscriber) error

	// Unsubscribe removes a subscription for a topic pattern from a subscriber.
	Unsubscribe(ctx context.Context, topic string, subscriberID string) error

	// GetSubscribers returns all subscribers interested in the given topic.
	// Performs wildcard matching against all subscription patterns.
	GetSubscribers(ctx context.Context, topic string) ([]Subscriber, error)

	// GetAllSubscriptions returns all current subscriptions.
	// Used for gossip protocol to share subscription state with peers.
	GetAllSubscriptions(ctx context.Context) ([]Subscription, error)

	// RebuildFromGossip rebuilds the routing table from gossip data.
	// Supports REQ-RT-002: gossip-based rebuild after node restart.
	RebuildFromGossip(ctx context.Context, subscriptions []Subscription) error

	// GetTopicCount returns the number of unique topics being tracked.
	// Useful for monitoring and metrics.
	GetTopicCount(ctx context.Context) (int, error)

	// GetSubscriberCount returns the total number of active subscribers.
	// Useful for monitoring and metrics.
	GetSubscriberCount(ctx context.Context) (int, error)
}