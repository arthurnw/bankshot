//go:build !darwin

package monitor

// ResolveProcessName returns the process name for a PID, or "" if unknown.
func ResolveProcessName(pid int) string { return procProcessName(pid) }

// ResolveParentPID returns the parent PID for a PID, or 0 if unknown.
func ResolveParentPID(pid int) int { return procParentPID(pid) }

// ResolveProcessCwd returns the working directory for a PID, or "" if unknown.
func ResolveProcessCwd(pid int) string { return procProcessCwd(pid) }
