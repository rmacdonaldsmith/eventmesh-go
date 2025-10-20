package meshnode

import (
	"errors"
	"fmt"

	"github.com/rmacdonaldsmith/eventmesh-go/internal/peerlink"
)

// BootstrapConfig represents configuration for node discovery and bootstrapping
type BootstrapConfig struct {
	// SeedNodes is a list of seed node addresses to connect to on startup
	// Format: ["host:port", "host2:port2", ...]
	SeedNodes []string
}

var (
	// ErrEmptyNodeID is returned when node ID is empty
	ErrEmptyNodeID = errors.New("node ID cannot be empty")
	// ErrInvalidListenAddress is returned when listen address is invalid
	ErrInvalidListenAddress = errors.New("listen address cannot be empty")
)

// Config represents configuration for a MeshNode
type Config struct {
	// NodeID uniquely identifies this mesh node
	NodeID string

	// ListenAddress is the address this node listens on for client connections
	// Format: "host:port" (e.g., "localhost:8080")
	ListenAddress string

	// EventLog configuration - will be passed to EventLog component
	EventLogConfig interface{}

	// RoutingTable configuration - will be passed to RoutingTable component
	RoutingTableConfig interface{}

	// PeerLink configuration - will be passed to PeerLink component
	PeerLinkConfig *peerlink.Config

	// Bootstrap configuration - contains discovery settings like seed nodes
	BootstrapConfig *BootstrapConfig
}

// NewConfig creates a new MeshNode configuration with safe defaults
func NewConfig(nodeID, listenAddress string) *Config {
	return &Config{
		NodeID:        nodeID,
		ListenAddress: listenAddress,
		// Use nil configs for components - they'll create their own defaults
		EventLogConfig:     nil,
		RoutingTableConfig: nil,
		PeerLinkConfig:     nil,
		BootstrapConfig:    nil,
	}
}

// Validate validates the configuration and returns an error if invalid
func (c *Config) Validate() error {
	if c.NodeID == "" {
		return ErrEmptyNodeID
	}
	if c.ListenAddress == "" {
		return ErrInvalidListenAddress
	}

	// Validate PeerLink config if provided
	if c.PeerLinkConfig != nil {
		if err := c.PeerLinkConfig.Validate(); err != nil {
			return fmt.Errorf("invalid PeerLink config: %w", err)
		}
	}

	return nil
}

// WithEventLogConfig sets the EventLog configuration
func (c *Config) WithEventLogConfig(config interface{}) *Config {
	c.EventLogConfig = config
	return c
}

// WithRoutingTableConfig sets the RoutingTable configuration
func (c *Config) WithRoutingTableConfig(config interface{}) *Config {
	c.RoutingTableConfig = config
	return c
}

// WithPeerLinkConfig sets the PeerLink configuration
func (c *Config) WithPeerLinkConfig(config *peerlink.Config) *Config {
	c.PeerLinkConfig = config
	return c
}

// WithBootstrapConfig sets the Bootstrap configuration
func (c *Config) WithBootstrapConfig(config *BootstrapConfig) *Config {
	c.BootstrapConfig = config
	return c
}

// NewBootstrapConfig creates a new BootstrapConfig with the specified seed nodes
func NewBootstrapConfig(seedNodes []string) *BootstrapConfig {
	return &BootstrapConfig{
		SeedNodes: seedNodes,
	}
}
