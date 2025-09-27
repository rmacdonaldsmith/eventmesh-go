package routingtable

import (
	"context"
	"errors"
	"sync"

	"github.com/rmacdonaldsmith/eventmesh-go/pkg/routingtable"
)

// InMemoryRoutingTable provides a simple thread-safe in-memory implementation
// of the RoutingTable interface for MVP functionality.
type InMemoryRoutingTable struct {
	mu           sync.RWMutex
	subscriptions map[string][]routingtable.Subscriber
	closed       bool
}

// NewInMemoryRoutingTable creates a new in-memory routing table
func NewInMemoryRoutingTable() *InMemoryRoutingTable {
	return &InMemoryRoutingTable{
		subscriptions: make(map[string][]routingtable.Subscriber),
	}
}

// Subscribe adds a subscription for a topic to a subscriber
func (rt *InMemoryRoutingTable) Subscribe(ctx context.Context, topic string, subscriber routingtable.Subscriber) error {
	if subscriber == nil {
		return errors.New("subscriber cannot be nil")
	}
	if topic == "" {
		return errors.New("topic cannot be empty")
	}

	rt.mu.Lock()
	defer rt.mu.Unlock()

	if rt.closed {
		return errors.New("routing table is closed")
	}

	// Check if subscriber already exists for this topic to avoid duplicates
	subscribers := rt.subscriptions[topic]
	for _, existing := range subscribers {
		if existing.ID() == subscriber.ID() {
			return nil // Already subscribed
		}
	}

	rt.subscriptions[topic] = append(subscribers, subscriber)
	return nil
}

// Unsubscribe removes a subscription for a topic from a subscriber
func (rt *InMemoryRoutingTable) Unsubscribe(ctx context.Context, topic string, subscriberID string) error {
	rt.mu.Lock()
	defer rt.mu.Unlock()

	if rt.closed {
		return errors.New("routing table is closed")
	}

	subscribers := rt.subscriptions[topic]
	for i, subscriber := range subscribers {
		if subscriber.ID() == subscriberID {
			// Remove subscriber by swapping with last element and truncating
			subscribers[i] = subscribers[len(subscribers)-1]
			rt.subscriptions[topic] = subscribers[:len(subscribers)-1]

			// Remove topic entry if no subscribers left
			if len(rt.subscriptions[topic]) == 0 {
				delete(rt.subscriptions, topic)
			}
			return nil
		}
	}

	return nil // Subscriber not found, but not an error
}

// GetSubscribers returns all subscribers interested in the given topic
// For MVP, this only does exact topic matching (no wildcards yet)
func (rt *InMemoryRoutingTable) GetSubscribers(ctx context.Context, topic string) ([]routingtable.Subscriber, error) {
	rt.mu.RLock()
	defer rt.mu.RUnlock()

	if rt.closed {
		return nil, errors.New("routing table is closed")
	}

	subscribers := rt.subscriptions[topic]
	if len(subscribers) == 0 {
		return []routingtable.Subscriber{}, nil
	}

	// Return a copy to avoid external modification
	result := make([]routingtable.Subscriber, len(subscribers))
	copy(result, subscribers)
	return result, nil
}

// GetAllSubscriptions returns all current subscriptions
func (rt *InMemoryRoutingTable) GetAllSubscriptions(ctx context.Context) ([]routingtable.Subscription, error) {
	rt.mu.RLock()
	defer rt.mu.RUnlock()

	if rt.closed {
		return nil, errors.New("routing table is closed")
	}

	var subscriptions []routingtable.Subscription
	for topic, subscribers := range rt.subscriptions {
		for _, subscriber := range subscribers {
			subscriptions = append(subscriptions, routingtable.Subscription{
				Topic:      topic,
				Subscriber: subscriber,
			})
		}
	}

	return subscriptions, nil
}

// RebuildFromGossip rebuilds the routing table from gossip data
func (rt *InMemoryRoutingTable) RebuildFromGossip(ctx context.Context, subscriptions []routingtable.Subscription) error {
	rt.mu.Lock()
	defer rt.mu.Unlock()

	if rt.closed {
		return errors.New("routing table is closed")
	}

	// Clear existing subscriptions
	rt.subscriptions = make(map[string][]routingtable.Subscriber)

	// Add all subscriptions from gossip data
	for _, subscription := range subscriptions {
		topic := subscription.Topic
		subscriber := subscription.Subscriber

		// Check for duplicates
		found := false
		for _, existing := range rt.subscriptions[topic] {
			if existing.ID() == subscriber.ID() {
				found = true
				break
			}
		}

		if !found {
			rt.subscriptions[topic] = append(rt.subscriptions[topic], subscriber)
		}
	}

	return nil
}

// GetTopicCount returns the number of unique topics being tracked
func (rt *InMemoryRoutingTable) GetTopicCount(ctx context.Context) (int, error) {
	rt.mu.RLock()
	defer rt.mu.RUnlock()

	if rt.closed {
		return 0, errors.New("routing table is closed")
	}

	return len(rt.subscriptions), nil
}

// GetSubscriberCount returns the total number of active subscribers
func (rt *InMemoryRoutingTable) GetSubscriberCount(ctx context.Context) (int, error) {
	rt.mu.RLock()
	defer rt.mu.RUnlock()

	if rt.closed {
		return 0, errors.New("routing table is closed")
	}

	count := 0
	for _, subscribers := range rt.subscriptions {
		count += len(subscribers)
	}

	return count, nil
}

// Close closes the routing table and releases resources
func (rt *InMemoryRoutingTable) Close() error {
	rt.mu.Lock()
	defer rt.mu.Unlock()

	rt.closed = true
	rt.subscriptions = nil
	return nil
}