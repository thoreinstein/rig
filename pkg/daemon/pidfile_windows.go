//go:build windows

package daemon

import (
	"path/filepath"
	"strings"

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

	// Other non-error values (e.g., WAIT_OBJECT_0) indicate the process has exited.
	switch event {
	case windows.WAIT_TIMEOUT:
		return true
	case windows.WAIT_OBJECT_0:
		return false
	default:
		// Treat any other non-error state as "not running".
		return false
	}
}

// checkProcessIdentity verifies that the process with the given PID is a Rig process.
func checkProcessIdentity(pid int) bool {
	const access = windows.PROCESS_QUERY_LIMITED_INFORMATION
	handle, err := windows.OpenProcess(access, false, uint32(pid))
	if err != nil {
		return false
	}
	defer windows.CloseHandle(handle)

	var buf [windows.MAX_PATH]uint16
	n := uint32(len(buf))
	err = windows.QueryFullProcessImageName(handle, 0, &buf[0], &n)
	if err != nil {
		return false
	}

	exePath := windows.UTF16ToString(buf[:n])
	name := filepath.Base(exePath)
	return name == "rig.exe" || strings.HasSuffix(name, "\\rig.exe")
}
