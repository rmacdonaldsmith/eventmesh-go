package peerlink

import (
	"errors"
	"time"
)

// Config holds configuration for PeerLink component
type Config struct {
	NodeID            string
	ListenAddress     string
	SendQueueSize     int
	HeartbeatInterval time.Duration
	MaxMessageSize    int
}

// Validate checks if the configuration is valid
func (c *Config) Validate() error {
	if c.NodeID == "" {
		return errors.New("node ID cannot be empty")
	}
	if c.ListenAddress == "" {
		return errors.New("listen address cannot be empty")
	}
	return nil
}

// SetDefaults sets sensible default values for unset configuration fields
func (c *Config) SetDefaults() {
	if c.SendQueueSize <= 0 {
		c.SendQueueSize = 1000
	}
	if c.HeartbeatInterval <= 0 {
		c.HeartbeatInterval = 5 * time.Second
	}
	if c.MaxMessageSize <= 0 {
		c.MaxMessageSize = 1024 * 1024 // 1MB
	}
}