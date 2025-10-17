package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
)

// ContextKey type for context keys to avoid collisions
type ContextKey string

const (
	// ClientIDKey is the context key for the authenticated client ID
	ClientIDKey ContextKey = "client_id"
	// IsAdminKey is the context key for admin status
	IsAdminKey ContextKey = "is_admin"
	// ClaimsKey is the context key for JWT claims
	ClaimsKey ContextKey = "jwt_claims"
	// SubscriptionIDKey is the context key for subscription ID from URL path
	SubscriptionIDKey ContextKey = "subscription_id"
	// TopicKey is the context key for topic name from URL path
	TopicKey ContextKey = "topic"
)

// Middleware provides HTTP middleware functions
type Middleware struct {
	jwtAuth *JWTAuth
	noAuth  bool // Development mode: bypass authentication
}

// NewMiddleware creates a new middleware instance
func NewMiddleware(jwtAuth *JWTAuth, noAuth bool) *Middleware {
	return &Middleware{
		jwtAuth: jwtAuth,
		noAuth:  noAuth,
	}
}

// AuthRequired middleware requires valid JWT authentication
func (m *Middleware) AuthRequired(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Development mode: bypass authentication
		if m.noAuth {
			// Set development context with default values
			ctx := context.WithValue(r.Context(), ClientIDKey, "dev-client")
			ctx = context.WithValue(ctx, IsAdminKey, false)
			ctx = context.WithValue(ctx, ClaimsKey, &JWTClaims{
				ClientID: "dev-client",
				IsAdmin:  false,
			})

			next(w, r.WithContext(ctx))
			return
		}

		// Normal authentication flow
		token := m.extractToken(r)
		if token == "" {
			m.writeError(w, "Authorization header required", http.StatusUnauthorized)
			return
		}

		claims, err := m.jwtAuth.ValidateToken(token)
		if err != nil {
			m.writeError(w, "Invalid token: "+err.Error(), http.StatusUnauthorized)
			return
		}

		// Add claims to request context
		ctx := context.WithValue(r.Context(), ClientIDKey, claims.ClientID)
		ctx = context.WithValue(ctx, IsAdminKey, claims.IsAdmin)
		ctx = context.WithValue(ctx, ClaimsKey, claims)

		next(w, r.WithContext(ctx))
	}
}

// AdminRequired middleware requires admin privileges
// Note: Admin endpoints are NEVER bypassed, even in no-auth mode
func (m *Middleware) AdminRequired(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Admin endpoints always require proper JWT authentication
		token := m.extractToken(r)
		if token == "" {
			m.writeError(w, "Authorization header required for admin access", http.StatusUnauthorized)
			return
		}

		claims, err := m.jwtAuth.ValidateToken(token)
		if err != nil {
			m.writeError(w, "Invalid token for admin access: "+err.Error(), http.StatusUnauthorized)
			return
		}

		if !claims.IsAdmin {
			m.writeError(w, "Admin privileges required", http.StatusForbidden)
			return
		}

		// Add claims to request context
		ctx := context.WithValue(r.Context(), ClientIDKey, claims.ClientID)
		ctx = context.WithValue(ctx, IsAdminKey, claims.IsAdmin)
		ctx = context.WithValue(ctx, ClaimsKey, claims)

		next(w, r.WithContext(ctx))
	}
}

// CORS middleware adds CORS headers for browser compatibility
func (m *Middleware) CORS(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Set CORS headers
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		w.Header().Set("Access-Control-Max-Age", "86400")

		// Handle preflight requests
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next(w, r)
	}
}

// ContentType middleware sets the content type to JSON
func (m *Middleware) ContentType(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		next(w, r)
	}
}

// Logging middleware logs HTTP requests
func (m *Middleware) Logging(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Simple request logging - in production, use structured logging
		// TODO: Replace with structured logging in production
		next(w, r)
	}
}

// Recovery middleware recovers from panics and returns 500 error
func (m *Middleware) Recovery(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				m.writeError(w, "Internal server error", http.StatusInternalServerError)
			}
		}()

		next(w, r)
	}
}

// Helper functions

// extractToken extracts the JWT token from the Authorization header
func (m *Middleware) extractToken(r *http.Request) string {
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		return ""
	}

	// Support both "Bearer token" and "token" formats
	if strings.HasPrefix(authHeader, "Bearer ") {
		return strings.TrimPrefix(authHeader, "Bearer ")
	}

	return authHeader
}

// writeError writes an error response as JSON
func (m *Middleware) writeError(w http.ResponseWriter, message string, statusCode int) {
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

// GetClientID extracts the client ID from the request context
func GetClientID(r *http.Request) string {
	if clientID, ok := r.Context().Value(ClientIDKey).(string); ok {
		return clientID
	}
	return ""
}

// IsAdmin checks if the current request is from an admin user
func IsAdmin(r *http.Request) bool {
	if isAdmin, ok := r.Context().Value(IsAdminKey).(bool); ok {
		return isAdmin
	}
	return false
}

// GetClaims extracts the JWT claims from the request context
func GetClaims(r *http.Request) *JWTClaims {
	if claims, ok := r.Context().Value(ClaimsKey).(*JWTClaims); ok {
		return claims
	}
	return nil
}

// GetSubscriptionID extracts the subscription ID from the request context
func GetSubscriptionID(r *http.Request) string {
	if subscriptionID, ok := r.Context().Value(SubscriptionIDKey).(string); ok {
		return subscriptionID
	}
	return ""
}

// GetTopicFromPath extracts the topic name from the request context
func GetTopicFromPath(r *http.Request) string {
	if topic, ok := r.Context().Value(TopicKey).(string); ok {
		return topic
	}
	return ""
}
