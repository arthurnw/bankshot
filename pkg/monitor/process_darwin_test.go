//go:build darwin

package monitor

import (
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"testing"
)

func TestResolveProcessInfoForSelf(t *testing.T) {
	pid := os.Getpid()

	if name := ResolveProcessName(pid); name != filepath.Base(os.Args[0]) {
		t.Errorf("ResolveProcessName() = %q, want %q", name, filepath.Base(os.Args[0]))
	}
	if ppid := ResolveParentPID(pid); ppid != os.Getppid() {
		t.Errorf("ResolveParentPID() = %d, want %d", ppid, os.Getppid())
	}

	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	wantCwd, _ := filepath.EvalSymlinks(wd)
	gotCwd, _ := filepath.EvalSymlinks(ResolveProcessCwd(pid))
	if gotCwd != wantCwd {
		t.Errorf("ResolveProcessCwd() = %q, want %q", gotCwd, wantCwd)
	}
}

func TestFindPortOwner(t *testing.T) {
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()

	m := NewSystemMonitor(slog.New(slog.NewTextHandler(os.Stderr, nil)), 0)
	port := listener.Addr().(*net.TCPAddr).Port
	if pid := m.findPortOwner(Port{Port: port, BindAddr: "127.0.0.1"}); pid != os.Getpid() {
		t.Errorf("findPortOwner() = %d, want %d", pid, os.Getpid())
	}
}
