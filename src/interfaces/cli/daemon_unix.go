//go:build !windows

package cli

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
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

// stopAllDaemons finds and stops all agent-collab daemon processes
func stopAllDaemons() error {
	fmt.Println("🔍 모든 agent-collab 데몬 프로세스 검색 중...")

	// Find all agent-collab daemon processes using pgrep
	// #nosec G204 - command arguments are hardcoded
	cmd := exec.Command("pgrep", "-f", "agent-collab daemon")
	output, err := cmd.Output()
	if err != nil {
		// pgrep returns exit code 1 if no processes found
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
			fmt.Println("실행 중인 agent-collab 데몬이 없습니다.")
			return cleanupAllStaleDaemonFiles()
		}
		return fmt.Errorf("프로세스 검색 실패: %w", err)
	}

	// Parse PIDs
	scanner := bufio.NewScanner(strings.NewReader(string(output)))
	var pids []int
	currentPID := os.Getpid()

	for scanner.Scan() {
		pidStr := strings.TrimSpace(scanner.Text())
		if pidStr == "" {
			continue
		}
		pid, err := strconv.Atoi(pidStr)
		if err != nil {
			continue
		}
		// Skip current process
		if pid == currentPID {
			continue
		}
		pids = append(pids, pid)
	}

	if len(pids) == 0 {
		fmt.Println("실행 중인 agent-collab 데몬이 없습니다.")
		return cleanupAllStaleDaemonFiles()
	}

	fmt.Printf("🛑 %d개의 데몬 프로세스 종료 중...\n", len(pids))

	// Send SIGTERM to all processes
	for _, pid := range pids {
		if err := syscall.Kill(pid, syscall.SIGTERM); err != nil {
			fmt.Printf("  PID %d: 종료 실패 (%v)\n", pid, err)
		} else {
			fmt.Printf("  PID %d: SIGTERM 전송\n", pid)
		}
	}

	// Wait for processes to terminate
	time.Sleep(500 * time.Millisecond)

	// Check if any processes are still running and force kill
	for _, pid := range pids {
		if isProcessRunning(pid) {
			fmt.Printf("  PID %d: 강제 종료 (SIGKILL)\n", pid)
			syscall.Kill(pid, syscall.SIGKILL)
		}
	}

	fmt.Println("✓ 모든 데몬 프로세스가 종료되었습니다.")

	return cleanupAllStaleDaemonFiles()
}

// isProcessRunning checks if a process with the given PID is running
func isProcessRunning(pid int) bool {
	err := syscall.Kill(pid, 0)
	return err == nil
}

// cleanupAllStaleDaemonFiles cleans up stale daemon files from all known locations
func cleanupAllStaleDaemonFiles() error {
	// Check default data directory
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil
	}

	dataDir := filepath.Join(homeDir, ".agent-collab")
	if dir := os.Getenv("AGENT_COLLAB_DATA_DIR"); dir != "" {
		dataDir = dir
	}

	pidPath := filepath.Join(dataDir, "daemon.pid")
	sockPath := filepath.Join(dataDir, "daemon.sock")

	// Check if daemon is actually running before cleaning up
	// #nosec G703 - pidPath/sockPath are constructed from trusted sources (home dir or env var)
	if _, err := os.Stat(pidPath); err == nil {
		data, err := os.ReadFile(pidPath) // #nosec G703
		if err == nil {
			pidStr := strings.TrimSpace(string(data))
			if pid, err := strconv.Atoi(pidStr); err == nil {
				if !isProcessRunning(pid) {
					// Process is not running, clean up files
					os.Remove(pidPath)  // #nosec G703
					os.Remove(sockPath) // #nosec G703
					fmt.Println("✓ 오래된 데몬 파일 정리 완료")
				}
			}
		}
	}

	return nil
}
