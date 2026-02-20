//go:build !windows

package daemon

import (
	"os"
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
