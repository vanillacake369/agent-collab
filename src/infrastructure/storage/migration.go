// Package storage provides storage interfaces and migration utilities.
package storage

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/dgraph-io/badger/v4"
)

// MigrationStatus represents the current migration state.
type MigrationStatus string

const (
	MigrationStatusNone      MigrationStatus = "none"
	MigrationStatusStarted   MigrationStatus = "started"
	MigrationStatusCompleted MigrationStatus = "completed"
	MigrationStatusRollback  MigrationStatus = "rollback"
	MigrationStatusFailed    MigrationStatus = "failed"
)

// MigrationManager handles safe migration to BadgerDB storage.
type MigrationManager struct {
	dataDir string
}

// NewMigrationManager creates a new migration manager.
func NewMigrationManager(dataDir string) *MigrationManager {
	return &MigrationManager{dataDir: dataDir}
}

// Status returns the current migration status.
func (m *MigrationManager) Status() (MigrationStatus, error) {
	markers := []struct {
		name   string
		status MigrationStatus
	}{
		{".migration_completed", MigrationStatusCompleted},
		{".migration_rollback", MigrationStatusRollback},
		{".migration_failed", MigrationStatusFailed},
		{".migration_started", MigrationStatusStarted},
	}

	for _, marker := range markers {
		path := filepath.Join(m.dataDir, marker.name)
		if _, err := os.Stat(path); err == nil {
			return marker.status, nil
		}
	}

	return MigrationStatusNone, nil
}

// Migrate safely migrates to BadgerDB storage.
func (m *MigrationManager) Migrate() error {
	// Check current status
	status, err := m.Status()
	if err != nil {
		return fmt.Errorf("failed to check status: %w", err)
	}

	if status == MigrationStatusCompleted {
		return fmt.Errorf("migration already completed")
	}

	// 1. Create backup
	backupDir, err := m.createBackup()
	if err != nil {
		return fmt.Errorf("backup failed: %w", err)
	}
	fmt.Printf("Backup created at: %s\n", backupDir)

	// 2. Test BadgerDB creation
	if err := m.testBadgerDB(); err != nil {
		return fmt.Errorf("BadgerDB test failed: %w", err)
	}
	fmt.Println("BadgerDB test passed")

	// 3. Mark migration started
	if err := m.markMigration("started"); err != nil {
		return fmt.Errorf("failed to mark started: %w", err)
	}

	// 4. Create BadgerDB directories
	badgerDir := filepath.Join(m.dataDir, "badger")
	for _, name := range []string{"delta", "audit"} {
		dir := filepath.Join(badgerDir, name)
		if err := os.MkdirAll(dir, 0750); err != nil {
			m.markMigration("failed")
			return fmt.Errorf("failed to create %s: %w", name, err)
		}
	}
	fmt.Println("BadgerDB directories created")

	// 5. Mark migration completed
	// Clean up started marker
	os.Remove(filepath.Join(m.dataDir, ".migration_started"))

	if err := m.markMigration("completed"); err != nil {
		return fmt.Errorf("failed to mark completed: %w", err)
	}

	fmt.Println("Migration completed successfully")
	return nil
}

// Rollback reverts to the previous storage state.
func (m *MigrationManager) Rollback() error {
	// Remove BadgerDB directories
	badgerDir := filepath.Join(m.dataDir, "badger")
	if err := os.RemoveAll(badgerDir); err != nil {
		return fmt.Errorf("failed to remove badger directory: %w", err)
	}
	fmt.Println("BadgerDB directories removed")

	// Remove migration markers
	markers := []string{
		".migration_started",
		".migration_completed",
		".migration_failed",
	}
	for _, marker := range markers {
		os.Remove(filepath.Join(m.dataDir, marker))
	}

	// Mark rollback
	if err := m.markMigration("rollback"); err != nil {
		return fmt.Errorf("failed to mark rollback: %w", err)
	}

	fmt.Println("Rollback completed successfully")
	return nil
}

// createBackup creates a backup of existing data.
func (m *MigrationManager) createBackup() (string, error) {
	timestamp := time.Now().Format("20060102_150405")
	backupDir := filepath.Join(m.dataDir, "backup_"+timestamp)

	if err := os.MkdirAll(backupDir, 0750); err != nil {
		return "", fmt.Errorf("failed to create backup dir: %w", err)
	}

	// Directories to backup
	dirs := []string{"vectors", "metrics"}

	for _, dir := range dirs {
		src := filepath.Join(m.dataDir, dir)
		if _, err := os.Stat(src); os.IsNotExist(err) {
			continue // Skip if doesn't exist
		}

		dst := filepath.Join(backupDir, dir)
		if err := copyDir(src, dst); err != nil {
			return "", fmt.Errorf("failed to backup %s: %w", dir, err)
		}
	}

	return backupDir, nil
}

// testBadgerDB tests that BadgerDB can be opened.
func (m *MigrationManager) testBadgerDB() error {
	testPath := filepath.Join(m.dataDir, ".badger_test")
	defer os.RemoveAll(testPath)

	opts := badger.DefaultOptions(testPath).
		WithLogger(nil)

	db, err := badger.Open(opts)
	if err != nil {
		return fmt.Errorf("failed to open test db: %w", err)
	}
	defer db.Close()

	// Test write/read
	err = db.Update(func(txn *badger.Txn) error {
		return txn.Set([]byte("test"), []byte("value"))
	})
	if err != nil {
		return fmt.Errorf("failed to write: %w", err)
	}

	err = db.View(func(txn *badger.Txn) error {
		item, err := txn.Get([]byte("test"))
		if err != nil {
			return err
		}
		return item.Value(func(val []byte) error {
			if string(val) != "value" {
				return fmt.Errorf("unexpected value: %s", val)
			}
			return nil
		})
	})
	if err != nil {
		return fmt.Errorf("failed to read: %w", err)
	}

	return nil
}

// markMigration writes a migration marker file.
func (m *MigrationManager) markMigration(status string) error {
	markerPath := filepath.Join(m.dataDir, ".migration_"+status)
	content := fmt.Sprintf("Migration %s at %s\n", status, time.Now().Format(time.RFC3339))
	return os.WriteFile(markerPath, []byte(content), 0600)
}

// ListBackups returns a list of available backups.
func (m *MigrationManager) ListBackups() ([]string, error) {
	entries, err := os.ReadDir(m.dataDir)
	if err != nil {
		return nil, err
	}

	var backups []string
	for _, entry := range entries {
		if entry.IsDir() && len(entry.Name()) > 7 && entry.Name()[:7] == "backup_" {
			backups = append(backups, entry.Name())
		}
	}

	return backups, nil
}

// RestoreBackup restores from a specific backup.
func (m *MigrationManager) RestoreBackup(backupName string) error {
	backupDir := filepath.Join(m.dataDir, backupName)
	if _, err := os.Stat(backupDir); os.IsNotExist(err) {
		return fmt.Errorf("backup not found: %s", backupName)
	}

	// Restore directories
	dirs := []string{"vectors", "metrics"}
	for _, dir := range dirs {
		src := filepath.Join(backupDir, dir)
		if _, err := os.Stat(src); os.IsNotExist(err) {
			continue
		}

		dst := filepath.Join(m.dataDir, dir)

		// Remove existing
		os.RemoveAll(dst)

		// Copy from backup
		if err := copyDir(src, dst); err != nil {
			return fmt.Errorf("failed to restore %s: %w", dir, err)
		}
	}

	return nil
}

// copyDir recursively copies a directory.
func copyDir(src, dst string) error {
	srcInfo, err := os.Stat(src)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(dst, srcInfo.Mode()); err != nil {
		return err
	}

	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())

		if entry.IsDir() {
			if err := copyDir(srcPath, dstPath); err != nil {
				return err
			}
		} else {
			if err := copyFile(srcPath, dstPath); err != nil {
				return err
			}
		}
	}

	return nil
}

// copyFile copies a single file.
// The src and dst paths are constructed internally from trusted base directories,
// so path traversal is not a concern here.
func copyFile(src, dst string) error {
	srcFile, err := os.Open(src) // #nosec G304 - paths are constructed from trusted base directories
	if err != nil {
		return err
	}
	defer srcFile.Close()

	srcInfo, err := srcFile.Stat()
	if err != nil {
		return err
	}

	dstFile, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, srcInfo.Mode()) // #nosec G304 - paths are constructed from trusted base directories
	if err != nil {
		return err
	}
	defer dstFile.Close()

	_, err = io.Copy(dstFile, srcFile)
	return err
}
