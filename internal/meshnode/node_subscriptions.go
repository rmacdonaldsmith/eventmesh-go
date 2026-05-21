package meshnode

import (
	"context"
	"fmt"
	"log/slog"
	"sort"
	"time"

	"github.com/rmacdonaldsmith/eventmesh-go/pkg/meshnode"
	peerlinkpkg "github.com/rmacdonaldsmith/eventmesh-go/pkg/peerlink"
	routingtablepkg "github.com/rmacdonaldsmith/eventmesh-go/pkg/routingtable"
)

// Subscribe registers a client's interest in a topic pattern.
// Implements REQ-MNODE-003: propagates subscription to peer nodes.
func (n *GRPCMeshNode) Subscribe(ctx context.Context, client meshnode.Client, topic string) error {
	n.mu.Lock()
	defer n.mu.Unlock()

	if n.closed {
		return fmt.Errorf("cannot subscribe to closed mesh node")
	}

	if !n.started {
		return fmt.Errorf("cannot subscribe to stopped mesh node")
	}

	// Validate inputs
	if client == nil {
		return fmt.Errorf("client cannot be nil")
	}
	if topic == "" {
		return fmt.Errorf("topic cannot be empty")
	}
	if !client.IsAuthenticated() {
		return fmt.Errorf("client is not authenticated")
	}

	// Cast client to subscriber (TrustedClient implements both interfaces)
	subscriber, ok := client.(routingtablepkg.Subscriber)
	if !ok {
		return fmt.Errorf("client does not implement Subscriber interface")
	}

	hadLocalInterest := n.localTopicInterestCount(topic) > 0

	// Add to local routing table
	err := n.routingTable.Subscribe(ctx, topic, subscriber)
	if err != nil {
		return fmt.Errorf("failed to add local subscription: %w", err)
	}

	// Add to local client tracking
	n.clients[client.ID()] = client

	// Store subscription metadata for HTTP API
	// Generate a simple subscription ID for this client/topic pair
	subscriptionID := fmt.Sprintf("sub-%s-%d", client.ID(), time.Now().UnixNano())
	n.addSubscriptionMetadata(client.ID(), subscriptionID, topic)

	slog.Info("subscription created",
		"client_id", client.ID(),
		"topic", topic,
		"subscription_id", subscriptionID,
		"node_id", n.config.NodeID)

	// Propagate aggregate interest only when this node's local interest changes
	// from zero subscribers to one or more subscribers.
	if !hadLocalInterest {
		err = n.propagateInterestUpdate(ctx, "subscribe", topic)
		if err != nil {
			slog.Warn("failed to propagate interest to peers",
				"topic", topic,
				"subscription_id", subscriptionID,
				"error", err)
		} else {
			slog.Debug("interest propagated to peers",
				"topic", topic,
				"subscription_id", subscriptionID)
		}
	}

	return nil
}

// Unsubscribe removes a client's subscription to a topic pattern.
// Updates local routing table and notifies peer nodes.
func (n *GRPCMeshNode) Unsubscribe(ctx context.Context, client meshnode.Client, topic string) error {
	n.mu.Lock()
	defer n.mu.Unlock()

	if n.closed {
		return fmt.Errorf("cannot unsubscribe from closed mesh node")
	}

	if !n.started {
		return fmt.Errorf("cannot unsubscribe from stopped mesh node")
	}

	// Validate inputs
	if client == nil {
		return fmt.Errorf("client cannot be nil")
	}
	if topic == "" {
		return fmt.Errorf("topic cannot be empty")
	}
	if !client.IsAuthenticated() {
		return fmt.Errorf("client is not authenticated")
	}

	// Remove from local routing table
	err := n.routingTable.Unsubscribe(ctx, topic, client.ID())
	if err != nil {
		return fmt.Errorf("failed to remove local subscription: %w", err)
	}

	slog.Info("subscription removed",
		"client_id", client.ID(),
		"topic", topic,
		"node_id", n.config.NodeID)

	n.removeSubscriptionMetadataByTopic(client.ID(), topic)

	// Propagate aggregate interest removal only when this node's local interest
	// drops to zero subscribers for the topic pattern.
	if n.localTopicInterestCount(topic) == 0 {
		err = n.propagateInterestUpdate(ctx, "unsubscribe", topic)
		if err != nil {
			slog.Warn("failed to propagate interest removal to peers",
				"topic", topic,
				"error", err)
		} else {
			slog.Debug("interest removal propagated to peers",
				"topic", topic)
		}
	}

	return nil
}

// propagateInterestUpdate sends aggregate topic-interest changes to connected
// peers over the PeerLink control plane.
func (n *GRPCMeshNode) propagateInterestUpdate(ctx context.Context, action, topic string) error {
	// Validate input parameters
	if action == "" || topic == "" {
		return fmt.Errorf("action and topic cannot be empty")
	}
	if action != "subscribe" && action != "unsubscribe" {
		return fmt.Errorf("action must be 'subscribe' or 'unsubscribe'")
	}

	// Get connected peers
	connectedPeers, err := n.peerConnections.GetConnectedPeers(ctx)
	if err != nil {
		return fmt.Errorf("failed to get connected peers: %w", err)
	}

	if len(connectedPeers) == 0 {
		// No peers to notify, not an error
		slog.Debug("interest update: no peers to notify",
			"action", action,
			"topic", topic,
			"node_id", n.config.NodeID)
		return nil
	}

	// Create aggregate interest update message.
	interestUpdate := &peerlinkpkg.InterestUpdate{
		Action: action,
		Topic:  topic,
		NodeId: n.config.NodeID,
	}

	// Track delivery results for observability
	var successCount, failureCount int
	var errors []error

	// Send to all connected peers using control plane
	for _, peer := range connectedPeers {
		err := n.controlPlanePeerLink.SendInterestUpdate(ctx, peer.ID(), interestUpdate)
		if err != nil {
			failureCount++
			errors = append(errors, fmt.Errorf("failed to send to peer %s: %w", peer.ID(), err))

			slog.Warn("failed to propagate interest update to peer",
				"peer_id", peer.ID(),
				"action", action,
				"topic", topic,
				"error", err)
		} else {
			successCount++
			n.interestUpdatesSent.Add(1)
		}
	}

	// Log propagation summary
	slog.Info("interest update propagation completed",
		"action", action,
		"topic", topic,
		"peers_notified", successCount,
		"peers_failed", failureCount,
		"total_peers", len(connectedPeers))

	// Return error if all peers failed, but log partial failures
	if failureCount > 0 && successCount == 0 {
		return fmt.Errorf("failed to propagate interest update to all %d peers: %v", failureCount, errors)
	}

	// Partial failures are logged but don't fail the operation
	// The subscription is still valid locally
	return nil
}

func (n *GRPCMeshNode) resyncSubscriptionsToNewPeers(ctx context.Context) {
	seenPeers := make(map[string]bool)
	ticker := time.NewTicker(peerSubscriptionResyncInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			connectedPeers, err := n.peerConnections.GetConnectedPeers(ctx)
			if err != nil {
				slog.Debug("subscription resync skipped: failed to get connected peers",
					"node_id", n.config.NodeID,
					"error", err)
				continue
			}

			currentPeers := make(map[string]bool, len(connectedPeers))
			for _, peer := range connectedPeers {
				peerID := peer.ID()
				currentPeers[peerID] = true
				if seenPeers[peerID] {
					continue
				}

				if err := n.syncLocalSubscriptionsToPeer(ctx, peerID); err != nil {
					slog.Warn("failed to resync subscriptions to peer",
						"node_id", n.config.NodeID,
						"peer_id", peerID,
						"error", err)
					continue
				}
				seenPeers[peerID] = true
			}

			for peerID := range seenPeers {
				if !currentPeers[peerID] {
					delete(seenPeers, peerID)
				}
			}
		}
	}
}

func (n *GRPCMeshNode) syncLocalSubscriptionsToPeer(ctx context.Context, peerID string) error {
	snapshot := &peerlinkpkg.InterestSnapshot{
		NodeId: n.config.NodeID,
		Topics: n.localInterestSnapshot(),
	}
	if err := n.controlPlanePeerLink.SendInterestSnapshot(ctx, peerID, snapshot); err != nil {
		return fmt.Errorf("failed to send interest snapshot to peer %s: %w", peerID, err)
	}
	n.interestSnapshotsSent.Add(1)
	return nil
}

func (n *GRPCMeshNode) localInterestSnapshot() []string {
	subscriptions := n.localSubscriptionSnapshot()
	seen := make(map[string]struct{})
	var topics []string
	for _, subscription := range subscriptions {
		if _, ok := seen[subscription.Topic]; ok {
			continue
		}
		seen[subscription.Topic] = struct{}{}
		topics = append(topics, subscription.Topic)
	}
	sort.Strings(topics)
	return topics
}

func (n *GRPCMeshNode) localTopicInterestCount(topic string) int {
	n.subscriptionsMu.RLock()
	defer n.subscriptionsMu.RUnlock()

	count := 0
	for _, clientSubscriptions := range n.subscriptions {
		for _, subscription := range clientSubscriptions {
			if subscription.Topic == topic {
				count++
			}
		}
	}
	return count
}

func (n *GRPCMeshNode) localSubscriptionSnapshot() []meshnode.ClientSubscription {
	n.subscriptionsMu.RLock()
	defer n.subscriptionsMu.RUnlock()

	var subscriptions []meshnode.ClientSubscription
	for _, clientSubscriptions := range n.subscriptions {
		for _, subscription := range clientSubscriptions {
			subscriptions = append(subscriptions, *subscription)
		}
	}
	return subscriptions
}

// addSubscriptionMetadata adds a subscription to the metadata store
func (n *GRPCMeshNode) addSubscriptionMetadata(clientID, subscriptionID, topic string) {
	n.subscriptionsMu.Lock()
	defer n.subscriptionsMu.Unlock()

	// Initialize client map if it doesn't exist
	if n.subscriptions[clientID] == nil {
		n.subscriptions[clientID] = make(map[string]*meshnode.ClientSubscription)
	}

	// Create and store subscription
	subscription := &meshnode.ClientSubscription{
		ID:        subscriptionID,
		Topic:     topic,
		ClientID:  clientID,
		CreatedAt: time.Now(),
	}

	n.subscriptions[clientID][subscriptionID] = subscription
}

// GetClientSubscriptions returns all subscriptions for a specific client
func (n *GRPCMeshNode) GetClientSubscriptions(ctx context.Context, clientID string) ([]meshnode.ClientSubscription, error) {
	n.subscriptionsMu.RLock()
	defer n.subscriptionsMu.RUnlock()

	clientSubs := n.subscriptions[clientID]
	if clientSubs == nil {
		return []meshnode.ClientSubscription{}, nil
	}

	// Convert to slice with value copies (not pointers)
	var result []meshnode.ClientSubscription
	for _, sub := range clientSubs {
		result = append(result, *sub)
	}

	return result, nil
}

// UnsubscribeByID removes a client's subscription by subscription ID
func (n *GRPCMeshNode) UnsubscribeByID(ctx context.Context, clientID, subscriptionID string) error {
	n.subscriptionsMu.RLock()
	// Find the subscription first to get the topic
	clientSubs := n.subscriptions[clientID]
	if clientSubs == nil {
		n.subscriptionsMu.RUnlock()
		return fmt.Errorf("%w: no subscriptions found for client %s", meshnode.ErrSubscriptionNotFound, clientID)
	}

	subscription, exists := clientSubs[subscriptionID]
	if !exists {
		n.subscriptionsMu.RUnlock()
		return fmt.Errorf("%w: subscription %s not found for client %s", meshnode.ErrSubscriptionNotFound, subscriptionID, clientID)
	}

	topic := subscription.Topic
	n.subscriptionsMu.RUnlock()

	// Get the client to call the regular Unsubscribe method
	n.mu.RLock()
	client := n.clients[clientID]
	n.mu.RUnlock()

	if client == nil {
		return fmt.Errorf("%w: client %s not found", meshnode.ErrClientNotFound, clientID)
	}

	// Call the regular Unsubscribe method to handle routing table and peer propagation
	err := n.Unsubscribe(ctx, client, topic)
	if err != nil {
		return fmt.Errorf("failed to unsubscribe: %w", err)
	}

	return nil
}

func (n *GRPCMeshNode) removeSubscriptionMetadataByTopic(clientID, topic string) {
	n.subscriptionsMu.Lock()
	defer n.subscriptionsMu.Unlock()

	clientSubs := n.subscriptions[clientID]
	if clientSubs == nil {
		return
	}

	for subscriptionID, subscription := range clientSubs {
		if subscription.Topic == topic {
			delete(clientSubs, subscriptionID)
		}
	}

	if len(clientSubs) == 0 {
		delete(n.subscriptions, clientID)
	}
}
