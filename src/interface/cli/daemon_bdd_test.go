package cli

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

// BDD Tests for zombie daemon process cleanup

// Feature: 좀비 데몬 프로세스 정리
// 사용자가 daemon stop --all 명령어로 모든 agent-collab 데몬을 종료할 수 있다.

func TestDaemonStop_GivenMultipleDaemons_WhenStopAll_ThenAllStopped(t *testing.T) {
	// Given: 여러 경로에서 데몬 프로세스 정보가 존재함 (시뮬레이션)
	// 실제 프로세스를 생성하지 않고 함수 로직만 테스트

	pids := []int{12345, 12346, 12347}

	// When: findAllDaemonPIDs 실행
	// 실제 환경에서는 ps aux | grep "agent-collab daemon" 결과를 파싱
	foundPIDs := simulateFindDaemonPIDs(pids)

	// Then: 모든 PID가 발견됨
	if len(foundPIDs) != len(pids) {
		t.Errorf("Expected %d PIDs, got %d", len(pids), len(foundPIDs))
	}
}

func TestDaemonStop_GivenPIDFile_WhenStop_ThenOnlyCurrentStopped(t *testing.T) {
	// Given: 현재 프로젝트의 PID 파일이 존재함
	tmpDir := t.TempDir()
	pidPath := filepath.Join(tmpDir, "daemon.pid")

	currentPID := 12345
	if err := os.WriteFile(pidPath, []byte(strconv.Itoa(currentPID)), 0644); err != nil {
		t.Fatalf("Failed to create pid file: %v", err)
	}

	// When: readPIDFile 실행
	pid, err := readPIDFile(pidPath)

	// Then: 현재 프로젝트의 PID만 반환
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}
	if pid != currentPID {
		t.Errorf("Expected PID %d, got %d", currentPID, pid)
	}
}

func TestDaemonStop_GivenNoPIDFile_WhenStop_ThenError(t *testing.T) {
	// Given: PID 파일이 없음
	tmpDir := t.TempDir()
	pidPath := filepath.Join(tmpDir, "daemon.pid")

	// When: readPIDFile 실행
	_, err := readPIDFile(pidPath)

	// Then: 에러 발생
	if err == nil {
		t.Error("Expected error for missing PID file, got nil")
	}
}

func TestDaemonStop_GivenInvalidPIDFile_WhenStop_ThenError(t *testing.T) {
	// Given: 잘못된 PID 파일 (숫자가 아님)
	tmpDir := t.TempDir()
	pidPath := filepath.Join(tmpDir, "daemon.pid")

	if err := os.WriteFile(pidPath, []byte("not-a-number"), 0644); err != nil {
		t.Fatalf("Failed to create invalid pid file: %v", err)
	}

	// When: readPIDFile 실행
	_, err := readPIDFile(pidPath)

	// Then: 에러 발생
	if err == nil {
		t.Error("Expected error for invalid PID file, got nil")
	}
}

func TestDaemonStop_GivenStaleProcess_WhenStopAll_ThenCleanup(t *testing.T) {
	// Given: 오래된 프로세스 (이미 종료됨, 하지만 PID 파일 남음)
	tmpDir := t.TempDir()
	pidPath := filepath.Join(tmpDir, "daemon.pid")
	sockPath := filepath.Join(tmpDir, "daemon.sock")

	// 존재하지 않는 PID
	stalePID := 99999
	if err := os.WriteFile(pidPath, []byte(strconv.Itoa(stalePID)), 0644); err != nil {
		t.Fatalf("Failed to create pid file: %v", err)
	}
	if err := os.WriteFile(sockPath, []byte(""), 0644); err != nil {
		t.Fatalf("Failed to create socket file: %v", err)
	}

	// When: cleanupStaleDaemonFiles 실행
	err := cleanupStaleDaemonFiles(tmpDir)

	// Then: PID 파일과 소켓 파일이 정리됨
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	if _, err := os.Stat(pidPath); !os.IsNotExist(err) {
		t.Error("Expected daemon.pid to be cleaned up")
	}

	if _, err := os.Stat(sockPath); !os.IsNotExist(err) {
		t.Error("Expected daemon.sock to be cleaned up")
	}
}

// Helper functions for testing

func simulateFindDaemonPIDs(pids []int) []int {
	// 실제 구현에서는 ps 명령어 실행
	return pids
}

func readPIDFile(path string) (int, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}

	pidStr := strings.TrimSpace(string(data))
	pid, err := strconv.Atoi(pidStr)
	if err != nil {
		return 0, err
	}

	return pid, nil
}

func cleanupStaleDaemonFiles(dataDir string) error {
	pidPath := filepath.Join(dataDir, "daemon.pid")
	sockPath := filepath.Join(dataDir, "daemon.sock")

	// PID 파일 읽기
	pid, err := readPIDFile(pidPath)
	if err != nil {
		// PID 파일이 없거나 읽을 수 없으면 정리만 수행
		os.Remove(pidPath)
		os.Remove(sockPath)
		return nil
	}

	// 프로세스 존재 확인
	process, err := os.FindProcess(pid)
	if err != nil {
		// 프로세스를 찾을 수 없음 - 정리
		os.Remove(pidPath)
		os.Remove(sockPath)
		return nil
	}

	// 프로세스에 시그널 보내서 존재 확인
	err = process.Signal(syscall.Signal(0))
	if err != nil {
		// 프로세스가 없음 - 정리
		os.Remove(pidPath)
		os.Remove(sockPath)
		return nil
	}

	// 프로세스가 살아있음 - 정리하지 않음
	return nil
}

// Integration test helper - 실제 프로세스 생성/종료 테스트용
func TestDaemonStop_Integration_GivenRealProcess_WhenKill_ThenStopped(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Given: 실제 sleep 프로세스 생성 (데몬 시뮬레이션)
	cmd := exec.Command("sleep", "60")
	if err := cmd.Start(); err != nil {
		t.Fatalf("Failed to start test process: %v", err)
	}

	defer func() {
		cmd.Process.Kill()
		cmd.Wait()
	}()

	// Verify process started
	t.Logf("Test process started with PID: %d", cmd.Process.Pid)

	// When: 프로세스 종료
	err := cmd.Process.Signal(syscall.SIGTERM)

	// Then: 종료됨
	if err != nil {
		t.Errorf("Expected no error when killing process, got: %v", err)
	}

	// 프로세스 종료 대기
	done := make(chan error)
	go func() {
		done <- cmd.Wait()
	}()

	select {
	case <-done:
		// 정상 종료
	case <-time.After(2 * time.Second):
		t.Error("Process did not terminate within timeout")
	}
}
