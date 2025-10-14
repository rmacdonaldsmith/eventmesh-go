package meshnode

import (
	"github.com/rmacdonaldsmith/eventmesh-go/pkg/eventlog"
	"github.com/rmacdonaldsmith/eventmesh-go/pkg/meshnode"
	"github.com/rmacdonaldsmith/eventmesh-go/pkg/routingtable"
)

// TrustedClient is a simple client implementation for MVP.
// Since REQ-MNODE-001 (authentication) is descoped from MVP,
// this client always reports as authenticated.
//
// For local event delivery, it maintains a channel where events are sent.
type TrustedClient struct {
	id          string
	eventChan   chan eventlog.EventRecord // Channel for receiving events
	eventBuffer []eventlog.EventRecord    // Buffer for testing (synchronous delivery)
}

// NewTrustedClient creates a new trusted client with the given ID.
// FOR MVP: Always reports as authenticated (no actual authentication)
func NewTrustedClient(id string) *TrustedClient {
	return &TrustedClient{
		id:          id,
		eventChan:   make(chan eventlog.EventRecord, 100), // Buffered channel
		eventBuffer: make([]eventlog.EventRecord, 0),
	}
}

// ID returns unique identifier for this client
func (c *TrustedClient) ID() string {
	return c.id
}

// IsAuthenticated returns whether the client is properly authenticated.
// FOR MVP: Always returns true (no authentication required)
func (c *TrustedClient) IsAuthenticated() bool {
	return true
}

// Type returns the subscriber type for routing table integration
func (c *TrustedClient) Type() routingtable.SubscriberType {
	return routingtable.LocalClient
}

// DeliverEvent delivers an event to this client (for local subscriber delivery)
// FOR MVP: Uses a simple buffer approach for testing
func (c *TrustedClient) DeliverEvent(event eventlog.EventRecord) {
	// For testing, add to buffer (synchronous)
	c.eventBuffer = append(c.eventBuffer, event)

	// Also try to send to channel (asynchronous, non-blocking)
	select {
	case c.eventChan <- event:
		// Event delivered successfully
	default:
		// Channel is full, skip (in production, we'd handle this better)
	}
}

// GetReceivedEvents returns all events received by this client (for testing)
func (c *TrustedClient) GetReceivedEvents() []eventlog.EventRecord {
	return append([]eventlog.EventRecord{}, c.eventBuffer...) // Return copy
}

// GetEventChannel returns the channel for receiving events (for production use)
func (c *TrustedClient) GetEventChannel() <-chan eventlog.EventRecord {
	return c.eventChan
}

// Verify that TrustedClient implements both Client and Subscriber interfaces at compile time
var _ meshnode.Client = (*TrustedClient)(nil)
var _ routingtable.Subscriber = (*TrustedClient)(nil)
