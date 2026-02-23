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

	// WAIT_TIMEOUT means the process is still running.
	return event == windows.WAIT_TIMEOUT
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
	err = windows.QueryFullProcessImageName(handle, 0, \u0026buf[0], \u0026n)
	if err != nil {
		return false
	}

	exePath := windows.UTF16ToString(buf[:n])
	name := filepath.Base(exePath)
	return name == "rig.exe" || strings.HasSuffix(name, "\\rig.exe")
}
