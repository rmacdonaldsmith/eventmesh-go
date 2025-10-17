package httpapi

import (
	"testing"
	"time"
)

// TestJWTAuth tests basic JWT authentication functionality
func TestJWTAuth(t *testing.T) {
	auth := NewJWTAuth("test-secret")

	// Test token generation
	token, expiresAt, err := auth.GenerateToken("test-client", false)
	if err != nil {
		t.Errorf("Expected no error generating token, got %v", err)
	}
	if token == "" {
		t.Error("Expected non-empty token")
	}
	if expiresAt.IsZero() {
		t.Error("Expected valid expiration time")
	}

	// Test token validation
	claims, err := auth.ValidateToken(token)
	if err != nil {
		t.Errorf("Expected no error validating token, got %v", err)
	}
	if claims == nil {
		t.Fatal("Expected claims to be returned")
	}
	if claims.ClientID != "test-client" {
		t.Errorf("Expected ClientID 'test-client', got '%s'", claims.ClientID)
	}
	if claims.IsAdmin {
		t.Error("Expected IsAdmin to be false")
	}

	// Test invalid token
	_, err = auth.ValidateToken("invalid-token")
	if err == nil {
		t.Error("Expected error for invalid token")
	}
}

// TestJWTLibraryMigration tests that the new library-based JWT auth maintains identical behavior
func TestJWTLibraryMigration(t *testing.T) {
	auth := NewJWTAuth("migration-test-secret")

	t.Run("admin_token_generation", func(t *testing.T) {
		// Test admin token generation
		token, expiresAt, err := auth.GenerateToken("admin-client", true)
		if err != nil {
			t.Errorf("Expected no error generating admin token, got %v", err)
		}
		if token == "" {
			t.Error("Expected non-empty admin token")
		}
		if expiresAt.IsZero() {
			t.Error("Expected valid admin expiration time")
		}

		// Validate admin token
		claims, err := auth.ValidateToken(token)
		if err != nil {
			t.Errorf("Expected no error validating admin token, got %v", err)
		}
		if claims == nil {
			t.Fatal("Expected admin claims to be returned")
		}
		if claims.ClientID != "admin-client" {
			t.Errorf("Expected admin ClientID 'admin-client', got '%s'", claims.ClientID)
		}
		if !claims.IsAdmin {
			t.Error("Expected IsAdmin to be true for admin token")
		}
	})

	t.Run("token_expiration_fields", func(t *testing.T) {
		// Test that expiration fields are properly set
		_, expiresAt, err := auth.GenerateToken("expiry-test", false)
		if err != nil {
			t.Errorf("Expected no error generating expiry test token, got %v", err)
		}

		// Should expire in approximately 24 hours
		expectedExpiry := time.Now().Add(24 * time.Hour)
		timeDiff := expiresAt.Sub(expectedExpiry).Abs()
		if timeDiff > time.Minute {
			t.Errorf("Token expiration time off by more than 1 minute: %v", timeDiff)
		}
	})

	t.Run("bearer_token_handling", func(t *testing.T) {
		// Test Bearer prefix handling
		token, _, err := auth.GenerateToken("bearer-test", false)
		if err != nil {
			t.Errorf("Expected no error generating bearer test token, got %v", err)
		}

		// Should work with Bearer prefix
		bearerToken := "Bearer " + token
		claims, err := auth.ValidateToken(bearerToken)
		if err != nil {
			t.Errorf("Expected no error validating bearer token, got %v", err)
		}
		if claims == nil || claims.ClientID != "bearer-test" {
			t.Error("Bearer token validation failed")
		}
	})
}
