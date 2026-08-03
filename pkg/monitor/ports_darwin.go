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
