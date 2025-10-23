package peerlink

import (
	"context"
	"testing"
	"time"

	"github.com/rmacdonaldsmith/eventmesh-go/pkg/peerlink"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// TestGRPCPeerLink_EstablishStreamWithHandshake_Success tests successful stream establishment and handshake
func TestGRPCPeerLink_EstablishStreamWithHandshake_Success(t *testing.T) {
	// Create a server PeerLink to act as the peer
	serverConfig := &Config{
		NodeID:        "server-node",
		ListenAddress: "localhost:0",
	}
	server, err := NewGRPCPeerLink(serverConfig)
	if err != nil {
		t.Fatalf("Failed to create server PeerLink: %v", err)
	}
	defer server.Close()

	ctx := context.Background()

	// Start server
	err = server.Start(ctx)
	if err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}

	// Create client PeerLink
	clientConfig := &Config{
		NodeID:        "client-node",
		ListenAddress: "localhost:0",
	}
	client, err := NewGRPCPeerLink(clientConfig)
	if err != nil {
		t.Fatalf("Failed to create client PeerLink: %v", err)
	}
	defer client.Close()

	// Create gRPC connection to server
	conn, err := grpc.NewClient(server.GetListeningAddress(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("Failed to create gRPC connection: %v", err)
	}
	defer conn.Close()

	// Set up client state
	client.mu.Lock()
	client.registerPeer("server-node")
	client.mu.Unlock()

	testCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	// Test the new establishStreamWithHandshake function
	stream, err := client.establishStreamWithHandshake(testCtx, "server-node", conn)
	if err != nil {
		t.Fatalf("Expected establishStreamWithHandshake to succeed, got error: %v", err)
	}
	if stream == nil {
		t.Fatal("Expected non-nil stream from establishStreamWithHandshake")
	}

	// Verify peer is marked as healthy after successful handshake
	health, err := client.GetPeerHealth(context.Background(), "server-node")
	if err != nil {
		t.Fatalf("Failed to get peer health: %v", err)
	}
	if health != peerlink.PeerHealthy {
		t.Errorf("Expected peer to be healthy after successful handshake, got: %s", health)
	}

	t.Log("✅ establishStreamWithHandshake succeeded and peer marked as healthy")

	// Clean up stream
	stream.CloseSend()
}

// TestGRPCPeerLink_EstablishStreamWithHandshake_ConnectionFailure tests connection failure handling
func TestGRPCPeerLink_EstablishStreamWithHandshake_ConnectionFailure(t *testing.T) {
	clientConfig := &Config{
		NodeID:        "client-node",
		ListenAddress: "localhost:0",
	}
	client, err := NewGRPCPeerLink(clientConfig)
	if err != nil {
		t.Fatalf("Failed to create client PeerLink: %v", err)
	}
	defer client.Close()

	// Try to connect to a non-existent server
	conn, err := grpc.NewClient("localhost:99999", grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("Failed to create gRPC connection: %v", err)
	}
	defer conn.Close()

	// Set up client state
	client.mu.Lock()
	client.registerPeer("nonexistent-node")
	client.mu.Unlock()

	testCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// Test establishStreamWithHandshake - should return error
	stream, err := client.establishStreamWithHandshake(testCtx, "nonexistent-node", conn)
	if err == nil {
		t.Errorf("Expected establishStreamWithHandshake to fail on connection error, got success")
	}
	if stream != nil {
		t.Errorf("Expected nil stream on failure, got non-nil stream")
		stream.CloseSend()
	}

	t.Log("✅ establishStreamWithHandshake correctly failed on connection error")
}

// TestGRPCPeerLink_EstablishStreamWithHandshake_HandshakeFailure tests handshake-specific failures
func TestGRPCPeerLink_EstablishStreamWithHandshake_HandshakeFailure(t *testing.T) {
	clientConfig := &Config{
		NodeID:        "client-node",
		ListenAddress: "localhost:0",
	}
	client, err := NewGRPCPeerLink(clientConfig)
	if err != nil {
		t.Fatalf("Failed to create client PeerLink: %v", err)
	}
	defer client.Close()

	// Create connection to invalid address (will cause dial errors)
	conn, err := grpc.NewClient("127.0.0.1:1", grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("Failed to create gRPC connection: %v", err)
	}
	defer conn.Close()

	// Set up client state
	client.mu.Lock()
	client.registerPeer("invalid-node")
	client.mu.Unlock()

	testCtx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	// Test establishStreamWithHandshake - should return error
	stream, err := client.establishStreamWithHandshake(testCtx, "invalid-node", conn)
	if err == nil {
		t.Errorf("Expected establishStreamWithHandshake to fail on handshake error, got success")
	}
	if stream != nil {
		t.Errorf("Expected nil stream on handshake failure, got non-nil stream")
		stream.CloseSend()
	}

	t.Log("✅ establishStreamWithHandshake correctly failed on handshake error")
}
