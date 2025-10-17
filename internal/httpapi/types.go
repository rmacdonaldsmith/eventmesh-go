package httpapi

import "time"

// Request/Response types for the HTTP API

// AuthRequest represents a login request
type AuthRequest struct {
	ClientID string `json:"clientId"`
}

// AuthResponse represents a login response
type AuthResponse struct {
	Token     string    `json:"token"`
	ClientID  string    `json:"clientId"`
	ExpiresAt time.Time `json:"expiresAt"`
}

// PublishRequest represents an event publishing request
type PublishRequest struct {
	Topic   string      `json:"topic"`
	Payload interface{} `json:"payload"`
}

// PublishResponse represents an event publishing response
type PublishResponse struct {
	EventID   string    `json:"eventId"`
	Offset    int64     `json:"offset"`
	Timestamp time.Time `json:"timestamp"`
}

// SubscriptionRequest represents a subscription creation request
type SubscriptionRequest struct {
	Topic string `json:"topic"`
}

// SubscriptionResponse represents a subscription response
type SubscriptionResponse struct {
	ID        string    `json:"id"`
	Topic     string    `json:"topic"`
	ClientID  string    `json:"clientId"`
	CreatedAt time.Time `json:"createdAt"`
}

// SubscriptionsListResponse represents a list of subscriptions
type SubscriptionsListResponse struct {
	Subscriptions []SubscriptionResponse `json:"subscriptions"`
}

// AdminClientsResponse represents admin view of connected clients
type AdminClientsResponse struct {
	Clients []ClientInfo `json:"clients"`
}

// ClientInfo represents information about a connected client
type ClientInfo struct {
	ID            string    `json:"id"`
	Authenticated bool      `json:"authenticated"`
	ConnectedAt   time.Time `json:"connectedAt"`
	Subscriptions []string  `json:"subscriptions"`
}

// AdminSubscriptionsResponse represents admin view of all subscriptions
type AdminSubscriptionsResponse struct {
	Subscriptions []AdminSubscriptionInfo `json:"subscriptions"`
}

// AdminSubscriptionInfo represents detailed subscription info for admins
type AdminSubscriptionInfo struct {
	ID        string    `json:"id"`
	Topic     string    `json:"topic"`
	ClientID  string    `json:"clientId"`
	CreatedAt time.Time `json:"createdAt"`
}

// AdminStatsResponse represents system statistics
type AdminStatsResponse struct {
	ConnectedClients   int `json:"connectedClients"`
	TotalSubscriptions int `json:"totalSubscriptions"`
	TotalTopics        int `json:"totalTopics"`
	EventsPublished    int `json:"eventsPublished"`
}

// HealthResponse represents health check response
type HealthResponse struct {
	Healthy             bool   `json:"healthy"`
	EventLogHealthy     bool   `json:"eventLogHealthy"`
	RoutingTableHealthy bool   `json:"routingTableHealthy"`
	PeerLinkHealthy     bool   `json:"peerLinkHealthy"`
	ConnectedClients    int    `json:"connectedClients"`
	ConnectedPeers      int    `json:"connectedPeers"`
	Message             string `json:"message"`
}

// ErrorResponse represents an error response
type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message"`
	Code    int    `json:"code"`
}

// EventStreamMessage represents a server-sent event message
type EventStreamMessage struct {
	EventID   string      `json:"eventId"`
	Topic     string      `json:"topic"`
	Payload   interface{} `json:"payload"`
	Timestamp time.Time   `json:"timestamp"`
	Offset    int64       `json:"offset"`
}

// ReadEventsResponse represents a response for reading events from a topic
type ReadEventsResponse struct {
	Events      []EventStreamMessage `json:"events"`
	Topic       string               `json:"topic"`
	StartOffset int64                `json:"startOffset"`
	Count       int                  `json:"count"`
}
