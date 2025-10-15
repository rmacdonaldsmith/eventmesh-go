package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestHTTPClient_SubscriptionTracking tests the subscription tracking functionality
func TestHTTPClient_SubscriptionTracking(t *testing.T) {
	t.Run("AddSubscription", func(t *testing.T) {
		// Create test JWT claims
		claims := &JWTClaims{
			ClientID: "test-client",
			IsAdmin:  false,
		}

		// Create HTTPClient
		client := NewHTTPClient(claims)
		// This should fail initially since AddSubscription doesn't exist yet
		subscriptionID := "sub-123"
		topic := "test.topic"

		err := client.AddSubscription(subscriptionID, topic)
		if err != nil {
			t.Fatalf("Expected no error, got: %v", err)
		}

		// Verify subscription was stored
		subscriptions := client.GetSubscriptions()
		if len(subscriptions) != 1 {
			t.Fatalf("Expected 1 subscription, got %d", len(subscriptions))
		}

		sub, exists := subscriptions[subscriptionID]
		if !exists {
			t.Fatalf("Subscription %s not found", subscriptionID)
		}

		if sub.Topic != topic {
			t.Errorf("Expected topic %s, got %s", topic, sub.Topic)
		}

		if sub.ID != subscriptionID {
			t.Errorf("Expected ID %s, got %s", subscriptionID, sub.ID)
		}
	})

	t.Run("RemoveSubscription", func(t *testing.T) {
		// Create test JWT claims
		claims := &JWTClaims{
			ClientID: "test-client",
			IsAdmin:  false,
		}

		// Create HTTPClient
		client := NewHTTPClient(claims)

		// Add a subscription first
		subscriptionID := "sub-456"
		topic := "test.removal"

		err := client.AddSubscription(subscriptionID, topic)
		if err != nil {
			t.Fatalf("Failed to add subscription: %v", err)
		}

		// Verify it exists
		subscriptions := client.GetSubscriptions()
		if len(subscriptions) != 1 {
			t.Fatalf("Expected 1 subscription before removal, got %d", len(subscriptions))
		}

		// Remove the subscription - this should fail initially since RemoveSubscription doesn't exist yet
		err = client.RemoveSubscription(subscriptionID)
		if err != nil {
			t.Fatalf("Expected no error removing subscription, got: %v", err)
		}

		// Verify it was removed
		subscriptions = client.GetSubscriptions()
		if len(subscriptions) != 0 {
			t.Errorf("Expected 0 subscriptions after removal, got %d", len(subscriptions))
		}

		// Verify specific subscription is gone
		_, exists := subscriptions[subscriptionID]
		if exists {
			t.Errorf("Subscription %s should have been removed", subscriptionID)
		}
	})

	t.Run("RemoveNonexistentSubscription", func(t *testing.T) {
		// Create test JWT claims
		claims := &JWTClaims{
			ClientID: "test-client",
			IsAdmin:  false,
		}

		// Create HTTPClient
		client := NewHTTPClient(claims)

		// Try to remove a subscription that doesn't exist
		err := client.RemoveSubscription("nonexistent-sub")
		// Should return an error since subscription doesn't exist
		if err == nil {
			t.Error("Expected error when removing nonexistent subscription, got nil")
		}
	})
}

// TestCreateSubscription tests the POST /api/v1/subscriptions endpoint
func TestCreateSubscription(t *testing.T) {
	// Use our shared test setup
	setup := NewTestServerSetup(t)
	defer setup.Close()

	handlers := setup.Server.handlers
	auth := setup.Auth

	// Generate test token
	token := setup.GenerateTestToken(t, "test-client", false)

	t.Run("successful_subscription_creation", func(t *testing.T) {
		// Create subscription request
		subscriptionReq := SubscriptionRequest{
			Topic: "test.events",
		}

		reqBody, err := json.Marshal(subscriptionReq)
		if err != nil {
			t.Fatal(err)
		}

		// Create HTTP request
		req, err := http.NewRequest("POST", "/api/v1/subscriptions", strings.NewReader(string(reqBody)))
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+token)

		// Add claims to context
		claims, err := auth.ValidateToken(token)
		if err != nil {
			t.Fatalf("Failed to validate token: %v", err)
		}
		ctx := context.WithValue(context.Background(), ClaimsKey, claims)
		req = req.WithContext(ctx)

		// Execute request
		rr := httptest.NewRecorder()
		handlers.CreateSubscription(rr, req)

		// Check response status
		if status := rr.Code; status != http.StatusCreated {
			t.Errorf("Expected status %d, got %d. Body: %s", http.StatusCreated, status, rr.Body.String())
		}

		// Parse response
		var resp SubscriptionResponse
		err = json.Unmarshal(rr.Body.Bytes(), &resp)
		if err != nil {
			t.Fatalf("Failed to parse response: %v", err)
		}

		// Verify response fields
		if resp.Topic != "test.events" {
			t.Errorf("Expected topic 'test.events', got '%s'", resp.Topic)
		}

		if resp.ClientID != "test-client" {
			t.Errorf("Expected clientId 'test-client', got '%s'", resp.ClientID)
		}

		if resp.ID == "" {
			t.Error("Expected non-empty subscription ID")
		}

		if resp.CreatedAt.IsZero() {
			t.Error("Expected non-zero CreatedAt timestamp")
		}
	})
}

// TestGetSubscriptions tests the GET /api/v1/subscriptions endpoint
func TestGetSubscriptions(t *testing.T) {
	// Use our shared test setup
	setup := NewTestServerSetup(t)
	defer setup.Close()

	handlers := setup.Server.handlers
	auth := setup.Auth

	// Generate test JWT token
	token := setup.GenerateTestToken(t, "test-client", false)

	t.Run("get_subscriptions_empty_list", func(t *testing.T) {
		// Create GET request
		req := httptest.NewRequest("GET", "/api/v1/subscriptions", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Content-Type", "application/json")

		// Add claims to context (simulating middleware)
		claims, err := auth.ValidateToken(token)
		if err != nil {
			t.Fatalf("Failed to validate token: %v", err)
		}
		ctx := context.WithValue(req.Context(), ClaimsKey, claims)
		req = req.WithContext(ctx)

		// Process request
		w := httptest.NewRecorder()
		handlers.ListSubscriptions(w, req)

		// Verify response
		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d. Body: %s", w.Code, w.Body.String())
		}

		var subscriptions []SubscriptionResponse
		if err := json.NewDecoder(w.Body).Decode(&subscriptions); err != nil {
			t.Fatalf("Failed to decode response: %v", err)
		}

		// Should have empty list
		if len(subscriptions) != 0 {
			t.Errorf("Expected empty subscriptions list, got %d items", len(subscriptions))
		}
	})

	t.Run("get_subscriptions_with_existing", func(t *testing.T) {
		// Prepare claims for reuse
		claims, err := auth.ValidateToken(token)
		if err != nil {
			t.Fatalf("Failed to validate token: %v", err)
		}

		// First create some subscriptions via POST
		subscriptionReq := SubscriptionRequest{Topic: "test.events.1"}
		reqBody, _ := json.Marshal(subscriptionReq)
		req := httptest.NewRequest("POST", "/api/v1/subscriptions", strings.NewReader(string(reqBody)))
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Content-Type", "application/json")

		// Add claims to context (simulating middleware)
		ctx := context.WithValue(req.Context(), ClaimsKey, claims)
		req = req.WithContext(ctx)

		w := httptest.NewRecorder()
		handlers.CreateSubscription(w, req)

		// Create second subscription
		subscriptionReq2 := SubscriptionRequest{Topic: "test.events.2"}
		reqBody2, _ := json.Marshal(subscriptionReq2)
		req2 := httptest.NewRequest("POST", "/api/v1/subscriptions", strings.NewReader(string(reqBody2)))
		req2.Header.Set("Authorization", "Bearer "+token)
		req2.Header.Set("Content-Type", "application/json")

		// Add claims to context (simulating middleware)
		ctx2 := context.WithValue(req2.Context(), ClaimsKey, claims)
		req2 = req2.WithContext(ctx2)

		w2 := httptest.NewRecorder()
		handlers.CreateSubscription(w2, req2)

		// Now GET subscriptions
		getReq := httptest.NewRequest("GET", "/api/v1/subscriptions", nil)
		getReq.Header.Set("Authorization", "Bearer "+token)
		getReq.Header.Set("Content-Type", "application/json")

		// Add claims to context (simulating middleware)
		getCtx := context.WithValue(getReq.Context(), ClaimsKey, claims)
		getReq = getReq.WithContext(getCtx)

		// Process GET request
		getW := httptest.NewRecorder()
		handlers.ListSubscriptions(getW, getReq)

		// Verify response
		if getW.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d. Body: %s", getW.Code, getW.Body.String())
		}

		var subscriptions []SubscriptionResponse
		if err := json.NewDecoder(getW.Body).Decode(&subscriptions); err != nil {
			t.Fatalf("Failed to decode response: %v", err)
		}

		// Should have 2 subscriptions
		if len(subscriptions) != 2 {
			t.Errorf("Expected 2 subscriptions, got %d", len(subscriptions))
		}

		// Verify subscription details
		topics := make(map[string]bool)
		for _, sub := range subscriptions {
			topics[sub.Topic] = true
			if sub.ClientID != "test-client" {
				t.Errorf("Expected clientId 'test-client', got '%s'", sub.ClientID)
			}
			if sub.ID == "" {
				t.Error("Expected non-empty subscription ID")
			}
			if sub.CreatedAt.IsZero() {
				t.Error("Expected non-zero CreatedAt timestamp")
			}
		}

		if !topics["test.events.1"] {
			t.Error("Missing topic 'test.events.1' in response")
		}
		if !topics["test.events.2"] {
			t.Error("Missing topic 'test.events.2' in response")
		}
	})
}

// TestDeleteSubscription tests the DELETE /api/v1/subscriptions/{id} endpoint
func TestDeleteSubscription(t *testing.T) {
	// Use our shared test setup
	setup := NewTestServerSetup(t)
	defer setup.Close()

	handlers := setup.Server.handlers
	auth := setup.Auth

	// Generate test JWT token
	token := setup.GenerateTestToken(t, "test-client", false)

	// Prepare claims for reuse
	claims, err := auth.ValidateToken(token)
	if err != nil {
		t.Fatalf("Failed to validate token: %v", err)
	}

	t.Run("successful_subscription_deletion", func(t *testing.T) {
		// First create a subscription to delete
		subscriptionReq := SubscriptionRequest{Topic: "test.events.delete"}
		reqBody, _ := json.Marshal(subscriptionReq)
		createReq := httptest.NewRequest("POST", "/api/v1/subscriptions", strings.NewReader(string(reqBody)))
		createReq.Header.Set("Authorization", "Bearer "+token)
		createReq.Header.Set("Content-Type", "application/json")

		// Add claims to context (simulating middleware)
		createCtx := context.WithValue(createReq.Context(), ClaimsKey, claims)
		createReq = createReq.WithContext(createCtx)

		createW := httptest.NewRecorder()
		handlers.CreateSubscription(createW, createReq)

		// Verify subscription was created
		if createW.Code != http.StatusCreated {
			t.Fatalf("Failed to create subscription: %d", createW.Code)
		}

		var createResponse SubscriptionResponse
		json.NewDecoder(createW.Body).Decode(&createResponse)
		subscriptionID := createResponse.ID

		// Now delete the subscription
		deleteReq := httptest.NewRequest("DELETE", "/api/v1/subscriptions/"+subscriptionID, nil)
		deleteReq.Header.Set("Authorization", "Bearer "+token)

		// Add claims to context (simulating middleware)
		deleteCtx := context.WithValue(deleteReq.Context(), ClaimsKey, claims)
		// Add subscription ID to context (simulating handleSubscriptionByID)
		deleteCtx = context.WithValue(deleteCtx, SubscriptionIDKey, subscriptionID)
		deleteReq = deleteReq.WithContext(deleteCtx)

		// Process DELETE request
		deleteW := httptest.NewRecorder()
		handlers.DeleteSubscription(deleteW, deleteReq)

		// Verify response
		if deleteW.Code != http.StatusNoContent {
			t.Errorf("Expected status 204, got %d. Body: %s", deleteW.Code, deleteW.Body.String())
		}

		// Verify subscription is actually deleted by checking GET response
		getReq := httptest.NewRequest("GET", "/api/v1/subscriptions", nil)
		getReq.Header.Set("Authorization", "Bearer "+token)
		getReq.Header.Set("Content-Type", "application/json")

		// Add claims to context (simulating middleware)
		getCtx := context.WithValue(getReq.Context(), ClaimsKey, claims)
		getReq = getReq.WithContext(getCtx)

		getW := httptest.NewRecorder()
		handlers.ListSubscriptions(getW, getReq)

		var subscriptions []SubscriptionResponse
		json.NewDecoder(getW.Body).Decode(&subscriptions)

		// Should have no subscriptions now
		if len(subscriptions) != 0 {
			t.Errorf("Expected no subscriptions after deletion, got %d", len(subscriptions))
		}
	})

	t.Run("delete_nonexistent_subscription", func(t *testing.T) {
		// Try to delete a subscription that doesn't exist
		nonexistentID := "nonexistent-sub-123"
		deleteReq := httptest.NewRequest("DELETE", "/api/v1/subscriptions/"+nonexistentID, nil)
		deleteReq.Header.Set("Authorization", "Bearer "+token)

		// Add claims to context (simulating middleware)
		deleteCtx := context.WithValue(deleteReq.Context(), ClaimsKey, claims)
		// Add subscription ID to context (simulating handleSubscriptionByID)
		deleteCtx = context.WithValue(deleteCtx, SubscriptionIDKey, nonexistentID)
		deleteReq = deleteReq.WithContext(deleteCtx)

		// Process DELETE request
		deleteW := httptest.NewRecorder()
		handlers.DeleteSubscription(deleteW, deleteReq)

		// Verify response
		if deleteW.Code != http.StatusNotFound {
			t.Errorf("Expected status 404, got %d. Body: %s", deleteW.Code, deleteW.Body.String())
		}
	})

	t.Run("delete_without_authentication", func(t *testing.T) {
		// Try to delete without authentication
		deleteReq := httptest.NewRequest("DELETE", "/api/v1/subscriptions/some-sub-123", nil)
		// No Authorization header

		// Process DELETE request
		deleteW := httptest.NewRecorder()
		handlers.DeleteSubscription(deleteW, deleteReq)

		// Verify response
		if deleteW.Code != http.StatusUnauthorized {
			t.Errorf("Expected status 401, got %d. Body: %s", deleteW.Code, deleteW.Body.String())
		}
	})
}