package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestAdminListClients tests the GET /api/v1/admin/clients endpoint
func TestAdminListClients(t *testing.T) {
	// Use our shared test setup
	setup := NewTestServerSetup(t)
	defer setup.Close()

	handlers := setup.Server.handlers
	auth := setup.Auth

	t.Run("admin_access_required", func(t *testing.T) {
		// Generate non-admin token
		token := setup.GenerateTestToken(t, "regular-client", false)

		// Create request
		req := httptest.NewRequest("GET", "/api/v1/admin/clients", nil)
		req.Header.Set("Authorization", "Bearer "+token)

		// Add claims to context
		claims, err := auth.ValidateToken(token)
		if err != nil {
			t.Fatalf("Failed to validate token: %v", err)
		}
		ctx := context.WithValue(req.Context(), ClaimsKey, claims)
		req = req.WithContext(ctx)

		// Execute request
		w := httptest.NewRecorder()
		handlers.AdminListClients(w, req)

		// Should return forbidden
		if w.Code != http.StatusForbidden {
			t.Errorf("Expected status 403, got %d. Body: %s", w.Code, w.Body.String())
		}
	})

	t.Run("successful_admin_request", func(t *testing.T) {
		// Generate admin token
		token := setup.GenerateTestToken(t, "admin", true)

		// Create request
		req := httptest.NewRequest("GET", "/api/v1/admin/clients", nil)
		req.Header.Set("Authorization", "Bearer "+token)

		// Add claims to context
		claims, err := auth.ValidateToken(token)
		if err != nil {
			t.Fatalf("Failed to validate token: %v", err)
		}
		ctx := context.WithValue(req.Context(), ClaimsKey, claims)
		req = req.WithContext(ctx)

		// Execute request
		w := httptest.NewRecorder()
		handlers.AdminListClients(w, req)

		// Should return success
		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d. Body: %s", w.Code, w.Body.String())
		}

		// Parse response
		var response AdminClientsResponse
		if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
			t.Fatalf("Failed to decode response: %v", err)
		}

		// Verify response structure (empty slice is valid)
		// In Go, empty slices can be nil, which is fine for JSON marshaling
	})

	t.Run("missing_authentication", func(t *testing.T) {
		// Create request without auth
		req := httptest.NewRequest("GET", "/api/v1/admin/clients", nil)

		// Execute request
		w := httptest.NewRecorder()
		handlers.AdminListClients(w, req)

		// Should return unauthorized
		if w.Code != http.StatusUnauthorized {
			t.Errorf("Expected status 401, got %d", w.Code)
		}
	})
}

// TestAdminListSubscriptions tests the GET /api/v1/admin/subscriptions endpoint
func TestAdminListSubscriptions(t *testing.T) {
	// Use our shared test setup
	setup := NewTestServerSetup(t)
	defer setup.Close()

	handlers := setup.Server.handlers
	auth := setup.Auth

	t.Run("admin_access_required", func(t *testing.T) {
		// Generate non-admin token
		token := setup.GenerateTestToken(t, "regular-client", false)

		// Create request
		req := httptest.NewRequest("GET", "/api/v1/admin/subscriptions", nil)
		req.Header.Set("Authorization", "Bearer "+token)

		// Add claims to context
		claims, err := auth.ValidateToken(token)
		if err != nil {
			t.Fatalf("Failed to validate token: %v", err)
		}
		ctx := context.WithValue(req.Context(), ClaimsKey, claims)
		req = req.WithContext(ctx)

		// Execute request
		w := httptest.NewRecorder()
		handlers.AdminListSubscriptions(w, req)

		// Should return forbidden
		if w.Code != http.StatusForbidden {
			t.Errorf("Expected status 403, got %d. Body: %s", w.Code, w.Body.String())
		}
	})

	t.Run("successful_admin_request", func(t *testing.T) {
		// Generate admin token
		token := setup.GenerateTestToken(t, "admin", true)

		// Create request
		req := httptest.NewRequest("GET", "/api/v1/admin/subscriptions", nil)
		req.Header.Set("Authorization", "Bearer "+token)

		// Add claims to context
		claims, err := auth.ValidateToken(token)
		if err != nil {
			t.Fatalf("Failed to validate token: %v", err)
		}
		ctx := context.WithValue(req.Context(), ClaimsKey, claims)
		req = req.WithContext(ctx)

		// Execute request
		w := httptest.NewRecorder()
		handlers.AdminListSubscriptions(w, req)

		// Should return success
		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d. Body: %s", w.Code, w.Body.String())
		}

		// Parse response
		var response AdminSubscriptionsResponse
		if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
			t.Fatalf("Failed to decode response: %v", err)
		}

		// Verify response structure (empty slice is valid)
		// In Go, empty slices can be nil, which is fine for JSON marshaling
	})
}

// TestAdminGetStats tests the GET /api/v1/admin/stats endpoint
func TestAdminGetStats(t *testing.T) {
	// Use our shared test setup
	setup := NewTestServerSetup(t)
	defer setup.Close()

	handlers := setup.Server.handlers
	auth := setup.Auth

	t.Run("admin_access_required", func(t *testing.T) {
		// Generate non-admin token
		token := setup.GenerateTestToken(t, "regular-client", false)

		// Create request
		req := httptest.NewRequest("GET", "/api/v1/admin/stats", nil)
		req.Header.Set("Authorization", "Bearer "+token)

		// Add claims to context
		claims, err := auth.ValidateToken(token)
		if err != nil {
			t.Fatalf("Failed to validate token: %v", err)
		}
		ctx := context.WithValue(req.Context(), ClaimsKey, claims)
		req = req.WithContext(ctx)

		// Execute request
		w := httptest.NewRecorder()
		handlers.AdminGetStats(w, req)

		// Should return forbidden
		if w.Code != http.StatusForbidden {
			t.Errorf("Expected status 403, got %d. Body: %s", w.Code, w.Body.String())
		}
	})

	t.Run("successful_admin_request", func(t *testing.T) {
		// Generate admin token
		token := setup.GenerateTestToken(t, "admin", true)

		// Create request
		req := httptest.NewRequest("GET", "/api/v1/admin/stats", nil)
		req.Header.Set("Authorization", "Bearer "+token)

		// Add claims to context
		claims, err := auth.ValidateToken(token)
		if err != nil {
			t.Fatalf("Failed to validate token: %v", err)
		}
		ctx := context.WithValue(req.Context(), ClaimsKey, claims)
		req = req.WithContext(ctx)

		// Execute request
		w := httptest.NewRecorder()
		handlers.AdminGetStats(w, req)

		// Should return success
		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d. Body: %s", w.Code, w.Body.String())
		}

		// Parse response
		var response AdminStatsResponse
		if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
			t.Fatalf("Failed to decode response: %v", err)
		}

		// Verify response structure and expected values
		if response.ConnectedClients < 0 {
			t.Error("Expected ConnectedClients to be non-negative")
		}
		if response.TotalSubscriptions < 0 {
			t.Error("Expected TotalSubscriptions to be non-negative")
		}
		if response.TotalTopics < 0 {
			t.Error("Expected TotalTopics to be non-negative")
		}
		if response.EventsPublished < 0 {
			t.Error("Expected EventsPublished to be non-negative")
		}
	})

	t.Run("missing_authentication", func(t *testing.T) {
		// Create request without auth
		req := httptest.NewRequest("GET", "/api/v1/admin/stats", nil)

		// Execute request
		w := httptest.NewRecorder()
		handlers.AdminGetStats(w, req)

		// Should return unauthorized
		if w.Code != http.StatusUnauthorized {
			t.Errorf("Expected status 401, got %d", w.Code)
		}
	})
}

// TestAdminEndpointsIntegration tests admin endpoints through the full server routing
func TestAdminEndpointsIntegration(t *testing.T) {
	// Use our shared test setup
	setup := NewTestServerSetup(t)
	defer setup.Close()

	server := setup.Server

	// Generate admin token
	token := setup.GenerateTestToken(t, "admin", true)

	// Set up full server routing
	handler := server.setupRoutes()

	t.Run("admin_clients_via_full_routing", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/v1/admin/clients", nil)
		req.Header.Set("Authorization", "Bearer "+token)

		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d. Body: %s", w.Code, w.Body.String())
		}
	})

	t.Run("admin_subscriptions_via_full_routing", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/v1/admin/subscriptions", nil)
		req.Header.Set("Authorization", "Bearer "+token)

		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d. Body: %s", w.Code, w.Body.String())
		}
	})

	t.Run("admin_stats_via_full_routing", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/v1/admin/stats", nil)
		req.Header.Set("Authorization", "Bearer "+token)

		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d. Body: %s", w.Code, w.Body.String())
		}
	})
}
