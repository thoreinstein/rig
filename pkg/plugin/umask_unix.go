//go:build unix

package plugin

import "syscall"

// setUmask sets the process umask and returns the previous value.
func setUmask(mask int) int {
	return syscall.Umask(mask)
}
