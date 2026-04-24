package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os/exec"
	"testing"

	"github.com/rmacdonaldsmith/eventmesh-go/internal/httpapi"
	"github.com/rmacdonaldsmith/eventmesh-go/internal/meshnode"
)

// TestHTTPIntegration tests that the HTTP API server starts successfully
// and can serve the root endpoint
func TestHTTPIntegration(t *testing.T) {
	config := meshnode.NewConfig("cmd-http-test-node", "localhost:0")
	node, err := meshnode.NewGRPCMeshNode(config)
	if err != nil {
		t.Fatalf("Failed to create mesh node: %v", err)
	}
	defer node.Close()

	server := httpapi.NewServer(node, httpapi.Config{
		Port:      "0",
		SecretKey: "test-secret",
	})

	// Test the HTTP API root endpoint
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	resp := httptest.NewRecorder()
	server.ServeHTTP(resp, req)

	// Verify we get a successful response
	if resp.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.Code)
	}

	// Verify the response is JSON with service info
	var apiInfo map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&apiInfo); err != nil {
		t.Fatalf("Failed to decode JSON response: %v", err)
	}

	// Verify expected fields in the response
	if service, ok := apiInfo["service"].(string); !ok || service != "EventMesh HTTP API" {
		t.Errorf("Expected service 'EventMesh HTTP API', got %v", apiInfo["service"])
	}

	if version, ok := apiInfo["version"].(string); !ok || version == "" {
		t.Errorf("Expected non-empty version, got %v", apiInfo["version"])
	}
}

// TestVersionFlag tests the --version flag
func TestVersionFlag(t *testing.T) {
	cmd := exec.Command("go", "run", ".", "--version")
	output, err := cmd.Output()
	if err != nil {
		t.Fatalf("Failed to run --version: %v", err)
	}

	outputStr := string(output)
	if !contains(outputStr, "EventMesh") || !contains(outputStr, "v0.1.0") {
		t.Errorf("Expected version output to contain 'EventMesh' and 'v0.1.0', got: %s", outputStr)
	}
}

// TestHealthFlag tests the --health flag
func TestHealthFlag(t *testing.T) {
	cmd := exec.Command("go", "run", ".", "--health")
	output, err := cmd.Output()
	if err != nil {
		t.Fatalf("Failed to run --health: %v", err)
	}

	outputStr := string(output)
	if !contains(outputStr, "Health Status") {
		t.Errorf("Expected health output to contain 'Health Status', got: %s", outputStr)
	}
}

// contains checks if s contains substr
func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
