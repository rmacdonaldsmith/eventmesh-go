package httpapi

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/rmacdonaldsmith/eventmesh-go/internal/meshnode"
)

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
	// TODO: Implement in Task 102
	h.writeError(w, "Not implemented yet - will be implemented in Task 102", http.StatusNotImplemented)
}

// StreamEvents handles GET /api/v1/events/stream
func (h *Handlers) StreamEvents(w http.ResponseWriter, r *http.Request) {
	// TODO: Implement in Task 103
	h.writeError(w, "Not implemented yet - will be implemented in Task 103", http.StatusNotImplemented)
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
