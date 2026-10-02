//go:build !linux && !darwin

package monitor

import (
	"fmt"
	"runtime"
)

// GetListeningPorts reports that system-wide listener discovery is unavailable
// rather than treating an unsupported platform as a machine with no listeners.
func GetListeningPorts() ([]Port, error) {
	return nil, fmt.Errorf("system-wide listener discovery is unsupported on %s", runtime.GOOS)
}

// PortListeners is unsupported on this platform.
func PortListeners(port int) ([]Listener, error) {
	return nil, ErrListenersUnsupported
}
