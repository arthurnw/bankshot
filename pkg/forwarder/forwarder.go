package forwarder

import (
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/phinze/bankshot/pkg/monitor"
)

// Forward represents an active port forward
type Forward struct {
	RemotePort     int
	LocalPort      int
	Host           string
	SocketPath     string
	ConnectionInfo string // SSH connection target (e.g., hostname)
	CreatedAt      time.Time

	// discovered marks a forward found by startup discovery whose
	// ConnectionInfo is a placeholder derived from the control socket name.
	discovered bool
}

// Forwarder manages SSH port forwards
type Forwarder struct {
	logger   *slog.Logger
	sshCmd   string
	forwards map[string]*Forward // key: "host:remotePort"
	mu       sync.RWMutex

	// Replaceable in tests.
	portListeners func(port int) ([]monitor.Listener, error)
	masterPID     func(connectionInfo string) (int, error)
}

// PortInUseError reports that a local process other than the connection's
// ControlMaster already listens on the port a forward would bind. Forwarding
// anyway would shadow that service on some or all loopback addresses.
type PortInUseError struct {
	Port    int
	PID     int
	Command string
}

func (e *PortInUseError) Error() string {
	return fmt.Sprintf("local port %d is in use by %s (pid %d)", e.Port, e.Command, e.PID)
}

// New creates a new Forwarder
func New(logger *slog.Logger, sshCmd string) *Forwarder {
	f := &Forwarder{
		logger:        logger,
		sshCmd:        sshCmd,
		forwards:      make(map[string]*Forward),
		portListeners: monitor.PortListeners,
	}
	f.masterPID = f.controlMasterPID
	return f
}

// AddForward creates a new port forward.
// Returns (true, nil) when a new forward is established, (false, nil) when the
// port was already forwarded, or (false, err) on failure.
func (f *Forwarder) AddForward(socketPath string, connectionInfo string, remotePort, localPort int, host string) (bool, error) {
	if host == "" {
		host = "localhost"
	}
	if localPort == 0 {
		localPort = remotePort
	}

	// Include connection info in key to support multiple SSH sessions
	key := fmt.Sprintf("%s:%s:%d", connectionInfo, host, remotePort)

	// Check if already forwarded
	f.mu.RLock()
	if existing, ok := f.forwards[key]; ok {
		f.mu.RUnlock()
		f.logger.Info("Port already forwarded",
			"remote", fmt.Sprintf("%s:%d", host, remotePort),
			"local", existing.LocalPort,
		)
		return false, nil
	}
	f.mu.RUnlock()

	existing, err := f.checkLocalPort(connectionInfo, localPort)
	if err != nil {
		f.logger.Warn("Not forwarding: local port in use",
			"remote", fmt.Sprintf("%s:%d", host, remotePort),
			"local", localPort,
			"error", err,
		)
		return false, err
	}
	if existing {
		return false, f.RegisterExistingForward(socketPath, connectionInfo, remotePort, localPort, host)
	}

	// Execute SSH forward command
	cmd := exec.Command(f.sshCmd,
		"-O", "forward",
		"-L", fmt.Sprintf("%d:%s:%d", localPort, host, remotePort),
		connectionInfo,
	)

	f.logger.Info("Executing port forward",
		"command", strings.Join(cmd.Args, " "),
		"remote", fmt.Sprintf("%s:%d", host, remotePort),
		"local", localPort,
		"socketPath", socketPath,
		"connectionInfo", connectionInfo,
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return false, fmt.Errorf("failed to forward port: %w (output: %s)", err, string(output))
	}

	// Store forward info
	forward := &Forward{
		RemotePort:     remotePort,
		LocalPort:      localPort,
		Host:           host,
		SocketPath:     socketPath,
		ConnectionInfo: connectionInfo,
		CreatedAt:      time.Now(),
	}

	f.mu.Lock()
	f.forwards[key] = forward
	f.mu.Unlock()

	f.logger.Info("Port forward established",
		"remote", fmt.Sprintf("%s:%d", host, remotePort),
		"local", localPort,
	)

	return true, nil
}

// checkLocalPort looks for loopback listeners on localPort. It returns a
// *PortInUseError when another process holds the port, and existing=true when
// only the connection's ControlMaster does: that is an earlier forward that
// outlived a daemon restart. When listener processes cannot be determined, it
// reports no conflict and leaves the outcome to ssh.
func (f *Forwarder) checkLocalPort(connectionInfo string, localPort int) (existing bool, err error) {
	listeners, lookupErr := f.portListeners(localPort)
	if lookupErr != nil {
		f.logger.Debug("Skipping local port check", "port", localPort, "error", lookupErr)
		return false, nil
	}

	var local []monitor.Listener
	for _, l := range listeners {
		if monitor.IsLocalAddr(l.BindAddr) {
			local = append(local, l)
		}
	}
	if len(local) == 0 {
		return false, nil
	}

	masterPID, pidErr := f.masterPID(connectionInfo)
	for _, l := range local {
		if pidErr != nil || l.PID != masterPID {
			return false, &PortInUseError{Port: localPort, PID: l.PID, Command: l.Command}
		}
	}
	return true, nil
}

// controlMasterPID returns the PID of the ControlMaster for connectionInfo,
// parsed from `ssh -O check`, which prints "Master running (pid=N)".
func (f *Forwarder) controlMasterPID(connectionInfo string) (int, error) {
	output, err := exec.Command(f.sshCmd, "-O", "check", connectionInfo).CombinedOutput()
	if err != nil {
		return 0, fmt.Errorf("ssh -O check %s: %w", connectionInfo, err)
	}
	return parseMasterPID(string(output))
}

func parseMasterPID(output string) (int, error) {
	_, rest, ok := strings.Cut(output, "pid=")
	if !ok {
		return 0, fmt.Errorf("no pid in ssh -O check output %q", output)
	}
	digits, _, _ := strings.Cut(rest, ")")
	pid, err := strconv.Atoi(digits)
	if err != nil {
		return 0, fmt.Errorf("parse ssh -O check pid %q: %w", digits, err)
	}
	return pid, nil
}

// RegisterExistingForward registers a forward that already exists under a
// known connection name, without running ssh.
func (f *Forwarder) RegisterExistingForward(socketPath string, connectionInfo string, remotePort, localPort int, host string) error {
	return f.registerExisting(socketPath, connectionInfo, remotePort, localPort, host, false)
}

// RegisterDiscoveredForward registers a forward found by startup discovery.
// Discovery sees only the control socket, so connectionInfo is a placeholder
// until ClaimDiscovered renames the forward.
func (f *Forwarder) RegisterDiscoveredForward(socketPath string, connectionInfo string, remotePort, localPort int, host string) error {
	return f.registerExisting(socketPath, connectionInfo, remotePort, localPort, host, true)
}

// ClaimDiscovered renames discovered forwards on socketPath to connectionInfo,
// the name the remote uses for that connection. Without this, a forward found
// at startup and the remote's later request for it are tracked twice, and the
// discovered copy is never reconciled because its name matches no remote.
// It returns the number of forwards claimed.
func (f *Forwarder) ClaimDiscovered(socketPath, connectionInfo string) int {
	if socketPath == "" || connectionInfo == "" {
		return 0
	}
	want := canonicalSocketPath(socketPath)

	f.mu.Lock()
	defer f.mu.Unlock()

	claimed := 0
	for key, fwd := range f.forwards {
		if !fwd.discovered || canonicalSocketPath(fwd.SocketPath) != want {
			continue
		}
		delete(f.forwards, key)
		claimed++

		newKey := fmt.Sprintf("%s:%s:%d", connectionInfo, fwd.Host, fwd.RemotePort)
		if _, exists := f.forwards[newKey]; exists {
			continue
		}
		fwd.ConnectionInfo = connectionInfo
		fwd.discovered = false
		f.forwards[newKey] = fwd
	}

	if claimed > 0 {
		f.logger.Info("Claimed discovered forwards",
			"connectionInfo", connectionInfo,
			"socketPath", socketPath,
			"count", claimed,
		)
	}
	return claimed
}

func canonicalSocketPath(path string) string {
	if resolved, err := filepath.EvalSymlinks(path); err == nil {
		return resolved
	}
	return filepath.Clean(path)
}

func (f *Forwarder) registerExisting(socketPath string, connectionInfo string, remotePort, localPort int, host string, discovered bool) error {
	if host == "" {
		host = "localhost"
	}
	if localPort == 0 {
		localPort = remotePort
	}

	// Include connection info in key to support multiple SSH sessions
	key := fmt.Sprintf("%s:%s:%d", connectionInfo, host, remotePort)

	// Check if already registered
	f.mu.RLock()
	if existing, ok := f.forwards[key]; ok {
		f.mu.RUnlock()
		f.logger.Debug("Forward already registered",
			"remote", fmt.Sprintf("%s:%d", host, remotePort),
			"local", existing.LocalPort,
		)
		return nil
	}
	f.mu.RUnlock()

	// Store forward info without executing SSH command
	forward := &Forward{
		RemotePort:     remotePort,
		LocalPort:      localPort,
		Host:           host,
		SocketPath:     socketPath,
		ConnectionInfo: connectionInfo,
		CreatedAt:      time.Now(),
		discovered:     discovered,
	}

	f.mu.Lock()
	f.forwards[key] = forward
	f.mu.Unlock()

	f.logger.Info("Registered existing forward",
		"remote", fmt.Sprintf("%s:%d", host, remotePort),
		"local", localPort,
		"connectionInfo", connectionInfo,
	)

	return nil
}

// RemoveForward removes a port forward
func (f *Forwarder) RemoveForward(connectionInfo string, remotePort int, host string) error {
	if host == "" {
		host = "localhost"
	}

	// Include connection info in key to support multiple SSH sessions
	key := fmt.Sprintf("%s:%s:%d", connectionInfo, host, remotePort)

	// Get forward info
	f.mu.RLock()
	forward, ok := f.forwards[key]
	if !ok {
		f.mu.RUnlock()
		return fmt.Errorf("forward not found: %s", key)
	}
	localPort := forward.LocalPort
	f.mu.RUnlock()

	// Execute SSH cancel command
	// WARNING: OpenSSH has a limitation where -O cancel will cancel ALL remote
	// socket forwards on the control socket, not just the specified one. This
	// includes any Unix socket forwards (like .bankshot.sock). See below for our
	// workaround to address this.
	cmd := exec.Command(f.sshCmd,
		"-O", "cancel",
		"-L", fmt.Sprintf("%d:%s:%d", localPort, host, remotePort),
		connectionInfo,
	)

	f.logger.Info("Canceling port forward",
		"command", strings.Join(cmd.Args, " "),
		"remote", fmt.Sprintf("%s:%d", host, remotePort),
		"local", localPort,
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		// Log but don't fail - forward might already be gone
		f.logger.Warn("Failed to cancel port forward",
			"error", err,
			"output", string(output),
		)
	}

	// Remove from map
	f.mu.Lock()
	delete(f.forwards, key)
	f.mu.Unlock()

	// Re-establish all configured forwards (including Unix socket forwards)
	// This is necessary because SSH -O cancel removes ALL socket remote forwards
	reestablishCmd := exec.Command(f.sshCmd, "-O", "forward", connectionInfo)

	f.logger.Info("Re-establishing configured forwards after cancel",
		"command", strings.Join(reestablishCmd.Args, " "),
	)

	reestablishOutput, reestablishErr := reestablishCmd.CombinedOutput()
	if reestablishErr != nil {
		f.logger.Error("Failed to re-establish forwards",
			"error", reestablishErr,
			"output", string(reestablishOutput),
		)
		// Don't fail the operation - the forward was still removed
	} else {
		f.logger.Info("Successfully re-established configured forwards",
			"output", string(reestablishOutput),
		)
	}

	return nil
}

// ListForwards returns all active forwards
func (f *Forwarder) ListForwards() []*Forward {
	f.mu.RLock()
	defer f.mu.RUnlock()

	forwards := make([]*Forward, 0, len(f.forwards))
	for _, fwd := range f.forwards {
		forwards = append(forwards, fwd)
	}
	return forwards
}

// FindControlSocket finds the SSH ControlMaster socket for a given connection
func FindControlSocket(connectionInfo string) (string, error) {
	// First, verify the connection is active
	checkCmd := exec.Command("ssh", "-O", "check", connectionInfo)
	if err := checkCmd.Run(); err != nil {
		return "", fmt.Errorf("no active SSH connection to %s", connectionInfo)
	}

	// Use ssh -G to get the actual configuration
	cmd := exec.Command("ssh", "-G", connectionInfo)
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get SSH config for %s: %w", connectionInfo, err)
	}

	// Parse the output to find ControlPath
	var controlPath string
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		parts := strings.Fields(line)
		if len(parts) >= 2 && parts[0] == "controlpath" {
			controlPath = strings.Join(parts[1:], " ")
			break
		}
	}

	if controlPath == "" {
		return "", fmt.Errorf("no ControlPath configured for %s", connectionInfo)
	}

	// The control path might contain % tokens that need to be expanded
	// ssh -G should have already expanded them, but let's verify the socket exists
	if _, err := os.Stat(controlPath); err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("control socket does not exist at %s", controlPath)
		}
		return "", fmt.Errorf("failed to stat control socket: %w", err)
	}

	// Verify it's actually a socket
	info, err := os.Stat(controlPath)
	if err != nil {
		return "", fmt.Errorf("failed to stat control socket: %w", err)
	}
	if info.Mode()&os.ModeSocket == 0 {
		return "", fmt.Errorf("path %s exists but is not a socket", controlPath)
	}

	return controlPath, nil
}

// CleanupForSocket removes all forwards for a specific socket
func (f *Forwarder) CleanupForSocket(socketPath string) {
	f.mu.RLock()
	var toRemove []struct {
		key            string
		connectionInfo string
		host           string
		port           int
	}
	for key, forward := range f.forwards {
		if forward.SocketPath == socketPath {
			toRemove = append(toRemove, struct {
				key            string
				connectionInfo string
				host           string
				port           int
			}{
				key:            key,
				connectionInfo: forward.ConnectionInfo,
				host:           forward.Host,
				port:           forward.RemotePort,
			})
		}
	}
	f.mu.RUnlock()

	for _, item := range toRemove {
		_ = f.RemoveForward(item.connectionInfo, item.port, item.host)
	}
}

// CleanupForConnection removes all forwards for a specific connection
func (f *Forwarder) CleanupForConnection(connectionInfo string) {
	f.mu.RLock()
	var toRemove []struct {
		host string
		port int
	}
	for _, forward := range f.forwards {
		if forward.ConnectionInfo == connectionInfo {
			toRemove = append(toRemove, struct {
				host string
				port int
			}{
				host: forward.Host,
				port: forward.RemotePort,
			})
		}
	}
	f.mu.RUnlock()

	for _, item := range toRemove {
		_ = f.RemoveForward(connectionInfo, item.port, item.host)
	}
}

// ListConnectionForwards returns forwards for a specific connection
func (f *Forwarder) ListConnectionForwards(connectionInfo string) []*Forward {
	f.mu.RLock()
	defer f.mu.RUnlock()

	var forwards []*Forward
	for _, fwd := range f.forwards {
		if fwd.ConnectionInfo == connectionInfo {
			forwards = append(forwards, fwd)
		}
	}
	return forwards
}

// Reconcile validates that tracked forwards still have active local ports listening.
// For stale forwards (port not listening), it attempts to re-establish them if the
// SSH connection is still alive, or removes them if the connection is dead.
// This helps recover from SSH reconnections that tear down port forwards.
func (f *Forwarder) Reconcile() error {
	// Get all listening ports on the system
	listeningPorts, err := monitor.GetListeningPorts()
	if err != nil {
		return fmt.Errorf("failed to get listening ports: %w", err)
	}

	// Build a set of listening port numbers for quick lookup
	portSet := make(map[int]bool)
	for _, port := range listeningPorts {
		portSet[port.Port] = true
	}

	// Find forwards that need attention (not listening)
	f.mu.RLock()
	var staleForwards []*Forward
	for _, fwd := range f.forwards {
		if !portSet[fwd.LocalPort] {
			// Make a copy to avoid holding the lock during SSH operations
			fwdCopy := *fwd
			staleForwards = append(staleForwards, &fwdCopy)
		}
	}
	f.mu.RUnlock()

	if len(staleForwards) == 0 {
		return nil
	}

	// Process each stale forward
	var reestablished, removed int
	var toRemove []string

	for _, fwd := range staleForwards {
		f.logger.Debug("Detected stale forward (port not listening)",
			"connectionInfo", fwd.ConnectionInfo,
			"remotePort", fwd.RemotePort,
			"localPort", fwd.LocalPort,
			"host", fwd.Host,
		)

		// Check if SSH connection is still alive
		socketPath, err := FindControlSocket(fwd.ConnectionInfo)
		if err != nil {
			// Connection is dead, mark for removal
			f.logger.Info("Removing stale forward (SSH connection dead)",
				"connectionInfo", fwd.ConnectionInfo,
				"remotePort", fwd.RemotePort,
				"localPort", fwd.LocalPort,
				"error", err,
			)
			key := fmt.Sprintf("%s:%s:%d", fwd.ConnectionInfo, fwd.Host, fwd.RemotePort)
			toRemove = append(toRemove, key)
			removed++
			continue
		}

		// SSH connection is alive, try to re-establish the forward
		f.logger.Info("Re-establishing forward (SSH connection alive)",
			"connectionInfo", fwd.ConnectionInfo,
			"remotePort", fwd.RemotePort,
			"localPort", fwd.LocalPort,
			"host", fwd.Host,
		)

		// Execute SSH forward command
		cmd := exec.Command(f.sshCmd,
			"-O", "forward",
			"-L", fmt.Sprintf("%d:%s:%d", fwd.LocalPort, fwd.Host, fwd.RemotePort),
			fwd.ConnectionInfo,
		)

		output, err := cmd.CombinedOutput()
		if err != nil {
			f.logger.Warn("Failed to re-establish forward",
				"connectionInfo", fwd.ConnectionInfo,
				"remotePort", fwd.RemotePort,
				"error", err,
				"output", string(output),
			)
			// Don't remove it yet - maybe it will work next time
			continue
		}

		// Update the forward with current info
		f.mu.Lock()
		key := fmt.Sprintf("%s:%s:%d", fwd.ConnectionInfo, fwd.Host, fwd.RemotePort)
		if existing, ok := f.forwards[key]; ok {
			existing.SocketPath = socketPath
			existing.CreatedAt = time.Now()
		}
		f.mu.Unlock()

		reestablished++
		f.logger.Info("Successfully re-established forward",
			"connectionInfo", fwd.ConnectionInfo,
			"remotePort", fwd.RemotePort,
			"localPort", fwd.LocalPort,
		)
	}

	// Remove forwards for dead connections
	if len(toRemove) > 0 {
		f.mu.Lock()
		for _, key := range toRemove {
			delete(f.forwards, key)
		}
		f.mu.Unlock()
	}

	if reestablished > 0 || removed > 0 {
		f.logger.Info("Reconciliation complete",
			"reestablished", reestablished,
			"removed", removed,
		)
	}

	return nil
}
