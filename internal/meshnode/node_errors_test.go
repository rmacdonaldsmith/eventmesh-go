package meshnode

import (
	"context"
	"errors"
	"testing"

	meshnodepkg "github.com/rmacdonaldsmith/eventmesh-go/pkg/meshnode"
)

func TestGRPCMeshNode_UnsubscribeByIDTypedErrors(t *testing.T) {
	ctx := context.Background()
	node, err := NewGRPCMeshNode(NewConfig("typed-errors-node", "localhost:0"))
	if err != nil {
		t.Fatalf("Failed to create node: %v", err)
	}
	defer func() { _ = node.Close() }()

	if err := node.Start(ctx); err != nil {
		t.Fatalf("Failed to start node: %v", err)
	}

	err = node.UnsubscribeByID(ctx, "missing-client", "missing-subscription")
	if !errors.Is(err, meshnodepkg.ErrSubscriptionNotFound) {
		t.Fatalf("Expected ErrSubscriptionNotFound for missing client subscriptions, got %v", err)
	}

	client := NewTrustedClient("client-with-subscription")
	if err := node.Subscribe(ctx, client, "orders.created"); err != nil {
		t.Fatalf("Failed to subscribe client: %v", err)
	}

	err = node.UnsubscribeByID(ctx, client.ID(), "missing-subscription")
	if !errors.Is(err, meshnodepkg.ErrSubscriptionNotFound) {
		t.Fatalf("Expected ErrSubscriptionNotFound for missing subscription ID, got %v", err)
	}

	node.mu.Lock()
	delete(node.clients, client.ID())
	node.mu.Unlock()

	subscriptions, err := node.GetClientSubscriptions(ctx, client.ID())
	if err != nil {
		t.Fatalf("Failed to get subscriptions: %v", err)
	}
	if len(subscriptions) != 1 {
		t.Fatalf("Expected one subscription before deleting client state, got %d", len(subscriptions))
	}

	err = node.UnsubscribeByID(ctx, client.ID(), subscriptions[0].ID)
	if !errors.Is(err, meshnodepkg.ErrClientNotFound) {
		t.Fatalf("Expected ErrClientNotFound for missing registered client, got %v", err)
	}
}
