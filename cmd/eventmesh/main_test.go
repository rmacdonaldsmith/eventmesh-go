package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os/exec"
	"strings"
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
	defer func() { _ = node.Close() }()

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

func TestParseConnectPeers(t *testing.T) {
	peers, err := parseConnectPeers("hub=127.0.0.1:9100,backup:9101")
	if err != nil {
		t.Fatalf("parseConnectPeers failed: %v", err)
	}
	if len(peers) != 2 {
		t.Fatalf("Expected 2 peers, got %d", len(peers))
	}
	if peers[0].ID() != "hub" || peers[0].Address() != "127.0.0.1:9100" {
		t.Fatalf("Unexpected first peer: id=%q address=%q", peers[0].ID(), peers[0].Address())
	}
	if peers[1].ID() != "backup:9101" || peers[1].Address() != "backup:9101" {
		t.Fatalf("Unexpected second peer: id=%q address=%q", peers[1].ID(), peers[1].Address())
	}

	if _, err := parseConnectPeers("hub="); err == nil {
		t.Fatal("Expected malformed peer to fail")
	}
}

func TestApplyEventLogConfig(t *testing.T) {
	t.Run("memory_backend_leaves_default_factory", func(t *testing.T) {
		config := meshnode.NewConfig("test-node", "localhost:0")
		if err := applyEventLogConfig(config, "memory", ""); err != nil {
			t.Fatalf("applyEventLogConfig failed: %v", err)
		}
		if config.EventLogFactory != nil {
			t.Fatal("Expected memory backend to leave EventLogFactory unset")
		}
	})

	t.Run("pebble_backend_sets_factory", func(t *testing.T) {
		config := meshnode.NewConfig("test-node", "localhost:0")
		if err := applyEventLogConfig(config, "pebble", t.TempDir()); err != nil {
			t.Fatalf("applyEventLogConfig failed: %v", err)
		}
		if config.EventLogFactory == nil {
			t.Fatal("Expected Pebble backend to set EventLogFactory")
		}

		log, err := config.EventLogFactory()
		if err != nil {
			t.Fatalf("EventLogFactory failed: %v", err)
		}
		defer func() { _ = log.Close() }()

		if _, err := log.AppendEvent(context.Background(), "test.topic", nil); err == nil {
			t.Fatal("Expected created EventLog to reject nil event")
		}
	})

	t.Run("pebble_backend_requires_path", func(t *testing.T) {
		config := meshnode.NewConfig("test-node", "localhost:0")
		if err := applyEventLogConfig(config, "pebble", ""); err == nil {
			t.Fatal("Expected missing Pebble path to fail")
		}
	})

	t.Run("unknown_backend_fails", func(t *testing.T) {
		config := meshnode.NewConfig("test-node", "localhost:0")
		if err := applyEventLogConfig(config, "unknown", ""); err == nil {
			t.Fatal("Expected unknown backend to fail")
		}
	})
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

func TestHealthStatus(t *testing.T) {
	tests := []struct {
		name    string
		healthy bool
		want    string
	}{
		{name: "healthy", healthy: true, want: "✅ Healthy"},
		{name: "unhealthy", healthy: false, want: "❌ Unhealthy"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := healthStatus(tt.healthy); got != tt.want {
				t.Fatalf("healthStatus(%v) = %q, want %q", tt.healthy, got, tt.want)
			}
		})
	}
}

func TestFormatAddress(t *testing.T) {
	tests := []struct {
		name string
		addr string
		want string
	}{
		{name: "empty", addr: "", want: ""},
		{name: "ipv6_unspecified", addr: "[::]:8081", want: "localhost:8081"},
		{name: "ipv4", addr: "127.0.0.1:8081", want: "127.0.0.1:8081"},
		{name: "hostname", addr: "node1:9090", want: "node1:9090"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := formatAddress(tt.addr); got != tt.want {
				t.Fatalf("formatAddress(%q) = %q, want %q", tt.addr, got, tt.want)
			}
		})
	}
}

func TestGetDefaultNodeID(t *testing.T) {
	nodeID := getDefaultNodeID()
	if !strings.HasPrefix(nodeID, "eventmesh-") {
		t.Fatalf("Expected default node ID to start with eventmesh-, got %q", nodeID)
	}
}

// contains checks if s contains substr
func contains(s, substr string) bool {
	return strings.Contains(s, substr)
}
