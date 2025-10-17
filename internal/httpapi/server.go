package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/rmacdonaldsmith/eventmesh-go/internal/meshnode"
)

// Server represents the HTTP API server
type Server struct {
	meshNode   *meshnode.GRPCMeshNode
	jwtAuth    *JWTAuth
	handlers   *Handlers
	middleware *Middleware
	server     *http.Server
	secretKey  string
}

// Config holds server configuration
type Config struct {
	Port      string
	SecretKey string
}

// NewServer creates a new HTTP API server
func NewServer(meshNode *meshnode.GRPCMeshNode, config Config) *Server {
	// Use default secret key if not provided (for MVP)
	secretKey := config.SecretKey
	if secretKey == "" {
		secretKey = "eventmesh-mvp-secret-key-change-in-production"
	}

	jwtAuth := NewJWTAuth(secretKey)
	handlers := NewHandlers(meshNode, jwtAuth)
	middleware := NewMiddleware(jwtAuth)

	server := &Server{
		meshNode:   meshNode,
		jwtAuth:    jwtAuth,
		handlers:   handlers,
		middleware: middleware,
		secretKey:  secretKey,
	}

	// Setup HTTP server
	mux := server.setupRoutes()
	httpServer := &http.Server{
		Addr:           ":" + config.Port,
		Handler:        mux,
		ReadTimeout:    30 * time.Second,
		WriteTimeout:   30 * time.Second,
		IdleTimeout:    120 * time.Second,
		MaxHeaderBytes: 1 << 20, // 1MB
	}

	server.server = httpServer
	return server
}

// Start starts the HTTP server
func (s *Server) Start() error {
	return s.server.ListenAndServe()
}

// Stop gracefully stops the HTTP server
func (s *Server) Stop(ctx context.Context) error {
	return s.server.Shutdown(ctx)
}

// setupRoutes configures all HTTP routes
func (s *Server) setupRoutes() http.Handler {
	mux := http.NewServeMux()

	// Apply global middleware
	withMiddleware := func(handler http.HandlerFunc) http.Handler {
		return s.middleware.Recovery(
			s.middleware.Logging(
				s.middleware.CORS(
					s.middleware.ContentType(handler))))
	}

	// Authentication endpoints (no auth required)
	mux.Handle("/api/v1/auth/login", withMiddleware(s.handlers.Login))

	// Event endpoints (auth required)
	mux.Handle("/api/v1/events", withMiddleware(s.middleware.AuthRequired(s.handleEvents)))
	mux.Handle("/api/v1/events/stream", withMiddleware(s.middleware.AuthRequired(s.handlers.StreamEvents)))

	// Subscription endpoints (auth required)
	mux.Handle("/api/v1/subscriptions", withMiddleware(s.middleware.AuthRequired(s.handleSubscriptions)))
	mux.Handle("/api/v1/subscriptions/", withMiddleware(s.middleware.AuthRequired(s.handleSubscriptionByID)))

	// Topic endpoints (auth required)
	mux.Handle("/api/v1/topics/", withMiddleware(s.middleware.AuthRequired(s.handleTopicEvents)))

	// Admin endpoints (admin auth required)
	mux.Handle("/api/v1/admin/clients", withMiddleware(s.middleware.AdminRequired(s.handlers.AdminListClients)))
	mux.Handle("/api/v1/admin/subscriptions", withMiddleware(s.middleware.AdminRequired(s.handlers.AdminListSubscriptions)))
	mux.Handle("/api/v1/admin/stats", withMiddleware(s.middleware.AdminRequired(s.handlers.AdminGetStats)))

	// Health endpoint (no auth required)
	mux.Handle("/api/v1/health", withMiddleware(s.handlers.Health))

	// Root endpoint with API info
	mux.Handle("/", withMiddleware(s.handleRoot))

	return mux
}

// Route handlers that dispatch based on HTTP method

// handleEvents routes event-related requests based on HTTP method
func (s *Server) handleEvents(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		s.handlers.PublishEvent(w, r)
	case http.MethodGet:
		// For now, redirect to stream endpoint
		http.Redirect(w, r, "/api/v1/events/stream", http.StatusTemporaryRedirect)
	default:
		s.writeError(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleSubscriptions routes subscription requests based on HTTP method
func (s *Server) handleSubscriptions(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		s.handlers.ListSubscriptions(w, r)
	case http.MethodPost:
		s.handlers.CreateSubscription(w, r)
	default:
		s.writeError(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleSubscriptionByID handles individual subscription operations
func (s *Server) handleSubscriptionByID(w http.ResponseWriter, r *http.Request) {
	// Extract subscription ID from path
	path := r.URL.Path
	if !strings.HasPrefix(path, "/api/v1/subscriptions/") {
		s.writeError(w, "Invalid subscription path", http.StatusNotFound)
		return
	}

	subscriptionID := strings.TrimPrefix(path, "/api/v1/subscriptions/")
	if subscriptionID == "" {
		s.writeError(w, "Subscription ID required", http.StatusBadRequest)
		return
	}

	switch r.Method {
	case http.MethodDelete:
		// Add subscription ID to request context
		ctx := context.WithValue(r.Context(), SubscriptionIDKey, subscriptionID)
		requestWithID := r.WithContext(ctx)
		s.handlers.DeleteSubscription(w, requestWithID)
	default:
		s.writeError(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleTopicEvents handles topic-related operations
func (s *Server) handleTopicEvents(w http.ResponseWriter, r *http.Request) {
	// Parse topic from URL path: /api/v1/topics/{topic}/events
	path := r.URL.Path
	if !strings.HasPrefix(path, "/api/v1/topics/") {
		s.writeError(w, "Invalid topic path", http.StatusNotFound)
		return
	}

	// Extract topic and ensure it ends with /events
	pathAfterTopics := strings.TrimPrefix(path, "/api/v1/topics/")
	if pathAfterTopics == "" {
		s.writeError(w, "Topic name required", http.StatusBadRequest)
		return
	}

	if !strings.HasSuffix(pathAfterTopics, "/events") {
		s.writeError(w, "Invalid path, expected /events", http.StatusNotFound)
		return
	}

	topic := strings.TrimSuffix(pathAfterTopics, "/events")
	if topic == "" {
		s.writeError(w, "Topic name required", http.StatusBadRequest)
		return
	}

	switch r.Method {
	case http.MethodGet:
		// Add topic to request context
		ctx := context.WithValue(r.Context(), TopicKey, topic)
		requestWithTopic := r.WithContext(ctx)
		s.handlers.ReadTopicEvents(w, requestWithTopic)
	default:
		s.writeError(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleRoot provides API information
func (s *Server) handleRoot(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		s.writeError(w, "Not found", http.StatusNotFound)
		return
	}

	info := map[string]interface{}{
		"service":     "EventMesh HTTP API",
		"version":     "1.0.0",
		"description": "RESTful HTTP API for EventMesh distributed event streaming",
		"endpoints": map[string]interface{}{
			"auth": map[string]string{
				"login": "POST /api/v1/auth/login",
			},
			"events": map[string]string{
				"publish": "POST /api/v1/events",
				"stream":  "GET /api/v1/events/stream",
			},
			"subscriptions": map[string]string{
				"list":   "GET /api/v1/subscriptions",
				"create": "POST /api/v1/subscriptions",
				"delete": "DELETE /api/v1/subscriptions/{id}",
			},
			"topics": map[string]string{
				"readEvents": "GET /api/v1/topics/{topic}/events?offset={offset}&limit={limit}",
			},
			"admin": map[string]string{
				"clients":       "GET /api/v1/admin/clients",
				"subscriptions": "GET /api/v1/admin/subscriptions",
				"stats":         "GET /api/v1/admin/stats",
			},
			"health": "GET /api/v1/health",
		},
		"authentication": "Bearer JWT token required for most endpoints",
	}

	s.writeJSON(w, info, http.StatusOK)
}

// Helper methods

// writeError writes an error response as JSON
func (s *Server) writeError(w http.ResponseWriter, message string, statusCode int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	errorResp := ErrorResponse{
		Error:   http.StatusText(statusCode),
		Message: message,
		Code:    statusCode,
	}

	s.writeJSON(w, errorResp, statusCode)
}

// writeJSON writes a JSON response
func (s *Server) writeJSON(w http.ResponseWriter, data interface{}, statusCode int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		// Fallback error handling
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}
