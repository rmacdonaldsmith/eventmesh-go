package httpapi

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/rmacdonaldsmith/eventmesh-go/internal/meshnode"
	eventlogpkg "github.com/rmacdonaldsmith/eventmesh-go/pkg/eventlog"
)

// HTTPClient represents an HTTP API client for the MeshNode
type HTTPClient struct {
	clientID        string
	isAuthenticated bool
}

// NewHTTPClient creates a new HTTP client from JWT claims
func NewHTTPClient(claims *JWTClaims) *HTTPClient {
	return &HTTPClient{
		clientID:        claims.ClientID,
		isAuthenticated: true,
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

// Subscription endpoints

// ListSubscriptions handles GET /api/v1/subscriptions
func (h *Handlers) ListSubscriptions(w http.ResponseWriter, r *http.Request) {
	// TODO: Implement in Task 104
	h.writeError(w, "Not implemented yet - will be implemented in Task 104", http.StatusNotImplemented)
}

// CreateSubscription handles POST /api/v1/subscriptions
func (h *Handlers) CreateSubscription(w http.ResponseWriter, r *http.Request) {
	// TODO: Implement in Task 104
	h.writeError(w, "Not implemented yet - will be implemented in Task 104", http.StatusNotImplemented)
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
