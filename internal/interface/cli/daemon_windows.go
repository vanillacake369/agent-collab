//go:build windows

package cli

import (
	"os"
	"os/exec"
	"syscall"
)

// setSysProcAttr sets Windows-specific process attributes for daemonization
func setSysProcAttr(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP,
	}
}

// signalTerm sends termination signal to the process on Windows
func signalTerm(pid int) error {
	proc, err := os.FindProcess(pid)
	if err != nil {
		return err
	}
	return proc.Kill()
}
