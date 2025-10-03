package peerlink

import (
	"testing"
	"time"
)

// TestConfig_Validation tests our config validation logic
func TestConfig_Validation(t *testing.T) {
	tests := []struct {
		name    string
		config  *Config
		wantErr bool
	}{
		{
			name: "valid config",
			config: &Config{
				NodeID:        "test-node",
				ListenAddress: "localhost:9090",
			},
			wantErr: false,
		},
		{
			name: "empty node ID",
			config: &Config{
				NodeID:        "",
				ListenAddress: "localhost:9090",
			},
			wantErr: true,
		},
		{
			name: "empty listen address",
			config: &Config{
				NodeID:        "test-node",
				ListenAddress: "",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Config.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestConfig_SetDefaults tests that config provides sensible defaults
func TestConfig_SetDefaults(t *testing.T) {
	config := &Config{
		NodeID:        "test-node",
		ListenAddress: "localhost:9090",
	}
	config.SetDefaults()

	if config.SendQueueSize <= 0 {
		t.Errorf("Expected positive SendQueueSize, got %d", config.SendQueueSize)
	}
	if config.SendTimeout <= 0 {
		t.Errorf("Expected positive SendTimeout, got %v", config.SendTimeout)
	}
	if config.HeartbeatInterval <= 0 {
		t.Errorf("Expected positive HeartbeatInterval, got %v", config.HeartbeatInterval)
	}
	if config.MaxMessageSize <= 0 {
		t.Errorf("Expected positive MaxMessageSize, got %d", config.MaxMessageSize)
	}

	// Test specific default values match the design spec
	if config.SendQueueSize != 100 {
		t.Errorf("Expected SendQueueSize default of 100, got %d", config.SendQueueSize)
	}
	if config.SendTimeout.Seconds() != 1.0 {
		t.Errorf("Expected SendTimeout default of 1s, got %v", config.SendTimeout)
	}
	if config.HeartbeatInterval.Seconds() != 5.0 {
		t.Errorf("Expected HeartbeatInterval default of 5s, got %v", config.HeartbeatInterval)
	}
}

// TestConfig_SetDefaults_PreservesExistingValues tests that non-zero values are preserved
func TestConfig_SetDefaults_PreservesExistingValues(t *testing.T) {
	config := &Config{
		NodeID:            "test-node",
		ListenAddress:     "localhost:9090",
		SendQueueSize:     200,
		SendTimeout:       2 * time.Second,
		HeartbeatInterval: 10 * time.Second,
		MaxMessageSize:    2048,
	}
	config.SetDefaults()

	// Verify existing values are preserved
	if config.SendQueueSize != 200 {
		t.Errorf("Expected existing SendQueueSize (200) to be preserved, got %d", config.SendQueueSize)
	}
	if config.SendTimeout != 2*time.Second {
		t.Errorf("Expected existing SendTimeout (2s) to be preserved, got %v", config.SendTimeout)
	}
	if config.HeartbeatInterval != 10*time.Second {
		t.Errorf("Expected existing HeartbeatInterval (10s) to be preserved, got %v", config.HeartbeatInterval)
	}
	if config.MaxMessageSize != 2048 {
		t.Errorf("Expected existing MaxMessageSize (2048) to be preserved, got %d", config.MaxMessageSize)
	}
}