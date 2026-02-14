package cli

import (
	"os"
	"path/filepath"
	"testing"
)

// BDD Tests for cluster cleanup (leave --reset)

// Feature: 클러스터 정리 (leave --reset)
// 사용자가 leave 명령어로 클러스터를 정리할 때
// --clean은 데이터만 삭제하고, --reset은 설정까지 모두 삭제한다.

func TestLeaveCluster_GivenClusterExists_WhenLeaveWithReset_ThenAllDeleted(t *testing.T) {
	// Given: "test" 클러스터가 존재함 (config, key, vectors 등)
	tmpDir := t.TempDir()
	originalDataDir := os.Getenv("AGENT_COLLAB_DATA_DIR")
	os.Setenv("AGENT_COLLAB_DATA_DIR", tmpDir)
	defer os.Setenv("AGENT_COLLAB_DATA_DIR", originalDataDir)

	// 클러스터 파일들 생성
	files := map[string]string{
		"config.json":    `{"project_name": "test"}`,
		"key.json":       `{"type": "ed25519", "private_key": "xxx"}`,
		"wireguard.json": `{"enabled": true}`,
	}
	for name, content := range files {
		path := filepath.Join(tmpDir, name)
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatalf("Failed to create %s: %v", name, err)
		}
	}

	// vectors 디렉토리 생성
	vectorsDir := filepath.Join(tmpDir, "vectors")
	if err := os.MkdirAll(vectorsDir, 0755); err != nil {
		t.Fatalf("Failed to create vectors dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(vectorsDir, "data.bin"), []byte("test"), 0644); err != nil {
		t.Fatalf("Failed to create vector data: %v", err)
	}

	// When: cleanupClusterData(reset=true) 실행
	err := cleanupClusterData(tmpDir, true)

	// Then: 모든 파일이 삭제됨
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	// config.json 삭제 확인
	if _, err := os.Stat(filepath.Join(tmpDir, "config.json")); !os.IsNotExist(err) {
		t.Error("Expected config.json to be deleted")
	}

	// key.json 삭제 확인
	if _, err := os.Stat(filepath.Join(tmpDir, "key.json")); !os.IsNotExist(err) {
		t.Error("Expected key.json to be deleted")
	}

	// vectors 디렉토리 삭제 확인
	if _, err := os.Stat(vectorsDir); !os.IsNotExist(err) {
		t.Error("Expected vectors directory to be deleted")
	}
}

func TestLeaveCluster_GivenClusterExists_WhenLeaveWithClean_ThenConfigPreserved(t *testing.T) {
	// Given: "test" 클러스터가 존재함
	tmpDir := t.TempDir()
	originalDataDir := os.Getenv("AGENT_COLLAB_DATA_DIR")
	os.Setenv("AGENT_COLLAB_DATA_DIR", tmpDir)
	defer os.Setenv("AGENT_COLLAB_DATA_DIR", originalDataDir)

	// 클러스터 파일들 생성
	configPath := filepath.Join(tmpDir, "config.json")
	keyPath := filepath.Join(tmpDir, "key.json")
	if err := os.WriteFile(configPath, []byte(`{"project_name": "test"}`), 0644); err != nil {
		t.Fatalf("Failed to create config: %v", err)
	}
	if err := os.WriteFile(keyPath, []byte(`{"type": "ed25519"}`), 0644); err != nil {
		t.Fatalf("Failed to create key: %v", err)
	}

	// vectors 디렉토리 생성
	vectorsDir := filepath.Join(tmpDir, "vectors")
	if err := os.MkdirAll(vectorsDir, 0755); err != nil {
		t.Fatalf("Failed to create vectors dir: %v", err)
	}

	// When: cleanupClusterData(reset=false) 실행 (--clean만)
	err := cleanupClusterData(tmpDir, false)

	// Then: vectors는 삭제되고, config/key는 유지됨
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	// config.json 유지 확인
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		t.Error("Expected config.json to be preserved")
	}

	// key.json 유지 확인
	if _, err := os.Stat(keyPath); os.IsNotExist(err) {
		t.Error("Expected key.json to be preserved")
	}

	// vectors 디렉토리 삭제 확인
	if _, err := os.Stat(vectorsDir); !os.IsNotExist(err) {
		t.Error("Expected vectors directory to be deleted")
	}
}

func TestLeaveCluster_GivenNoCluster_WhenLeaveWithReset_ThenNoError(t *testing.T) {
	// Given: 클러스터가 없음 (빈 디렉토리)
	tmpDir := t.TempDir()
	originalDataDir := os.Getenv("AGENT_COLLAB_DATA_DIR")
	os.Setenv("AGENT_COLLAB_DATA_DIR", tmpDir)
	defer os.Setenv("AGENT_COLLAB_DATA_DIR", originalDataDir)

	// When: cleanupClusterData(reset=true) 실행
	err := cleanupClusterData(tmpDir, true)

	// Then: 에러 없이 완료 (idempotent)
	if err != nil {
		t.Errorf("Expected no error for empty dir, got: %v", err)
	}
}

func TestLeaveCluster_GivenDaemonRunning_WhenLeaveWithReset_ThenDaemonStopped(t *testing.T) {
	// Given: 데몬이 실행 중 (PID 파일 존재)
	tmpDir := t.TempDir()
	originalDataDir := os.Getenv("AGENT_COLLAB_DATA_DIR")
	os.Setenv("AGENT_COLLAB_DATA_DIR", tmpDir)
	defer os.Setenv("AGENT_COLLAB_DATA_DIR", originalDataDir)

	// PID 파일 생성 (가상의 PID)
	pidPath := filepath.Join(tmpDir, "daemon.pid")
	if err := os.WriteFile(pidPath, []byte("99999"), 0644); err != nil {
		t.Fatalf("Failed to create pid file: %v", err)
	}

	// Socket 파일 생성
	sockPath := filepath.Join(tmpDir, "daemon.sock")
	if err := os.WriteFile(sockPath, []byte(""), 0644); err != nil {
		t.Fatalf("Failed to create socket file: %v", err)
	}

	// When: cleanupClusterData(reset=true) 실행
	err := cleanupClusterData(tmpDir, true)

	// Then: PID 파일과 소켓 파일도 삭제됨
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	if _, err := os.Stat(pidPath); !os.IsNotExist(err) {
		t.Error("Expected daemon.pid to be deleted")
	}

	if _, err := os.Stat(sockPath); !os.IsNotExist(err) {
		t.Error("Expected daemon.sock to be deleted")
	}
}
