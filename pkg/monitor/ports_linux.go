//go:build linux

package monitor

// GetListeningPorts returns all ports in LISTEN state from procfs.
func GetListeningPorts() ([]Port, error) {
	return getListeningPortsFromProc("/proc/net/tcp", "/proc/net/tcp6")
}

// PortListeners is unsupported on Linux: procfs maps a socket to its process
// only through a scan of every process's file descriptors.
func PortListeners(port int) ([]Listener, error) {
	return nil, ErrListenersUnsupported
}
