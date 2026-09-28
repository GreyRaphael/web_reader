//go:build !windows

package updater

import (
	"os/exec"
	"syscall"
)

// SetupDetachedProcess configures the command to run independently
// of the calling process in POSIX systems.
func SetupDetachedProcess(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
	}
}
