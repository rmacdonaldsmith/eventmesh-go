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


	t.Run("sse_message_formatting", func(t *testing.T) {
		// This test verifies that SSE messages are properly formatted when events are delivered
		// It uses the unified model: create subscription → start SSE → publish event → verify format

		// Create a valid JWT token
		token, _, err := auth.GenerateToken("test-sse-client", false)
		if err != nil {
			t.Fatalf("Failed to generate token: %v", err)
		}

		// Add claims to context for all requests
		claims, err := auth.ValidateToken(token)
		if err != nil {
			t.Fatalf("Failed to validate token: %v", err)
		}

		// Step 1: Create subscription for formatting test
		subReq := SubscriptionRequest{Topic: "format.test"}
		subBody, err := json.Marshal(subReq)
		if err != nil {
			t.Fatal(err)
		}

		req, err := http.NewRequest("POST", "/api/v1/subscriptions", strings.NewReader(string(subBody)))
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+token)
		ctx := context.WithValue(context.Background(), ClaimsKey, claims)
		req = req.WithContext(ctx)

		// Execute subscription request
		rr := httptest.NewRecorder()
		handlers.CreateSubscription(rr, req)

		// Verify subscription created
		if status := rr.Code; status != http.StatusCreated {
			t.Fatalf("Failed to create subscription: got status %d", status)
		}

		// Step 2: Start SSE stream
		sseReq, err := http.NewRequest("GET", "/api/v1/events/stream", nil)
		if err != nil {
			t.Fatal(err)
		}
		sseReq.Header.Set("Authorization", "Bearer "+token)
		sseReq.Header.Set("Accept", "text/event-stream")

		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()

		ctx = context.WithValue(ctx, ClaimsKey, claims)
		sseReq = sseReq.WithContext(ctx)

		recorder := NewStreamingRecorder()

		go func() {
			defer recorder.Close()
			handlers.StreamEvents(recorder, sseReq)
		}()

		// Wait for connection
		select {
		case <-recorder.Data:
			// Connection established
		case <-time.After(1 * time.Second):
			t.Error("Timeout waiting for SSE connection")
			return
		}

		// Step 3: Publish event with specific payload for format testing
		publishReq := PublishRequest{
			Topic:   "format.test",
			Payload: map[string]interface{}{"testKey": "testValue", "number": 42},
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

		pubCtx := context.WithValue(context.Background(), ClaimsKey, claims)
		pubReq = pubReq.WithContext(pubCtx)

		pubRR := httptest.NewRecorder()
		handlers.PublishEvent(pubRR, pubReq)

		if status := pubRR.Code; status != http.StatusCreated {
			t.Errorf("Failed to publish event: got status %d", status)
		}

		// Step 4: Verify SSE message formatting
		timeout := time.After(2 * time.Second)
		for {
			select {
			case data := <-recorder.Data:
				// Look for properly formatted SSE data message
				if strings.Contains(data, "data: {") {
					// Verify JSON structure
					if !strings.Contains(data, "eventId") {
						t.Errorf("Expected eventId field in SSE message, got: %s", data)
					}
					if !strings.Contains(data, "format.test") {
						t.Errorf("Expected topic field in SSE message, got: %s", data)
					}
					if !strings.Contains(data, "testKey") {
						t.Errorf("Expected payload content in SSE message, got: %s", data)
					}
					if !strings.HasSuffix(data, "\n\n") {
						t.Errorf("Expected SSE message to end with \\n\\n, got: %s", data)
					}
					// Test passes
					return
				}
			case <-timeout:
				t.Error("Timeout waiting for properly formatted SSE message")
				return
			case <-recorder.Done:
				t.Error("Stream ended without receiving formatted message")
				return
			}
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

		// Add claims to context for all requests
		claims, err := auth.ValidateToken(token)
		if err != nil {
			t.Fatalf("Failed to validate token: %v", err)
		}

		// Step 1: Create subscription for test.events topic (required for unified model)
		subReq := SubscriptionRequest{Topic: "test.events"}
		subBody, err := json.Marshal(subReq)
		if err != nil {
			t.Fatal(err)
		}

		req, err := http.NewRequest("POST", "/api/v1/subscriptions", strings.NewReader(string(subBody)))
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+token)
		ctx := context.WithValue(context.Background(), ClaimsKey, claims)
		req = req.WithContext(ctx)

		// Execute subscription request
		rr := httptest.NewRecorder()
		handlers.CreateSubscription(rr, req)

		// Verify subscription created
		if status := rr.Code; status != http.StatusCreated {
			t.Fatalf("Failed to create subscription: got status %d", status)
		}

		// Step 2: Start SSE stream (no topic parameter in unified model)
		sseReq, err := http.NewRequest("GET", "/api/v1/events/stream", nil)
		if err != nil {
			t.Fatal(err)
		}
		sseReq.Header.Set("Authorization", "Bearer "+token)
		sseReq.Header.Set("Accept", "text/event-stream")

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
			if !strings.Contains(data, "SSE connection established") {
				t.Errorf("Expected SSE connection message, got: %s", data)
			}
			t.Logf("✓ SSE connection established: %s", strings.TrimSpace(data))
		case <-time.After(1 * time.Second):
			t.Error("Timeout waiting for SSE connection message")
			return
		}

		// Step 3: Publish an event to the subscribed topic via the HTTP API
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

	// NEW TEST: Unified SSE behavior (this should FAIL initially)
	t.Run("unified_sse_streams_all_subscriptions", func(t *testing.T) {
		// Create a valid JWT token
		token, _, err := auth.GenerateToken("unified-test-client", false)
		if err != nil {
			t.Fatalf("Failed to generate token: %v", err)
		}

		// Add claims to context for all requests
		claims, err := auth.ValidateToken(token)
		if err != nil {
			t.Fatalf("Failed to validate token: %v", err)
		}

		// Step 1: Create multiple subscriptions for the client
		topics := []string{"orders.created", "users.registered", "payments.processed"}

		for _, topic := range topics {
			// Create subscription request
			subReq := SubscriptionRequest{Topic: topic}
			subBody, err := json.Marshal(subReq)
			if err != nil {
				t.Fatal(err)
			}

			req, err := http.NewRequest("POST", "/api/v1/subscriptions", strings.NewReader(string(subBody)))
			if err != nil {
				t.Fatal(err)
			}
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Authorization", "Bearer "+token)
			ctx := context.WithValue(req.Context(), ClaimsKey, claims)
			req = req.WithContext(ctx)

			// Execute subscription request
			rr := httptest.NewRecorder()
			handlers.CreateSubscription(rr, req)

			// Verify subscription created
			if status := rr.Code; status != http.StatusCreated {
				t.Fatalf("Failed to create subscription for topic %s: got status %d", topic, status)
			}
		}

		// Step 2: Start SSE stream WITHOUT topic parameter (unified behavior)
		sseReq, err := http.NewRequest("GET", "/api/v1/events/stream", nil) // NO topic param!
		if err != nil {
			t.Fatal(err)
		}
		sseReq.Header.Set("Authorization", "Bearer "+token)
		sseReq.Header.Set("Accept", "text/event-stream")

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		ctx = context.WithValue(ctx, ClaimsKey, claims)
		sseReq = sseReq.WithContext(ctx)

		// Use streaming recorder
		recorder := NewStreamingRecorder()

		// Start SSE streaming in a goroutine
		go func() {
			defer recorder.Close()
			handlers.StreamEvents(recorder, sseReq)
		}()

		// Wait for SSE connection to be established
		select {
		case data := <-recorder.Data:
			// Should establish connection for ALL topics, not just one
			if !strings.Contains(data, "SSE connection established") {
				t.Errorf("Expected SSE connection message, got: %s", data)
			}
			t.Logf("✓ SSE connection established: %s", strings.TrimSpace(data))
		case <-time.After(1 * time.Second):
			t.Error("Timeout waiting for SSE connection message")
			return
		}

		// Step 3: Publish events to different topics that client is subscribed to
		testEvents := []struct {
			topic   string
			payload map[string]interface{}
		}{
			{"orders.created", map[string]interface{}{"orderId": 123, "amount": 99.99}},
			{"users.registered", map[string]interface{}{"userId": 456, "email": "test@example.com"}},
			{"payments.processed", map[string]interface{}{"paymentId": 789, "status": "completed"}},
		}

		// Publish all test events
		for _, event := range testEvents {
			publishReq := PublishRequest{
				Topic:   event.topic,
				Payload: event.payload,
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

			pubCtx := context.WithValue(context.Background(), ClaimsKey, claims)
			pubReq = pubReq.WithContext(pubCtx)

			// Execute publish request
			pubRR := httptest.NewRecorder()
			handlers.PublishEvent(pubRR, pubReq)

			// Verify publish succeeded
			if status := pubRR.Code; status != http.StatusCreated {
				t.Errorf("Failed to publish event to %s: got status %d", event.topic, status)
			}
		}

		// Step 4: Verify SSE client receives ALL published events (from all subscriptions)
		eventsReceived := make(map[string]bool)
		expectedEvents := map[string]bool{
			"orders.created":     false,
			"users.registered":   false,
			"payments.processed": false,
		}

		timeout := time.After(3 * time.Second)

		for len(eventsReceived) < len(expectedEvents) {
			select {
			case data := <-recorder.Data:
				// Look for published events in SSE format
				if strings.Contains(data, "data: {") {
					// Check which topic this event belongs to
					for topic := range expectedEvents {
						if strings.Contains(data, topic) && !eventsReceived[topic] {
							eventsReceived[topic] = true
							t.Logf("✅ Received event for topic %s: %s", topic, strings.TrimSpace(data))
							break
						}
					}
				}
			case <-timeout:
				// List missing events
				var missing []string
				for topic := range expectedEvents {
					if !eventsReceived[topic] {
						missing = append(missing, topic)
					}
				}
				t.Errorf("Timeout: SSE client should receive events from ALL subscriptions, missing: %v", missing)
				t.Error("This test expects unified SSE behavior where client gets events from all subscriptions")
				return
			case <-recorder.Done:
				return
			}
		}

		// Verify we got all events
		if len(eventsReceived) != len(expectedEvents) {
			t.Errorf("Expected events from %d topics, got %d", len(expectedEvents), len(eventsReceived))
		}

		cancel()
	})
}
