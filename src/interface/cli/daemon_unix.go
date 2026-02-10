//go:build !windows

package cli

import (
	"os/exec"
	"syscall"
)

// setSysProcAttr sets Unix-specific process attributes for daemonization
func setSysProcAttr(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setsid: true,
	}
}

// signalTerm sends SIGTERM to the process
func signalTerm(pid int) error {
	return syscall.Kill(pid, syscall.SIGTERM)
}
