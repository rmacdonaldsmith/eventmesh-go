package meshnode

import (
	"testing"

	"github.com/rmacdonaldsmith/eventmesh-go/internal/peerlink"
)

// TestConfig_NewConfig tests creating new configuration with defaults
func TestConfig_NewConfig(t *testing.T) {
	config := NewConfig("node-1", "localhost:8080")

	if config.NodeID != "node-1" {
		t.Errorf("Expected NodeID 'node-1', got '%s'", config.NodeID)
	}
	if config.ListenAddress != "localhost:8080" {
		t.Errorf("Expected ListenAddress 'localhost:8080', got '%s'", config.ListenAddress)
	}
	if config.EventLogConfig != nil {
		t.Errorf("Expected EventLogConfig to be nil, got %v", config.EventLogConfig)
	}
	if config.RoutingTableConfig != nil {
		t.Errorf("Expected RoutingTableConfig to be nil, got %v", config.RoutingTableConfig)
	}
	if config.PeerLinkConfig != nil {
		t.Errorf("Expected PeerLinkConfig to be nil, got %v", config.PeerLinkConfig)
	}
	if config.BootstrapConfig != nil {
		t.Errorf("Expected BootstrapConfig to be nil, got %v", config.BootstrapConfig)
	}
}

// TestConfig_Validate tests configuration validation
func TestConfig_Validate(t *testing.T) {
	tests := []struct {
		name      string
		config    *Config
		wantError bool
		errorType error
	}{
		{
			name:      "valid config",
			config:    NewConfig("node-1", "localhost:8080"),
			wantError: false,
		},
		{
			name:      "empty node ID",
			config:    NewConfig("", "localhost:8080"),
			wantError: true,
			errorType: ErrEmptyNodeID,
		},
		{
			name:      "empty listen address",
			config:    NewConfig("node-1", ""),
			wantError: true,
			errorType: ErrInvalidListenAddress,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.wantError {
				if err == nil {
					t.Errorf("Expected error for %s, got nil", tt.name)
				}
				if tt.errorType != nil && err != tt.errorType {
					t.Errorf("Expected error %v, got %v", tt.errorType, err)
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error for %s, got %v", tt.name, err)
				}
			}
		})
	}
}

// TestConfig_WithMethods tests the fluent configuration methods
func TestConfig_WithMethods(t *testing.T) {
	config := NewConfig("node-1", "localhost:8080")

	// Test WithEventLogConfig
	eventLogConfig := "some-eventlog-config"
	config = config.WithEventLogConfig(eventLogConfig)
	if config.EventLogConfig != eventLogConfig {
		t.Errorf("Expected EventLogConfig to be set, got %v", config.EventLogConfig)
	}

	// Test WithRoutingTableConfig
	routingTableConfig := "some-routing-config"
	config = config.WithRoutingTableConfig(routingTableConfig)
	if config.RoutingTableConfig != routingTableConfig {
		t.Errorf("Expected RoutingTableConfig to be set, got %v", config.RoutingTableConfig)
	}

	// Test WithPeerLinkConfig
	peerLinkConfig := &peerlink.Config{
		NodeID:        "node-1",
		ListenAddress: "localhost:9090",
	}
	peerLinkConfig.SetDefaults()
	config = config.WithPeerLinkConfig(peerLinkConfig)
	if config.PeerLinkConfig != peerLinkConfig {
		t.Errorf("Expected PeerLinkConfig to be set, got %v", config.PeerLinkConfig)
	}
}

// TestConfig_ValidateWithPeerLinkConfig tests validation with PeerLink configuration
func TestConfig_ValidateWithPeerLinkConfig(t *testing.T) {
	// Valid PeerLink config
	validPeerLinkConfig := &peerlink.Config{
		NodeID:        "node-1",
		ListenAddress: "localhost:9090",
	}
	validPeerLinkConfig.SetDefaults()
	config := NewConfig("node-1", "localhost:8080").WithPeerLinkConfig(validPeerLinkConfig)

	err := config.Validate()
	if err != nil {
		t.Errorf("Expected no error for valid PeerLink config, got %v", err)
	}

	// Invalid PeerLink config (empty node ID)
	invalidPeerLinkConfig := &peerlink.Config{
		NodeID:        "",
		ListenAddress: "localhost:9090",
	}
	invalidPeerLinkConfig.SetDefaults()
	config = NewConfig("node-1", "localhost:8080").WithPeerLinkConfig(invalidPeerLinkConfig)

	err = config.Validate()
	if err == nil {
		t.Error("Expected error for invalid PeerLink config, got nil")
	}
}

// TestBootstrapConfig_Creation tests creating BootstrapConfig with seed nodes
func TestBootstrapConfig_Creation(t *testing.T) {
	// Test creating BootstrapConfig with seed nodes
	seedNodes := []string{"node1:8080", "node2:8080"}
	bootstrapConfig := NewBootstrapConfig(seedNodes)

	if bootstrapConfig == nil {
		t.Fatal("Expected BootstrapConfig to be created, got nil")
	}
	if len(bootstrapConfig.SeedNodes) != 2 {
		t.Errorf("Expected 2 seed nodes, got %d", len(bootstrapConfig.SeedNodes))
	}
	if bootstrapConfig.SeedNodes[0] != "node1:8080" {
		t.Errorf("Expected first seed node 'node1:8080', got '%s'", bootstrapConfig.SeedNodes[0])
	}
	if bootstrapConfig.SeedNodes[1] != "node2:8080" {
		t.Errorf("Expected second seed node 'node2:8080', got '%s'", bootstrapConfig.SeedNodes[1])
	}
}

// TestConfig_WithBootstrapConfig tests setting bootstrap configuration
func TestConfig_WithBootstrapConfig(t *testing.T) {
	config := NewConfig("node-1", "localhost:8080")
	seedNodes := []string{"seed1:8080", "seed2:8080"}
	bootstrapConfig := NewBootstrapConfig(seedNodes)

	config = config.WithBootstrapConfig(bootstrapConfig)

	if config.BootstrapConfig == nil {
		t.Fatal("Expected BootstrapConfig to be set, got nil")
	}
	if len(config.BootstrapConfig.SeedNodes) != 2 {
		t.Errorf("Expected 2 seed nodes, got %d", len(config.BootstrapConfig.SeedNodes))
	}
}
