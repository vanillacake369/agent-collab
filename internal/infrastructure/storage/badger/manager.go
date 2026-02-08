// Package badger provides BadgerDB storage implementations.
package badger

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/dgraph-io/badger/v4"
)

// Manager manages multiple named BadgerDB instances.
// Each store (delta, audit) gets its own isolated instance to prevent
// namespace collision and cache pollution.
type Manager struct {
	mu        sync.RWMutex
	baseDir   string
	instances map[string]*badger.DB
	opts      badger.Options
}

// NewManager creates a new BadgerDB manager.
// baseDir is the parent directory where all BadgerDB instances will be stored.
func NewManager(baseDir string) *Manager {
	badgerDir := filepath.Join(baseDir, "badger")

	opts := badger.DefaultOptions("").
		WithLogger(nil).                 // Disable verbose logging
		WithValueLogFileSize(64 << 20).  // 64MB value log
		WithNumVersionsToKeep(1).        // Keep only latest version
		WithCompactL0OnClose(true).      // Compact on close
		WithDetectConflicts(false).      // No MVCC conflicts needed
		WithNumCompactors(2).            // Background compaction workers
		WithBlockCacheSize(32 << 20).    // 32MB block cache
		WithIndexCacheSize(16 << 20)     // 16MB index cache

	return &Manager{
		baseDir:   badgerDir,
		instances: make(map[string]*badger.DB),
		opts:      opts,
	}
}

// Open opens or returns an existing BadgerDB instance by name.
// The instance is stored in a subdirectory named after the given name.
func (m *Manager) Open(name string) (*badger.DB, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Return existing instance if already open
	if db, exists := m.instances[name]; exists {
		return db, nil
	}

	// Create instance directory
	dbPath := filepath.Join(m.baseDir, name)
	if err := os.MkdirAll(dbPath, 0750); err != nil {
		return nil, fmt.Errorf("failed to create directory %s: %w", dbPath, err)
	}

	// Open BadgerDB with configured options
	opts := m.opts.WithDir(dbPath).WithValueDir(dbPath)

	db, err := badger.Open(opts)
	if err != nil {
		return nil, fmt.Errorf("failed to open badger db %s: %w", name, err)
	}

	m.instances[name] = db
	return db, nil
}

// Get returns an existing BadgerDB instance without opening a new one.
// Returns nil if the instance doesn't exist.
func (m *Manager) Get(name string) *badger.DB {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.instances[name]
}

// Close closes a specific BadgerDB instance.
func (m *Manager) Close(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	db, exists := m.instances[name]
	if !exists {
		return nil
	}

	delete(m.instances, name)
	return db.Close()
}

// CloseAll closes all managed BadgerDB instances.
func (m *Manager) CloseAll() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	var firstErr error
	for name, db := range m.instances {
		if err := db.Close(); err != nil && firstErr == nil {
			firstErr = fmt.Errorf("failed to close %s: %w", name, err)
		}
	}
	m.instances = make(map[string]*badger.DB)
	return firstErr
}

// RunGC runs garbage collection on all instances.
// discardRatio is the ratio of old data to reclaim (0.0 to 1.0).
// Recommended value is 0.5.
func (m *Manager) RunGC(discardRatio float64) error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for name, db := range m.instances {
		for {
			err := db.RunValueLogGC(discardRatio)
			if err == badger.ErrNoRewrite {
				break // No more GC needed
			}
			if err != nil {
				return fmt.Errorf("GC failed for %s: %w", name, err)
			}
		}
	}
	return nil
}

// RunGCFor runs garbage collection on a specific instance.
func (m *Manager) RunGCFor(name string, discardRatio float64) error {
	m.mu.RLock()
	db, exists := m.instances[name]
	m.mu.RUnlock()

	if !exists {
		return fmt.Errorf("instance %s not found", name)
	}

	for {
		err := db.RunValueLogGC(discardRatio)
		if err == badger.ErrNoRewrite {
			return nil
		}
		if err != nil {
			return fmt.Errorf("GC failed for %s: %w", name, err)
		}
	}
}

// Stats returns database statistics for a specific instance.
func (m *Manager) Stats(name string) (map[string]interface{}, error) {
	m.mu.RLock()
	db, exists := m.instances[name]
	m.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("instance %s not found", name)
	}

	lsm, vlog := db.Size()
	return map[string]interface{}{
		"lsm_size":   lsm,
		"vlog_size":  vlog,
		"total_size": lsm + vlog,
	}, nil
}

// Instances returns the names of all open instances.
func (m *Manager) Instances() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	names := make([]string, 0, len(m.instances))
	for name := range m.instances {
		names = append(names, name)
	}
	return names
}
