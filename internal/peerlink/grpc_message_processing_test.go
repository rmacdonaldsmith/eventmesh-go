package peerlink

import (
	"context"
	"testing"
	"time"

	"github.com/rmacdonaldsmith/eventmesh-go/pkg/eventlog"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// TestGRPCPeerLink_StartReceiveGoroutine tests the receive goroutine functionality
func TestGRPCPeerLink_StartReceiveGoroutine(t *testing.T) {
	// Create a mock stream setup for testing
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

	// Start server
	err = server.Start(ctx)
	if err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}

	// Start client
	err = client.Start(ctx)
	if err != nil {
		t.Fatalf("Failed to start client: %v", err)
	}

	// Set up receiver for events
	eventChan, errChan := client.ReceiveEvents(ctx)

	// Create connection and establish stream
	conn, err := grpc.NewClient(server.GetListeningAddress(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("Failed to create gRPC connection: %v", err)
	}
	defer conn.Close()

	client.mu.Lock()
	client.registerPeer("server-node")
	client.mu.Unlock()

	testCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Establish stream
	stream, err := client.establishStreamWithHandshake(testCtx, "server-node", conn)
	if err != nil {
		t.Fatalf("Failed to establish stream: %v", err)
	}
	defer stream.CloseSend()

	// Test startReceiveGoroutine - this function doesn't exist yet, so test will fail
	recvDone := client.startReceiveGoroutine(testCtx, "server-node", stream)

	// Send a test event from server to client to verify receive goroutine works
	testEvent := eventlog.NewEvent("test-topic", []byte("hello from server"))
	err = server.SendEvent(testCtx, "client-node", testEvent)
	if err != nil {
		t.Fatalf("Failed to send test event: %v", err)
	}

	// Verify we receive the event through the receive goroutine
	select {
	case receivedEvent := <-eventChan:
		if receivedEvent.Topic != "test-topic" {
			t.Errorf("Expected topic 'test-topic', got '%s'", receivedEvent.Topic)
		}
		if string(receivedEvent.Payload) != "hello from server" {
			t.Errorf("Expected payload 'hello from server', got '%s'", string(receivedEvent.Payload))
		}
		t.Log("✅ startReceiveGoroutine working: received event from peer")
	case err := <-errChan:
		t.Fatalf("Receive error: %v", err)
	case <-time.After(2 * time.Second):
		t.Error("Did not receive event within timeout")
	}

	// Verify recvDone channel works properly
	cancel() // This should cause the receive goroutine to finish
	select {
	case err := <-recvDone:
		if err == nil {
			t.Error("Expected error from receive goroutine on context cancellation")
		}
		t.Log("✅ startReceiveGoroutine correctly finished with error on context cancellation")
	case <-time.After(2 * time.Second):
		t.Error("Receive goroutine did not finish within timeout")
	}
}

// TestGRPCPeerLink_ProcessOutboundMessage tests message processing and sending
func TestGRPCPeerLink_ProcessOutboundMessage(t *testing.T) {
	// This test will fail because processOutboundMessage doesn't exist yet
	clientConfig := &Config{
		NodeID:        "client-node",
		ListenAddress: "localhost:0",
	}
	client, err := NewGRPCPeerLink(clientConfig)
	if err != nil {
		t.Fatalf("Failed to create client PeerLink: %v", err)
	}
	defer client.Close()

	// Create a test message
	testEvent := eventlog.NewEvent("test-topic", []byte("test payload"))
	testMsg := queuedMessage{
		peerID: "test-peer",
		event:  testEvent,
		sentAt: time.Now(),
	}

	// Test processOutboundMessage - this function doesn't exist yet, so test will fail
	err = client.processOutboundMessage("test-peer", testMsg, nil) // nil stream for now

	// We expect this to fail gracefully for nil stream
	if err == nil {
		t.Error("Expected error for nil stream")
	}

	t.Log("✅ processOutboundMessage test setup complete (function doesn't exist yet)")
}

// TestGRPCPeerLink_HandleSendFailure tests send failure handling and requeueing
func TestGRPCPeerLink_HandleSendFailure(t *testing.T) {
	clientConfig := &Config{
		NodeID:        "client-node",
		ListenAddress: "localhost:0",
		SendQueueSize: 10, // Small queue for testing
	}
	client, err := NewGRPCPeerLink(clientConfig)
	if err != nil {
		t.Fatalf("Failed to create client PeerLink: %v", err)
	}
	defer client.Close()

	// Set up client state
	client.mu.Lock()
	client.registerPeer("test-peer")
	sendQueue := client.sendQueues["test-peer"]
	client.mu.Unlock()

	// Create a test message
	testEvent := eventlog.NewEvent("test-topic", []byte("test payload"))
	testMsg := queuedMessage{
		peerID: "test-peer",
		event:  testEvent,
		sentAt: time.Now(),
	}

	// Test handleSendFailure - this function doesn't exist yet, so test will fail
	testError := grpc.ErrServerStopped
	client.handleSendFailure("test-peer", testMsg, sendQueue, testError)

	// Verify message was requeued
	select {
	case requeuedMsg := <-sendQueue:
		if requeuedMsg.event.Topic != "test-topic" {
			t.Errorf("Expected requeued topic 'test-topic', got '%s'", requeuedMsg.event.Topic)
		}
		t.Log("✅ handleSendFailure correctly requeued message")
	case <-time.After(1 * time.Second):
		t.Error("Message was not requeued within timeout")
	}

	// Test drops counter increase when queue is full
	// Fill the queue first
	for i := 0; i < 10; i++ {
		dummyEvent := eventlog.NewEvent("dummy", []byte("dummy"))
		dummyMsg := queuedMessage{
			peerID: "test-peer",
			event:  dummyEvent,
			sentAt: time.Now(),
		}
		select {
		case sendQueue <- dummyMsg:
			// Successfully queued
		default:
			break // Queue full
		}
	}

	// Now handleSendFailure should increment drops count
	initialDrops := client.GetDropsCount("test-peer")
	client.handleSendFailure("test-peer", testMsg, sendQueue, testError)
	finalDrops := client.GetDropsCount("test-peer")

	if finalDrops <= initialDrops {
		t.Errorf("Expected drops count to increase, got initial=%d final=%d", initialDrops, finalDrops)
	}

	t.Log("✅ handleSendFailure correctly incremented drops counter when queue full")
}
