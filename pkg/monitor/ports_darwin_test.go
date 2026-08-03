//go:build darwin

package monitor

import (
	"net"
	"testing"
)

func TestGetListeningPortsIncludesActiveListener(t *testing.T) {
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("net.Listen() error = %v", err)
	}
	defer listener.Close()

	wantPort := listener.Addr().(*net.TCPAddr).Port
	ports, err := GetListeningPorts()
	if err != nil {
		t.Fatalf("GetListeningPorts() error = %v", err)
	}
	for _, port := range ports {
		if port.Port == wantPort {
			return
		}
	}

	t.Fatalf("GetListeningPorts() did not include active port %d", wantPort)
}
