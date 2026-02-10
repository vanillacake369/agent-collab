package storage

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMigrationManager_StatusNone(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "migration-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	mgr := NewMigrationManager(tmpDir)

	status, err := mgr.Status()
	if err != nil {
		t.Fatalf("failed to get status: %v", err)
	}
	if status != MigrationStatusNone {
		t.Fatalf("expected status none, got %s", status)
	}
}

func TestMigrationManager_Migrate(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "migration-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create some test data to backup
	vectorsDir := filepath.Join(tmpDir, "vectors")
	os.MkdirAll(vectorsDir, 0755)
	os.WriteFile(filepath.Join(vectorsDir, "test.json"), []byte(`{"test": true}`), 0644)

	mgr := NewMigrationManager(tmpDir)

	// Run migration
	if err := mgr.Migrate(); err != nil {
		t.Fatalf("migration failed: %v", err)
	}

	// Check status
	status, err := mgr.Status()
	if err != nil {
		t.Fatalf("failed to get status: %v", err)
	}
	if status != MigrationStatusCompleted {
		t.Fatalf("expected status completed, got %s", status)
	}

	// Check BadgerDB directories created
	badgerDir := filepath.Join(tmpDir, "badger")
	for _, name := range []string{"delta", "audit"} {
		dir := filepath.Join(badgerDir, name)
		if _, err := os.Stat(dir); os.IsNotExist(err) {
			t.Fatalf("expected %s directory to exist", name)
		}
	}

	// Check backup created
	backups, err := mgr.ListBackups()
	if err != nil {
		t.Fatalf("failed to list backups: %v", err)
	}
	if len(backups) != 1 {
		t.Fatalf("expected 1 backup, got %d", len(backups))
	}
}

func TestMigrationManager_Rollback(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "migration-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	mgr := NewMigrationManager(tmpDir)

	// Run migration first
	if err := mgr.Migrate(); err != nil {
		t.Fatalf("migration failed: %v", err)
	}

	// Verify BadgerDB exists
	badgerDir := filepath.Join(tmpDir, "badger")
	if _, err := os.Stat(badgerDir); os.IsNotExist(err) {
		t.Fatal("expected badger directory to exist after migration")
	}

	// Rollback
	if err := mgr.Rollback(); err != nil {
		t.Fatalf("rollback failed: %v", err)
	}

	// Check status
	status, err := mgr.Status()
	if err != nil {
		t.Fatalf("failed to get status: %v", err)
	}
	if status != MigrationStatusRollback {
		t.Fatalf("expected status rollback, got %s", status)
	}

	// Check BadgerDB removed
	if _, err := os.Stat(badgerDir); !os.IsNotExist(err) {
		t.Fatal("expected badger directory to be removed after rollback")
	}
}

func TestMigrationManager_ListBackups(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "migration-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create fake backups
	for _, name := range []string{"backup_20260201_100000", "backup_20260202_100000"} {
		os.MkdirAll(filepath.Join(tmpDir, name), 0755)
	}

	mgr := NewMigrationManager(tmpDir)

	backups, err := mgr.ListBackups()
	if err != nil {
		t.Fatalf("failed to list backups: %v", err)
	}
	if len(backups) != 2 {
		t.Fatalf("expected 2 backups, got %d", len(backups))
	}
}

func TestMigrationManager_RestoreBackup(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "migration-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create backup with test data
	backupName := "backup_20260201_100000"
	backupDir := filepath.Join(tmpDir, backupName)
	vectorsBackup := filepath.Join(backupDir, "vectors")
	os.MkdirAll(vectorsBackup, 0755)
	os.WriteFile(filepath.Join(vectorsBackup, "test.json"), []byte(`{"restored": true}`), 0644)

	// Create current vectors (different content)
	vectorsDir := filepath.Join(tmpDir, "vectors")
	os.MkdirAll(vectorsDir, 0755)
	os.WriteFile(filepath.Join(vectorsDir, "test.json"), []byte(`{"current": true}`), 0644)

	mgr := NewMigrationManager(tmpDir)

	// Restore
	if err := mgr.RestoreBackup(backupName); err != nil {
		t.Fatalf("restore failed: %v", err)
	}

	// Verify content restored
	data, err := os.ReadFile(filepath.Join(vectorsDir, "test.json"))
	if err != nil {
		t.Fatalf("failed to read restored file: %v", err)
	}
	if string(data) != `{"restored": true}` {
		t.Fatalf("expected restored content, got %s", data)
	}
}

func TestMigrationManager_MigrateAlreadyCompleted(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "migration-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	mgr := NewMigrationManager(tmpDir)

	// Run migration
	if err := mgr.Migrate(); err != nil {
		t.Fatalf("first migration failed: %v", err)
	}

	// Try to migrate again
	err = mgr.Migrate()
	if err == nil {
		t.Fatal("expected error for already completed migration")
	}
}
