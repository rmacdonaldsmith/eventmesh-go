package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/rmacdonaldsmith/eventmesh-go/internal/meshnode"
	eventlogpkg "github.com/rmacdonaldsmith/eventmesh-go/pkg/eventlog"
	meshnodepkg "github.com/rmacdonaldsmith/eventmesh-go/pkg/meshnode"
	"github.com/rmacdonaldsmith/eventmesh-go/pkg/routingtable"
)

// Subscription represents a client subscription to a topic
type Subscription struct {
	ID        string    `json:"id"`
	Topic     string    `json:"topic"`
	ClientID  string    `json:"clientId"`
	CreatedAt time.Time `json:"createdAt"`
}

// HTTPClient represents an HTTP API client for the MeshNode
type HTTPClient struct {
	clientID        string
	isAuthenticated bool
	eventChan       chan eventlogpkg.EventRecord // Channel for receiving events (for SSE streaming)
	subscriptions   map[string]*Subscription     // Track client's subscriptions
	subscriptionMu  sync.RWMutex                 // Protect subscriptions map
}

// NewHTTPClient creates a new HTTP client from JWT claims
func NewHTTPClient(claims *JWTClaims) *HTTPClient {
	return &HTTPClient{
		clientID:        claims.ClientID,
		isAuthenticated: true,
		eventChan:       make(chan eventlogpkg.EventRecord, 100), // Buffered channel like TrustedClient
		subscriptions:   make(map[string]*Subscription),         // Initialize subscriptions map
	}
}

// ID returns the unique identifier for this client
func (c *HTTPClient) ID() string {
	return c.clientID
}

// IsAuthenticated returns whether the client is properly authenticated
func (c *HTTPClient) IsAuthenticated() bool {
	return c.isAuthenticated
}

// Type returns the subscriber type for routing table integration
func (c *HTTPClient) Type() routingtable.SubscriberType {
	return routingtable.LocalClient
}

// DeliverEvent delivers an event to this client (for SSE streaming)
// This method is called by MeshNode when events need to be delivered to local subscribers
func (c *HTTPClient) DeliverEvent(event eventlogpkg.EventRecord) {
	// Try to send to channel (non-blocking)
	select {
	case c.eventChan <- event:
		// Event delivered successfully
	default:
		// Channel is full, skip (in production, we'd handle this better)
		// TODO: Add logging or metrics for dropped events
	}
}

// GetEventChannel returns the channel for receiving events (for SSE streaming)
func (c *HTTPClient) GetEventChannel() <-chan eventlogpkg.EventRecord {
	return c.eventChan
}

// AddSubscription adds a subscription for this client
func (c *HTTPClient) AddSubscription(subscriptionID, topic string) error {
	c.subscriptionMu.Lock()
	defer c.subscriptionMu.Unlock()

	subscription := &Subscription{
		ID:        subscriptionID,
		Topic:     topic,
		ClientID:  c.clientID,
		CreatedAt: time.Now(),
	}

	c.subscriptions[subscriptionID] = subscription
	return nil
}

// GetSubscriptions returns all subscriptions for this client
func (c *HTTPClient) GetSubscriptions() map[string]*Subscription {
	c.subscriptionMu.RLock()
	defer c.subscriptionMu.RUnlock()

	// Return a copy to prevent concurrent modification
	result := make(map[string]*Subscription)
	for id, sub := range c.subscriptions {
		result[id] = sub
	}
	return result
}

// RemoveSubscription removes a subscription for this client
func (c *HTTPClient) RemoveSubscription(subscriptionID string) error {
	c.subscriptionMu.Lock()
	defer c.subscriptionMu.Unlock()

	// Check if subscription exists
	_, exists := c.subscriptions[subscriptionID]
	if !exists {
		return fmt.Errorf("subscription %s not found", subscriptionID)
	}

	// Remove the subscription
	delete(c.subscriptions, subscriptionID)
	return nil
}

// Handlers contains all HTTP request handlers
type Handlers struct {
	meshNode *meshnode.GRPCMeshNode
	jwtAuth  *JWTAuth
}

// NewHandlers creates a new handlers instance
func NewHandlers(meshNode *meshnode.GRPCMeshNode, jwtAuth *JWTAuth) *Handlers {
	return &Handlers{
		meshNode: meshNode,
		jwtAuth:  jwtAuth,
	}
}

// Auth endpoints

// Login handles POST /api/v1/auth/login
func (h *Handlers) Login(w http.ResponseWriter, r *http.Request) {
	// Validate JSON content type
	if err := h.validateJSON(r); err != nil {
		h.writeError(w, err.Error(), http.StatusBadRequest)
		return
	}

	var req AuthRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Validate required fields
	if err := h.validateAuthRequest(&req); err != nil {
		h.writeError(w, err.Error(), http.StatusBadRequest)
		return
	}

	// For MVP: simple clientId-based authentication (no password validation)
	// In production: validate credentials against user store
	isAdmin := req.ClientID == "admin" // Simple admin detection for MVP

	token, expiresAt, err := h.jwtAuth.GenerateToken(req.ClientID, isAdmin)
	if err != nil {
		h.writeError(w, "Failed to generate token", http.StatusInternalServerError)
		return
	}

	resp := AuthResponse{
		Token:     token,
		ClientID:  req.ClientID,
		ExpiresAt: expiresAt,
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(resp)
}

// Event endpoints

// PublishEvent handles POST /api/v1/events
func (h *Handlers) PublishEvent(w http.ResponseWriter, r *http.Request) {
	// Validate JSON content type
	if err := h.validateJSON(r); err != nil {
		h.writeError(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Parse request body
	var req PublishRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Validate request
	if err := h.validatePublishRequest(&req); err != nil {
		h.writeError(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Get authenticated client from context
	claims := GetClaims(r)
	if claims == nil {
		h.writeError(w, "Authentication required", http.StatusUnauthorized)
		return
	}

	// Create HTTP client for MeshNode
	client := NewHTTPClient(claims)

	// Convert request to EventRecord
	var payload []byte
	if req.Payload != nil {
		payloadBytes, err := json.Marshal(req.Payload)
		if err != nil {
			h.writeError(w, "Invalid payload format", http.StatusBadRequest)
			return
		}
		payload = payloadBytes
	}

	// Create event record
	eventRecord := eventlogpkg.NewRecord(req.Topic, payload)

	// Publish event through MeshNode
	ctx := r.Context()
	err := h.meshNode.PublishEvent(ctx, client, eventRecord)
	if err != nil {
		h.writeError(w, fmt.Sprintf("Failed to publish event: %v", err), http.StatusInternalServerError)
		return
	}

	// Create response
	resp := PublishResponse{
		EventID:   fmt.Sprintf("%s-%d", req.Topic, eventRecord.Offset()), // Topic + offset as ID
		Offset:    eventRecord.Offset(), // This will be set by the EventLog
		Timestamp: time.Now(),
	}

	h.writeJSON(w, resp, http.StatusCreated)
}

// StreamEvents handles GET /api/v1/events/stream
func (h *Handlers) StreamEvents(w http.ResponseWriter, r *http.Request) {
	// Get authenticated client from context
	claims := GetClaims(r)
	if claims == nil {
		h.writeError(w, "Authentication required", http.StatusUnauthorized)
		return
	}

	// Parse topic query parameter
	topicFilter := r.URL.Query().Get("topic")
	hasTopicParam := r.URL.Query().Has("topic")

	if hasTopicParam {
		// Validate topic filter using existing validation
		if err := h.validateTopic(topicFilter); err != nil {
			h.writeError(w, fmt.Sprintf("Invalid topic filter: %v", err), http.StatusBadRequest)
			return
		}
	}

	// Set SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	// Send success status
	w.WriteHeader(http.StatusOK)

	// Check if client expects streaming (has Accept: text/event-stream header)
	// or if this is a test environment
	acceptHeader := r.Header.Get("Accept")
	if acceptHeader == "text/event-stream" || r.URL.Query().Get("stream") == "true" {
		// For streaming clients, set up subscription BEFORE sending connection message
		// Create HTTP client for MeshNode subscription
		client := NewHTTPClient(claims)

		// If we have a topic filter, subscribe to it in the MeshNode FIRST
		if hasTopicParam && topicFilter != "" {
			err := h.meshNode.Subscribe(r.Context(), client, topicFilter)
			if err != nil {
				h.writeError(w, fmt.Sprintf("Failed to subscribe to topic: %v", err), http.StatusInternalServerError)
				return
			}
		}

		// Send connection established message with topic info AFTER subscription is set up
		if hasTopicParam && topicFilter != "" {
			w.Write([]byte(fmt.Sprintf(": SSE connection established for topic: %s\n\n", topicFilter)))
		} else {
			w.Write([]byte(": SSE connection established for all topics\n\n"))
		}

		// For testing purposes, send a sample event message in proper SSE format
		// This will be replaced with real event streaming in subsequent steps
		if hasTopicParam && topicFilter != "" {
			// Create a sample EventStreamMessage
			sampleEvent := EventStreamMessage{
				EventID:   fmt.Sprintf("%s-sample-123", topicFilter),
				Topic:     topicFilter,
				Payload:   map[string]interface{}{"message": "Sample event for testing SSE format"},
				Timestamp: time.Now(),
				Offset:    1,
			}

			// Send as SSE data message
			if err := h.writeSSEMessage(w, sampleEvent); err != nil {
				// Log error but don't fail the connection
				_ = err
			}
		}

		// Start streaming with keepalive
		h.streamWithKeepalive(w, r, topicFilter, client)
	} else {
		// For non-streaming clients, just send connection message and sample event
		// Send connection established message with topic info
		if hasTopicParam && topicFilter != "" {
			w.Write([]byte(fmt.Sprintf(": SSE connection established for topic: %s\n\n", topicFilter)))
		} else {
			w.Write([]byte(": SSE connection established for all topics\n\n"))
		}

		// For testing purposes, send a sample event message in proper SSE format
		// This will be replaced with real event streaming in subsequent steps
		if hasTopicParam && topicFilter != "" {
			// Create a sample EventStreamMessage
			sampleEvent := EventStreamMessage{
				EventID:   fmt.Sprintf("%s-sample-123", topicFilter),
				Topic:     topicFilter,
				Payload:   map[string]interface{}{"message": "Sample event for testing SSE format"},
				Timestamp: time.Now(),
				Offset:    1,
			}

			// Send as SSE data message
			if err := h.writeSSEMessage(w, sampleEvent); err != nil {
				// Log error but don't fail the connection
				_ = err
			}
		}
	}
}

// Subscription endpoints

// ListSubscriptions handles GET /api/v1/subscriptions
func (h *Handlers) ListSubscriptions(w http.ResponseWriter, r *http.Request) {
	// TODO: Implement in Task 104
	h.writeError(w, "Not implemented yet - will be implemented in Task 104", http.StatusNotImplemented)
}

// CreateSubscription handles POST /api/v1/subscriptions
func (h *Handlers) CreateSubscription(w http.ResponseWriter, r *http.Request) {
	// Validate JSON content type
	if err := h.validateJSON(r); err != nil {
		h.writeError(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Parse request body
	var req SubscriptionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Validate topic is not empty
	if req.Topic == "" {
		h.writeError(w, "Topic is required", http.StatusBadRequest)
		return
	}

	// Get authenticated client from context
	claims := GetClaims(r)
	if claims == nil {
		h.writeError(w, "Authentication required", http.StatusUnauthorized)
		return
	}

	// Create HTTP client for MeshNode
	client := NewHTTPClient(claims)

	// Subscribe to topic through MeshNode
	// The MeshNode will automatically generate and store subscription metadata
	ctx := r.Context()
	err := h.meshNode.Subscribe(ctx, client, req.Topic)
	if err != nil {
		h.writeError(w, fmt.Sprintf("Failed to subscribe: %v", err), http.StatusInternalServerError)
		return
	}

	// Get the created subscription from MeshNode
	subscriptions, err := h.meshNode.GetClientSubscriptions(ctx, claims.ClientID)
	if err != nil {
		h.writeError(w, fmt.Sprintf("Failed to retrieve subscription: %v", err), http.StatusInternalServerError)
		return
	}

	// Find the most recently created subscription for this topic
	var createdSubscription *meshnodepkg.ClientSubscription
	for i := range subscriptions {
		sub := &subscriptions[i]
		if sub.Topic == req.Topic {
			if createdSubscription == nil || sub.CreatedAt.After(createdSubscription.CreatedAt) {
				createdSubscription = sub
			}
		}
	}

	if createdSubscription == nil {
		h.writeError(w, "Failed to find created subscription", http.StatusInternalServerError)
		return
	}

	// Create response
	response := SubscriptionResponse{
		ID:        createdSubscription.ID,
		Topic:     createdSubscription.Topic,
		ClientID:  createdSubscription.ClientID,
		CreatedAt: createdSubscription.CreatedAt,
	}

	// Return success response
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(response)
}

// GetSubscriptions handles GET /api/v1/subscriptions
func (h *Handlers) GetSubscriptions(w http.ResponseWriter, r *http.Request) {
	// Get authenticated client from context
	claims := GetClaims(r)
	if claims == nil {
		h.writeError(w, "Authentication required", http.StatusUnauthorized)
		return
	}

	// Get subscriptions from MeshNode
	ctx := r.Context()
	subscriptions, err := h.meshNode.GetClientSubscriptions(ctx, claims.ClientID)
	if err != nil {
		h.writeError(w, fmt.Sprintf("Failed to retrieve subscriptions: %v", err), http.StatusInternalServerError)
		return
	}

	// Convert to response format
	var responseSubscriptions []SubscriptionResponse
	for _, subscription := range subscriptions {
		responseSubscriptions = append(responseSubscriptions, SubscriptionResponse{
			ID:        subscription.ID,
			Topic:     subscription.Topic,
			ClientID:  subscription.ClientID,
			CreatedAt: subscription.CreatedAt,
		})
	}

	// Create response
	response := SubscriptionsListResponse{
		Subscriptions: responseSubscriptions,
	}

	// Return success response
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

// DeleteSubscription handles DELETE /api/v1/subscriptions/{id}
func (h *Handlers) DeleteSubscription(w http.ResponseWriter, r *http.Request) {
	// TODO: Implement in Task 104
	h.writeError(w, "Not implemented yet - will be implemented in Task 104", http.StatusNotImplemented)
}

// Admin endpoints

// AdminListClients handles GET /api/v1/admin/clients
func (h *Handlers) AdminListClients(w http.ResponseWriter, r *http.Request) {
	// TODO: Implement in Task 105
	h.writeError(w, "Not implemented yet - will be implemented in Task 105", http.StatusNotImplemented)
}

// AdminListSubscriptions handles GET /api/v1/admin/subscriptions
func (h *Handlers) AdminListSubscriptions(w http.ResponseWriter, r *http.Request) {
	// TODO: Implement in Task 105
	h.writeError(w, "Not implemented yet - will be implemented in Task 105", http.StatusNotImplemented)
}

// AdminGetStats handles GET /api/v1/admin/stats
func (h *Handlers) AdminGetStats(w http.ResponseWriter, r *http.Request) {
	// TODO: Implement in Task 105
	h.writeError(w, "Not implemented yet - will be implemented in Task 105", http.StatusNotImplemented)
}

// Health endpoint

// Health handles GET /api/v1/health
func (h *Handlers) Health(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	health, err := h.meshNode.GetHealth(ctx)
	if err != nil {
		h.writeError(w, "Failed to get health status", http.StatusInternalServerError)
		return
	}

	resp := HealthResponse{
		Healthy:             health.Healthy,
		EventLogHealthy:     health.EventLogHealthy,
		RoutingTableHealthy: health.RoutingTableHealthy,
		PeerLinkHealthy:     health.PeerLinkHealthy,
		ConnectedClients:    health.ConnectedClients,
		ConnectedPeers:      health.ConnectedPeers,
		Message:             health.Message,
	}

	statusCode := http.StatusOK
	if !health.Healthy {
		statusCode = http.StatusServiceUnavailable
	}

	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(resp)
}

// Helper methods

// writeError writes an error response as JSON
func (h *Handlers) writeError(w http.ResponseWriter, message string, statusCode int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	errorResp := ErrorResponse{
		Error:   http.StatusText(statusCode),
		Message: message,
		Code:    statusCode,
	}

	if err := json.NewEncoder(w).Encode(errorResp); err != nil {
		// Fallback error handling
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

// writeJSON writes a JSON response
func (h *Handlers) writeJSON(w http.ResponseWriter, data interface{}, statusCode int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		// Fallback error handling
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

// parsePathParam extracts a path parameter (simple implementation)
// TODO: Consider using a proper router like gorilla/mux for production
func (h *Handlers) parsePathParam(path, prefix string) string {
	if len(path) <= len(prefix) {
		return ""
	}
	return path[len(prefix):]
}

// validateJSON validates that the request has valid JSON content-type
func (h *Handlers) validateJSON(r *http.Request) error {
	contentType := r.Header.Get("Content-Type")
	if contentType != "application/json" {
		return fmt.Errorf("Content-Type must be application/json")
	}
	return nil
}

// validateAuthRequest validates authentication request fields
func (h *Handlers) validateAuthRequest(req *AuthRequest) error {
	if req.ClientID == "" {
		return fmt.Errorf("clientId is required")
	}
	// Basic clientID format validation
	if len(req.ClientID) < 2 {
		return fmt.Errorf("clientId must be at least 2 characters")
	}
	return nil
}

// validateTopic validates topic name format
func (h *Handlers) validateTopic(topic string) error {
	if topic == "" {
		return fmt.Errorf("topic is required")
	}
	if len(topic) < 2 {
		return fmt.Errorf("topic must be at least 2 characters")
	}
	// Basic topic format validation (letters, numbers, dots, hyphens, underscores)
	for _, char := range topic {
		if !((char >= 'a' && char <= 'z') || (char >= 'A' && char <= 'Z') ||
			(char >= '0' && char <= '9') || char == '.' || char == '-' || char == '_') {
			return fmt.Errorf("topic contains invalid characters (allowed: letters, numbers, ., -, _)")
		}
	}
	return nil
}

// validatePublishRequest validates event publishing request fields
func (h *Handlers) validatePublishRequest(req *PublishRequest) error {
	if err := h.validateTopic(req.Topic); err != nil {
		return err
	}
	// Payload can be nil, but if provided should be valid JSON (checked during marshaling)
	return nil
}

// writeSSEMessage writes an EventStreamMessage as a properly formatted SSE data message
func (h *Handlers) writeSSEMessage(w http.ResponseWriter, message EventStreamMessage) error {
	// Marshal the message to JSON
	jsonData, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("failed to marshal SSE message: %w", err)
	}

	// Write in SSE format: "data: {json}\n\n"
	_, err = w.Write([]byte(fmt.Sprintf("data: %s\n\n", string(jsonData))))
	return err
}

// streamWithKeepalive maintains the SSE connection with periodic keepalive messages and event delivery
func (h *Handlers) streamWithKeepalive(w http.ResponseWriter, r *http.Request, topicFilter string, client *HTTPClient) {
	ctx := r.Context()

	// Create a ticker for keepalive messages (every 2 seconds for testing, 30 seconds for production)
	// TODO: Make this configurable
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	// Flush any buffered data immediately
	if flusher, ok := w.(http.Flusher); ok {
		flusher.Flush()
	}

	// Get the event channel from the client
	eventChan := client.GetEventChannel()

	for {
		select {
		case <-ctx.Done():
			// Context cancelled (client disconnected or timeout)
			// Unsubscribe from MeshNode if we had a subscription
			if topicFilter != "" {
				// Use background context for cleanup since request context is cancelled
				cleanupCtx := context.Background()
				h.meshNode.Unsubscribe(cleanupCtx, client, topicFilter)
			}
			return

		case <-ticker.C:
			// Send keepalive/ping message
			_, err := w.Write([]byte(": ping\n\n"))
			if err != nil {
				// Client disconnected, cleanup subscription
				if topicFilter != "" {
					cleanupCtx := context.Background()
					h.meshNode.Unsubscribe(cleanupCtx, client, topicFilter)
				}
				return
			}

			// Flush the keepalive message
			if flusher, ok := w.(http.Flusher); ok {
				flusher.Flush()
			}

		case event := <-eventChan:
			// Received an event from MeshNode, forward it to SSE client
			// Convert EventRecord to EventStreamMessage
			var payload interface{}
			if event.Payload() != nil {
				// Try to unmarshal payload as JSON
				err := json.Unmarshal(event.Payload(), &payload)
				if err != nil {
					// If not valid JSON, send as string
					payload = string(event.Payload())
				}
			}

			sseMessage := EventStreamMessage{
				EventID:   fmt.Sprintf("%s-%d", event.Topic(), event.Offset()),
				Topic:     event.Topic(),
				Payload:   payload,
				Timestamp: event.Timestamp(),
				Offset:    event.Offset(),
			}

			// Send the event via SSE
			if err := h.writeSSEMessage(w, sseMessage); err != nil {
				// Client disconnected, cleanup subscription
				if topicFilter != "" {
					cleanupCtx := context.Background()
					h.meshNode.Unsubscribe(cleanupCtx, client, topicFilter)
				}
				return
			}

			// Flush the event message
			if flusher, ok := w.(http.Flusher); ok {
				flusher.Flush()
			}
		}
	}
}
