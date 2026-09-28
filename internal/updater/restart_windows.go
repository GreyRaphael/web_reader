//go:build windows

package updater

import (
	"os/exec"
)

// SetupDetachedProcess configures the command to run independently
// of the calling process in Windows systems.
func SetupDetachedProcess(cmd *exec.Cmd) {
	// On Windows, child processes launched via exec.Cmd.Start()
	// remain alive after parent termination unless part of a Job Object.
}
