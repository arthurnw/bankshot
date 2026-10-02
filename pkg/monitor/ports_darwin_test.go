//go:build darwin

package monitor

import (
	"net"
	"os"
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

func TestPortListenersReportsOwningProcess(t *testing.T) {
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("net.Listen() error = %v", err)
	}
	defer listener.Close()

	port := listener.Addr().(*net.TCPAddr).Port
	listeners, err := PortListeners(port)
	if err != nil {
		t.Fatalf("PortListeners() error = %v", err)
	}
	if len(listeners) != 1 {
		t.Fatalf("PortListeners(%d) = %+v, want one listener", port, listeners)
	}
	if got := listeners[0]; got.PID != os.Getpid() || got.BindAddr != "127.0.0.1" || got.Command == "" {
		t.Errorf("PortListeners(%d)[0] = %+v, want this process on 127.0.0.1", port, got)
	}
}
