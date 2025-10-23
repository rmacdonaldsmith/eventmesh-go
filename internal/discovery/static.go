package discovery

import (
	"context"

	"github.com/rmacdonaldsmith/eventmesh-go/pkg/peerlink"
)

// StaticDiscovery implements Discovery using a static list of seed nodes
type StaticDiscovery struct {
	seedNodes []string
}

// staticPeerNode implements peerlink.PeerNode for static seed nodes
type staticPeerNode struct {
	id      string
	address string
}

func (p *staticPeerNode) ID() string      { return p.id }
func (p *staticPeerNode) Address() string { return p.address }
func (p *staticPeerNode) IsHealthy() bool { return true } // Static discovery assumes healthy

// NewStaticDiscovery creates a new static discovery service with the given seed nodes
func NewStaticDiscovery(seedNodes []string) *StaticDiscovery {
	return &StaticDiscovery{
		seedNodes: seedNodes,
	}
}

// FindPeers returns peer nodes from the static seed node list
func (s *StaticDiscovery) FindPeers(ctx context.Context) ([]peerlink.PeerNode, error) {
	peers := make([]peerlink.PeerNode, len(s.seedNodes))
	for i, address := range s.seedNodes {
		peers[i] = &staticPeerNode{
			id:      address, // For now, use address as ID
			address: address,
		}
	}
	return peers, nil
}
