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

		// Use our shared streaming recorder
		recorder := NewStreamingRecorder()

		// Run StreamEvents in a goroutine since it should stream
		go func() {
			defer recorder.Close()
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

	t.Run("end_to_end_event_delivery", func(t *testing.T) {
		// Create a valid JWT token
		token, _, err := auth.GenerateToken("test-sse-client", false)
		if err != nil {
			t.Fatalf("Failed to generate token: %v", err)
		}

		// Start SSE client subscribing to "test.events" topic
		sseReq, err := http.NewRequest("GET", "/api/v1/events/stream?topic=test.events", nil)
		if err != nil {
			t.Fatal(err)
		}
		sseReq.Header.Set("Authorization", "Bearer "+token)
		sseReq.Header.Set("Accept", "text/event-stream")

		// Add claims to context for SSE request
		claims, err := auth.ValidateToken(token)
		if err != nil {
			t.Fatalf("Failed to validate token: %v", err)
		}

		// Create a context that we can cancel
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		ctx = context.WithValue(ctx, ClaimsKey, claims)
		sseReq = sseReq.WithContext(ctx)

		// Use our shared streaming recorder
		recorder := NewStreamingRecorder()

		// Start SSE streaming in a goroutine
		go func() {
			defer recorder.Close()
			handlers.StreamEvents(recorder, sseReq)
		}()

		// Wait for SSE connection to be established
		select {
		case data := <-recorder.Data:
			if !strings.Contains(data, "SSE connection established for topic: test.events") {
				t.Errorf("Expected SSE connection message, got: %s", data)
			}
			t.Logf("✓ SSE connection established: %s", strings.TrimSpace(data))
		case <-time.After(1 * time.Second):
			t.Error("Timeout waiting for SSE connection message")
			return
		}

		// Now publish an event to the same topic via the HTTP API
		publishReq := PublishRequest{
			Topic:   "test.events",
			Payload: map[string]interface{}{"message": "Hello SSE World", "timestamp": "2025-01-01T00:00:00Z"},
		}

		publishReqBody, err := json.Marshal(publishReq)
		if err != nil {
			t.Fatal(err)
		}

		pubReq, err := http.NewRequest("POST", "/api/v1/events", strings.NewReader(string(publishReqBody)))
		if err != nil {
			t.Fatal(err)
		}
		pubReq.Header.Set("Content-Type", "application/json")
		pubReq.Header.Set("Authorization", "Bearer "+token)

		// Add claims to context for publish request
		pubCtx := context.WithValue(context.Background(), ClaimsKey, claims)
		pubReq = pubReq.WithContext(pubCtx)

		// Execute publish request
		pubRR := httptest.NewRecorder()
		handlers.PublishEvent(pubRR, pubReq)

		// Verify publish succeeded
		if status := pubRR.Code; status != http.StatusCreated {
			t.Errorf("Expected publish status %d, got %d. Body: %s", http.StatusCreated, status, pubRR.Body.String())
		}

		// Now the SSE client should receive the published event
		eventReceived := false
		timeout := time.After(3 * time.Second)

		for !eventReceived {
			select {
			case data := <-recorder.Data:
				// Look for the published event in SSE format
				if strings.Contains(data, "data: {") && strings.Contains(data, "Hello SSE World") {
					// Parse the event to verify it's properly formatted
					if strings.Contains(data, "test.events") && strings.Contains(data, "eventId") {
						eventReceived = true
						t.Logf("✅ End-to-end event delivery working: %s", data)
					}
				}
			case <-timeout:
				t.Error("Timeout waiting for published event to be delivered via SSE - integration not working")
				return
			case <-recorder.Done:
				if !eventReceived {
					t.Error("SSE stream ended without receiving the published event")
				}
				return
			}
		}

		// Cancel context to end the stream
		cancel()

		// Verify we got the event
		if !eventReceived {
			t.Error("Published event was not delivered to SSE client")
		}
	})
}