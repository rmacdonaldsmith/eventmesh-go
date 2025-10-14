package routingtable

import (
	"context"
	"errors"
	"strings"
	"sync"

	"github.com/rmacdonaldsmith/eventmesh-go/pkg/routingtable"
)

// InMemoryRoutingTable provides a simple thread-safe in-memory implementation
// of the RoutingTable interface for MVP functionality.
type InMemoryRoutingTable struct {
	mu            sync.RWMutex
	subscriptions map[string]map[string]routingtable.Subscriber // topic -> subscriberID -> subscriber
	closed        bool
}

// NewInMemoryRoutingTable creates a new in-memory routing table
func NewInMemoryRoutingTable() *InMemoryRoutingTable {
	return &InMemoryRoutingTable{
		subscriptions: make(map[string]map[string]routingtable.Subscriber),
	}
}

// matchesPattern checks if a topic matches a subscription pattern
// Supports single-level wildcards: orders.*, *.created
func matchesPattern(pattern, topic string) bool {
	// Handle exact matches first (fast path)
	if pattern == topic {
		return true
	}

	// Only handle single-level wildcards
	patternParts := strings.Split(pattern, ".")
	topicParts := strings.Split(topic, ".")

	if len(patternParts) != len(topicParts) {
		return false
	}

	for i, part := range patternParts {
		if part != "*" && part != topicParts[i] {
			return false
		}
	}
	return true
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

	// Initialize topic map if needed
	if rt.subscriptions[topic] == nil {
		rt.subscriptions[topic] = make(map[string]routingtable.Subscriber)
	}

	// O(1) duplicate check and insertion
	rt.subscriptions[topic][subscriber.ID()] = subscriber
	return nil
}

// Unsubscribe removes a subscription for a topic from a subscriber
func (rt *InMemoryRoutingTable) Unsubscribe(ctx context.Context, topic string, subscriberID string) error {
	rt.mu.Lock()
	defer rt.mu.Unlock()

	if rt.closed {
		return errors.New("routing table is closed")
	}

	if subscribers := rt.subscriptions[topic]; subscribers != nil {
		// O(1) removal
		delete(subscribers, subscriberID)

		// Remove topic entry if no subscribers left
		if len(subscribers) == 0 {
			delete(rt.subscriptions, topic)
		}
	}

	return nil // Subscriber not found, but not an error
}

// GetSubscribers returns all subscribers interested in the given topic
// Supports both exact topic matching and wildcard pattern matching
func (rt *InMemoryRoutingTable) GetSubscribers(ctx context.Context, topic string) ([]routingtable.Subscriber, error) {
	rt.mu.RLock()
	defer rt.mu.RUnlock()

	if rt.closed {
		return nil, errors.New("routing table is closed")
	}

	var result []routingtable.Subscriber

	// 1. Add exact match subscribers
	if exactSubs := rt.subscriptions[topic]; len(exactSubs) > 0 {
		for _, subscriber := range exactSubs {
			result = append(result, subscriber)
		}
	}

	// 2. Check all subscription patterns for wildcard matches
	for pattern, subs := range rt.subscriptions {
		if pattern != topic && matchesPattern(pattern, topic) {
			for _, subscriber := range subs {
				result = append(result, subscriber)
			}
		}
	}

	if len(result) == 0 {
		return []routingtable.Subscriber{}, nil
	}

	// Return a copy to avoid external modification
	resultCopy := make([]routingtable.Subscriber, len(result))
	copy(resultCopy, result)
	return resultCopy, nil
}

// GetAllSubscriptions returns all current subscriptions
func (rt *InMemoryRoutingTable) GetAllSubscriptions(ctx context.Context) ([]routingtable.Subscription, error) {
	rt.mu.RLock()
	defer rt.mu.RUnlock()

	if rt.closed {
		return nil, errors.New("routing table is closed")
	}

	var subscriptions []routingtable.Subscription
	for topic, subscriberMap := range rt.subscriptions {
		for _, subscriber := range subscriberMap {
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
	rt.subscriptions = make(map[string]map[string]routingtable.Subscriber)

	// Add all subscriptions from gossip data
	for _, subscription := range subscriptions {
		topic := subscription.Topic
		subscriber := subscription.Subscriber

		// Initialize topic map if needed
		if rt.subscriptions[topic] == nil {
			rt.subscriptions[topic] = make(map[string]routingtable.Subscriber)
		}

		// O(1) insertion (duplicates are automatically handled by map)
		rt.subscriptions[topic][subscriber.ID()] = subscriber
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
	for _, subscriberMap := range rt.subscriptions {
		count += len(subscriberMap)
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
