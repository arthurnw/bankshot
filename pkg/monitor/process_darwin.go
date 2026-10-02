//go:build darwin

package monitor

import (
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

// ResolveProcessName returns the executable name for a PID, or "" if unknown.
func ResolveProcessName(pid int) string {
	out, err := exec.Command("/bin/ps", "-o", "comm=", "-p", strconv.Itoa(pid)).Output()
	if err != nil {
		return ""
	}
	comm := strings.TrimSpace(string(out))
	if comm == "" {
		return ""
	}
	return filepath.Base(comm)
}

// ResolveParentPID returns the parent PID for a PID, or 0 if unknown.
func ResolveParentPID(pid int) int {
	out, err := exec.Command("/bin/ps", "-o", "ppid=", "-p", strconv.Itoa(pid)).Output()
	if err != nil {
		return 0
	}
	ppid, err := strconv.Atoi(strings.TrimSpace(string(out)))
	if err != nil {
		return 0
	}
	return ppid
}

// ResolveProcessCwd returns the working directory for a PID, or "" if unknown.
func ResolveProcessCwd(pid int) string {
	out, err := exec.Command("/usr/sbin/lsof", "-a", "-p", strconv.Itoa(pid), "-d", "cwd", "-Fn").Output()
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(out), "\n") {
		if strings.HasPrefix(line, "n") {
			return line[1:]
		}
	}
	return ""
}
