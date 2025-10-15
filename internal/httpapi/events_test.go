package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestInputValidation tests request input validation
func TestInputValidation(t *testing.T) {
	// Use our shared test setup
	setup := NewTestServerSetup(t)
	defer setup.Close()

	handlers := setup.Server.handlers

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
	// Use our shared test setup
	setup := NewTestServerSetup(t)
	defer setup.Close()

	handlers := setup.Server.handlers
	auth := setup.Auth

	t.Run("successful_event_publish", func(t *testing.T) {
		// Generate test token
		token := setup.GenerateTestToken(t, "test-publisher", false)

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
		// Generate test token
		token := setup.GenerateTestToken(t, "test-publisher", false)

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
		token := setup.GenerateTestToken(t, "test-publisher", false)

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
		// Generate test token
		token := setup.GenerateTestToken(t, "test-publisher", false)

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