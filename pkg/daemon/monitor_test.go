package daemon

import (
	"encoding/json"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"testing"

	"github.com/phinze/bankshot/pkg/protocol"
)

func TestLocalDaemonClientUsesConfiguredNetwork(t *testing.T) {
	tests := []struct {
		name    string
		network string
		address func(t *testing.T) string
	}{
		{
			name:    "unix",
			network: "unix",
			address: func(t *testing.T) string {
				return filepath.Join(t.TempDir(), "bankshot.sock")
			},
		},
		{
			name:    "tcp",
			network: "tcp",
			address: func(t *testing.T) string {
				return "127.0.0.1:0"
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			listener, err := net.Listen(tt.network, tt.address(t))
			if err != nil {
				t.Fatalf("listen: %v", err)
			}
			defer listener.Close()

			done := make(chan error, 1)
			go func() {
				conn, err := listener.Accept()
				if err != nil {
					done <- err
					return
				}
				defer conn.Close()

				var req protocol.Request
				if err := json.NewDecoder(conn).Decode(&req); err != nil {
					done <- err
					return
				}

				done <- json.NewEncoder(conn).Encode(&protocol.Response{
					ID:      req.ID,
					Success: true,
				})
			}()

			client := &localDaemonClient{
				network: tt.network,
				address: listener.Addr().String(),
				logger:  slog.New(slog.NewTextHandler(os.Stderr, nil)),
			}
			resp, err := client.SendRequest(&protocol.Request{
				ID:   "transport-test",
				Type: protocol.CommandStatus,
			})
			if err != nil {
				t.Fatalf("SendRequest: %v", err)
			}
			if !resp.Success {
				t.Fatal("SendRequest returned unsuccessful response")
			}
			if err := <-done; err != nil {
				t.Fatalf("server: %v", err)
			}
		})
	}
}
