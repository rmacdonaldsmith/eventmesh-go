package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/rmacdonaldsmith/eventmesh-go/pkg/httpclient"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHTTPClientIntegration(t *testing.T) {
	// Create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/auth/login":
			response := httpclient.AuthResponse{
				Token:     "test-token-123",
				ExpiresAt: time.Now().Add(time.Hour),
				ClientID:  "test-client",
			}
			w.Header().Set("Content-Type", "application/json")
			if err := json.NewEncoder(w).Encode(response); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}

		case "/api/v1/health":
			response := httpclient.HealthResponse{
				Healthy:             true,
				EventLogHealthy:     true,
				RoutingTableHealthy: true,
				PeerLinkHealthy:     true,
				ConnectedClients:    5,
				ConnectedPeers:      3,
				Message:             "All systems operational",
			}
			w.Header().Set("Content-Type", "application/json")
			if err := json.NewEncoder(w).Encode(response); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}

		case "/api/v1/events":
			switch r.Method {
			case "POST":
				response := httpclient.PublishResponse{
					EventID:   "event-123",
					Offset:    456,
					Timestamp: time.Now(),
				}
				w.Header().Set("Content-Type", "application/json")
				if err := json.NewEncoder(w).Encode(response); err != nil {
					http.Error(w, err.Error(), http.StatusInternalServerError)
					return
				}
			}

		case "/api/v1/subscriptions":
			switch r.Method {
			case "POST":
				response := httpclient.SubscriptionResponse{
					ID:        "sub-123",
					Topic:     "test.topic",
					ClientID:  "test-client",
					CreatedAt: time.Now(),
				}
				w.Header().Set("Content-Type", "application/json")
				if err := json.NewEncoder(w).Encode(response); err != nil {
					http.Error(w, err.Error(), http.StatusInternalServerError)
					return
				}
			case "GET":
				subscriptions := []httpclient.SubscriptionResponse{
					{
						ID:        "sub-123",
						Topic:     "test.topic",
						ClientID:  "test-client",
						CreatedAt: time.Now(),
					},
				}
				w.Header().Set("Content-Type", "application/json")
				if err := json.NewEncoder(w).Encode(subscriptions); err != nil {
					http.Error(w, err.Error(), http.StatusInternalServerError)
					return
				}
			}

		default:
			// Handle ReadEvents endpoint with pattern matching
			if strings.Contains(r.URL.Path, "/api/v1/topics/") && strings.Contains(r.URL.Path, "/events") && r.Method == "GET" {
				events := []httpclient.EventStreamMessage{
					{
						EventID:   "test.topic-0",
						Topic:     "test.topic",
						Payload:   map[string]string{"message": "Event 0"},
						Timestamp: time.Now().Add(-10 * time.Minute),
						Offset:    0,
					},
					{
						EventID:   "test.topic-1",
						Topic:     "test.topic",
						Payload:   map[string]string{"message": "Event 1"},
						Timestamp: time.Now().Add(-5 * time.Minute),
						Offset:    1,
					},
				}
				response := httpclient.ReadEventsResponse{
					Events:      events,
					Topic:       "test.topic",
					StartOffset: 0,
					Count:       len(events),
				}
				w.Header().Set("Content-Type", "application/json")
				if err := json.NewEncoder(w).Encode(response); err != nil {
					http.Error(w, err.Error(), http.StatusInternalServerError)
					return
				}
			} else {
				w.WriteHeader(http.StatusNotFound)
			}
		}
	}))
	defer server.Close()

	// Test HTTP client operations directly
	config := httpclient.Config{
		ServerURL: server.URL,
		ClientID:  "test-client",
		Timeout:   5 * time.Second,
	}
	client, err := httpclient.NewClient(config)
	require.NoError(t, err)

	t.Run("authenticate", func(t *testing.T) {
		ctx := context.Background()
		err := client.Authenticate(ctx)
		require.NoError(t, err)
		assert.True(t, client.IsAuthenticated())
		assert.Equal(t, "test-token-123", client.GetToken())
	})

	t.Run("get health", func(t *testing.T) {
		ctx := context.Background()
		health, err := client.GetHealth(ctx)
		require.NoError(t, err)
		assert.True(t, health.Healthy)
		assert.Equal(t, 5, health.ConnectedClients)
		assert.Equal(t, 3, health.ConnectedPeers)
	})

	t.Run("publish event", func(t *testing.T) {
		ctx := context.Background()
		client.SetToken("test-token")

		payload := map[string]string{"message": "hello"}
		response, err := client.PublishEvent(ctx, "test.topic", payload)
		require.NoError(t, err)
		assert.Equal(t, "event-123", response.EventID)
		assert.Equal(t, int64(456), response.Offset)
	})

	t.Run("create subscription", func(t *testing.T) {
		ctx := context.Background()
		client.SetToken("test-token")

		response, err := client.CreateSubscription(ctx, "test.topic")
		require.NoError(t, err)
		assert.Equal(t, "sub-123", response.ID)
		assert.Equal(t, "test.topic", response.Topic)
	})

	t.Run("list subscriptions", func(t *testing.T) {
		ctx := context.Background()
		client.SetToken("test-token")

		subscriptions, err := client.ListSubscriptions(ctx)
		require.NoError(t, err)
		require.Len(t, subscriptions, 1)
		assert.Equal(t, "sub-123", subscriptions[0].ID)
	})

	t.Run("read events", func(t *testing.T) {
		ctx := context.Background()
		client.SetToken("test-token")

		response, err := client.ReadEvents(ctx, "test.topic", 0, 10)
		require.NoError(t, err)
		assert.Equal(t, "test.topic", response.Topic)
		assert.Equal(t, int64(0), response.StartOffset)
		assert.Equal(t, 2, response.Count)
		require.Len(t, response.Events, 2)
		assert.Equal(t, "test.topic-0", response.Events[0].EventID)
		assert.Equal(t, int64(0), response.Events[0].Offset)
	})
}

func TestRequireAuthentication(t *testing.T) {
	t.Run("returns error when client is nil", func(t *testing.T) {
		originalClient := client
		client = nil
		defer func() { client = originalClient }()

		err := requireAuthentication()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "client not initialized")
	})

	t.Run("returns error when not authenticated", func(t *testing.T) {
		config := httpclient.Config{
			ServerURL: "http://localhost:8081",
			ClientID:  "test-client",
			Timeout:   5 * time.Second,
		}
		testClient, err := httpclient.NewClient(config)
		require.NoError(t, err)

		originalClient := client
		client = testClient
		defer func() { client = originalClient }()

		err = requireAuthentication()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not authenticated")
	})

	t.Run("succeeds when authenticated", func(t *testing.T) {
		config := httpclient.Config{
			ServerURL: "http://localhost:8081",
			ClientID:  "test-client",
			Timeout:   5 * time.Second,
		}
		testClient, err := httpclient.NewClient(config)
		require.NoError(t, err)
		testClient.SetToken("test-token")

		originalClient := client
		client = testClient
		defer func() { client = originalClient }()

		err = requireAuthentication()
		assert.NoError(t, err)
	})
}

func TestMainCommandHelp(t *testing.T) {
	// Create a new root command for testing
	rootCmd := &cobra.Command{
		Use:   "eventmesh-cli",
		Short: "EventMesh HTTP API command line interface",
	}

	// Add subcommands
	rootCmd.AddCommand(newAuthCommand())
	rootCmd.AddCommand(newHealthCommand())
	rootCmd.AddCommand(newPublishCommand())
	rootCmd.AddCommand(newSubscribeCommand())
	rootCmd.AddCommand(newSubscriptionsCommand())
	rootCmd.AddCommand(newStreamCommand())
	rootCmd.AddCommand(newReplayCommand())
	rootCmd.AddCommand(newTopicsCommand())
	rootCmd.AddCommand(newAdminCommand())

	// Capture output
	output := &bytes.Buffer{}
	rootCmd.SetOut(output)
	rootCmd.SetArgs([]string{"--help"})

	// Execute help command
	err := rootCmd.Execute()
	require.NoError(t, err)

	helpOutput := output.String()

	// Check that all expected commands are listed
	assert.Contains(t, helpOutput, "auth")
	assert.Contains(t, helpOutput, "health")
	assert.Contains(t, helpOutput, "publish")
	assert.Contains(t, helpOutput, "subscribe")
	assert.Contains(t, helpOutput, "subscriptions")
	assert.Contains(t, helpOutput, "stream")
	assert.Contains(t, helpOutput, "replay")
	assert.Contains(t, helpOutput, "topics")
	assert.Contains(t, helpOutput, "admin")
}

func TestInvalidJSONPayload(t *testing.T) {
	cmd := newPublishCommand()

	// Capture output
	output := &bytes.Buffer{}
	cmd.SetOut(output)
	cmd.SetArgs([]string{"--topic", "test.topic", "--payload", "invalid-json"})

	// Initialize client first
	config := httpclient.Config{
		ServerURL: "http://localhost:8081",
		ClientID:  "test-client",
		Timeout:   5 * time.Second,
	}
	var err error
	client, err = httpclient.NewClient(config)
	require.NoError(t, err)
	client.SetToken("test-token")

	// Execute command - should fail with JSON error
	err = cmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid JSON payload")
}

func TestGlobalFlags(t *testing.T) {
	// Test that global flags are properly configured
	rootCmd := &cobra.Command{
		Use: "eventmesh-cli",
	}

	// Add global flags like in main
	rootCmd.PersistentFlags().StringVar(&serverURL, "server", "http://localhost:8081", "EventMesh server URL")
	rootCmd.PersistentFlags().StringVar(&clientID, "client-id", "", "Client ID for authentication")
	rootCmd.PersistentFlags().StringVar(&token, "token", "", "JWT token (if already authenticated)")
	rootCmd.PersistentFlags().DurationVar(&timeout, "timeout", 30*time.Second, "Request timeout")

	// Test setting flags
	rootCmd.SetArgs([]string{"--server", "http://example.com", "--client-id", "test", "--timeout", "10s"})

	// Parse flags
	err := rootCmd.ParseFlags([]string{"--server", "http://example.com", "--client-id", "test", "--timeout", "10s"})
	require.NoError(t, err)

	// Check that flags were set
	assert.Equal(t, "http://example.com", serverURL)
	assert.Equal(t, "test", clientID)
	assert.Equal(t, 10*time.Second, timeout)
}
