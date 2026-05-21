package meshnode

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	eventlogpkg "github.com/rmacdonaldsmith/eventmesh-go/pkg/eventlog"
	peerlinkpkg "github.com/rmacdonaldsmith/eventmesh-go/pkg/peerlink"
	routingtablepkg "github.com/rmacdonaldsmith/eventmesh-go/pkg/routingtable"
)

// handleIncomingPeerEvents starts independent data-plane and control-plane
// consumers for messages received from peers.
func (n *GRPCMeshNode) handleIncomingPeerEvents(ctx context.Context) {
	go n.handleIncomingDataPlaneEvents(ctx)
	go n.handleIncomingControlPlaneMessages(ctx)
}

// handleIncomingDataPlaneEvents processes user events from peer nodes
func (n *GRPCMeshNode) handleIncomingDataPlaneEvents(ctx context.Context) {
	// Get event channel from PeerLink data plane
	eventChan, errChan := n.dataPlanePeerLink.ReceiveEvents(ctx)

	for {
		select {
		case <-ctx.Done():
			// Context cancelled, stop processing
			return

		case event, ok := <-eventChan:
			if !ok {
				// Event channel closed, stop processing
				return
			}

			// Process the incoming user event (no subscription filtering needed)
			n.processIncomingUserEvent(ctx, event)

		case err, ok := <-errChan:
			if !ok {
				// Error channel closed, stop processing
				return
			}
			if err != nil {
				// Log error but continue processing
				slog.Error("error receiving events from peers",
					"node_id", n.config.NodeID,
					"error", err)
			}
		}
	}
}

// handleIncomingControlPlaneMessages processes aggregate peer-interest messages.
func (n *GRPCMeshNode) handleIncomingControlPlaneMessages(ctx context.Context) {
	messageChan, errChan := n.controlPlanePeerLink.ReceiveInterestMessages(ctx)

	for {
		select {
		case <-ctx.Done():
			// Context cancelled, stop processing
			return

		case message, ok := <-messageChan:
			if !ok {
				// Change channel closed, stop processing
				return
			}

			// Process the incoming peer-interest message.
			n.processIncomingInterestMessage(ctx, message)

		case err, ok := <-errChan:
			if !ok {
				// Error channel closed, stop processing
				return
			}
			if err != nil {
				// Log error but continue processing
				slog.Error("error receiving interest messages from peers",
					"node_id", n.config.NodeID,
					"error", err)
			}
		}
	}
}

// processIncomingUserEvent handles a data-plane event received from a peer.
func (n *GRPCMeshNode) processIncomingUserEvent(ctx context.Context, event *eventlogpkg.Event) {
	topic := event.Topic

	// This is a regular user event - persist locally and deliver to local subscribers
	n.mu.RLock()
	if n.closed || !n.started {
		n.mu.RUnlock()
		return // Node is not running, ignore event
	}
	n.mu.RUnlock()

	// Persist the event locally (we received it from a peer)
	persistedEvent, err := n.eventLog.AppendEvent(ctx, topic, event)
	if err != nil {
		// Log error but continue
		// In production, we'd use structured logging
		_ = err // Failed to persist incoming event
		return
	}

	// Deliver to local subscribers
	n.mu.RLock()
	defer n.mu.RUnlock()

	localSubscribers, err := n.routingTable.GetSubscribers(ctx, topic)
	if err != nil {
		// Log error but continue
		_ = err // Failed to get local subscribers
		return
	}

	for _, subscriber := range localSubscribers {
		if subscriber.Type() == routingtablepkg.LocalClient {
			// Check if subscriber has error-returning DeliverEvent method
			type eventDeliverer interface {
				DeliverEvent(*eventlogpkg.Event) error
			}

			if ed, ok := subscriber.(eventDeliverer); ok {
				err := ed.DeliverEvent(persistedEvent)
				if err != nil {
					slog.Warn("failed to deliver incoming peer event to local subscriber",
						"client_id", subscriber.ID(),
						"topic", persistedEvent.Topic,
						"error", err)
				}
			} else {
				// Fallback: use non-error returning method if available
				type legacyEventDeliverer interface {
					DeliverEvent(*eventlogpkg.Event)
				}
				if led, ok := subscriber.(legacyEventDeliverer); ok {
					led.DeliverEvent(persistedEvent)
				} else {
					slog.Warn("subscriber has no DeliverEvent method",
						"client_id", subscriber.ID(),
						"topic", persistedEvent.Topic,
						"subscriber_type", fmt.Sprintf("%T", subscriber))
				}
			}
		}
	}
}

func (n *GRPCMeshNode) processIncomingInterestMessage(ctx context.Context, message *peerlinkpkg.InterestMessage) {
	if message == nil {
		return
	}
	if message.Update != nil {
		n.processIncomingInterestUpdate(ctx, message.Update)
		return
	}
	if message.Snapshot != nil {
		n.processIncomingInterestSnapshot(ctx, message.Snapshot)
	}
}

func (n *GRPCMeshNode) processIncomingInterestUpdate(ctx context.Context, update *peerlinkpkg.InterestUpdate) {
	if update == nil {
		return
	}
	n.interestUpdatesReceived.Add(1)
	n.peerSubscriptionsMu.Lock()
	defer n.peerSubscriptionsMu.Unlock()

	if n.peerSubscriptions[update.NodeId] == nil {
		n.peerSubscriptions[update.NodeId] = make(map[string]bool)
	}

	switch update.Action {
	case "subscribe":
		n.peerSubscriptions[update.NodeId][update.Topic] = true
		slog.Debug("peer interest added",
			"peer_node_id", update.NodeId,
			"topic", update.Topic)
	case "unsubscribe":
		delete(n.peerSubscriptions[update.NodeId], update.Topic)
		slog.Debug("peer interest removed",
			"peer_node_id", update.NodeId,
			"topic", update.Topic)
	default:
		slog.Warn("received interest update with unknown action",
			"action", update.Action,
			"peer_node_id", update.NodeId,
			"topic", update.Topic)
	}
}

func (n *GRPCMeshNode) processIncomingInterestSnapshot(ctx context.Context, snapshot *peerlinkpkg.InterestSnapshot) {
	if snapshot == nil {
		return
	}
	n.interestSnapshotsReceived.Add(1)
	if len(snapshot.Topics) == 0 {
		n.emptySnapshotsReceived.Add(1)
	}
	n.snapshotTopicsReceived.Add(int64(len(snapshot.Topics)))
	n.peerSubscriptionsMu.Lock()
	defer n.peerSubscriptionsMu.Unlock()

	topics := make(map[string]bool, len(snapshot.Topics))
	for _, topic := range snapshot.Topics {
		if topic != "" {
			topics[topic] = true
		}
	}
	n.peerSubscriptions[snapshot.NodeId] = topics
	slog.Debug("peer interest snapshot applied",
		"peer_node_id", snapshot.NodeId,
		"topics", len(topics))
}

// getInterestedPeers returns only peers that have subscribers for the given topic
// This enables intelligent routing instead of broadcasting to all peers
func (n *GRPCMeshNode) getInterestedPeers(topic string, allPeers []peerlinkpkg.PeerNode) []peerlinkpkg.PeerNode {
	n.peerSubscriptionsMu.RLock()
	defer n.peerSubscriptionsMu.RUnlock()

	var interestedPeers []peerlinkpkg.PeerNode

	for _, peer := range allPeers {
		peerID := peer.ID()

		// Check if this peer has subscriptions for the topic
		if peerTopics, exists := n.peerSubscriptions[peerID]; exists {
			for subscriptionPattern, hasSubscriber := range peerTopics {
				if hasSubscriber && matchesPeerSubscriptionPattern(subscriptionPattern, topic) {
					interestedPeers = append(interestedPeers, peer)
					break
				}
			}
		}
	}

	return interestedPeers
}

func matchesPeerSubscriptionPattern(pattern, topic string) bool {
	if pattern == topic {
		return true
	}

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
