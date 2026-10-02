package forwarder

import (
	"log/slog"
	"net"
	"os"
	"os/exec"
	"testing"
	"time"
)

func freePort(t *testing.T) int {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := l.Addr().(*net.TCPAddr).Port
	l.Close()
	return port
}

func TestReconcileConnectionKeepsDeadConnectionThroughGrace(t *testing.T) {
	if _, err := exec.LookPath("ssh"); err != nil {
		t.Skip("ssh command not found")
	}
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	f := New(logger, "ssh")

	// Neither host has a ControlMaster, so `ssh -O check` reports both dead.
	portA, portB := freePort(t), freePort(t)
	if err := f.RegisterExistingForward("/tmp/a.sock", "bankshot-test-dead-a", portA, portA, "localhost"); err != nil {
		t.Fatal(err)
	}
	if err := f.RegisterExistingForward("/tmp/b.sock", "bankshot-test-dead-b", portB, portB, "localhost"); err != nil {
		t.Fatal(err)
	}

	if err := f.ReconcileConnection("bankshot-test-dead-a"); err != nil {
		t.Fatalf("ReconcileConnection() error = %v", err)
	}
	if n := len(f.ListForwards()); n != 2 {
		t.Fatalf("after first pass: %d forwards, want 2 (dead connection within grace)", n)
	}
	for _, fwd := range f.ListForwards() {
		dead := !fwd.deadSince.IsZero()
		if want := fwd.ConnectionInfo == "bankshot-test-dead-a"; dead != want {
			t.Errorf("%s deadSince set = %v, want %v", fwd.ConnectionInfo, dead, want)
		}
	}

	// Age the dead connection past the grace period.
	for _, fwd := range f.ListForwards() {
		if fwd.ConnectionInfo == "bankshot-test-dead-a" {
			fwd.deadSince = time.Now().Add(-deadConnectionGrace - time.Second)
		}
	}
	if err := f.ReconcileConnection("bankshot-test-dead-a"); err != nil {
		t.Fatalf("ReconcileConnection() error = %v", err)
	}
	forwards := f.ListForwards()
	if len(forwards) != 1 || forwards[0].ConnectionInfo != "bankshot-test-dead-b" {
		t.Errorf("after grace: forwards = %+v, want only bankshot-test-dead-b", forwards)
	}
}
