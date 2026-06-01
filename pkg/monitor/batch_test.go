package monitor

import (
	"encoding/json"
	"log/slog"
	"testing"
	"time"

	"github.com/phinze/bankshot/pkg/protocol"
)

// TestBatchForward_CoalescesBurst verifies that a burst of newly-opened ports
// (the initial-connection case) collapses into a single batched forward request
// rather than one request per port.
func TestBatchForward_CoalescesBurst(t *testing.T) {
	client := &mockDaemonClient{}
	sm, _ := NewSessionMonitor(SessionConfig{
		SessionID:       "foxtrotbase",
		DaemonClient:    client,
		Logger:          slog.Default(),
		BatchWindow:     50 * time.Millisecond,
		PortEventSource: &mockPortEventSource{},
	})

	// A burst of ports opening at once (PID 0 bypasses the process filter).
	ports := []int{8080, 3000, 9229, 5173}
	for _, p := range ports {
		sm.handlePortEvent(PortEvent{
			Type: PortOpened, PID: 0, Port: p,
			BindAddr: "0.0.0.0", Timestamp: time.Now(),
		})
	}

	// Nothing should have been sent yet — it's still buffering.
	if got := len(client.requestsOfType(protocol.CommandForwardBatch)); got != 0 {
		t.Fatalf("expected no batch request before flush, got %d", got)
	}

	// Flush the window deterministically rather than sleeping on the timer.
	sm.flushBatch()

	batches := client.requestsOfType(protocol.CommandForwardBatch)
	if len(batches) != 1 {
		t.Fatalf("expected exactly 1 batch request, got %d", len(batches))
	}

	var br protocol.BatchForwardRequest
	if err := json.Unmarshal(batches[0].Payload, &br); err != nil {
		t.Fatalf("failed to unmarshal batch payload: %v", err)
	}
	if br.ConnectionInfo != "foxtrotbase" {
		t.Errorf("ConnectionInfo = %q, want %q", br.ConnectionInfo, "foxtrotbase")
	}
	if len(br.Forwards) != len(ports) {
		t.Fatalf("batch carried %d ports, want %d", len(br.Forwards), len(ports))
	}

	// Ports should be present and sorted ascending for stable display.
	want := []int{3000, 5173, 8080, 9229}
	for i, f := range br.Forwards {
		if f.RemotePort != want[i] {
			t.Errorf("Forwards[%d].RemotePort = %d, want %d", i, f.RemotePort, want[i])
		}
	}

	// Every port should now be tracked as active.
	if got := sm.GetStatus()["activeForwards"]; got != len(ports) {
		t.Errorf("activeForwards = %v, want %d", got, len(ports))
	}

	// A subsequent flush with nothing pending should be a no-op.
	sm.flushBatch()
	if got := len(client.requestsOfType(protocol.CommandForwardBatch)); got != 1 {
		t.Errorf("expected no extra batch on empty flush, got %d total", got)
	}
}

// TestBatchForward_SkipsAlreadyForwarded verifies that a port already tracked as
// forwarded doesn't get re-enqueued into a later batch.
func TestBatchForward_SkipsAlreadyForwarded(t *testing.T) {
	client := &mockDaemonClient{}
	sm, _ := NewSessionMonitor(SessionConfig{
		SessionID:       "foxtrotbase",
		DaemonClient:    client,
		Logger:          slog.Default(),
		BatchWindow:     50 * time.Millisecond,
		PortEventSource: &mockPortEventSource{},
	})

	sm.handlePortEvent(PortEvent{Type: PortOpened, PID: 0, Port: 3000, BindAddr: "0.0.0.0", Timestamp: time.Now()})
	sm.flushBatch()

	// Re-open the same port — it's already active, so nothing new should queue.
	sm.handlePortEvent(PortEvent{Type: PortOpened, PID: 0, Port: 3000, BindAddr: "0.0.0.0", Timestamp: time.Now()})
	sm.flushBatch()

	if got := len(client.requestsOfType(protocol.CommandForwardBatch)); got != 1 {
		t.Errorf("expected 1 batch request for repeated port, got %d", got)
	}
}
