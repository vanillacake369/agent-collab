package badger

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/dgraph-io/badger/v4"
)

func TestManager_OpenClose(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "badger-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	mgr := NewManager(tmpDir)
	defer mgr.CloseAll()

	// Open first instance
	db1, err := mgr.Open("test1")
	if err != nil {
		t.Fatalf("failed to open test1: %v", err)
	}
	if db1 == nil {
		t.Fatal("expected non-nil db")
	}

	// Opening same instance should return same db
	db1Again, err := mgr.Open("test1")
	if err != nil {
		t.Fatalf("failed to reopen test1: %v", err)
	}
	if db1 != db1Again {
		t.Fatal("expected same db instance")
	}

	// Open second instance
	db2, err := mgr.Open("test2")
	if err != nil {
		t.Fatalf("failed to open test2: %v", err)
	}
	if db2 == nil {
		t.Fatal("expected non-nil db2")
	}
	if db1 == db2 {
		t.Fatal("expected different db instances")
	}

	// Verify instances list
	instances := mgr.Instances()
	if len(instances) != 2 {
		t.Fatalf("expected 2 instances, got %d", len(instances))
	}

	// Close one instance
	if err := mgr.Close("test1"); err != nil {
		t.Fatalf("failed to close test1: %v", err)
	}

	// Verify closed instance is removed
	if mgr.Get("test1") != nil {
		t.Fatal("expected nil after close")
	}

	// test2 should still be open
	if mgr.Get("test2") == nil {
		t.Fatal("expected test2 to still be open")
	}
}

func TestManager_MultipleInstances(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "badger-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	mgr := NewManager(tmpDir)

	// Open multiple instances
	names := []string{"delta", "audit", "cache"}
	for _, name := range names {
		db, err := mgr.Open(name)
		if err != nil {
			t.Fatalf("failed to open %s: %v", name, err)
		}

		// Test write/read
		err = db.Update(func(txn *badger.Txn) error {
			return txn.Set([]byte("key"), []byte("value-"+name))
		})
		if err != nil {
			t.Fatalf("failed to write to %s: %v", name, err)
		}
	}

	// Verify directories created
	for _, name := range names {
		dbPath := filepath.Join(tmpDir, "badger", name)
		if _, err := os.Stat(dbPath); os.IsNotExist(err) {
			t.Fatalf("expected directory %s to exist", dbPath)
		}
	}

	// Close all
	if err := mgr.CloseAll(); err != nil {
		t.Fatalf("failed to close all: %v", err)
	}

	// Verify all closed
	if len(mgr.Instances()) != 0 {
		t.Fatal("expected no instances after CloseAll")
	}
}

func TestManager_Stats(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "badger-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	mgr := NewManager(tmpDir)
	defer mgr.CloseAll()

	db, err := mgr.Open("test")
	if err != nil {
		t.Fatalf("failed to open: %v", err)
	}

	// Write some data
	for i := 0; i < 100; i++ {
		err = db.Update(func(txn *badger.Txn) error {
			key := []byte("key-" + string(rune('a'+i%26)))
			val := make([]byte, 1024) // 1KB value
			return txn.Set(key, val)
		})
		if err != nil {
			t.Fatalf("failed to write: %v", err)
		}
	}

	// Get stats
	stats, err := mgr.Stats("test")
	if err != nil {
		t.Fatalf("failed to get stats: %v", err)
	}

	if stats["lsm_size"] == nil {
		t.Fatal("expected lsm_size in stats")
	}
	if stats["vlog_size"] == nil {
		t.Fatal("expected vlog_size in stats")
	}
	if stats["total_size"] == nil {
		t.Fatal("expected total_size in stats")
	}

	// Stats for non-existent instance should fail
	_, err = mgr.Stats("nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent instance")
	}
}

func TestManager_GC(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "badger-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	mgr := NewManager(tmpDir)
	defer mgr.CloseAll()

	_, err = mgr.Open("test")
	if err != nil {
		t.Fatalf("failed to open: %v", err)
	}

	// GC should not fail on empty database
	err = mgr.RunGC(0.5)
	if err != nil {
		t.Fatalf("GC failed: %v", err)
	}

	// GC on specific instance
	err = mgr.RunGCFor("test", 0.5)
	if err != nil {
		t.Fatalf("GC for test failed: %v", err)
	}

	// GC on non-existent should fail
	err = mgr.RunGCFor("nonexistent", 0.5)
	if err == nil {
		t.Fatal("expected error for nonexistent instance")
	}
}
