package forwarder

import (
	"errors"
	"log/slog"
	"os"
	"testing"

	"github.com/phinze/bankshot/pkg/monitor"
)

const testMasterPID = 4242

func newTestForwarder(listeners []monitor.Listener, lookupErr, masterErr error) *Forwarder {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	// A missing ssh binary makes any attempt to run ssh fail the test's
	// expectations, so passing cases prove ssh was never needed.
	f := New(logger, "/nonexistent/ssh")
	f.portListeners = func(int) ([]monitor.Listener, error) { return listeners, lookupErr }
	f.masterPID = func(string) (int, error) { return testMasterPID, masterErr }
	return f
}

func TestCheckLocalPort(t *testing.T) {
	other := monitor.Listener{PID: 100, Command: "node", Port: 3000, BindAddr: "127.0.0.1"}
	master := monitor.Listener{PID: testMasterPID, Command: "ssh", Port: 3000, BindAddr: "::1"}

	tests := []struct {
		name         string
		listeners    []monitor.Listener
		lookupErr    error
		masterErr    error
		wantExisting bool
		wantConflict bool
	}{
		{name: "free port"},
		{name: "lookup unsupported", lookupErr: monitor.ErrListenersUnsupported},
		{name: "LAN-only listener", listeners: []monitor.Listener{{PID: 100, Port: 3000, BindAddr: "192.168.1.5"}}},
		{name: "other process", listeners: []monitor.Listener{other}, wantConflict: true},
		{name: "wildcard listener", listeners: []monitor.Listener{{PID: 100, Command: "node", Port: 3000, BindAddr: "0.0.0.0"}}, wantConflict: true},
		{name: "control master only", listeners: []monitor.Listener{master}, wantExisting: true},
		{name: "control master and other process", listeners: []monitor.Listener{master, other}, wantConflict: true},
		{name: "master unknown", listeners: []monitor.Listener{master}, masterErr: errors.New("no master"), wantConflict: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newTestForwarder(tt.listeners, tt.lookupErr, tt.masterErr)
			existing, err := f.checkLocalPort("test-host", 3000)

			var inUse *PortInUseError
			if gotConflict := errors.As(err, &inUse); gotConflict != tt.wantConflict {
				t.Fatalf("checkLocalPort() error = %v, wantConflict %v", err, tt.wantConflict)
			}
			if existing != tt.wantExisting {
				t.Errorf("checkLocalPort() existing = %v, want %v", existing, tt.wantExisting)
			}
		})
	}
}

func TestAddForwardSkipsPortInUse(t *testing.T) {
	f := newTestForwarder([]monitor.Listener{{PID: 100, Command: "node", Port: 3000, BindAddr: "127.0.0.1"}}, nil, nil)

	created, err := f.AddForward("/tmp/ctl.sock", "test-host", 3000, 3000, "localhost")

	var inUse *PortInUseError
	if !errors.As(err, &inUse) {
		t.Fatalf("AddForward() error = %v, want *PortInUseError", err)
	}
	if created || inUse.Port != 3000 || inUse.Command != "node" {
		t.Errorf("AddForward() = (%v, %+v), want (false, port 3000 held by node)", created, inUse)
	}
	if len(f.ListForwards()) != 0 {
		t.Errorf("AddForward() tracked a skipped forward: %+v", f.ListForwards())
	}
}

func TestAddForwardAdoptsControlMasterListener(t *testing.T) {
	f := newTestForwarder([]monitor.Listener{{PID: testMasterPID, Command: "ssh", Port: 3000, BindAddr: "127.0.0.1"}}, nil, nil)

	created, err := f.AddForward("/tmp/ctl.sock", "test-host", 3000, 3000, "localhost")
	if err != nil || created {
		t.Fatalf("AddForward() = (%v, %v), want (false, nil)", created, err)
	}
	forwards := f.ListForwards()
	if len(forwards) != 1 || forwards[0].LocalPort != 3000 || forwards[0].ConnectionInfo != "test-host" {
		t.Errorf("ListForwards() = %+v, want the adopted forward", forwards)
	}
}

func TestParseMasterPID(t *testing.T) {
	pid, err := parseMasterPID("Master running (pid=32832)\r\n")
	if err != nil || pid != 32832 {
		t.Errorf("parseMasterPID() = (%d, %v), want (32832, nil)", pid, err)
	}
	if _, err := parseMasterPID("Control socket connect(/tmp/x): No such file or directory\n"); err == nil {
		t.Error("parseMasterPID() succeeded on output without a pid")
	}
}
