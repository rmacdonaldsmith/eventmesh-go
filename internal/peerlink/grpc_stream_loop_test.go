package peerlink

import (
	"context"
	"testing"
	"time"

	"github.com/rmacdonaldsmith/eventmesh-go/pkg/eventlog"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// TestGRPCPeerLink_RunBidirectionalStreamLoop_ContextCancellation tests context cancellation behavior
func TestGRPCPeerLink_RunBidirectionalStreamLoop_ContextCancellation(t *testing.T) {
	// Create server and client for testing
	serverConfig := &Config{
		NodeID:        "server-node",
		ListenAddress: "localhost:0",
	}
	server, err := NewGRPCPeerLink(serverConfig)
	if err != nil {
		t.Fatalf("Failed to create server PeerLink: %v", err)
	}
	defer server.Close()

	clientConfig := &Config{
		NodeID:        "client-node",
		ListenAddress: "localhost:0",
	}
	client, err := NewGRPCPeerLink(clientConfig)
	if err != nil {
		t.Fatalf("Failed to create client PeerLink: %v", err)
	}
	defer client.Close()

	ctx := context.Background()

	// Start both
	err = server.Start(ctx)
	if err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}

	err = client.Start(ctx)
	if err != nil {
		t.Fatalf("Failed to start client: %v", err)
	}

	// Create connection and establish stream
	conn, err := grpc.NewClient(server.GetListeningAddress(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("Failed to create gRPC connection: %v", err)
	}
	defer conn.Close()

	client.mu.Lock()
	client.registerPeer("server-node")
	sendQueue := client.sendQueues["server-node"]
	client.mu.Unlock()

	testCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// Establish stream
	stream, err := client.establishStreamWithHandshake(testCtx, "server-node", conn)
	if err != nil {
		t.Fatalf("Failed to establish stream: %v", err)
	}
	defer stream.CloseSend()

	// Test runBidirectionalStreamLoop - this function doesn't exist yet, so test will fail
	result := make(chan bool, 1)
	go func() {
		shouldRetry := client.runBidirectionalStreamLoop(testCtx, "server-node", stream, sendQueue)
		result <- shouldRetry
	}()

	// Let it run for a moment
	time.Sleep(100 * time.Millisecond)

	// Cancel context - should cause function to return false (don't retry)
	cancel()

	// Verify result
	select {
	case shouldRetry := <-result:
		if shouldRetry {
			t.Errorf("Expected runBidirectionalStreamLoop to return false (don't retry) on context cancellation, got true")
		}
		t.Log("✅ runBidirectionalStreamLoop correctly returned false on context cancellation")
	case <-time.After(3 * time.Second):
		t.Error("runBidirectionalStreamLoop did not return within timeout")
	}
}

// TestGRPCPeerLink_RunBidirectionalStreamLoop_SendReceive tests message sending and receiving
func TestGRPCPeerLink_RunBidirectionalStreamLoop_SendReceive(t *testing.T) {
	// Create server and client for testing
	serverConfig := &Config{
		NodeID:        "server-node",
		ListenAddress: "localhost:0",
	}
	server, err := NewGRPCPeerLink(serverConfig)
	if err != nil {
		t.Fatalf("Failed to create server PeerLink: %v", err)
	}
	defer server.Close()

	clientConfig := &Config{
		NodeID:        "client-node",
		ListenAddress: "localhost:0",
	}
	client, err := NewGRPCPeerLink(clientConfig)
	if err != nil {
		t.Fatalf("Failed to create client PeerLink: %v", err)
	}
	defer client.Close()

	ctx := context.Background()

	// Start both
	err = server.Start(ctx)
	if err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}

	err = client.Start(ctx)
	if err != nil {
		t.Fatalf("Failed to start client: %v", err)
	}

	// Set up event receiver
	eventChan, errChan := client.ReceiveEvents(ctx)

	// Create connection and establish stream
	conn, err := grpc.NewClient(server.GetListeningAddress(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("Failed to create gRPC connection: %v", err)
	}
	defer conn.Close()

	client.mu.Lock()
	client.registerPeer("server-node")
	sendQueue := client.sendQueues["server-node"]
	client.mu.Unlock()

	testCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Establish stream
	stream, err := client.establishStreamWithHandshake(testCtx, "server-node", conn)
	if err != nil {
		t.Fatalf("Failed to establish stream: %v", err)
	}
	defer stream.CloseSend()

	// Start runBidirectionalStreamLoop in background
	go func() {
		client.runBidirectionalStreamLoop(testCtx, "server-node", stream, sendQueue)
	}()

	// Give it a moment to start
	time.Sleep(100 * time.Millisecond)

	// Test: Send event from server to client (client should receive it through the loop)
	testEvent := eventlog.NewEvent("test-topic", []byte("hello from server"))
	err = server.SendEvent(testCtx, "client-node", testEvent)
	if err != nil {
		t.Fatalf("Failed to send test event: %v", err)
	}

	// Verify client receives the event
	select {
	case receivedEvent := <-eventChan:
		if receivedEvent.Topic != "test-topic" {
			t.Errorf("Expected topic 'test-topic', got '%s'", receivedEvent.Topic)
		}
		if string(receivedEvent.Payload) != "hello from server" {
			t.Errorf("Expected payload 'hello from server', got '%s'", string(receivedEvent.Payload))
		}
		t.Log("✅ runBidirectionalStreamLoop correctly processed received event")
	case err := <-errChan:
		t.Fatalf("Receive error: %v", err)
	case <-time.After(2 * time.Second):
		t.Error("Did not receive event within timeout")
	}

	// Test: Send event from client to server (through the send queue)
	outboundEvent := eventlog.NewEvent("outbound-topic", []byte("hello from client"))
	err = client.SendEvent(testCtx, "server-node", outboundEvent)
	if err != nil {
		t.Fatalf("Failed to send outbound event: %v", err)
	}

	// Let the stream loop process the outbound event
	time.Sleep(100 * time.Millisecond)

	t.Log("✅ runBidirectionalStreamLoop test setup complete")
}

// TestGRPCPeerLink_GetSendQueue tests the helper function for getting send queues
func TestGRPCPeerLink_GetSendQueue(t *testing.T) {
	clientConfig := &Config{
		NodeID:        "client-node",
		ListenAddress: "localhost:0",
	}
	client, err := NewGRPCPeerLink(clientConfig)
	if err != nil {
		t.Fatalf("Failed to create client PeerLink: %v", err)
	}
	defer client.Close()

	// Set up peer
	client.mu.Lock()
	client.registerPeer("test-peer")
	expectedQueue := client.sendQueues["test-peer"]
	client.mu.Unlock()

	// Test getSendQueue - this function doesn't exist yet, so test will fail
	actualQueue := client.getSendQueue("test-peer")

	if actualQueue != expectedQueue {
		t.Errorf("Expected getSendQueue to return the registered queue")
	}

	t.Log("✅ getSendQueue test setup complete")
}
