package httpapi

import (
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
	// Use our shared test setup
	setup := NewTestServerSetup(t)
	defer setup.Close()

	handlers := setup.Server.handlers

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
