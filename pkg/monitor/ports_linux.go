//go:build linux

package monitor

// GetListeningPorts returns all ports in LISTEN state from procfs.
func GetListeningPorts() ([]Port, error) {
	return getListeningPortsFromProc("/proc/net/tcp", "/proc/net/tcp6")
}
