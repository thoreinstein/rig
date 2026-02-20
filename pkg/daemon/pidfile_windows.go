//go:build windows

package daemon

import (
	"golang.org/x/sys/windows"
)

// isProcessRunning checks if a process with the given PID is currently running.
// On Windows, it uses OpenProcess and WaitForSingleObject to verify existence.
func isProcessRunning(pid int) bool {
	const access = windows.PROCESS_QUERY_LIMITED_INFORMATION | windows.SYNCHRONIZE
	handle, err := windows.OpenProcess(access, false, uint32(pid))
	if err != nil {
		return false
	}
	defer windows.CloseHandle(handle)

	event, err := windows.WaitForSingleObject(handle, 0)
	if err != nil {
		return false
	}

	// WAIT_TIMEOUT means the process is still running.
	return event == windows.WAIT_TIMEOUT
}
