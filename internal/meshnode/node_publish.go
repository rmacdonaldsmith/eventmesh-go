package meshnode

import (
	"context"
	"fmt"
	"log/slog"

	eventlogpkg "github.com/rmacdonaldsmith/eventmesh-go/pkg/eventlog"
	"github.com/rmacdonaldsmith/eventmesh-go/pkg/meshnode"
	routingtablepkg "github.com/rmacdonaldsmith/eventmesh-go/pkg/routingtable"
)

// DeliveryResult tracks the results of event delivery attempts
type DeliveryResult struct {
	LocalSuccesses int
	LocalFailures  int
	PeerSuccesses  int
	PeerFailures   int
	Errors         []error
}

// HasFailures returns true if any deliveries failed
func (r DeliveryResult) HasFailures() bool {
	return r.LocalFailures > 0 || r.PeerFailures > 0
}

// TotalAttempts returns the total number of delivery attempts
func (r DeliveryResult) TotalAttempts() int {
	return r.LocalSuccesses + r.LocalFailures + r.PeerSuccesses + r.PeerFailures
}

// Error returns a formatted error describing delivery failures
func (r DeliveryResult) Error() string {
	if !r.HasFailures() {
		return ""
	}
	return fmt.Sprintf("event delivery partial failure: %d local successes, %d local failures, %d peer successes, %d peer failures",
		r.LocalSuccesses, r.LocalFailures, r.PeerSuccesses, r.PeerFailures)
}

// validatePublishRequest validates the node state and publish request parameters
func (n *GRPCMeshNode) validatePublishRequest(client meshnode.Client, event *eventlogpkg.Event) error {
	if n.closed {
		return fmt.Errorf("cannot publish to closed mesh node")
	}

	if !n.started {
		return fmt.Errorf("cannot publish to stopped mesh node")
	}

	if client == nil {
		return fmt.Errorf("client cannot be nil")
	}
	if event == nil {
		return fmt.Errorf("event cannot be nil")
	}
	if !client.IsAuthenticated() {
		return fmt.Errorf("client is not authenticated")
	}

	if len(event.Payload) > MaxEventPayloadSize {
		return fmt.Errorf("event payload size %d bytes exceeds maximum allowed size %d bytes",
			len(event.Payload), MaxEventPayloadSize)
	}

	return nil
}

// eventDeliverer interface for subscribers that can receive events with error reporting
type eventDeliverer interface {
	DeliverEvent(*eventlogpkg.Event) error
}

// deliverToLocalSubscribers delivers an event to all local subscribers
func (n *GRPCMeshNode) deliverToLocalSubscribers(ctx context.Context, event *eventlogpkg.Event) DeliveryResult {
	var result DeliveryResult

	localSubscribers, err := n.routingTable.GetSubscribers(ctx, event.Topic)
	if err != nil {
		result.Errors = append(result.Errors, fmt.Errorf("failed to get local subscribers: %w", err))
		return result
	}

	for _, subscriber := range localSubscribers {
		// Check for context cancellation during delivery loop
		select {
		case <-ctx.Done():
			result.Errors = append(result.Errors, ctx.Err())
			return result
		default:
		}

		if subscriber.Type() == routingtablepkg.LocalClient {
			if ed, ok := subscriber.(eventDeliverer); ok {
				err := ed.DeliverEvent(event)
				if err != nil {
					result.LocalFailures++
					result.Errors = append(result.Errors, err)

					slog.Warn("failed to deliver event to local subscriber",
						"client_id", subscriber.ID(),
						"topic", event.Topic,
						"error", err)
				} else {
					result.LocalSuccesses++
				}
			} else {
				// Fallback: subscriber doesn't implement error-returning interface
				result.LocalFailures++
				err := fmt.Errorf("subscriber %s does not implement error-returning DeliverEvent interface", subscriber.ID())
				result.Errors = append(result.Errors, err)

				slog.Warn("subscriber missing error-returning DeliverEvent interface",
					"client_id", subscriber.ID(),
					"topic", event.Topic)
			}
		}
	}

	return result
}

// forwardToPeers forwards an event to interested remote peers using intelligent routing
func (n *GRPCMeshNode) forwardToPeers(ctx context.Context, event *eventlogpkg.Event, result *DeliveryResult) {
	connectedPeers, err := n.peerConnections.GetConnectedPeers(ctx)
	if err != nil {
		// Log peer connectivity issue but don't fail the publish (local delivery succeeded)
		slog.Warn("failed to get connected peers for forwarding",
			"topic", event.Topic,
			"error", err)
		result.Errors = append(result.Errors, fmt.Errorf("failed to get peers: %w", err))
		return
	}

	// Use intelligent routing: only send to peers with interested subscribers
	interestedPeers := n.getInterestedPeers(event.Topic, connectedPeers)
	for _, peer := range interestedPeers {
		// Check for context cancellation during peer forwarding loop
		select {
		case <-ctx.Done():
			result.Errors = append(result.Errors, ctx.Err())
			return
		default:
		}

		err := n.dataPlanePeerLink.SendEvent(ctx, peer.ID(), event)
		if err != nil {
			result.PeerFailures++
			result.Errors = append(result.Errors, err)

			slog.Warn("failed to forward event to peer",
				"peer_id", peer.ID(),
				"topic", event.Topic,
				"error", err)
		} else {
			result.PeerSuccesses++
		}
	}
}

// PublishEvent accepts an event from a local client and handles routing.
// Implements REQ-MNODE-002: persists locally before forwarding to peers.
//
// Event Flow (REQ-MNODE-002):
// 1. Validate client and event
// 2. Persist event to local EventLog FIRST (durability)
// 3. Find local subscribers via RoutingTable
// 4. Forward to remote peers via PeerLink (for remote subscribers)
func (n *GRPCMeshNode) PublishEvent(ctx context.Context, client meshnode.Client, event *eventlogpkg.Event) error {
	_, err := n.PublishEventWithResult(ctx, client, event)
	return err
}

// PublishEventWithResult accepts an event from a local client and returns the persisted event.
// The returned event contains the EventLog-assigned offset and should be used for API responses.
func (n *GRPCMeshNode) PublishEventWithResult(ctx context.Context, client meshnode.Client, event *eventlogpkg.Event) (*eventlogpkg.Event, error) {
	n.mu.RLock()
	defer n.mu.RUnlock()

	// Validate inputs and node state
	if err := n.validatePublishRequest(client, event); err != nil {
		return nil, err
	}

	// Step 1: PERSIST LOCALLY FIRST (REQ-MNODE-002)
	// This ensures durability before any forwarding occurs
	persistedEvent, err := n.eventLog.AppendEvent(ctx, event.Topic, event)
	if err != nil {
		return nil, fmt.Errorf("failed to persist event locally: %w", err)
	}

	// Step 2: Deliver to local subscribers
	deliveryResult := n.deliverToLocalSubscribers(ctx, persistedEvent)

	// Check for context cancellation after local delivery
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}

	// Step 3: Forward to remote peers using intelligent routing
	n.forwardToPeers(ctx, persistedEvent, &deliveryResult)

	// Check for context cancellation after peer forwarding
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}

	// Return based on delivery results
	if deliveryResult.HasFailures() {
		// Log summary of delivery issues
		slog.Warn("event publish completed with delivery failures",
			"topic", persistedEvent.Topic,
			"local_successes", deliveryResult.LocalSuccesses,
			"local_failures", deliveryResult.LocalFailures,
			"peer_successes", deliveryResult.PeerSuccesses,
			"peer_failures", deliveryResult.PeerFailures,
			"total_errors", len(deliveryResult.Errors))

		// Return error with detailed information
		return nil, fmt.Errorf("%s", deliveryResult.Error())
	}

	// Success: Event was persisted locally and delivered to all subscribers
	slog.Debug("event publish completed successfully",
		"topic", persistedEvent.Topic,
		"local_deliveries", deliveryResult.LocalSuccesses,
		"peer_deliveries", deliveryResult.PeerSuccesses)

	return persistedEvent, nil
}
