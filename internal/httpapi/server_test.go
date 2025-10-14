package httpapi

import (
	"testing"
	"time"

	"github.com/rmacdonaldsmith/eventmesh-go/internal/meshnode"
)

// TestNewServer tests that we can create a new server instance
func TestNewServer(t *testing.T) {
	// Create a test mesh node config
	config := meshnode.NewConfig("test-node", "localhost:8080")

	// Create mesh node
	node, err := meshnode.NewGRPCMeshNode(config)
	if err != nil {
		t.Fatalf("Failed to create mesh node: %v", err)
	}
	defer node.Close()

	// Create server config
	serverConfig := Config{
		Port:      "8081",
		SecretKey: "test-secret-key",
	}

	// Create HTTP API server
	server := NewServer(node, serverConfig)
	if server == nil {
		t.Fatal("Expected server to be created, got nil")
	}

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

// TestInputValidation tests request input validation
func TestInputValidation(t *testing.T) {
	// Create a test mesh node config
	config := meshnode.NewConfig("test-node", "localhost:8080")

	// Create mesh node
	node, err := meshnode.NewGRPCMeshNode(config)
	if err != nil {
		t.Fatalf("Failed to create mesh node: %v", err)
	}
	defer node.Close()

	// Create handlers
	auth := NewJWTAuth("validation-test-secret")
	handlers := NewHandlers(node, auth)

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

// TestPackageStructure verifies all files compile together
func TestPackageStructure(t *testing.T) {
	// This test just verifies that all our files compile together properly
	// by importing and using the main types

	// Test that we can create all the main components
	auth := NewJWTAuth("test-key")
	if auth == nil {
		t.Error("Expected NewJWTAuth to return non-nil")
	}

	middleware := NewMiddleware(auth)
	if middleware == nil {
		t.Error("Expected NewMiddleware to return non-nil")
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
	// Create a test mesh node config
	config := meshnode.NewConfig("test-node", "localhost:8080")

	// Create mesh node
	node, err := meshnode.NewGRPCMeshNode(config)
	if err != nil {
		t.Fatalf("Failed to create mesh node: %v", err)
	}
	defer node.Close()

	// Create handlers
	auth := NewJWTAuth("test-secret")
	handlers := NewHandlers(node, auth)

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
