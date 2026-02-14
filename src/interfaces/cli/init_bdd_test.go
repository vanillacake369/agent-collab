package cli

import (
	"os"
	"path/filepath"
	"testing"
)

// BDD Tests for cluster initialization duplicate check

// Feature: 클러스터 초기화 중복 체크
// 사용자가 init 명령어 실행 시 기존 클러스터 존재 여부를 확인하고
// 적절한 에러 메시지를 표시하거나 --force로 덮어쓰기를 허용한다.

func TestInitCluster_GivenEmptyDataDir_WhenInit_ThenSuccess(t *testing.T) {
	// Given: 데이터 디렉토리가 비어있음
	tmpDir := t.TempDir()
	originalDataDir := os.Getenv("AGENT_COLLAB_DATA_DIR")
	os.Setenv("AGENT_COLLAB_DATA_DIR", tmpDir)
	defer os.Setenv("AGENT_COLLAB_DATA_DIR", originalDataDir)

	// When: checkExistingCluster 실행
	err := checkExistingCluster("test-project")

	// Then: 에러 없이 성공
	if err != nil {
		t.Errorf("Expected no error for empty data dir, got: %v", err)
	}
}

func TestInitCluster_GivenSameProjectExists_WhenInit_ThenError(t *testing.T) {
	// Given: "test-project" 프로젝트 클러스터가 존재함
	tmpDir := t.TempDir()
	originalDataDir := os.Getenv("AGENT_COLLAB_DATA_DIR")
	os.Setenv("AGENT_COLLAB_DATA_DIR", tmpDir)
	defer os.Setenv("AGENT_COLLAB_DATA_DIR", originalDataDir)

	// 기존 config.json 생성
	configPath := filepath.Join(tmpDir, "config.json")
	configContent := `{"project_name": "test-project"}`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to create test config: %v", err)
	}

	// When: 동일 프로젝트명으로 checkExistingCluster 실행
	err := checkExistingCluster("test-project")

	// Then: "이미 존재합니다" 에러 발생
	if err == nil {
		t.Error("Expected error for existing same project, got nil")
	}
	if err != nil && !containsString(err.Error(), "이미 존재합니다") {
		t.Errorf("Expected '이미 존재합니다' error, got: %v", err)
	}
}

func TestInitCluster_GivenDifferentProjectExists_WhenInit_ThenError(t *testing.T) {
	// Given: "project-a" 클러스터가 존재함
	tmpDir := t.TempDir()
	originalDataDir := os.Getenv("AGENT_COLLAB_DATA_DIR")
	os.Setenv("AGENT_COLLAB_DATA_DIR", tmpDir)
	defer os.Setenv("AGENT_COLLAB_DATA_DIR", originalDataDir)

	// 기존 config.json 생성 (다른 프로젝트명)
	configPath := filepath.Join(tmpDir, "config.json")
	configContent := `{"project_name": "project-a"}`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to create test config: %v", err)
	}

	// When: 다른 프로젝트명으로 checkExistingCluster 실행
	err := checkExistingCluster("project-b")

	// Then: "다른 클러스터가 존재합니다" 에러 발생
	if err == nil {
		t.Error("Expected error for different existing project, got nil")
	}
	if err != nil && !containsString(err.Error(), "다른 클러스터") {
		t.Errorf("Expected '다른 클러스터' error, got: %v", err)
	}
}

func TestInitCluster_GivenCorruptedConfig_WhenInit_ThenSuccess(t *testing.T) {
	// Given: 손상된 config.json이 존재함
	tmpDir := t.TempDir()
	originalDataDir := os.Getenv("AGENT_COLLAB_DATA_DIR")
	os.Setenv("AGENT_COLLAB_DATA_DIR", tmpDir)
	defer os.Setenv("AGENT_COLLAB_DATA_DIR", originalDataDir)

	// 손상된 config.json 생성
	configPath := filepath.Join(tmpDir, "config.json")
	if err := os.WriteFile(configPath, []byte("invalid json{{{"), 0644); err != nil {
		t.Fatalf("Failed to create corrupted config: %v", err)
	}

	// When: checkExistingCluster 실행
	err := checkExistingCluster("test-project")

	// Then: 손상된 config는 덮어쓰기 허용 (에러 없음)
	if err != nil {
		t.Errorf("Expected no error for corrupted config (allow overwrite), got: %v", err)
	}
}

func TestInitCluster_GivenSameProjectExists_WhenInitWithForce_ThenSuccess(t *testing.T) {
	// Given: "test-project" 프로젝트 클러스터가 존재함
	tmpDir := t.TempDir()
	originalDataDir := os.Getenv("AGENT_COLLAB_DATA_DIR")
	os.Setenv("AGENT_COLLAB_DATA_DIR", tmpDir)
	defer os.Setenv("AGENT_COLLAB_DATA_DIR", originalDataDir)

	// 기존 config.json 생성
	configPath := filepath.Join(tmpDir, "config.json")
	configContent := `{"project_name": "test-project"}`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to create test config: %v", err)
	}

	// When: --force 플래그와 함께 checkExistingClusterWithForce 실행
	err := checkExistingClusterWithForce("test-project", true)

	// Then: force=true면 에러 없이 성공
	if err != nil {
		t.Errorf("Expected no error with force=true, got: %v", err)
	}
}

// Helper function
func containsString(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsSubstring(s, substr))
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
