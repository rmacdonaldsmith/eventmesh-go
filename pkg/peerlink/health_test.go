package peerlink

import "testing"

func TestPeerHealthStateString(t *testing.T) {
	tests := []struct {
		name  string
		state PeerHealthState
		want  string
	}{
		{name: "healthy", state: PeerHealthy, want: "Healthy"},
		{name: "unhealthy", state: PeerUnhealthy, want: "Unhealthy"},
		{name: "disconnected", state: PeerDisconnected, want: "Disconnected"},
		{name: "unknown", state: PeerHealthState(99), want: "Unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.state.String(); got != tt.want {
				t.Fatalf("PeerHealthState(%d).String() = %q, want %q", tt.state, got, tt.want)
			}
		})
	}
}
