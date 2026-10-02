package forwarder

import (
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"testing"
)

func forwardsByConnection(f *Forwarder) map[string][]int {
	got := make(map[string][]int)
	for _, fwd := range f.ListForwards() {
		got[fwd.ConnectionInfo] = append(got[fwd.ConnectionInfo], fwd.LocalPort)
	}
	for _, ports := range got {
		sort.Ints(ports)
	}
	return got
}

func TestClaimDiscovered(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	f := New(logger, "ssh")

	dir := t.TempDir()
	socket := filepath.Join(dir, "anw@100.122.16.71:22.sock")
	if err := os.WriteFile(socket, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	// FindControlSocket reports the path from `ssh -G`, which may reach the
	// socket through a symlink that discovery did not see.
	alias := filepath.Join(dir, "alias.sock")
	if err := os.Symlink(socket, alias); err != nil {
		t.Fatal(err)
	}

	mustRegister := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	mustRegister(f.RegisterDiscoveredForward(socket, "anw@100.122.16.71:22.sock", 3000, 3000, "localhost"))
	mustRegister(f.RegisterDiscoveredForward(socket, "anw@100.122.16.71:22.sock", 3001, 3001, "localhost"))
	mustRegister(f.RegisterDiscoveredForward("/tmp/other.sock", "other.sock", 4000, 4000, "localhost"))
	// Already tracked under the real name: the discovered copy is a duplicate.
	mustRegister(f.RegisterExistingForward(socket, "mini", 3001, 3001, "localhost"))
	// Not discovered, so it keeps its name even though it shares the socket.
	mustRegister(f.RegisterExistingForward(socket, "manual", 5000, 5000, "localhost"))

	if claimed := f.ClaimDiscovered(alias, "mini"); claimed != 2 {
		t.Errorf("ClaimDiscovered() = %d, want 2", claimed)
	}

	got := forwardsByConnection(f)
	want := map[string][]int{
		"mini":       {3000, 3001},
		"other.sock": {4000},
		"manual":     {5000},
	}
	if len(got) != len(want) {
		t.Fatalf("forwards by connection = %v, want %v", got, want)
	}
	for conn, ports := range want {
		if len(got[conn]) != len(ports) {
			t.Errorf("%s forwards = %v, want %v", conn, got[conn], ports)
			continue
		}
		for i := range ports {
			if got[conn][i] != ports[i] {
				t.Errorf("%s forwards = %v, want %v", conn, got[conn], ports)
			}
		}
	}

	if claimed := f.ClaimDiscovered(socket, "mini"); claimed != 0 {
		t.Errorf("second ClaimDiscovered() = %d, want 0", claimed)
	}
}
