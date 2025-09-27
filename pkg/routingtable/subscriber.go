package routingtable

// LocalSubscriber represents a local client connected to this mesh node
type LocalSubscriber struct {
	id string
}

// NewLocalSubscriber creates a new local subscriber with the given ID
func NewLocalSubscriber(id string) *LocalSubscriber {
	return &LocalSubscriber{id: id}
}

// ID returns the unique identifier for this subscriber
func (s *LocalSubscriber) ID() string {
	return s.id
}

// Type returns LocalClient to indicate this is a local client subscriber
func (s *LocalSubscriber) Type() SubscriberType {
	return LocalClient
}

// PeerSubscriber represents a remote peer node in the mesh
type PeerSubscriber struct {
	id string
}

// NewPeerSubscriber creates a new peer subscriber with the given node ID
func NewPeerSubscriber(nodeID string) *PeerSubscriber {
	return &PeerSubscriber{id: nodeID}
}

// ID returns the unique identifier for this peer node
func (s *PeerSubscriber) ID() string {
	return s.id
}

// Type returns PeerNode to indicate this is a peer node subscriber
func (s *PeerSubscriber) Type() SubscriberType {
	return PeerNode
}