package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/rmacdonaldsmith/eventmesh-go/internal/meshnode"
)

// TestNewServer tests that we can create a new server instance
func TestNewServer(t *testing.T) {
	// Create a test mesh node config
	config := meshnode.NewConfig("test-node", "localhost:8080")

	// Create mesh node
	node, err := meshnode.NewGRPCMeshNode(config)
	if err != nil {
		t.Fatalf("Failed to create mesh node: %v", err)
	}
	defer node.Close()

	// Create server config
	serverConfig := Config{
		Port:      "8081",
		SecretKey: "test-secret-key",
	}

	// Create HTTP API server
	server := NewServer(node, serverConfig)
	if server == nil {
		t.Fatal("Expected server to be created, got nil")
	}

	// Verify server components are initialized
	if server.meshNode == nil {
		t.Error("Expected meshNode to be initialized")
	}
	if server.jwtAuth == nil {
		t.Error("Expected jwtAuth to be initialized")
	}
	if server.handlers == nil {
		t.Error("Expected handlers to be initialized")
	}
	if server.middleware == nil {
		t.Error("Expected middleware to be initialized")
	}
	if server.server == nil {
		t.Error("Expected HTTP server to be initialized")
	}
}

// TestJWTAuth tests basic JWT authentication functionality
func TestJWTAuth(t *testing.T) {
	auth := NewJWTAuth("test-secret")

	// Test token generation
	token, expiresAt, err := auth.GenerateToken("test-client", false)
	if err != nil {
		t.Errorf("Expected no error generating token, got %v", err)
	}
	if token == "" {
		t.Error("Expected non-empty token")
	}
	if expiresAt.IsZero() {
		t.Error("Expected valid expiration time")
	}

	// Test token validation
	claims, err := auth.ValidateToken(token)
	if err != nil {
		t.Errorf("Expected no error validating token, got %v", err)
	}
	if claims == nil {
		t.Fatal("Expected claims to be returned")
	}
	if claims.ClientID != "test-client" {
		t.Errorf("Expected ClientID 'test-client', got '%s'", claims.ClientID)
	}
	if claims.IsAdmin {
		t.Error("Expected IsAdmin to be false")
	}

	// Test invalid token
	_, err = auth.ValidateToken("invalid-token")
	if err == nil {
		t.Error("Expected error for invalid token")
	}
}

// TestJWTLibraryMigration tests that the new library-based JWT auth maintains identical behavior
func TestJWTLibraryMigration(t *testing.T) {
	auth := NewJWTAuth("migration-test-secret")

	t.Run("admin_token_generation", func(t *testing.T) {
		// Test admin token generation
		token, expiresAt, err := auth.GenerateToken("admin-client", true)
		if err != nil {
			t.Errorf("Expected no error generating admin token, got %v", err)
		}
		if token == "" {
			t.Error("Expected non-empty admin token")
		}
		if expiresAt.IsZero() {
			t.Error("Expected valid admin expiration time")
		}

		// Validate admin token
		claims, err := auth.ValidateToken(token)
		if err != nil {
			t.Errorf("Expected no error validating admin token, got %v", err)
		}
		if claims == nil {
			t.Fatal("Expected admin claims to be returned")
		}
		if claims.ClientID != "admin-client" {
			t.Errorf("Expected admin ClientID 'admin-client', got '%s'", claims.ClientID)
		}
		if !claims.IsAdmin {
			t.Error("Expected IsAdmin to be true for admin token")
		}
	})

	t.Run("token_expiration_fields", func(t *testing.T) {
		// Test that expiration fields are properly set
		_, expiresAt, err := auth.GenerateToken("expiry-test", false)
		if err != nil {
			t.Errorf("Expected no error generating expiry test token, got %v", err)
		}

		// Should expire in approximately 24 hours
		expectedExpiry := time.Now().Add(24 * time.Hour)
		timeDiff := expiresAt.Sub(expectedExpiry).Abs()
		if timeDiff > time.Minute {
			t.Errorf("Token expiration time off by more than 1 minute: %v", timeDiff)
		}
	})

	t.Run("bearer_token_handling", func(t *testing.T) {
		// Test Bearer prefix handling
		token, _, err := auth.GenerateToken("bearer-test", false)
		if err != nil {
			t.Errorf("Expected no error generating bearer test token, got %v", err)
		}

		// Should work with Bearer prefix
		bearerToken := "Bearer " + token
		claims, err := auth.ValidateToken(bearerToken)
		if err != nil {
			t.Errorf("Expected no error validating bearer token, got %v", err)
		}
		if claims == nil || claims.ClientID != "bearer-test" {
			t.Error("Bearer token validation failed")
		}
	})
}

// TestInputValidation tests request input validation
func TestInputValidation(t *testing.T) {
	// Create a test mesh node config
	config := meshnode.NewConfig("test-node", "localhost:8080")

	// Create mesh node
	node, err := meshnode.NewGRPCMeshNode(config)
	if err != nil {
		t.Fatalf("Failed to create mesh node: %v", err)
	}
	defer node.Close()

	// Create handlers
	auth := NewJWTAuth("validation-test-secret")
	handlers := NewHandlers(node, auth)

	t.Run("validateJSON_missing_content_type", func(t *testing.T) {
		// This test should fail initially - we need to implement validateJSON usage
		// Test that requests without proper content-type are rejected
		if handlers == nil {
			t.Error("Expected handlers to be created")
		}
		// TODO: Add actual HTTP request validation testing
		// This is a placeholder that will be filled when we implement validation
	})

	t.Run("empty_topic_validation", func(t *testing.T) {
		// Test that empty topics are rejected
		// This should fail until we add topic validation
		if handlers == nil {
			t.Error("Expected handlers to be created")
		}
		// TODO: Add topic validation testing
	})

	t.Run("empty_client_id_validation", func(t *testing.T) {
		// Test that empty client IDs in auth requests are rejected
		// This should already work based on existing Login handler logic
		if handlers == nil {
			t.Error("Expected handlers to be created")
		}
		// TODO: Add client ID validation testing
	})
}

// TestPublishEvent tests the POST /api/v1/events endpoint
func TestPublishEvent(t *testing.T) {
	// Create a test mesh node config
	config := meshnode.NewConfig("test-node", "localhost:8080")

	// Create mesh node
	node, err := meshnode.NewGRPCMeshNode(config)
	if err != nil {
		t.Fatalf("Failed to create mesh node: %v", err)
	}
	defer node.Close()

	// Start the mesh node
	ctx := context.Background()
	if err := node.Start(ctx); err != nil {
		t.Fatalf("Failed to start mesh node: %v", err)
	}

	// Create handlers
	auth := NewJWTAuth("publish-test-secret")
	handlers := NewHandlers(node, auth)

	t.Run("successful_event_publish", func(t *testing.T) {
		// Create a valid JWT token
		token, _, err := auth.GenerateToken("test-publisher", false)
		if err != nil {
			t.Fatalf("Failed to generate token: %v", err)
		}

		// Create test request
		requestBody := `{"topic": "test.events", "payload": {"message": "Hello EventMesh", "timestamp": "2025-01-01T00:00:00Z"}}`
		req, err := http.NewRequest("POST", "/api/v1/events", strings.NewReader(requestBody))
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+token)

		// Add claims to context (simulating middleware)
		claims, err := auth.ValidateToken(token)
		if err != nil {
			t.Fatalf("Failed to validate token: %v", err)
		}
		ctx := context.WithValue(req.Context(), ClaimsKey, claims)
		req = req.WithContext(ctx)

		// Execute request
		rr := httptest.NewRecorder()
		handlers.PublishEvent(rr, req)

		// Verify response
		if status := rr.Code; status != http.StatusCreated {
			t.Errorf("Expected status %d, got %d. Body: %s", http.StatusCreated, status, rr.Body.String())
		}

		// Parse response
		var response PublishResponse
		if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
			t.Errorf("Failed to parse response: %v", err)
		}

		// Verify response fields
		if response.EventID == "" {
			t.Error("Expected EventID to be set")
		}
		// EventID should be topic-offset format
		if !strings.HasPrefix(response.EventID, "test.events-") {
			t.Errorf("Expected EventID to start with 'test.events-', got %s", response.EventID)
		}
		if response.Timestamp.IsZero() {
			t.Error("Expected Timestamp to be set")
		}
	})

	t.Run("missing_authentication", func(t *testing.T) {
		requestBody := `{"topic": "test.events", "payload": {"message": "Hello"}}`
		req, err := http.NewRequest("POST", "/api/v1/events", strings.NewReader(requestBody))
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Content-Type", "application/json")

		rr := httptest.NewRecorder()
		handlers.PublishEvent(rr, req)

		if status := rr.Code; status != http.StatusUnauthorized {
			t.Errorf("Expected status %d, got %d", http.StatusUnauthorized, status)
		}
	})

	t.Run("invalid_topic", func(t *testing.T) {
		// Create a valid JWT token
		token, _, err := auth.GenerateToken("test-publisher", false)
		if err != nil {
			t.Fatalf("Failed to generate token: %v", err)
		}

		// Test empty topic
		requestBody := `{"topic": "", "payload": {"message": "Hello"}}`
		req, err := http.NewRequest("POST", "/api/v1/events", strings.NewReader(requestBody))
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+token)

		claims, _ := auth.ValidateToken(token)
		ctx := context.WithValue(req.Context(), ClaimsKey, claims)
		req = req.WithContext(ctx)

		rr := httptest.NewRecorder()
		handlers.PublishEvent(rr, req)

		if status := rr.Code; status != http.StatusBadRequest {
			t.Errorf("Expected status %d, got %d", http.StatusBadRequest, status)
		}
	})

	t.Run("invalid_content_type", func(t *testing.T) {
		token, _, err := auth.GenerateToken("test-publisher", false)
		if err != nil {
			t.Fatalf("Failed to generate token: %v", err)
		}

		req, err := http.NewRequest("POST", "/api/v1/events", strings.NewReader("invalid"))
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Content-Type", "text/plain") // Wrong content type
		req.Header.Set("Authorization", "Bearer "+token)

		claims, _ := auth.ValidateToken(token)
		ctx := context.WithValue(req.Context(), ClaimsKey, claims)
		req = req.WithContext(ctx)

		rr := httptest.NewRecorder()
		handlers.PublishEvent(rr, req)

		if status := rr.Code; status != http.StatusBadRequest {
			t.Errorf("Expected status %d, got %d", http.StatusBadRequest, status)
		}
	})

	t.Run("nil_payload", func(t *testing.T) {
		// Create a valid JWT token
		token, _, err := auth.GenerateToken("test-publisher", false)
		if err != nil {
			t.Fatalf("Failed to generate token: %v", err)
		}

		// Test nil payload (should be allowed)
		requestBody := `{"topic": "test.events"}`
		req, err := http.NewRequest("POST", "/api/v1/events", strings.NewReader(requestBody))
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+token)

		claims, _ := auth.ValidateToken(token)
		ctx := context.WithValue(req.Context(), ClaimsKey, claims)
		req = req.WithContext(ctx)

		rr := httptest.NewRecorder()
		handlers.PublishEvent(rr, req)

		if status := rr.Code; status != http.StatusCreated {
			t.Errorf("Expected status %d for nil payload, got %d. Body: %s", http.StatusCreated, status, rr.Body.String())
		}
	})
}

// TestPackageStructure verifies all files compile together
func TestPackageStructure(t *testing.T) {
	// This test just verifies that all our files compile together properly
	// by importing and using the main types

	// Test that we can create all the main components
	auth := NewJWTAuth("test-key")
	if auth == nil {
		t.Error("Expected NewJWTAuth to return non-nil")
	}

	middleware := NewMiddleware(auth)
	if middleware == nil {
		t.Error("Expected NewMiddleware to return non-nil")
	}

	// Test HTTPClient methods
	claims := &JWTClaims{ClientID: "test-client", IsAdmin: false}
	client := NewHTTPClient(claims)
	if client.ID() != "test-client" {
		t.Errorf("Expected client ID 'test-client', got %s", client.ID())
	}
	if !client.IsAuthenticated() {
		t.Error("Expected client to be authenticated")
	}

	// Test that our types can be created
	var authReq AuthRequest
	var authResp AuthResponse
	var pubReq PublishRequest
	var pubResp PublishResponse

	// Basic field access test
	authReq.ClientID = "test"
	authResp.ClientID = "test"
	pubReq.Topic = "test.topic"
	pubResp.EventID = "test-event-123"

	// If we reach here, all types are properly defined
	t.Log("✅ Package structure verification complete")
}

// TestErrorHelpers tests centralized error handling
func TestErrorHelpers(t *testing.T) {
	// Create a test mesh node config
	config := meshnode.NewConfig("test-node", "localhost:8080")

	// Create mesh node
	node, err := meshnode.NewGRPCMeshNode(config)
	if err != nil {
		t.Fatalf("Failed to create mesh node: %v", err)
	}
	defer node.Close()

	// Create handlers
	auth := NewJWTAuth("test-secret")
	handlers := NewHandlers(node, auth)

	// Test writeError consistency across handlers and middleware
	// This test will initially fail until we consolidate the error handling
	t.Run("handlers_writeError", func(t *testing.T) {
		// This should not panic and should produce consistent error format
		// We'll verify the error format is standardized
		if handlers == nil {
			t.Error("Expected handlers to be created")
		}
	})
}

// TestStreamEvents tests the GET /api/v1/events/stream SSE endpoint
func TestStreamEvents(t *testing.T) {
	// Create a test mesh node config
	config := meshnode.NewConfig("test-node", "localhost:8080")

	// Create mesh node
	node, err := meshnode.NewGRPCMeshNode(config)
	if err != nil {
		t.Fatalf("Failed to create mesh node: %v", err)
	}
	defer node.Close()

	// Start the mesh node
	ctx := context.Background()
	if err := node.Start(ctx); err != nil {
		t.Fatalf("Failed to start mesh node: %v", err)
	}

	// Create handlers
	auth := NewJWTAuth("sse-test-secret")
	handlers := NewHandlers(node, auth)

	t.Run("basic_sse_connection", func(t *testing.T) {
		// Create a valid JWT token
		token, _, err := auth.GenerateToken("test-sse-client", false)
		if err != nil {
			t.Fatalf("Failed to generate token: %v", err)
		}

		// Create test request
		req, err := http.NewRequest("GET", "/api/v1/events/stream", nil)
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Authorization", "Bearer "+token)

		// Add claims to context (simulating middleware)
		claims, err := auth.ValidateToken(token)
		if err != nil {
			t.Fatalf("Failed to validate token: %v", err)
		}
		ctx := context.WithValue(req.Context(), ClaimsKey, claims)
		req = req.WithContext(ctx)

		// Execute request
		rr := httptest.NewRecorder()
		handlers.StreamEvents(rr, req)

		// Verify SSE response headers
		if contentType := rr.Header().Get("Content-Type"); contentType != "text/event-stream" {
			t.Errorf("Expected Content-Type 'text/event-stream', got '%s'", contentType)
		}
		if cacheControl := rr.Header().Get("Cache-Control"); cacheControl != "no-cache" {
			t.Errorf("Expected Cache-Control 'no-cache', got '%s'", cacheControl)
		}
		if connection := rr.Header().Get("Connection"); connection != "keep-alive" {
			t.Errorf("Expected Connection 'keep-alive', got '%s'", connection)
		}
		if accessControl := rr.Header().Get("Access-Control-Allow-Origin"); accessControl != "*" {
			t.Errorf("Expected Access-Control-Allow-Origin '*', got '%s'", accessControl)
		}

		// Verify response status
		if status := rr.Code; status != http.StatusOK {
			t.Errorf("Expected status %d, got %d. Body: %s", http.StatusOK, status, rr.Body.String())
		}
	})

	t.Run("topic_parameter_parsing", func(t *testing.T) {
		// Create a valid JWT token
		token, _, err := auth.GenerateToken("test-sse-client", false)
		if err != nil {
			t.Fatalf("Failed to generate token: %v", err)
		}

		// Test valid topic parameter
		req, err := http.NewRequest("GET", "/api/v1/events/stream?topic=test.events", nil)
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Authorization", "Bearer "+token)

		// Add claims to context (simulating middleware)
		claims, err := auth.ValidateToken(token)
		if err != nil {
			t.Fatalf("Failed to validate token: %v", err)
		}
		ctx := context.WithValue(req.Context(), ClaimsKey, claims)
		req = req.WithContext(ctx)

		// Execute request
		rr := httptest.NewRecorder()
		handlers.StreamEvents(rr, req)

		// Should succeed with valid topic
		if status := rr.Code; status != http.StatusOK {
			t.Errorf("Expected status %d for valid topic, got %d. Body: %s", http.StatusOK, status, rr.Body.String())
		}

		// Response should include confirmation of topic filter
		body := rr.Body.String()
		if !strings.Contains(body, "test.events") {
			t.Errorf("Expected response to mention topic 'test.events', got: %s", body)
		}
	})

	t.Run("invalid_topic_parameter", func(t *testing.T) {
		// Create a valid JWT token
		token, _, err := auth.GenerateToken("test-sse-client", false)
		if err != nil {
			t.Fatalf("Failed to generate token: %v", err)
		}

		// Test invalid topic parameter (empty)
		req, err := http.NewRequest("GET", "/api/v1/events/stream?topic=", nil)
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Authorization", "Bearer "+token)

		// Add claims to context (simulating middleware)
		claims, err := auth.ValidateToken(token)
		if err != nil {
			t.Fatalf("Failed to validate token: %v", err)
		}
		ctx := context.WithValue(req.Context(), ClaimsKey, claims)
		req = req.WithContext(ctx)

		// Execute request
		rr := httptest.NewRecorder()
		handlers.StreamEvents(rr, req)

		// Should fail with bad request for invalid topic
		if status := rr.Code; status != http.StatusBadRequest {
			t.Errorf("Expected status %d for invalid topic, got %d", http.StatusBadRequest, status)
		}
	})

	t.Run("sse_message_formatting", func(t *testing.T) {
		// Create a valid JWT token
		token, _, err := auth.GenerateToken("test-sse-client", false)
		if err != nil {
			t.Fatalf("Failed to generate token: %v", err)
		}

		// Test SSE message formatting
		req, err := http.NewRequest("GET", "/api/v1/events/stream?topic=test.events", nil)
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Authorization", "Bearer "+token)

		// Add claims to context (simulating middleware)
		claims, err := auth.ValidateToken(token)
		if err != nil {
			t.Fatalf("Failed to validate token: %v", err)
		}
		ctx := context.WithValue(req.Context(), ClaimsKey, claims)
		req = req.WithContext(ctx)

		// Execute request
		rr := httptest.NewRecorder()

		// For this test, we'll simulate sending an event message
		// This will require extending the handler to send a test message
		handlers.StreamEvents(rr, req)

		// Should succeed
		if status := rr.Code; status != http.StatusOK {
			t.Errorf("Expected status %d, got %d", http.StatusOK, status)
		}

		body := rr.Body.String()

		// Look for properly formatted SSE data message
		// SSE format should be: "data: {json}\n\n"
		if !strings.Contains(body, "data: {") {
			t.Errorf("Expected SSE data message with JSON, got: %s", body)
		}

		// Should contain EventStreamMessage structure
		if !strings.Contains(body, "eventId") || !strings.Contains(body, "topic") {
			t.Errorf("Expected EventStreamMessage with eventId and topic fields, got: %s", body)
		}
	})

	t.Run("keepalive_messages", func(t *testing.T) {
		// Create a valid JWT token
		token, _, err := auth.GenerateToken("test-sse-client", false)
		if err != nil {
			t.Fatalf("Failed to generate token: %v", err)
		}

		// Test keepalive functionality with a channel-based approach
		req, err := http.NewRequest("GET", "/api/v1/events/stream", nil)
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Accept", "text/event-stream")

		// Add claims to context (simulating middleware)
		claims, err := auth.ValidateToken(token)
		if err != nil {
			t.Fatalf("Failed to validate token: %v", err)
		}

		// Create a context that we can cancel
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()

		ctx = context.WithValue(ctx, ClaimsKey, claims)
		req = req.WithContext(ctx)

		// Use a custom ResponseWriter that captures streaming data
		recorder := &StreamingRecorder{
			ResponseRecorder: httptest.NewRecorder(),
			Data:             make(chan string, 10),
			Done:             make(chan struct{}),
		}

		// Run StreamEvents in a goroutine since it should stream
		go func() {
			defer close(recorder.Done)
			handlers.StreamEvents(recorder, req)
		}()

		// Wait for initial connection message
		select {
		case data := <-recorder.Data:
			if !strings.Contains(data, "SSE connection established") {
				t.Errorf("Expected connection established message, got: %s", data)
			}
		case <-time.After(1 * time.Second):
			t.Error("Timeout waiting for initial connection message")
			return
		}

		// Should receive at least one keepalive/ping message within 3 seconds
		keepaliveReceived := false
		timeout := time.After(2500 * time.Millisecond)

		for !keepaliveReceived {
			select {
			case data := <-recorder.Data:
				// Look for keepalive/ping messages (SSE comments starting with ":")
				if strings.Contains(data, ": ping") || strings.Contains(data, ": keepalive") {
					keepaliveReceived = true
				}
			case <-timeout:
				t.Error("Timeout waiting for keepalive message - connection should send periodic pings")
				return
			case <-recorder.Done:
				if !keepaliveReceived {
					t.Error("Stream ended without sending keepalive messages")
				}
				return
			}
		}

		// Cancel context to end the stream
		cancel()

		// Wait for stream to end
		select {
		case <-recorder.Done:
			// Stream ended gracefully
		case <-time.After(1 * time.Second):
			t.Error("Stream did not end gracefully after context cancellation")
		}
	})
}

// StreamingRecorder captures streaming data for testing
type StreamingRecorder struct {
	*httptest.ResponseRecorder
	Data chan string
	Done chan struct{}
}

func (r *StreamingRecorder) Write(data []byte) (int, error) {
	// Send data to channel for testing
	select {
	case r.Data <- string(data):
	default:
		// Channel full, skip
	}

	// Also write to the underlying recorder
	return r.ResponseRecorder.Write(data)
}
