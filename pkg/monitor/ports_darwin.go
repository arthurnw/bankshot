//go:build darwin

package monitor

import (
	"errors"
	"fmt"
	"os/exec"
)

// GetListeningPorts returns all TCP listeners reported by macOS lsof.
func GetListeningPorts() ([]Port, error) {
	cmd := exec.Command("/usr/sbin/lsof", "-nP", "-iTCP", "-sTCP:LISTEN", "-Fn")
	output, err := cmd.Output()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) && exitErr.ExitCode() == 1 {
			return nil, nil
		}
		return nil, fmt.Errorf("list TCP listeners with lsof: %w", err)
	}

	return parseLsofListeningPorts(output)
}

// PortListeners returns the processes listening on a TCP port. Without root,
// lsof reports only this user's processes.
func PortListeners(port int) ([]Listener, error) {
	cmd := exec.Command("/usr/sbin/lsof", "-nP", fmt.Sprintf("-iTCP:%d", port), "-sTCP:LISTEN", "-Fpcn")
	output, err := cmd.Output()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) && exitErr.ExitCode() == 1 {
			return nil, nil
		}
		return nil, fmt.Errorf("list TCP listeners on port %d with lsof: %w", port, err)
	}

	return parseLsofListeners(output)
}
