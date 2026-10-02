//go:build !darwin

package monitor

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// ResolveProcessName returns the process name for a given PID.
// It reads /proc/<pid>/cmdline first to get the full (untruncated) argv[0]
// basename, falling back to /proc/<pid>/comm (which the kernel truncates
// to 15 characters). Returns empty string if the process is gone or unreadable.
func ResolveProcessName(pid int) string {
	// Try cmdline first for the full name
	data, err := os.ReadFile(fmt.Sprintf("/proc/%d/cmdline", pid))
	if err == nil && len(data) > 0 {
		// cmdline is NUL-delimited; argv[0] is everything before the first NUL
		argv0 := string(data)
		if i := strings.IndexByte(argv0, 0); i >= 0 {
			argv0 = argv0[:i]
		}
		if argv0 != "" {
			return filepath.Base(argv0)
		}
	}
	// Fall back to comm (truncated to 15 chars)
	data, err = os.ReadFile(fmt.Sprintf("/proc/%d/comm", pid))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

// ResolveParentPID returns the parent PID for a given PID by reading PPid from
// /proc/<pid>/status. Returns 0 if the process is gone, unreadable, or at init.
func ResolveParentPID(pid int) int {
	data, err := os.ReadFile(fmt.Sprintf("/proc/%d/status", pid))
	if err != nil {
		return 0
	}
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, "PPid:") {
			fields := strings.Fields(line)
			if len(fields) == 2 {
				ppid, err := strconv.Atoi(fields[1])
				if err != nil {
					return 0
				}
				return ppid
			}
		}
	}
	return 0
}

// ResolveProcessCwd reads /proc/<pid>/cwd symlink and returns the working directory.
// Returns empty string if the process is gone or unreadable.
func ResolveProcessCwd(pid int) string {
	cwd, err := os.Readlink(fmt.Sprintf("/proc/%d/cwd", pid))
	if err != nil {
		return ""
	}
	return cwd
}
