package daemon

import (
	"os"
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

func TestSocketDirWritableByOthers(t *testing.T) {
	tests := []struct {
		mode os.FileMode
		want bool
	}{
		{0o700, false},
		{0o750, false}, // macOS home directories
		{0o755, false},
		{0o770, true},
		{0o1777, true}, // /tmp
	}
	for _, tt := range tests {
		if got := socketDirWritableByOthers(tt.mode); got != tt.want {
			t.Errorf("socketDirWritableByOthers(%o) = %v, want %v", tt.mode, got, tt.want)
		}
	}
}
