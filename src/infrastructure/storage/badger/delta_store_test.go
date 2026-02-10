package badger

import (
	"os"
	"testing"
	"time"

	"agent-collab/src/domain/ctxsync"
)

func TestDeltaStore_SaveAndGet(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "delta-store-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	mgr := NewManager(tmpDir)
	defer mgr.CloseAll()

	db, err := mgr.Open("delta")
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}

	store := NewDeltaStore(db)

	// Create test delta
	vc := ctxsync.NewVectorClock()
	vc.Increment("node1")

	delta := ctxsync.NewDelta(ctxsync.DeltaFileChange, "source1", "Source One", vc)
	delta.Payload.FilePath = "/test/file.go"

	// Save
	if err := store.Save(delta); err != nil {
		t.Fatalf("failed to save delta: %v", err)
	}

	// Get by source
	deltas, err := store.GetBySource("source1", 10)
	if err != nil {
		t.Fatalf("failed to get by source: %v", err)
	}
	if len(deltas) != 1 {
		t.Fatalf("expected 1 delta, got %d", len(deltas))
	}
	if deltas[0].ID != delta.ID {
		t.Fatalf("expected delta ID %s, got %s", delta.ID, deltas[0].ID)
	}
	if deltas[0].Payload.FilePath != "/test/file.go" {
		t.Fatalf("expected file path /test/file.go, got %s", deltas[0].Payload.FilePath)
	}
}

func TestDeltaStore_SaveBatch(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "delta-store-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	mgr := NewManager(tmpDir)
	defer mgr.CloseAll()

	db, err := mgr.Open("delta")
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}

	store := NewDeltaStore(db)

	// Create batch of deltas
	vc := ctxsync.NewVectorClock()
	var deltas []*ctxsync.Delta

	for i := 0; i < 100; i++ {
		vc.Increment("node1")
		delta := ctxsync.NewDelta(ctxsync.DeltaFileChange, "source1", "Source One", vc)
		delta.Timestamp = time.Now().Add(time.Duration(i) * time.Millisecond)
		deltas = append(deltas, delta)
	}

	// Save batch
	if err := store.SaveBatch(deltas); err != nil {
		t.Fatalf("failed to save batch: %v", err)
	}

	// Verify count
	count, err := store.Count()
	if err != nil {
		t.Fatalf("failed to count: %v", err)
	}
	if count != 100 {
		t.Fatalf("expected 100 deltas, got %d", count)
	}
}

func TestDeltaStore_GetRange(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "delta-store-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	mgr := NewManager(tmpDir)
	defer mgr.CloseAll()

	db, err := mgr.Open("delta")
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}

	store := NewDeltaStore(db)

	// Create deltas across different times
	baseTime := time.Now()
	vc := ctxsync.NewVectorClock()

	for i := 0; i < 10; i++ {
		vc.Increment("node1")
		delta := ctxsync.NewDelta(ctxsync.DeltaFileChange, "source1", "Source One", vc)
		delta.Timestamp = baseTime.Add(time.Duration(i) * time.Hour)
		if err := store.Save(delta); err != nil {
			t.Fatalf("failed to save delta %d: %v", i, err)
		}
	}

	// Query range (hours 2-5)
	start := baseTime.Add(2 * time.Hour)
	end := baseTime.Add(5 * time.Hour)

	deltas, err := store.GetRange(start, end)
	if err != nil {
		t.Fatalf("failed to get range: %v", err)
	}

	// Should get deltas at hours 2, 3, 4 (not 5, as end is exclusive in comparison)
	if len(deltas) < 2 || len(deltas) > 4 {
		t.Fatalf("expected 2-4 deltas in range, got %d", len(deltas))
	}
}

func TestDeltaStore_GetBySource(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "delta-store-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	mgr := NewManager(tmpDir)
	defer mgr.CloseAll()

	db, err := mgr.Open("delta")
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}

	store := NewDeltaStore(db)

	vc := ctxsync.NewVectorClock()

	// Create deltas from multiple sources
	sources := []string{"source1", "source2", "source3"}
	for _, source := range sources {
		for i := 0; i < 5; i++ {
			vc.Increment(source)
			delta := ctxsync.NewDelta(ctxsync.DeltaFileChange, source, source, vc)
			delta.Timestamp = time.Now().Add(time.Duration(i) * time.Millisecond)
			if err := store.Save(delta); err != nil {
				t.Fatalf("failed to save: %v", err)
			}
		}
	}

	// Get by source1
	deltas, err := store.GetBySource("source1", 10)
	if err != nil {
		t.Fatalf("failed to get by source: %v", err)
	}
	if len(deltas) != 5 {
		t.Fatalf("expected 5 deltas for source1, got %d", len(deltas))
	}

	// Get by source2 with limit
	deltas, err = store.GetBySource("source2", 3)
	if err != nil {
		t.Fatalf("failed to get by source: %v", err)
	}
	if len(deltas) != 3 {
		t.Fatalf("expected 3 deltas for source2, got %d", len(deltas))
	}

	// Get non-existent source
	deltas, err = store.GetBySource("nonexistent", 10)
	if err != nil {
		t.Fatalf("failed to get by source: %v", err)
	}
	if len(deltas) != 0 {
		t.Fatalf("expected 0 deltas for nonexistent, got %d", len(deltas))
	}
}

func TestDeltaStore_Compact(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "delta-store-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	mgr := NewManager(tmpDir)
	defer mgr.CloseAll()

	db, err := mgr.Open("delta")
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}

	store := NewDeltaStore(db)

	baseTime := time.Now()
	vc := ctxsync.NewVectorClock()

	// Create deltas at different times
	for i := 0; i < 10; i++ {
		vc.Increment("node1")
		delta := ctxsync.NewDelta(ctxsync.DeltaFileChange, "source1", "Source One", vc)
		delta.Timestamp = baseTime.Add(time.Duration(i) * time.Hour)
		if err := store.Save(delta); err != nil {
			t.Fatalf("failed to save: %v", err)
		}
	}

	// Verify initial count
	count, err := store.Count()
	if err != nil {
		t.Fatalf("failed to count: %v", err)
	}
	if count != 10 {
		t.Fatalf("expected 10 deltas, got %d", count)
	}

	// Compact deltas older than 5 hours
	cutoff := baseTime.Add(5 * time.Hour)
	deleted, err := store.Compact(cutoff)
	if err != nil {
		t.Fatalf("failed to compact: %v", err)
	}
	if deleted != 5 {
		t.Fatalf("expected 5 deleted, got %d", deleted)
	}

	// Verify remaining count
	count, err = store.Count()
	if err != nil {
		t.Fatalf("failed to count: %v", err)
	}
	if count != 5 {
		t.Fatalf("expected 5 remaining, got %d", count)
	}
}

func TestDeltaStore_GetSince(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "delta-store-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	mgr := NewManager(tmpDir)
	defer mgr.CloseAll()

	db, err := mgr.Open("delta")
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}

	store := NewDeltaStore(db)

	// Create deltas with incrementing vector clocks
	vc := ctxsync.NewVectorClock()
	for i := 0; i < 5; i++ {
		vc.Increment("node1")
		delta := ctxsync.NewDelta(ctxsync.DeltaFileChange, "source1", "Source One", vc)
		if err := store.Save(delta); err != nil {
			t.Fatalf("failed to save: %v", err)
		}
	}

	// Get all deltas since the beginning
	emptyVC := ctxsync.NewVectorClock()
	deltas, err := store.GetSince(emptyVC)
	if err != nil {
		t.Fatalf("failed to get since: %v", err)
	}
	if len(deltas) != 5 {
		t.Fatalf("expected 5 deltas, got %d", len(deltas))
	}
}
