package monitor

import "testing"

func TestParseLsofListeningPorts(t *testing.T) {
	output := []byte("p123\nf7\nn*:8080\nf8\nn127.0.0.1:8081\nf9\nn[::1]:8082\n")

	ports, err := parseLsofListeningPorts(output)
	if err != nil {
		t.Fatalf("parseLsofListeningPorts() error = %v", err)
	}

	want := []Port{
		{Port: 8080, Protocol: "tcp", State: "LISTEN", BindAddr: "0.0.0.0"},
		{Port: 8081, Protocol: "tcp", State: "LISTEN", BindAddr: "127.0.0.1"},
		{Port: 8082, Protocol: "tcp6", State: "LISTEN", BindAddr: "::1"},
	}
	if len(ports) != len(want) {
		t.Fatalf("parseLsofListeningPorts() returned %d ports, want %d", len(ports), len(want))
	}
	for i := range want {
		if ports[i] != want[i] {
			t.Errorf("parseLsofListeningPorts()[%d] = %+v, want %+v", i, ports[i], want[i])
		}
	}
}

func TestParseLsofListeners(t *testing.T) {
	output := []byte("p100\ncnode\nf20\nn*:3000\nf21\nn[::1]:3000\np200\ncssh\nf5\nn127.0.0.1:3000\n")

	listeners, err := parseLsofListeners(output)
	if err != nil {
		t.Fatalf("parseLsofListeners() error = %v", err)
	}

	want := []Listener{
		{PID: 100, Command: "node", Port: 3000, BindAddr: "0.0.0.0"},
		{PID: 100, Command: "node", Port: 3000, BindAddr: "::1"},
		{PID: 200, Command: "ssh", Port: 3000, BindAddr: "127.0.0.1"},
	}
	if len(listeners) != len(want) {
		t.Fatalf("parseLsofListeners() returned %d listeners, want %d", len(listeners), len(want))
	}
	for i := range want {
		if listeners[i] != want[i] {
			t.Errorf("parseLsofListeners()[%d] = %+v, want %+v", i, listeners[i], want[i])
		}
	}
}
