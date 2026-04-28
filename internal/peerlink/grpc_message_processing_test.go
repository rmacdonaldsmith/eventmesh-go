package peerlink

import (
	"context"
	"testing"
	"time"

	"github.com/rmacdonaldsmith/eventmesh-go/pkg/eventlog"
	peerlinkpkg "github.com/rmacdonaldsmith/eventmesh-go/pkg/peerlink"
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
	case <-time.After(2 * time.Second):
		t.Error("Receive goroutine did not finish within timeout")
	}
}

// TestGRPCPeerLink_ProcessOutboundMessage tests message processing and sending
func TestGRPCPeerLink_ProcessOutboundMessage(t *testing.T) {
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
	testMsg := &DataPlaneMessage{
		peerID: "test-peer",
		event:  testEvent,
		sentAt: time.Now(),
	}

	err = client.processOutboundMessage("test-peer", testMsg, nil)

	// We expect this to fail gracefully for nil stream
	if err == nil {
		t.Error("Expected error for nil stream")
	}
}

// TestGRPCPeerLink_HandleSendFailure tests send failure handling and requeueing
func TestGRPCPeerLink_HandleSendFailure(t *testing.T) {
	clientConfig := &Config{
		NodeID:          "client-node",
		ListenAddress:   "localhost:0",
		SendQueueSize:   10,
		MaxSendAttempts: 2,
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
	testMsg := &DataPlaneMessage{
		peerID: "test-peer",
		event:  testEvent,
		sentAt: time.Now(),
	}

	testError := grpc.ErrServerStopped
	client.handleSendFailure("test-peer", testMsg, sendQueue, testError)

	// Verify message was requeued
	select {
	case requeuedMsg := <-sendQueue:
		if dataMsg, ok := requeuedMsg.(*DataPlaneMessage); !ok || dataMsg.Event().Topic != "test-topic" {
			t.Errorf("Expected requeued topic 'test-topic', got wrong message type or topic")
		}
	case <-time.After(1 * time.Second):
		t.Error("Message was not requeued within timeout")
	}
	if testMsg.SendAttempts() != 1 {
		t.Fatalf("Expected failed message to record 1 send attempt, got %d", testMsg.SendAttempts())
	}
	healthState, err := client.GetPeerHealth(context.Background(), "test-peer")
	if err != nil {
		t.Fatalf("Expected peer health lookup to succeed: %v", err)
	}
	if healthState != peerlinkpkg.PeerUnhealthy {
		t.Fatalf("Expected failed send to mark peer Unhealthy, got %s", healthState)
	}

	// A message is dropped after its configured retry budget is exhausted.
	boundedEvent := eventlog.NewEvent("bounded-retry", []byte("payload"))
	boundedMsg := &DataPlaneMessage{
		peerID: "test-peer",
		event:  boundedEvent,
		sentAt: time.Now(),
	}
	initialDrops := client.GetDropsCount("test-peer")
	client.handleSendFailure("test-peer", boundedMsg, sendQueue, testError)
	<-sendQueue
	client.handleSendFailure("test-peer", boundedMsg, sendQueue, testError)
	<-sendQueue
	client.handleSendFailure("test-peer", boundedMsg, sendQueue, testError)
	if boundedMsg.SendAttempts() != 3 {
		t.Fatalf("Expected bounded message to record 3 failed attempts, got %d", boundedMsg.SendAttempts())
	}
	if finalDrops := client.GetDropsCount("test-peer"); finalDrops != initialDrops+1 {
		t.Fatalf("Expected drops count to increase after max attempts, got initial=%d final=%d", initialDrops, finalDrops)
	}
	select {
	case droppedMsg := <-sendQueue:
		t.Fatalf("Expected message to be dropped after max attempts, but queue received %#v", droppedMsg)
	default:
	}

	// Test drops counter increase when queue is full
	// Fill the queue first
	for i := 0; i < 10; i++ {
		dummyEvent := eventlog.NewEvent("dummy", []byte("dummy"))
		dummyMsg := &DataPlaneMessage{
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
	initialDrops = client.GetDropsCount("test-peer")
	client.handleSendFailure("test-peer", testMsg, sendQueue, testError)
	finalDrops := client.GetDropsCount("test-peer")

	if finalDrops <= initialDrops {
		t.Errorf("Expected drops count to increase, got initial=%d final=%d", initialDrops, finalDrops)
	}

}
