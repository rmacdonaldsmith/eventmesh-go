package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestNewServer tests that we can create a new server instance
func TestNewServer(t *testing.T) {
	// Use our shared test setup
	setup := NewTestServerSetup(t)
	defer setup.Close()

	server := setup.Server

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

// TestPackageStructure verifies all files compile together
func TestPackageStructure(t *testing.T) {
	// This test just verifies that all our files compile together properly
	// by importing and using the main types

	// Test that we can create all the main components
	auth := NewJWTAuth("test-key")
	if auth == nil {
		t.Error("Expected NewJWTAuth to return non-nil")
	}

	middleware := NewMiddleware(auth, false)
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

	authReq := AuthRequest{ClientID: "test"}
	authResp := AuthResponse{ClientID: "test"}
	pubReq := PublishRequest{Topic: "test.topic"}
	pubResp := PublishResponse{EventID: "test-event-123"}
	if authReq.ClientID != authResp.ClientID {
		t.Errorf("Expected auth request/response client IDs to match")
	}
	if pubReq.Topic == "" || pubResp.EventID == "" {
		t.Errorf("Expected publish request/response fields to be assignable")
	}
}

// TestErrorHelpers tests centralized error handling
func TestErrorHelpers(t *testing.T) {
	// Use our shared test setup
	setup := NewTestServerSetup(t)
	defer setup.Close()

	handlers := setup.Server.handlers

	t.Run("handlers_writeError", func(t *testing.T) {
		rr := httptest.NewRecorder()
		handlers.writeError(rr, "custom validation failed", http.StatusBadRequest)

		if rr.Code != http.StatusBadRequest {
			t.Fatalf("Expected status %d, got %d", http.StatusBadRequest, rr.Code)
		}
		if contentType := rr.Header().Get("Content-Type"); contentType != "application/json" {
			t.Fatalf("Expected JSON content type, got %q", contentType)
		}

		var response ErrorResponse
		if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
			t.Fatalf("Failed to parse error response: %v", err)
		}
		if response.Error != http.StatusText(http.StatusBadRequest) {
			t.Errorf("Expected error %q, got %q", http.StatusText(http.StatusBadRequest), response.Error)
		}
		if response.Message != "custom validation failed" {
			t.Errorf("Expected custom message, got %q", response.Message)
		}
		if response.Code != http.StatusBadRequest {
			t.Errorf("Expected code %d, got %d", http.StatusBadRequest, response.Code)
		}
	})
}
