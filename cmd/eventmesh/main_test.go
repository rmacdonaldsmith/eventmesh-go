package main

import (
	"context"
	"encoding/json"
	"net/http"
	"os/exec"
	"testing"
	"time"
)

// TestHTTPIntegration tests that the HTTP API server starts successfully
// and can serve the root endpoint
func TestHTTPIntegration(t *testing.T) {
	// Skip this test in short mode since it's an integration test
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Start the server in the background
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "go", "run", ".", "--http", "--http-port", "8082")

	// Start the command
	if err := cmd.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}

	// Wait a moment for server to start
	time.Sleep(2 * time.Second)

	// Test the HTTP API root endpoint
	resp, err := http.Get("http://localhost:8082/")
	if err != nil {
		t.Fatalf("Failed to connect to HTTP API: %v", err)
	}
	defer resp.Body.Close()

	// Verify we get a successful response
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
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

	// Stop the server
	if err := cmd.Process.Kill(); err != nil {
		t.Errorf("Failed to kill server process: %v", err)
	}

	// Wait for process to exit
	cmd.Wait()
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
