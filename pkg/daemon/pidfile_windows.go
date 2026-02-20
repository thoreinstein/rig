//go:build windows

package daemon

import (
	"os"
)

// isProcessRunning checks if a process with the given PID is currently running.
// On Windows, FindProcess always succeeds, so we just return true.
// A more robust check would involve calling Windows APIs.
func isProcessRunning(pid int) bool {
	_, err := os.FindProcess(pid)
	return err == nil
}
