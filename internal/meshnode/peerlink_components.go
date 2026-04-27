package meshnode

import (
	"context"

	eventlogpkg "github.com/rmacdonaldsmith/eventmesh-go/pkg/eventlog"
	peerlinkpkg "github.com/rmacdonaldsmith/eventmesh-go/pkg/peerlink"
)

type composedPeerLink struct {
	dataPlane   peerlinkpkg.DataPlanePeerLink
	control     peerlinkpkg.ControlPlanePeerLink
	connections peerlinkpkg.PeerConnectionManager
}

func (c composedPeerLink) SendEvent(ctx context.Context, peerID string, event *eventlogpkg.Event) error {
	return c.dataPlane.SendEvent(ctx, peerID, event)
}

func (c composedPeerLink) ReceiveEvents(ctx context.Context) (<-chan *eventlogpkg.Event, <-chan error) {
	return c.dataPlane.ReceiveEvents(ctx)
}

func (c composedPeerLink) SendSubscriptionChange(ctx context.Context, peerID string, change *peerlinkpkg.SubscriptionChange) error {
	return c.control.SendSubscriptionChange(ctx, peerID, change)
}

func (c composedPeerLink) ReceiveSubscriptionChanges(ctx context.Context) (<-chan *peerlinkpkg.SubscriptionChange, <-chan error) {
	return c.control.ReceiveSubscriptionChanges(ctx)
}

func (c composedPeerLink) SendHeartbeat(ctx context.Context, peerID string) error {
	return c.control.SendHeartbeat(ctx, peerID)
}

func (c composedPeerLink) GetPeerHealth(ctx context.Context, peerID string) (peerlinkpkg.PeerHealthState, error) {
	return c.control.GetPeerHealth(ctx, peerID)
}

func (c composedPeerLink) StartHeartbeats(ctx context.Context) error {
	return c.control.StartHeartbeats(ctx)
}

func (c composedPeerLink) StopHeartbeats(ctx context.Context) error {
	return c.control.StopHeartbeats(ctx)
}

func (c composedPeerLink) Connect(ctx context.Context, peer peerlinkpkg.PeerNode) error {
	return c.connections.Connect(ctx, peer)
}

func (c composedPeerLink) Disconnect(ctx context.Context, peerID string) error {
	return c.connections.Disconnect(ctx, peerID)
}

func (c composedPeerLink) GetConnectedPeers(ctx context.Context) ([]peerlinkpkg.PeerNode, error) {
	return c.connections.GetConnectedPeers(ctx)
}

func (c composedPeerLink) Close() error {
	return c.connections.Close()
}

func (c composedPeerLink) GetListeningAddress() string {
	listener, ok := c.connections.(interface {
		GetListeningAddress() string
	})
	if !ok {
		return ""
	}
	return listener.GetListeningAddress()
}

func (c composedPeerLink) GetDropsCount(peerID string) int64 {
	dropCounter, ok := c.dataPlane.(interface {
		GetDropsCount(string) int64
	})
	if !ok {
		return 0
	}
	return dropCounter.GetDropsCount(peerID)
}

var _ peerlinkpkg.CompletePeerLink = composedPeerLink{}
