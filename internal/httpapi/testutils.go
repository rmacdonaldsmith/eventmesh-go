package httpapi

import (
	"context"
	"net/http/httptest"
	"testing"

	"github.com/rmacdonaldsmith/eventmesh-go/internal/meshnode"
)

// TestServerSetup holds common test dependencies
type TestServerSetup struct {
	Node   *meshnode.GRPCMeshNode
	Server *Server
	Auth   *JWTAuth
}

// NewTestServerSetup creates a common test setup with mesh node and HTTP server
func NewTestServerSetup(t *testing.T) *TestServerSetup {
	t.Helper()

	// Create test mesh node
	config := meshnode.NewConfig("test-node", "localhost:8080")
	node, err := meshnode.NewGRPCMeshNode(config)
	if err != nil {
		t.Fatalf("Failed to create mesh node: %v", err)
	}

	// Start the mesh node
	ctx := context.Background()
	if err := node.Start(ctx); err != nil {
		t.Fatalf("Failed to start mesh node: %v", err)
	}

	// Create HTTP server
	serverConfig := Config{
		Port:      "8081",
		SecretKey: "test-secret-key",
	}

	server := NewServer(node, serverConfig)
	if server == nil {
		t.Fatal("Expected server to be created, got nil")
	}

	return &TestServerSetup{
		Node:   node,
		Server: server,
		Auth:   server.jwtAuth,
	}
}

// Close cleans up test resources
func (setup *TestServerSetup) Close() {
	setup.Node.Close()
}

// GenerateTestToken creates a JWT token for testing
func (setup *TestServerSetup) GenerateTestToken(t *testing.T, clientID string, isAdmin bool) string {
	t.Helper()

	token, _, err := setup.Auth.GenerateToken(clientID, isAdmin)
	if err != nil {
		t.Fatalf("Failed to generate test token: %v", err)
	}
	return token
}

// StreamingRecorder captures streaming data for testing SSE endpoints
type StreamingRecorder struct {
	*httptest.ResponseRecorder
	Data chan string
	Done chan struct{}
}

// NewStreamingRecorder creates a new streaming recorder for SSE testing
func NewStreamingRecorder() *StreamingRecorder {
	return &StreamingRecorder{
		ResponseRecorder: httptest.NewRecorder(),
		Data:             make(chan string, 100), // Buffered to prevent blocking
		Done:             make(chan struct{}),
	}
}

// Write implements io.Writer for capturing streaming data
func (r *StreamingRecorder) Write(data []byte) (int, error) {
	// Send data to channel for testing
	select {
	case r.Data <- string(data):
	default:
		// Channel full, skip
	}

	// Also write to the underlying recorder
	return r.ResponseRecorder.Write(data)
}

// Close signals that streaming is complete
func (r *StreamingRecorder) Close() {
	close(r.Done)
}
