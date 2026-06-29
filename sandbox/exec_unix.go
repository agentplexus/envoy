//go:build !windows

package sandbox

import (
	"os/exec"
	"syscall"
)

// setProcessGroup configures the command to run in its own process group.
// This allows killing all child processes when the parent is killed.
func setProcessGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
	}
}

// killProcessGroup kills the entire process group.
// This ensures child processes are also terminated on timeout.
func killProcessGroup(cmd *exec.Cmd) {
	if cmd.Process != nil {
		// Kill the entire process group by sending signal to negative PID
		// This kills all processes in the group, including children
		_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	}
}
