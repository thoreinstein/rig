//go:build !windows

package daemon

import (
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
)

// isProcessRunning checks if a process with the given PID is currently running.
// On Unix, it sends signal 0 to check for existence.
func isProcessRunning(pid int) bool {
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	err = process.Signal(syscall.Signal(0))
	return err == nil
}

// checkProcessIdentity verifies that the process with the given PID is a Rig process.
// It checks the command name/executable path.
func checkProcessIdentity(pid int) bool {
	// Try to get the command name using ps
	// #nosec G204 - pid is an integer and safe from injection
	cmd := exec.Command("ps", "-p", strconv.Itoa(pid), "-o", "comm=")
	out, err := cmd.Output()
	if err != nil {
		return false
	}
	name := strings.TrimSpace(string(out))
	// On macOS, it might be the full path or just 'rig'
	return name == "rig" || strings.HasSuffix(name, "/rig")
}
