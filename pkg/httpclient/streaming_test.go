package httpclient

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStreamConfig_SetDefaults(t *testing.T) {
	t.Run("sets_default_values", func(t *testing.T) {
		config := StreamConfig{}
		config.SetDefaults()

		assert.Equal(t, 100, config.BufferSize)
		assert.Equal(t, 2*time.Second, config.ReconnectDelay)
		assert.Equal(t, 0, config.MaxReconnectAttempts) // 0 = infinite
	})

	t.Run("preserves_custom_values", func(t *testing.T) {
		config := StreamConfig{
			Topic:                "custom.topic",
			BufferSize:           200,
			ReconnectDelay:       5 * time.Second,
			MaxReconnectAttempts: 3,
		}
		config.SetDefaults()

		assert.Equal(t, "custom.topic", config.Topic)
		assert.Equal(t, 200, config.BufferSize)
		assert.Equal(t, 5*time.Second, config.ReconnectDelay)
		assert.Equal(t, 3, config.MaxReconnectAttempts)
	})
}

func TestClient_Stream(t *testing.T) {
	t.Run("requires_authentication", func(t *testing.T) {
		config := Config{
			ServerURL: "http://localhost:8081",
			ClientID:  "test-client",
		}
		client, err := NewClient(config)
		require.NoError(t, err)

		// Don't set token - client is not authenticated
		streamConfig := StreamConfig{Topic: "test.events"}
		streamClient, err := client.Stream(context.Background(), streamConfig)

		assert.Error(t, err)
		assert.Nil(t, streamClient)
		assert.Contains(t, err.Error(), "client not authenticated")
	})

	t.Run("creates_stream_client_successfully", func(t *testing.T) {
		// Create a mock server for this test to avoid connection issues
		streamRequested := make(chan struct{})
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch {
			case r.Method == http.MethodGet && r.URL.Path == "/api/v1/subscriptions":
				_ = json.NewEncoder(w).Encode([]SubscriptionResponse{})
			case r.Method == http.MethodPost && r.URL.Path == "/api/v1/subscriptions":
				w.WriteHeader(http.StatusCreated)
				_ = json.NewEncoder(w).Encode(SubscriptionResponse{
					ID:       "sub-temp-stream",
					Topic:    "test.events",
					ClientID: "test-client",
				})
			case r.Method == http.MethodGet && r.URL.Path == "/api/v1/events/stream":
				assert.Empty(t, r.URL.Query().Get("topic"))
				w.Header().Set("Content-Type", "text/event-stream")
				close(streamRequested)
				<-r.Context().Done()
			case r.Method == http.MethodDelete && r.URL.Path == "/api/v1/subscriptions/sub-temp-stream":
				w.WriteHeader(http.StatusNoContent)
			default:
				t.Errorf("unexpected request: %s %s", r.Method, r.URL.String())
				http.NotFound(w, r)
			}
		}))
		defer server.Close()

		config := Config{
			ServerURL: server.URL,
			ClientID:  "test-client",
		}
		client, err := NewClient(config)
		require.NoError(t, err)
		client.SetToken("test-token")

		streamConfig := StreamConfig{Topic: "test.events"}
		streamClient, err := client.Stream(context.Background(), streamConfig)

		require.NoError(t, err)
		assert.NotNil(t, streamClient)
		assert.NotNil(t, streamClient.Events())
		assert.NotNil(t, streamClient.Errors())
		assert.NotNil(t, streamClient.Done())
		select {
		case <-streamRequested:
		case <-time.After(1 * time.Second):
			t.Fatal("stream endpoint was not requested")
		}

		// Clean up
		err = streamClient.Close()
		assert.NoError(t, err)
	})
}

func TestStreamClient_SSEProcessing(t *testing.T) {
	t.Run("successful_sse_connection", func(t *testing.T) {
		// Create mock SSE server
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch {
			case r.Method == http.MethodGet && r.URL.Path == "/api/v1/subscriptions":
				_ = json.NewEncoder(w).Encode([]SubscriptionResponse{})
				return
			case r.Method == http.MethodPost && r.URL.Path == "/api/v1/subscriptions":
				w.WriteHeader(http.StatusCreated)
				_ = json.NewEncoder(w).Encode(SubscriptionResponse{
					ID:       "sub-stream-topic",
					Topic:    "test.events",
					ClientID: "test-client",
				})
				return
			case r.Method == http.MethodDelete && r.URL.Path == "/api/v1/subscriptions/sub-stream-topic":
				w.WriteHeader(http.StatusNoContent)
				return
			}

			// Verify SSE headers are set correctly by client
			assert.Equal(t, "text/event-stream", r.Header.Get("Accept"))
			assert.Equal(t, "no-cache", r.Header.Get("Cache-Control"))
			assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))

			// Verify streaming endpoint uses the unified stream without a topic query.
			assert.Equal(t, "/api/v1/events/stream", r.URL.Path)
			assert.Empty(t, r.URL.Query().Get("topic"))

			// Set SSE headers
			w.Header().Set("Content-Type", "text/event-stream")
			w.Header().Set("Cache-Control", "no-cache")
			w.Header().Set("Connection", "keep-alive")

			flusher, ok := w.(http.Flusher)
			require.True(t, ok, "ResponseWriter should support flushing")

			// Send a test event in SSE format
			event := EventStreamMessage{
				EventID:   "test-event-123",
				Topic:     "test.events",
				Payload:   map[string]interface{}{"message": "test payload"},
				Offset:    42,
				Timestamp: time.Now(),
			}

			eventJSON, err := json.Marshal(event)
			require.NoError(t, err)

			otherEvent := EventStreamMessage{
				EventID:   "other-event-123",
				Topic:     "other.events",
				Payload:   map[string]interface{}{"message": "filtered payload"},
				Offset:    43,
				Timestamp: time.Now(),
			}
			otherEventJSON, err := json.Marshal(otherEvent)
			require.NoError(t, err)

			_, _ = fmt.Fprintf(w, "data: %s\n\n", string(otherEventJSON))
			flusher.Flush()

			// Send SSE formatted data
			_, _ = fmt.Fprintf(w, "data: %s\n\n", string(eventJSON))
			flusher.Flush()

			select {
			case <-r.Context().Done():
			case <-time.After(100 * time.Millisecond):
			}
		}))
		defer server.Close()

		// Create client
		config := Config{
			ServerURL: server.URL,
			ClientID:  "test-client",
		}
		client, err := NewClient(config)
		require.NoError(t, err)
		client.SetToken("test-token")

		// Start streaming
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		streamConfig := StreamConfig{
			Topic:      "test.events",
			BufferSize: 10,
		}
		streamClient, err := client.Stream(ctx, streamConfig)
		require.NoError(t, err)
		defer func() { _ = streamClient.Close() }()

		// Should receive the event
		select {
		case event := <-streamClient.Events():
			assert.Equal(t, "test-event-123", event.EventID)
			assert.Equal(t, "test.events", event.Topic)
			assert.Equal(t, int64(42), event.Offset)
			assert.Contains(t, event.Payload, "message")
		case err := <-streamClient.Errors():
			t.Fatalf("Unexpected error: %v", err)
		case <-time.After(1 * time.Second):
			t.Fatal("Timeout waiting for event")
		}
	})

	t.Run("handles_sse_keepalive_comments", func(t *testing.T) {
		// Create mock server that sends keepalive comments
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/event-stream")
			w.Header().Set("Cache-Control", "no-cache")

			flusher := w.(http.Flusher)

			// Send keepalive comment (should be ignored)
			_, _ = fmt.Fprintf(w, ": keepalive\n\n")
			flusher.Flush()

			// Send actual event
			event := EventStreamMessage{
				EventID: "keepalive-test-event",
				Topic:   "test.keepalive",
			}
			eventJSON, _ := json.Marshal(event)
			_, _ = fmt.Fprintf(w, "data: %s\n\n", string(eventJSON))
			flusher.Flush()

			select {
			case <-r.Context().Done():
			case <-time.After(100 * time.Millisecond):
			}
		}))
		defer server.Close()

		config := Config{
			ServerURL: server.URL,
			ClientID:  "test-client",
		}
		client, err := NewClient(config)
		require.NoError(t, err)
		client.SetToken("test-token")

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		streamClient, err := client.Stream(ctx, StreamConfig{})
		require.NoError(t, err)
		defer func() { _ = streamClient.Close() }()

		// Should receive the actual event, not the keepalive
		select {
		case event := <-streamClient.Events():
			assert.Equal(t, "keepalive-test-event", event.EventID)
		case err := <-streamClient.Errors():
			t.Fatalf("Unexpected error: %v", err)
		case <-time.After(1 * time.Second):
			t.Fatal("Timeout waiting for event")
		}
	})

	t.Run("handles_malformed_json", func(t *testing.T) {
		// Create mock server that sends invalid JSON
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/event-stream")

			flusher := w.(http.Flusher)

			// Send malformed JSON
			_, _ = fmt.Fprintf(w, "data: {invalid json}\n\n")
			flusher.Flush()

			// Send valid JSON after
			event := EventStreamMessage{EventID: "valid-event"}
			eventJSON, _ := json.Marshal(event)
			_, _ = fmt.Fprintf(w, "data: %s\n\n", string(eventJSON))
			flusher.Flush()

			select {
			case <-r.Context().Done():
			case <-time.After(100 * time.Millisecond):
			}
		}))
		defer server.Close()

		config := Config{
			ServerURL: server.URL,
			ClientID:  "test-client",
		}
		client, err := NewClient(config)
		require.NoError(t, err)
		client.SetToken("test-token")

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		streamClient, err := client.Stream(ctx, StreamConfig{})
		require.NoError(t, err)
		defer func() { _ = streamClient.Close() }()

		// Should receive error for malformed JSON
		select {
		case err := <-streamClient.Errors():
			assert.Contains(t, err.Error(), "failed to parse event")
		case <-time.After(500 * time.Millisecond):
			// Continue to check for valid event
		}

		// Should still receive valid event after error
		select {
		case event := <-streamClient.Events():
			assert.Equal(t, "valid-event", event.EventID)
		case <-time.After(500 * time.Millisecond):
			t.Fatal("Should receive valid event after parse error")
		}
	})

	t.Run("handles_http_errors", func(t *testing.T) {
		// Create mock server that returns HTTP error
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte("Unauthorized"))
		}))
		defer server.Close()

		config := Config{
			ServerURL: server.URL,
			ClientID:  "test-client",
		}
		client, err := NewClient(config)
		require.NoError(t, err)
		client.SetToken("invalid-token")

		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
		defer cancel()

		streamClient, err := client.Stream(ctx, StreamConfig{})
		require.NoError(t, err)
		defer func() { _ = streamClient.Close() }()

		// Should receive HTTP error
		select {
		case err := <-streamClient.Errors():
			assert.Contains(t, err.Error(), "streaming failed with status 401")
		case <-time.After(1 * time.Second):
			t.Fatal("Should receive HTTP error")
		}
	})
}

func TestStreamClient_TracksSSEIDsForResume(t *testing.T) {
	eventID, err := encodeSSECursor(sseEventCursor{
		Version: sseCursorVersion,
		NodeID:  "node-a",
		Topic:   "orders.created",
		Offset:  42,
	})
	require.NoError(t, err)

	streamClient := &StreamClient{
		events:        make(chan EventStreamMessage, 1),
		errors:        make(chan error, 1),
		resumeOffsets: make(map[string]int64),
	}

	event := EventStreamMessage{Topic: "orders.created", Offset: 42}
	eventJSON, err := json.Marshal(event)
	require.NoError(t, err)

	err = streamClient.processSSEStream(context.Background(), strings.NewReader("id: "+eventID+"\n"+"data: "+string(eventJSON)+"\n\n"), StreamConfig{})
	require.NoError(t, err)

	select {
	case received := <-streamClient.Events():
		assert.Equal(t, eventID, received.EventID)
	case <-time.After(1 * time.Second):
		t.Fatal("Timed out waiting for parsed event")
	}

	resumeHeader, err := streamClient.resumeCursorHeader()
	require.NoError(t, err)
	assert.NotEmpty(t, resumeHeader)

	decoded, err := decodeTestMultiTopicCursor(resumeHeader)
	require.NoError(t, err)
	assert.Equal(t, "node-a", decoded.NodeID)
	assert.Equal(t, int64(42), decoded.Topics["orders.created"])
}

func TestStreamClient_SendsLastEventIDOnReconnect(t *testing.T) {
	firstEventID, err := encodeSSECursor(sseEventCursor{Version: sseCursorVersion, NodeID: "node-a", Topic: "orders.created", Offset: 0})
	require.NoError(t, err)

	requestCount := 0
	secondRequestLastEventID := make(chan string, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		w.Header().Set("Content-Type", "text/event-stream")
		flusher := w.(http.Flusher)

		if requestCount == 1 {
			eventJSON, _ := json.Marshal(EventStreamMessage{Topic: "orders.created", Offset: 0})
			_, _ = fmt.Fprintf(w, "id: %s\ndata: %s\n\n", firstEventID, string(eventJSON))
			flusher.Flush()
			return
		}

		secondRequestLastEventID <- r.Header.Get("Last-Event-ID")
	}))
	defer server.Close()

	client, err := NewClient(Config{ServerURL: server.URL, ClientID: "test-client"})
	require.NoError(t, err)
	client.SetToken("test-token")

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	streamClient, err := client.Stream(ctx, StreamConfig{ReconnectDelay: 10 * time.Millisecond, MaxReconnectAttempts: 2})
	require.NoError(t, err)
	defer func() { _ = streamClient.Close() }()

	select {
	case <-streamClient.Events():
	case err := <-streamClient.Errors():
		t.Fatalf("Unexpected stream error: %v", err)
	case <-time.After(1 * time.Second):
		t.Fatal("Timed out waiting for first event")
	}

	select {
	case header := <-secondRequestLastEventID:
		decoded, err := decodeTestMultiTopicCursor(header)
		require.NoError(t, err)
		assert.Equal(t, "node-a", decoded.NodeID)
		assert.Equal(t, int64(0), decoded.Topics["orders.created"])
	case <-time.After(1 * time.Second):
		t.Fatal("Timed out waiting for reconnect with Last-Event-ID")
	}
}

func decodeTestMultiTopicCursor(raw string) (sseMultiTopicCursor, error) {
	decoded, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil {
		return sseMultiTopicCursor{}, err
	}
	var cursor sseMultiTopicCursor
	if err := json.Unmarshal(decoded, &cursor); err != nil {
		return sseMultiTopicCursor{}, err
	}
	return cursor, nil
}

func TestStreamClient_Reconnection(t *testing.T) {
	t.Run("reconnects_on_connection_failure", func(t *testing.T) {
		connectionAttempts := 0
		eventsToSend := []string{"event-1", "event-2"}

		// Create mock server that fails first connection, succeeds on second
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			connectionAttempts++

			if connectionAttempts == 1 {
				// First connection fails
				w.WriteHeader(http.StatusInternalServerError)
				return
			}

			// Second connection succeeds
			w.Header().Set("Content-Type", "text/event-stream")
			flusher := w.(http.Flusher)

			for _, eventID := range eventsToSend {
				event := EventStreamMessage{EventID: eventID}
				eventJSON, _ := json.Marshal(event)
				_, _ = fmt.Fprintf(w, "data: %s\n\n", string(eventJSON))
				flusher.Flush()
			}
		}))
		defer server.Close()

		config := Config{
			ServerURL: server.URL,
			ClientID:  "test-client",
		}
		client, err := NewClient(config)
		require.NoError(t, err)
		client.SetToken("test-token")

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		streamConfig := StreamConfig{
			ReconnectDelay:       100 * time.Millisecond, // Fast reconnect for test
			MaxReconnectAttempts: 2,
		}
		streamClient, err := client.Stream(ctx, streamConfig)
		require.NoError(t, err)
		defer func() { _ = streamClient.Close() }()

		// Should receive error from first failed connection
		select {
		case err := <-streamClient.Errors():
			assert.Contains(t, err.Error(), "streaming failed with status 500")
		case <-time.After(1 * time.Second):
			t.Fatal("Should receive connection error")
		}

		// Should eventually receive events from successful reconnection
		receivedEvents := 0
		timeout := time.After(3 * time.Second)
		for receivedEvents < len(eventsToSend) {
			select {
			case event := <-streamClient.Events():
				assert.Contains(t, eventsToSend, event.EventID)
				receivedEvents++
			case err := <-streamClient.Errors():
				assert.Error(t, err)
			case <-timeout:
				t.Fatalf("Timeout waiting for events after reconnection. Received %d/%d events", receivedEvents, len(eventsToSend))
			}
		}

		assert.GreaterOrEqual(t, connectionAttempts, 2, "Should have attempted reconnection")
	})

	t.Run("respects_max_reconnect_attempts", func(t *testing.T) {
		connectionAttempts := 0

		// Create mock server that always fails
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			connectionAttempts++
			w.WriteHeader(http.StatusInternalServerError)
		}))
		defer server.Close()

		config := Config{
			ServerURL: server.URL,
			ClientID:  "test-client",
		}
		client, err := NewClient(config)
		require.NoError(t, err)
		client.SetToken("test-token")

		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()

		streamConfig := StreamConfig{
			ReconnectDelay:       100 * time.Millisecond,
			MaxReconnectAttempts: 2, // Limit to 2 attempts
		}
		streamClient, err := client.Stream(ctx, streamConfig)
		require.NoError(t, err)
		defer func() { _ = streamClient.Close() }()

		// Should receive multiple errors during reconnection attempts
		errorCount := 0
		maxReconnectError := false
		streamEnded := false

		for !streamEnded {
			select {
			case err := <-streamClient.Errors():
				if err != nil {
					errorCount++
					errorMsg := err.Error()
					if strings.Contains(errorMsg, "max reconnect attempts") || strings.Contains(errorMsg, "exceeded") {
						maxReconnectError = true
					}
				}
			case <-streamClient.Done():
				streamEnded = true
			case <-time.After(2 * time.Second):
				t.Fatal("Timed out waiting for stream to end after max reconnect attempts")
			}
		}

		for err := range streamClient.Errors() {
			if err == nil {
				continue
			}
			errorCount++
			errorMsg := err.Error()
			if strings.Contains(errorMsg, "max reconnect attempts") || strings.Contains(errorMsg, "exceeded") {
				maxReconnectError = true
			}
		}

		assert.True(t, streamEnded, "Stream should end after max reconnect attempts")
		assert.Greater(t, errorCount, 0, "Should receive at least one error")
		assert.True(t, maxReconnectError, "Should report max reconnect attempts exceeded")
		assert.LessOrEqual(t, connectionAttempts, streamConfig.MaxReconnectAttempts+1, "Should not exceed max reconnect attempts")
	})
}

func TestStreamClient_Lifecycle(t *testing.T) {
	t.Run("close_cancels_context", func(t *testing.T) {
		// Create mock server with long-running connection
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/event-stream")
			flusher := w.(http.Flusher)

			_, _ = fmt.Fprintf(w, ": keepalive\n\n")
			flusher.Flush()
			<-r.Context().Done()
		}))
		defer server.Close()

		config := Config{
			ServerURL: server.URL,
			ClientID:  "test-client",
		}
		client, err := NewClient(config)
		require.NoError(t, err)
		client.SetToken("test-token")

		streamClient, err := client.Stream(context.Background(), StreamConfig{})
		require.NoError(t, err)

		// Stream should be active after the client is created.
		select {
		case <-streamClient.Done():
			t.Fatal("Stream should not be done immediately")
		default:
		}

		// Close should terminate the stream
		start := time.Now()
		err = streamClient.Close()
		duration := time.Since(start)

		assert.NoError(t, err)
		assert.Less(t, duration, 1*time.Second, "Close should be fast")

		// Stream should be done after close
		select {
		case <-streamClient.Done():
			// Expected - stream is done
		case <-time.After(500 * time.Millisecond):
			t.Fatal("Stream should be done after close")
		}
	})

	t.Run("graceful_shutdown_via_close", func(t *testing.T) {
		// Create mock server that can handle streaming
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/event-stream")
			w.Header().Set("Cache-Control", "no-cache")
			flusher := w.(http.Flusher)

			event := EventStreamMessage{EventID: "shutdown-test-0"}
			eventJSON, _ := json.Marshal(event)
			_, _ = fmt.Fprintf(w, "data: %s\n\n", string(eventJSON))
			flusher.Flush()
			<-r.Context().Done()
		}))
		defer server.Close()

		config := Config{
			ServerURL: server.URL,
			ClientID:  "test-client",
		}
		client, err := NewClient(config)
		require.NoError(t, err)
		client.SetToken("test-token")

		streamClient, err := client.Stream(context.Background(), StreamConfig{})
		require.NoError(t, err)

		// Wait to receive at least one event
		select {
		case event := <-streamClient.Events():
			assert.Contains(t, event.EventID, "shutdown-test-")
		case <-time.After(1 * time.Second):
			t.Fatal("Should receive at least one event")
		}

		// Close the stream gracefully
		err = streamClient.Close()
		assert.NoError(t, err)

		// Verify the Done channel is closed after Close()
		select {
		case <-streamClient.Done():
			// Expected - stream should be done after Close()
		case <-time.After(1 * time.Second):
			t.Fatal("Stream should be done after Close()")
		}
	})
}

func TestStreamClient_EventChannelBuffering(t *testing.T) {
	t.Run("handles_channel_buffer_overflow", func(t *testing.T) {
		// Create mock server that sends many events rapidly
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/event-stream")
			flusher := w.(http.Flusher)

			// Send more events than buffer size rapidly.
			for i := 0; i < 10; i++ {
				event := EventStreamMessage{EventID: fmt.Sprintf("overflow-event-%d", i)}
				eventJSON, _ := json.Marshal(event)
				_, _ = fmt.Fprintf(w, "data: %s\n\n", string(eventJSON))
				flusher.Flush()
			}

			select {
			case <-r.Context().Done():
			case <-time.After(100 * time.Millisecond):
			}
		}))
		defer server.Close()

		config := Config{
			ServerURL: server.URL,
			ClientID:  "test-client",
		}
		client, err := NewClient(config)
		require.NoError(t, err)
		client.SetToken("test-token")

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		streamConfig := StreamConfig{
			BufferSize: 3, // Small buffer to test overflow
		}
		streamClient, err := client.Stream(ctx, streamConfig)
		require.NoError(t, err)
		defer func() { _ = streamClient.Close() }()

		// Read events slowly to test buffer behavior
		receivedEvents := 0
		for receivedEvents < 5 { // Read some but not all events
			select {
			case event := <-streamClient.Events():
				assert.Contains(t, event.EventID, "overflow-event-")
				receivedEvents++
			case err := <-streamClient.Errors():
				assert.Error(t, err)
			case <-time.After(1 * time.Second):
				goto done
			}
		}

	done:
		assert.Greater(t, receivedEvents, 0, "Should receive at least some events")
		// Note: Some events may be dropped due to buffer overflow, which is expected behavior
	})
}
