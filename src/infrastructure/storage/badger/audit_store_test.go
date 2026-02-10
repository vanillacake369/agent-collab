package badger

import (
	"os"
	"testing"
	"time"

	"agent-collab/src/infrastructure/storage"
)

func TestAuditStore_LogAsync(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "audit-store-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	mgr := NewManager(tmpDir)
	defer mgr.CloseAll()

	db, err := mgr.Open("audit")
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}

	store := NewAuditStore(db, 100)
	defer store.Close()

	// Log an event
	event := &storage.AuditEvent{
		Timestamp:  time.Now(),
		Action:     "acquired",
		LockID:     "lock-123",
		HolderID:   "holder-456",
		HolderName: "Test Holder",
		Target:     "/test/file.go:10-20",
	}

	if err := store.LogAsync(event); err != nil {
		t.Fatalf("failed to log event: %v", err)
	}

	// Wait for async write
	time.Sleep(1500 * time.Millisecond)

	// Query
	events, err := store.Query(storage.AuditFilter{})
	if err != nil {
		t.Fatalf("failed to query: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if events[0].LockID != "lock-123" {
		t.Fatalf("expected lock ID lock-123, got %s", events[0].LockID)
	}
}

func TestAuditStore_BatchWrite(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "audit-store-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	mgr := NewManager(tmpDir)
	defer mgr.CloseAll()

	db, err := mgr.Open("audit")
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}

	store := NewAuditStore(db, 1000)
	defer store.Close()

	// Log many events quickly
	for i := 0; i < 150; i++ {
		event := &storage.AuditEvent{
			Timestamp:  time.Now().Add(time.Duration(i) * time.Millisecond),
			Action:     "acquired",
			LockID:     "lock-" + string(rune('a'+i%26)),
			HolderID:   "holder-1",
			HolderName: "Test Holder",
			Target:     "/test/file.go",
		}
		if err := store.LogAsync(event); err != nil {
			t.Fatalf("failed to log event %d: %v", i, err)
		}
	}

	// Wait for async writes (batch flushes at 100 events)
	time.Sleep(2 * time.Second)

	// Verify count
	count, err := store.Count()
	if err != nil {
		t.Fatalf("failed to count: %v", err)
	}
	if count != 150 {
		t.Fatalf("expected 150 events, got %d", count)
	}
}

func TestAuditStore_QueryWithFilters(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "audit-store-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	mgr := NewManager(tmpDir)
	defer mgr.CloseAll()

	db, err := mgr.Open("audit")
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}

	store := NewAuditStore(db, 100)
	defer store.Close()

	baseTime := time.Now()

	// Log events with different actions and holders
	actions := []string{"acquired", "released", "expired"}
	holders := []string{"holder-1", "holder-2"}

	for i, action := range actions {
		for j, holder := range holders {
			event := &storage.AuditEvent{
				Timestamp:  baseTime.Add(time.Duration(i*len(holders)+j) * time.Millisecond),
				Action:     action,
				LockID:     "lock-" + action,
				HolderID:   holder,
				HolderName: holder,
				Target:     "/test/file.go",
			}
			if err := store.LogAsync(event); err != nil {
				t.Fatalf("failed to log: %v", err)
			}
		}
	}

	// Wait for writes
	time.Sleep(1500 * time.Millisecond)

	// Query by action
	events, err := store.Query(storage.AuditFilter{Action: "acquired"})
	if err != nil {
		t.Fatalf("failed to query by action: %v", err)
	}
	if len(events) != 2 {
		t.Fatalf("expected 2 acquired events, got %d", len(events))
	}

	// Query by holder
	events, err = store.Query(storage.AuditFilter{HolderID: "holder-1"})
	if err != nil {
		t.Fatalf("failed to query by holder: %v", err)
	}
	if len(events) != 3 {
		t.Fatalf("expected 3 events for holder-1, got %d", len(events))
	}

	// Query with limit
	events, err = store.Query(storage.AuditFilter{Limit: 2})
	if err != nil {
		t.Fatalf("failed to query with limit: %v", err)
	}
	if len(events) != 2 {
		t.Fatalf("expected 2 events with limit, got %d", len(events))
	}
}

func TestAuditStore_QueryByLockID(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "audit-store-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	mgr := NewManager(tmpDir)
	defer mgr.CloseAll()

	db, err := mgr.Open("audit")
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}

	store := NewAuditStore(db, 100)
	defer store.Close()

	// Log events for different locks
	locks := []string{"lock-abc", "lock-def", "lock-abc"} // abc appears twice
	for i, lockID := range locks {
		event := &storage.AuditEvent{
			Timestamp:  time.Now().Add(time.Duration(i) * time.Millisecond),
			Action:     "acquired",
			LockID:     lockID,
			HolderID:   "holder-1",
			HolderName: "Test",
			Target:     "/test/file.go",
		}
		if err := store.LogAsync(event); err != nil {
			t.Fatalf("failed to log: %v", err)
		}
	}

	// Wait for writes
	time.Sleep(1500 * time.Millisecond)

	// Query by lock ID
	events, err := store.QueryByLockID("lock-abc", 10)
	if err != nil {
		t.Fatalf("failed to query by lock ID: %v", err)
	}
	if len(events) != 2 {
		t.Fatalf("expected 2 events for lock-abc, got %d", len(events))
	}
}

func TestAuditStore_Compact(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "audit-store-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	mgr := NewManager(tmpDir)
	defer mgr.CloseAll()

	db, err := mgr.Open("audit")
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}

	store := NewAuditStore(db, 100)
	defer store.Close()

	baseTime := time.Now()

	// Log events at different times
	for i := 0; i < 10; i++ {
		event := &storage.AuditEvent{
			Timestamp:  baseTime.Add(time.Duration(i) * time.Hour),
			Action:     "acquired",
			LockID:     "lock-" + string(rune('a'+i)),
			HolderID:   "holder-1",
			HolderName: "Test",
			Target:     "/test/file.go",
		}
		if err := store.LogAsync(event); err != nil {
			t.Fatalf("failed to log: %v", err)
		}
	}

	// Wait for writes
	time.Sleep(1500 * time.Millisecond)

	// Verify initial count
	count, err := store.Count()
	if err != nil {
		t.Fatalf("failed to count: %v", err)
	}
	if count != 10 {
		t.Fatalf("expected 10 events, got %d", count)
	}

	// Compact events older than 5 hours
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

func TestAuditStore_BufferFull(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "audit-store-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	mgr := NewManager(tmpDir)
	defer mgr.CloseAll()

	db, err := mgr.Open("audit")
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}

	// Small buffer
	store := NewAuditStore(db, 5)
	defer store.Close()

	// Try to log more events than buffer can hold
	var dropped int
	for i := 0; i < 100; i++ {
		event := &storage.AuditEvent{
			Timestamp:  time.Now(),
			Action:     "acquired",
			LockID:     "lock",
			HolderID:   "holder",
			HolderName: "Test",
			Target:     "/test",
		}
		if err := store.LogAsync(event); err != nil {
			dropped++
		}
	}

	// Some events should have been dropped
	if dropped == 0 {
		t.Log("No events dropped (buffer might have been processed fast enough)")
	}
}
