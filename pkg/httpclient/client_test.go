package httpclient

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewClient(t *testing.T) {
	t.Run("valid_config", func(t *testing.T) {
		config := Config{
			ServerURL: "http://localhost:8081",
			ClientID:  "test-client",
		}

		client, err := NewClient(config)
		require.NoError(t, err)
		assert.NotNil(t, client)
		assert.Equal(t, "test-client", client.config.ClientID)
		assert.Equal(t, 30*time.Second, client.config.Timeout)
		assert.Equal(t, 3, client.config.MaxRetries)
	})

	t.Run("missing_server_url", func(t *testing.T) {
		config := Config{
			ClientID: "test-client",
		}

		client, err := NewClient(config)
		assert.Error(t, err)
		assert.Nil(t, client)
		assert.Contains(t, err.Error(), "ServerURL is required")
	})

	t.Run("missing_client_id", func(t *testing.T) {
		config := Config{
			ServerURL: "http://localhost:8081",
		}

		client, err := NewClient(config)
		assert.Error(t, err)
		assert.Nil(t, client)
		assert.Contains(t, err.Error(), "ClientID is required")
	})

	t.Run("invalid_server_url", func(t *testing.T) {
		config := Config{
			ServerURL: "://invalid-url",
			ClientID:  "test-client",
		}

		client, err := NewClient(config)
		assert.Error(t, err)
		assert.Nil(t, client)
		assert.Contains(t, err.Error(), "invalid ServerURL")
	})
}

func TestClient_Authenticate(t *testing.T) {
	t.Run("successful_authentication", func(t *testing.T) {
		// Mock server
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "POST", r.Method)
			assert.Equal(t, "/api/v1/auth/login", r.URL.Path)
			assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

			// Parse request
			var authReq map[string]string
			err := json.NewDecoder(r.Body).Decode(&authReq)
			require.NoError(t, err)
			assert.Equal(t, "test-client", authReq["clientId"])

			// Return mock response
			response := AuthResponse{
				Token:     "mock-token-123",
				ClientID:  "test-client",
				ExpiresAt: time.Now().Add(time.Hour),
			}
			json.NewEncoder(w).Encode(response)
		}))
		defer server.Close()

		config := Config{
			ServerURL: server.URL,
			ClientID:  "test-client",
		}
		client, err := NewClient(config)
		require.NoError(t, err)

		// Test authentication
		err = client.Authenticate(context.Background())
		require.NoError(t, err)

		assert.True(t, client.IsAuthenticated())
		assert.Equal(t, "mock-token-123", client.GetToken())
	})

	t.Run("authentication_failure", func(t *testing.T) {
		// Mock server that returns error
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(ErrorResponse{
				Error:   "Unauthorized",
				Message: "Invalid client credentials",
				Code:    401,
			})
		}))
		defer server.Close()

		config := Config{
			ServerURL: server.URL,
			ClientID:  "invalid-client",
		}
		client, err := NewClient(config)
		require.NoError(t, err)

		// Test authentication failure
		err = client.Authenticate(context.Background())
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "authentication failed")
		assert.False(t, client.IsAuthenticated())
	})
}

func TestClient_PublishEvent(t *testing.T) {
	t.Run("successful_publish", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "POST", r.Method)
			assert.Equal(t, "/api/v1/events", r.URL.Path)
			assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
			assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))

			// Parse request
			var publishReq PublishRequest
			err := json.NewDecoder(r.Body).Decode(&publishReq)
			require.NoError(t, err)
			assert.Equal(t, "test.topic", publishReq.Topic)

			// Return mock response
			response := PublishResponse{
				EventID:   "test.topic-1",
				Offset:    1,
				Timestamp: time.Now(),
			}
			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(response)
		}))
		defer server.Close()

		config := Config{
			ServerURL: server.URL,
			ClientID:  "test-client",
		}
		client, err := NewClient(config)
		require.NoError(t, err)
		client.SetToken("test-token")

		// Test publish
		payload := map[string]interface{}{"message": "hello world"}
		response, err := client.PublishEvent(context.Background(), "test.topic", payload)
		require.NoError(t, err)

		assert.Equal(t, "test.topic-1", response.EventID)
		assert.Equal(t, int64(1), response.Offset)
	})

	t.Run("publish_without_authentication", func(t *testing.T) {
		config := Config{
			ServerURL: "http://localhost:8081",
			ClientID:  "test-client",
		}
		client, err := NewClient(config)
		require.NoError(t, err)

		// Test publish without authentication
		_, err = client.PublishEvent(context.Background(), "test.topic", map[string]string{"msg": "test"})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not authenticated")
	})
}

func TestClient_CreateSubscription(t *testing.T) {
	t.Run("successful_subscription", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "POST", r.Method)
			assert.Equal(t, "/api/v1/subscriptions", r.URL.Path)
			assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))

			// Parse request
			var subReq SubscriptionRequest
			err := json.NewDecoder(r.Body).Decode(&subReq)
			require.NoError(t, err)
			assert.Equal(t, "test.*", subReq.Topic)

			// Return mock response
			response := SubscriptionResponse{
				ID:        "sub-123",
				Topic:     "test.*",
				ClientID:  "test-client",
				CreatedAt: time.Now(),
			}
			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(response)
		}))
		defer server.Close()

		config := Config{
			ServerURL: server.URL,
			ClientID:  "test-client",
		}
		client, err := NewClient(config)
		require.NoError(t, err)
		client.SetToken("test-token")

		// Test create subscription
		response, err := client.CreateSubscription(context.Background(), "test.*")
		require.NoError(t, err)

		assert.Equal(t, "sub-123", response.ID)
		assert.Equal(t, "test.*", response.Topic)
		assert.Equal(t, "test-client", response.ClientID)
	})
}

func TestClient_ListSubscriptions(t *testing.T) {
	t.Run("successful_list", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "GET", r.Method)
			assert.Equal(t, "/api/v1/subscriptions", r.URL.Path)
			assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))

			// Return mock response
			subscriptions := []SubscriptionResponse{
				{
					ID:        "sub-123",
					Topic:     "test.*",
					ClientID:  "test-client",
					CreatedAt: time.Now(),
				},
				{
					ID:        "sub-456",
					Topic:     "prod.events",
					ClientID:  "test-client",
					CreatedAt: time.Now(),
				},
			}
			json.NewEncoder(w).Encode(subscriptions)
		}))
		defer server.Close()

		config := Config{
			ServerURL: server.URL,
			ClientID:  "test-client",
		}
		client, err := NewClient(config)
		require.NoError(t, err)
		client.SetToken("test-token")

		// Test list subscriptions
		subscriptions, err := client.ListSubscriptions(context.Background())
		require.NoError(t, err)

		assert.Len(t, subscriptions, 2)
		assert.Equal(t, "sub-123", subscriptions[0].ID)
		assert.Equal(t, "sub-456", subscriptions[1].ID)
	})
}

func TestClient_DeleteSubscription(t *testing.T) {
	t.Run("successful_delete", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "DELETE", r.Method)
			assert.Equal(t, "/api/v1/subscriptions/sub-123", r.URL.Path)
			assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))

			w.WriteHeader(http.StatusNoContent)
		}))
		defer server.Close()

		config := Config{
			ServerURL: server.URL,
			ClientID:  "test-client",
		}
		client, err := NewClient(config)
		require.NoError(t, err)
		client.SetToken("test-token")

		// Test delete subscription
		err = client.DeleteSubscription(context.Background(), "sub-123")
		require.NoError(t, err)
	})
}

func TestClient_GetHealth(t *testing.T) {
	t.Run("successful_health_check", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "GET", r.Method)
			assert.Equal(t, "/api/v1/health", r.URL.Path)
			// Health endpoint doesn't require authentication

			response := HealthResponse{
				Healthy:             true,
				EventLogHealthy:     true,
				RoutingTableHealthy: true,
				PeerLinkHealthy:     true,
				ConnectedClients:    0,
				ConnectedPeers:      0,
				Message:             "All systems healthy",
			}
			json.NewEncoder(w).Encode(response)
		}))
		defer server.Close()

		config := Config{
			ServerURL: server.URL,
			ClientID:  "test-client",
		}
		client, err := NewClient(config)
		require.NoError(t, err)

		// Test health check (no authentication required)
		health, err := client.GetHealth(context.Background())
		require.NoError(t, err)

		assert.True(t, health.Healthy)
		assert.Equal(t, "All systems healthy", health.Message)
	})
}