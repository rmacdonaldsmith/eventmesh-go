package httpapi

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/rmacdonaldsmith/eventmesh-go/internal/meshnode"
	eventlogpkg "github.com/rmacdonaldsmith/eventmesh-go/pkg/eventlog"
	meshnodepkg "github.com/rmacdonaldsmith/eventmesh-go/pkg/meshnode"
	"github.com/rmacdonaldsmith/eventmesh-go/pkg/routingtable"
)

// HTTPClient represents an HTTP API client for the MeshNode
type HTTPClient struct {
	clientID        string
	isAuthenticated bool
	connectedAt     time.Time               // When this client connected
	eventChan       chan *eventlogpkg.Event // Channel for receiving events (for SSE streaming)
}

// NewHTTPClient creates a new HTTP client from JWT claims
func NewHTTPClient(claims *JWTClaims) *HTTPClient {
	return &HTTPClient{
		clientID:        claims.ClientID,
		isAuthenticated: true,
		connectedAt:     time.Now(),
		eventChan:       make(chan *eventlogpkg.Event, 100), // Buffered channel like TrustedClient
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

// ConnectedAt returns when this client connected
func (c *HTTPClient) ConnectedAt() time.Time {
	return c.connectedAt
}

// Type returns the subscriber type for routing table integration
func (c *HTTPClient) Type() routingtable.SubscriberType {
	return routingtable.LocalClient
}

// DeliverEvent delivers an event to this client (for SSE streaming)
// Returns an error if delivery fails (e.g., channel full, client disconnected)
func (c *HTTPClient) DeliverEvent(event *eventlogpkg.Event) error {
	if event == nil {
		return fmt.Errorf("cannot deliver nil event to client %s", c.ID())
	}

	// Try to send to channel (non-blocking)
	select {
	case c.eventChan <- event:
		// Event delivered successfully
		return nil
	default:
		// Channel is full - this is now an error
		err := fmt.Errorf("failed to deliver event to client %s: channel full (slow consumer)", c.ID())

		// Still log for observability
		slog.Warn("event delivery failed due to full channel",
			"client_id", c.ID(),
			"topic", event.Topic,
			"event_offset", event.Offset,
			"channel_capacity", cap(c.eventChan),
			"reason", "client_slow_consumer")

		return err
	}
}

// GetEventChannel returns the channel for receiving events (for SSE streaming)
func (c *HTTPClient) GetEventChannel() <-chan *eventlogpkg.Event {
	return c.eventChan
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
	eventRecord := eventlogpkg.NewEvent(req.Topic, payload)

	// Publish event through MeshNode
	ctx := r.Context()
	err := h.meshNode.PublishEvent(ctx, client, eventRecord)
	if err != nil {
		h.writeError(w, fmt.Sprintf("Failed to publish event: %v", err), http.StatusInternalServerError)
		return
	}

	// Log successful event publication
	slog.Info("event published",
		"client_id", client.ID(),
		"topic", req.Topic,
		"event_offset", eventRecord.Offset,
		"payload_size", len(payload))

	// Create response
	resp := PublishResponse{
		EventID:   fmt.Sprintf("%s-%d", req.Topic, eventRecord.Offset), // Topic + offset as ID
		Offset:    eventRecord.Offset,                                  // This will be set by the EventLog
		Timestamp: time.Now(),
	}

	h.writeJSON(w, resp, http.StatusCreated)
}

// ReadTopicEvents handles GET /api/v1/topics/{topic}/events
func (h *Handlers) ReadTopicEvents(w http.ResponseWriter, r *http.Request) {
	// Get authenticated client from context
	claims := GetClaims(r)
	if claims == nil {
		h.writeError(w, "Authentication required", http.StatusUnauthorized)
		return
	}

	// Extract topic from URL path (will be set by routing handler)
	topic := GetTopicFromPath(r)
	if topic == "" {
		h.writeError(w, "Topic is required", http.StatusBadRequest)
		return
	}

	// Validate topic format
	if err := h.validateTopic(topic); err != nil {
		h.writeError(w, fmt.Sprintf("Invalid topic: %v", err), http.StatusBadRequest)
		return
	}

	// Parse query parameters
	offsetStr := r.URL.Query().Get("offset")
	limitStr := r.URL.Query().Get("limit")

	// Default values
	var startOffset int64 = 0
	limit := 100

	// Parse offset if provided
	if offsetStr != "" {
		parsedOffset, err := strconv.ParseInt(offsetStr, 10, 64)
		if err != nil {
			h.writeError(w, "Invalid offset parameter", http.StatusBadRequest)
			return
		}
		if parsedOffset < 0 {
			h.writeError(w, "Offset must be non-negative", http.StatusBadRequest)
			return
		}
		startOffset = parsedOffset
	}

	// Parse limit if provided
	if limitStr != "" {
		parsedLimit, err := strconv.Atoi(limitStr)
		if err != nil {
			h.writeError(w, "Invalid limit parameter", http.StatusBadRequest)
			return
		}
		if parsedLimit <= 0 || parsedLimit > 1000 {
			h.writeError(w, "Limit must be between 1 and 1000", http.StatusBadRequest)
			return
		}
		limit = parsedLimit
	}

	// Get EventLog from MeshNode and read events
	ctx := r.Context()
	eventLog := h.meshNode.GetEventLog()
	events, err := eventLog.ReadEvents(ctx, topic, startOffset, limit)
	if err != nil {
		h.writeError(w, fmt.Sprintf("Failed to read events: %v", err), http.StatusInternalServerError)
		return
	}

	// Convert EventRecords to EventStreamMessages
	var eventMessages []EventStreamMessage
	for _, event := range events {
		// Parse payload as JSON if possible, otherwise use as string
		var payload interface{}
		if event.Payload != nil {
			err := json.Unmarshal(event.Payload, &payload)
			if err != nil {
				// If not valid JSON, use as string
				payload = string(event.Payload)
			}
		}

		message := EventStreamMessage{
			EventID:   fmt.Sprintf("%s-%d", event.Topic, event.Offset),
			Topic:     event.Topic,
			Payload:   payload,
			Timestamp: event.Timestamp,
			Offset:    event.Offset,
		}
		eventMessages = append(eventMessages, message)
	}

	// Create response
	response := ReadEventsResponse{
		Events:      eventMessages,
		Topic:       topic,
		StartOffset: startOffset,
		Count:       len(eventMessages),
	}

	h.writeJSON(w, response, http.StatusOK)
}

// StreamEvents handles GET /api/v1/events/stream
func (h *Handlers) StreamEvents(w http.ResponseWriter, r *http.Request) {
	// Get authenticated client from context
	claims := GetClaims(r)
	if claims == nil {
		h.writeError(w, "Authentication required", http.StatusUnauthorized)
		return
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
		// Create HTTP client for MeshNode streaming
		client := NewHTTPClient(claims)

		// Register the HTTPClient as a subscriber for ALL of the client's existing subscriptions
		ctx := r.Context()
		existingSubscriptions, err := h.meshNode.GetClientSubscriptions(ctx, claims.ClientID)
		if err != nil {
			h.writeError(w, fmt.Sprintf("Failed to get client subscriptions: %v", err), http.StatusInternalServerError)
			return
		}

		// Subscribe the HTTPClient to each existing subscription topic
		// This allows the MeshNode to deliver events to the SSE stream via HTTPClient.DeliverEvent()
		for _, subscription := range existingSubscriptions {
			err := h.meshNode.Subscribe(ctx, client, subscription.Topic)
			if err != nil {
				h.writeError(w, fmt.Sprintf("Failed to register SSE client for topic %s: %v", subscription.Topic, err), http.StatusInternalServerError)
				return
			}
		}

		// Send connection established message
		w.Write([]byte(": SSE connection established for all topics\n\n"))

		// Start streaming with keepalive
		h.streamWithKeepalive(w, r, client)
	} else {
		// For non-streaming clients, just send connection message
		w.Write([]byte(": SSE connection established for all topics\n\n"))
	}
}

// Subscription endpoints

// ListSubscriptions handles GET /api/v1/subscriptions
func (h *Handlers) ListSubscriptions(w http.ResponseWriter, r *http.Request) {
	// Get client ID from context
	claims := GetClaims(r)
	if claims == nil {
		h.writeError(w, "Authentication required", http.StatusUnauthorized)
		return
	}

	// Get client subscriptions from mesh node
	subscriptions, err := h.meshNode.GetClientSubscriptions(r.Context(), claims.ClientID)
	if err != nil {
		h.writeError(w, "Failed to get subscriptions: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Return subscriptions as JSON
	h.writeJSON(w, subscriptions, http.StatusOK)
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
	// Get authenticated client from context
	claims := GetClaims(r)
	if claims == nil {
		h.writeError(w, "Authentication required", http.StatusUnauthorized)
		return
	}

	// Get subscription ID from context (set by handleSubscriptionByID)
	subscriptionID := GetSubscriptionID(r)
	if subscriptionID == "" {
		h.writeError(w, "Subscription ID required", http.StatusBadRequest)
		return
	}

	// Delete subscription through MeshNode
	ctx := r.Context()
	err := h.meshNode.UnsubscribeByID(ctx, claims.ClientID, subscriptionID)
	if err != nil {
		// Check if it's a "not found" error (either no subscriptions for client or specific subscription not found)
		if strings.Contains(err.Error(), "not found") || strings.Contains(err.Error(), "no subscriptions found") {
			h.writeError(w, fmt.Sprintf("Subscription not found: %s", subscriptionID), http.StatusNotFound)
			return
		}
		h.writeError(w, fmt.Sprintf("Failed to delete subscription: %v", err), http.StatusInternalServerError)
		return
	}

	// Return 204 No Content on successful deletion
	w.WriteHeader(http.StatusNoContent)
}

// Admin endpoints

// AdminListClients handles GET /api/v1/admin/clients
func (h *Handlers) AdminListClients(w http.ResponseWriter, r *http.Request) {
	// Get authenticated client from context and verify admin privileges
	claims := GetClaims(r)
	if claims == nil {
		h.writeError(w, "Authentication required", http.StatusUnauthorized)
		return
	}

	if !claims.IsAdmin {
		h.writeError(w, "Admin privileges required", http.StatusForbidden)
		return
	}

	ctx := r.Context()

	// Get all connected clients from mesh node
	clients, err := h.meshNode.GetConnectedClients(ctx)
	if err != nil {
		h.writeError(w, fmt.Sprintf("Failed to get connected clients: %v", err), http.StatusInternalServerError)
		return
	}

	var clientInfos []ClientInfo
	for _, client := range clients {
		// Get subscriptions for this client
		subscriptions, err := h.meshNode.GetClientSubscriptions(ctx, client.ID())
		if err != nil {
			// Log error but continue with other clients
			// In production, we'd use proper logging here
			subscriptions = []meshnodepkg.ClientSubscription{}
		}

		// Extract subscription topics
		var topics []string
		for _, sub := range subscriptions {
			topics = append(topics, sub.Topic)
		}

		clientInfo := ClientInfo{
			ID:            client.ID(),
			Authenticated: client.IsAuthenticated(),
			ConnectedAt:   client.ConnectedAt(),
			Subscriptions: topics,
		}
		clientInfos = append(clientInfos, clientInfo)
	}

	response := AdminClientsResponse{
		Clients: clientInfos,
	}

	h.writeJSON(w, response, http.StatusOK)
}

// AdminListSubscriptions handles GET /api/v1/admin/subscriptions
func (h *Handlers) AdminListSubscriptions(w http.ResponseWriter, r *http.Request) {
	// Get authenticated client from context and verify admin privileges
	claims := GetClaims(r)
	if claims == nil {
		h.writeError(w, "Authentication required", http.StatusUnauthorized)
		return
	}

	if !claims.IsAdmin {
		h.writeError(w, "Admin privileges required", http.StatusForbidden)
		return
	}

	ctx := r.Context()

	// Get all connected clients from the MeshNode
	clients, err := h.meshNode.GetConnectedClients(ctx)
	if err != nil {
		h.writeError(w, fmt.Sprintf("Failed to get connected clients: %v", err), http.StatusInternalServerError)
		return
	}

	// Get all client subscriptions with actual creation times
	var adminSubscriptions []AdminSubscriptionInfo
	for _, client := range clients {
		// Get subscriptions for this client
		subscriptions, err := h.meshNode.GetClientSubscriptions(ctx, client.ID())
		if err != nil {
			// Log error but continue with other clients
			continue
		}

		// Convert to admin format using actual creation times
		for _, subscription := range subscriptions {
			adminSub := AdminSubscriptionInfo{
				ID:        subscription.ID,
				Topic:     subscription.Topic,
				ClientID:  subscription.ClientID,
				CreatedAt: subscription.CreatedAt,
			}
			adminSubscriptions = append(adminSubscriptions, adminSub)
		}
	}

	response := AdminSubscriptionsResponse{
		Subscriptions: adminSubscriptions,
	}

	h.writeJSON(w, response, http.StatusOK)
}

// AdminGetStats handles GET /api/v1/admin/stats
func (h *Handlers) AdminGetStats(w http.ResponseWriter, r *http.Request) {
	// Get authenticated client from context and verify admin privileges
	claims := GetClaims(r)
	if claims == nil {
		h.writeError(w, "Authentication required", http.StatusUnauthorized)
		return
	}

	if !claims.IsAdmin {
		h.writeError(w, "Admin privileges required", http.StatusForbidden)
		return
	}

	ctx := r.Context()

	// Get health status for connected clients count
	health, err := h.meshNode.GetHealth(ctx)
	if err != nil {
		h.writeError(w, fmt.Sprintf("Failed to get health status: %v", err), http.StatusInternalServerError)
		return
	}

	// Get routing table for additional statistics
	routingTable := h.meshNode.GetRoutingTable()

	// Get total number of topics
	topicCount, err := routingTable.GetTopicCount(ctx)
	if err != nil {
		h.writeError(w, fmt.Sprintf("Failed to get topic count: %v", err), http.StatusInternalServerError)
		return
	}

	// Get total number of subscriptions
	subscriberCount, err := routingTable.GetSubscriberCount(ctx)
	if err != nil {
		h.writeError(w, fmt.Sprintf("Failed to get subscriber count: %v", err), http.StatusInternalServerError)
		return
	}

	// Get EventLog for statistics
	eventLog := h.meshNode.GetEventLog()

	// Get event statistics from EventLog
	eventStats, err := eventLog.GetStatistics(ctx)
	if err != nil {
		h.writeError(w, fmt.Sprintf("Failed to get event statistics: %v", err), http.StatusInternalServerError)
		return
	}

	response := AdminStatsResponse{
		ConnectedClients:   health.ConnectedClients,
		TotalSubscriptions: subscriberCount,
		TotalTopics:        topicCount,
		EventsPublished:    int(eventStats.TotalEvents),
	}

	h.writeJSON(w, response, http.StatusOK)
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
		// Can't use http.Error here as headers are already written
		// Just log the error since we've already committed to the response
		slog.Error("failed to encode JSON response", "error", err)
	}
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
		if (char < 'a' || char > 'z') && (char < 'A' || char > 'Z') &&
			(char < '0' || char > '9') && char != '.' && char != '-' && char != '_' {
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
	_, err = fmt.Fprintf(w, "data: %s\n\n", string(jsonData))
	return err
}

// streamWithKeepalive maintains the SSE connection with periodic keepalive messages and event delivery
func (h *Handlers) streamWithKeepalive(w http.ResponseWriter, r *http.Request, client *HTTPClient) {
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
			// TODO: Cleanup will be handled by MeshNode subscription system
			return

		case <-ticker.C:
			// Send keepalive/ping message
			_, err := w.Write([]byte(": ping\n\n"))
			if err != nil {
				// Client disconnected
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
			if event.Payload != nil {
				// Try to unmarshal payload as JSON
				err := json.Unmarshal(event.Payload, &payload)
				if err != nil {
					// If not valid JSON, send as string
					payload = string(event.Payload)
				}
			}

			sseMessage := EventStreamMessage{
				EventID:   fmt.Sprintf("%s-%d", event.Topic, event.Offset),
				Topic:     event.Topic,
				Payload:   payload,
				Timestamp: event.Timestamp,
				Offset:    event.Offset,
			}

			// Send the event via SSE
			if err := h.writeSSEMessage(w, sseMessage); err != nil {
				// Client disconnected
				return
			}

			// Flush the event message
			if flusher, ok := w.(http.Flusher); ok {
				flusher.Flush()
			}
		}
	}
}
