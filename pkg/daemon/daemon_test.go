package daemon

import (
	"testing"

	"github.com/phinze/bankshot/pkg/protocol"
)

func TestIsConnectivityProbe(t *testing.T) {
	tests := []struct {
		name string
		req  *protocol.Request
		want bool
	}{
		{
			name: "monitor status probe",
			req:  &protocol.Request{Type: protocol.CommandStatus, ID: "connectivity-check-123"},
			want: true,
		},
		{
			name: "interactive status",
			req:  &protocol.Request{Type: protocol.CommandStatus, ID: "5cb973e6"},
			want: false,
		},
		{
			name: "other command with probe prefix",
			req:  &protocol.Request{Type: protocol.CommandList, ID: "connectivity-check-123"},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isConnectivityProbe(tt.req); got != tt.want {
				t.Errorf("isConnectivityProbe() = %v, want %v", got, tt.want)
			}
		})
	}
}
