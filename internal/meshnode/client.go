package meshnode

import (
	"fmt"
	"time"

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
	connectedAt time.Time            // When this client connected
	eventChan   chan *eventlog.Event // Channel for receiving events
	eventBuffer []*eventlog.Event    // Buffer for testing (synchronous delivery)
}

// NewTrustedClient creates a new trusted client with the given ID.
// FOR MVP: Always reports as authenticated (no actual authentication)
func NewTrustedClient(id string) *TrustedClient {
	return &TrustedClient{
		id:          id,
		connectedAt: time.Now(),
		eventChan:   make(chan *eventlog.Event, 100), // Buffered channel
		eventBuffer: make([]*eventlog.Event, 0),
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

// ConnectedAt returns when this client connected
func (c *TrustedClient) ConnectedAt() time.Time {
	return c.connectedAt
}

// Type returns the subscriber type for routing table integration
func (c *TrustedClient) Type() routingtable.SubscriberType {
	return routingtable.LocalClient
}

// DeliverEvent delivers an event to this client (for local subscriber delivery)
// Returns an error if delivery fails (e.g., channel full, client disconnected)
func (c *TrustedClient) DeliverEvent(event *eventlog.Event) error {
	if event == nil {
		return fmt.Errorf("cannot deliver nil event to client %s", c.id)
	}

	// For testing, add to buffer (synchronous)
	c.eventBuffer = append(c.eventBuffer, event)

	// Try to send to channel (asynchronous, non-blocking)
	select {
	case c.eventChan <- event:
		// Event delivered successfully
		return nil
	default:
		// Channel is full - this is now an error
		return fmt.Errorf("failed to deliver event to client %s: channel full", c.id)
	}
}

// GetReceivedEvents returns all events received by this client (for testing)
func (c *TrustedClient) GetReceivedEvents() []*eventlog.Event {
	return append([]*eventlog.Event{}, c.eventBuffer...) // Return copy
}

// GetEventChannel returns the channel for receiving events (for production use)
func (c *TrustedClient) GetEventChannel() <-chan *eventlog.Event {
	return c.eventChan
}

// Verify that TrustedClient implements required interfaces at compile time
var _ meshnode.Client = (*TrustedClient)(nil)
var _ routingtable.Subscriber = (*TrustedClient)(nil)
