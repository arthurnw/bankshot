package monitor

import "errors"

// Listener is a process listening on a TCP port.
type Listener struct {
	PID      int
	Command  string
	Port     int
	BindAddr string
}

// ErrListenersUnsupported is returned by PortListeners on platforms that cannot
// map a listening socket to its process cheaply.
var ErrListenersUnsupported = errors.New("listener process lookup is unsupported on this platform")
