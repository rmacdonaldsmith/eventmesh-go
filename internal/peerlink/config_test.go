package peerlink

import (
	"testing"
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
	if config.HeartbeatInterval <= 0 {
		t.Errorf("Expected positive HeartbeatInterval, got %v", config.HeartbeatInterval)
	}
	if config.MaxMessageSize <= 0 {
		t.Errorf("Expected positive MaxMessageSize, got %d", config.MaxMessageSize)
	}
}