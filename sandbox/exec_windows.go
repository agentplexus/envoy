//go:build windows

package sandbox

import (
	"os/exec"
)

// setProcessGroup configures the command to run in its own process group.
// On Windows, this is a no-op as process groups work differently.
func setProcessGroup(cmd *exec.Cmd) {
	// Windows handles process groups differently
	// The context cancellation will kill the process
}

// killProcessGroup kills the process.
// On Windows, this just kills the main process.
func killProcessGroup(cmd *exec.Cmd) {
	if cmd.Process != nil {
		_ = cmd.Process.Kill()
	}
}
