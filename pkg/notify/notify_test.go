package notify

import (
	"log/slog"
	"os"
	"testing"
)

func TestEmptyHelperPath(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	n := New(logger, "")

	// Should be a graceful no-op (no panic, no error)
	n.NotifyForward(3000, 3000, "localhost", "python3", "/home/user/projects/myapp")
}

func TestShortPath(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"/home/user/projects/myapp", "projects/myapp"},
		{"/var/app", "var/app"},
		{"/root", "/root"},
		{"relative", "relative"},
		{"/a/b/c/d/e", "d/e"},
	}
	for _, tt := range tests {
		got := shortPath(tt.input)
		if got != tt.want {
			t.Errorf("shortPath(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestFormatSingleForward(t *testing.T) {
	title, body, url := formatSingleForward(ForwardEvent{
		RemotePort: 3000, LocalPort: 3000, Host: "localhost",
		ProcessName: "vite", ProcessCwd: "/home/user/projects/myapp",
	})
	if title != "Port 3000 forwarded" {
		t.Errorf("title = %q", title)
	}
	if body != "localhost:3000 → localhost:3000\nvite in projects/myapp" {
		t.Errorf("body = %q", body)
	}
	if url != "http://localhost:3000" {
		t.Errorf("url = %q", url)
	}
}

func TestFormatForwardRollup(t *testing.T) {
	events := []ForwardEvent{
		{RemotePort: 8080}, {RemotePort: 3000}, {RemotePort: 5173},
	}
	title, body := formatForwardRollup("foxtrotbase", events)
	if title != "3 ports forwarded from foxtrotbase" {
		t.Errorf("title = %q", title)
	}
	// Ports are sorted ascending.
	if body != "3000, 5173, 8080" {
		t.Errorf("body = %q", body)
	}

	// Without a connection name, the title omits the "from" clause.
	title, _ = formatForwardRollup("", events)
	if title != "3 ports forwarded" {
		t.Errorf("anonymous title = %q", title)
	}
}

func TestFormatForwardRollup_TruncatesLongLists(t *testing.T) {
	var events []ForwardEvent
	for p := 3000; p < 3015; p++ { // 15 ports, over the limit of 10
		events = append(events, ForwardEvent{RemotePort: p})
	}
	_, body := formatForwardRollup("host", events)
	if want := ", +5 more"; body[len(body)-len(want):] != want {
		t.Errorf("body = %q, expected it to end with %q", body, want)
	}
}

func TestNonexistentBinary(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	n := New(logger, "/nonexistent/bankshot-notify")

	// Should not panic; the goroutine logs a warning but doesn't block.
	n.NotifyForward(8080, 8080, "localhost", "", "")
}

func TestFormatPortsInUse(t *testing.T) {
	title, body := formatPortsInUse("mini", []PortConflict{{Port: 5000, Command: "ControlCenter"}})
	if title != "Port 5000 not forwarded" {
		t.Errorf("title = %q", title)
	}
	if body != "In use on this machine by ControlCenter. Move the service on mini to another port." {
		t.Errorf("body = %q", body)
	}

	title, body = formatPortsInUse("mini", []PortConflict{{Port: 7000, Command: "ControlCenter"}, {Port: 5000}})
	if title != "2 ports not forwarded from mini" {
		t.Errorf("rollup title = %q", title)
	}
	if body != "In use on this machine: 5000 (another process), 7000 (ControlCenter)" {
		t.Errorf("rollup body = %q", body)
	}
}
