//go:build windows

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

// stopAllDaemons finds and stops all agent-collab daemon processes on Windows
func stopAllDaemons() error {
	fmt.Println("🔍 모든 agent-collab 데몬 프로세스 검색 중...")

	// Use tasklist to find agent-collab processes
	// #nosec G204 - command arguments are hardcoded
	cmd := exec.Command("tasklist", "/FI", "IMAGENAME eq agent-collab.exe", "/FO", "CSV", "/NH")
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("프로세스 검색 실패: %w", err)
	}

	// Parse PIDs from CSV output
	var pids []int
	currentPID := os.Getpid()
	scanner := bufio.NewScanner(strings.NewReader(string(output)))

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.Contains(line, "정보 없음") || strings.Contains(line, "No tasks") {
			continue
		}

		// CSV format: "agent-collab.exe","1234","Console","1","12,345 K"
		parts := strings.Split(line, ",")
		if len(parts) < 2 {
			continue
		}

		pidStr := strings.Trim(parts[1], "\"")
		pid, err := strconv.Atoi(pidStr)
		if err != nil {
			continue
		}

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

	// Kill all processes
	for _, pid := range pids {
		proc, err := os.FindProcess(pid)
		if err != nil {
			fmt.Printf("  PID %d: 프로세스 찾기 실패 (%v)\n", pid, err)
			continue
		}

		if err := proc.Kill(); err != nil {
			fmt.Printf("  PID %d: 종료 실패 (%v)\n", pid, err)
		} else {
			fmt.Printf("  PID %d: 종료됨\n", pid)
		}
	}

	time.Sleep(500 * time.Millisecond)
	fmt.Println("✓ 모든 데몬 프로세스가 종료되었습니다.")

	return cleanupAllStaleDaemonFiles()
}

// isProcessRunning checks if a process with the given PID is running on Windows
func isProcessRunning(pid int) bool {
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	// On Windows, FindProcess always succeeds, so we need to try to kill with signal 0
	// However, Windows doesn't support signal 0, so we use a different approach
	// We'll try to open the process and check
	// #nosec G204 - command arguments are hardcoded
	cmd := exec.Command("tasklist", "/FI", fmt.Sprintf("PID eq %d", pid), "/NH")
	output, err := cmd.Output()
	if err != nil {
		return false
	}
	_ = proc // silence unused variable warning
	return strings.Contains(string(output), strconv.Itoa(pid))
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
	if _, err := os.Stat(pidPath); err == nil {
		data, err := os.ReadFile(pidPath)
		if err == nil {
			pidStr := strings.TrimSpace(string(data))
			if pid, err := strconv.Atoi(pidStr); err == nil {
				if !isProcessRunning(pid) {
					// Process is not running, clean up files
					os.Remove(pidPath)
					os.Remove(sockPath)
					fmt.Println("✓ 오래된 데몬 파일 정리 완료")
				}
			}
		}
	}

	return nil
}
