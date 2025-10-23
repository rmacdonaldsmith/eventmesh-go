package discovery

import (
	"context"

	"github.com/rmacdonaldsmith/eventmesh-go/pkg/peerlink"
)

// Discovery defines the interface for node discovery mechanisms
type Discovery interface {
	// FindPeers discovers and returns available peer nodes
	FindPeers(ctx context.Context) ([]peerlink.PeerNode, error)
}
