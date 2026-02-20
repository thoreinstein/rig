//go:build windows

package daemon

import (
	"os/exec"
)

func configureSysProcAttr(cmd *exec.Cmd) {
	// No-op on Windows
}
