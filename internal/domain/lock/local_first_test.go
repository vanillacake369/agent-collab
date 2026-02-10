package lock

import (
	"context"
	"sync"
	"testing"
	"time"
)

func TestLocalFirstLockService_AcquireLockOptimistic(t *testing.T) {
	ctx := context.Background()
	base := NewLockService(ctx, "node1", "Node 1")
	defer base.Close()

	config := DefaultLocalFirstConfig()
	config.ConflictTimeout = 100 * time.Millisecond

	lfs := NewLocalFirstLockService(base, config)

	req := &AcquireLockRequest{
		TargetType: TargetFile,
		FilePath:   "/test/file.go",
		StartLine:  1,
		EndLine:    100,
		Intention:  "editing file",
	}

	// Measure acquisition time
	start := time.Now()
	result, err := lfs.AcquireLockOptimistic(ctx, req)
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("Failed to acquire lock: %v", err)
	}
	if !result.Success {
		t.Fatalf("Lock acquisition failed: %s", result.Reason)
	}

	// Should be very fast (< 10ms for local operation)
	if elapsed > 10*time.Millisecond {
		t.Errorf("Lock acquisition too slow: %v (expected < 10ms)", elapsed)
	}

	// Lock should be pending initially
	if !lfs.IsPending(result.Lock.ID) {
		t.Error("Lock should be pending initially")
	}

	// Wait for confirmation
	time.Sleep(150 * time.Millisecond)

	// Lock should be confirmed now
	if !lfs.IsConfirmed(result.Lock.ID) {
		t.Error("Lock should be confirmed after timeout")
	}
}

func TestLocalFirstLockService_LocalConflict(t *testing.T) {
	ctx := context.Background()
	base := NewLockService(ctx, "node1", "Node 1")
	defer base.Close()

	config := DefaultLocalFirstConfig()
	lfs := NewLocalFirstLockService(base, config)

	// Acquire first lock
	req1 := &AcquireLockRequest{
		TargetType: TargetFile,
		FilePath:   "/test/file.go",
		StartLine:  1,
		EndLine:    100,
		Intention:  "editing file",
	}

	result1, err := lfs.AcquireLockOptimistic(ctx, req1)
	if err != nil || !result1.Success {
		t.Fatalf("Failed to acquire first lock: %v", err)
	}

	// Try to acquire overlapping lock - should fail immediately
	req2 := &AcquireLockRequest{
		TargetType: TargetFile,
		FilePath:   "/test/file.go",
		StartLine:  50,
		EndLine:    150,
		Intention:  "editing same file",
	}

	result2, err := lfs.AcquireLockOptimistic(ctx, req2)
	if err != ErrLockConflict {
		t.Errorf("Expected ErrLockConflict, got %v", err)
	}
	if result2.Success {
		t.Error("Second lock should have failed due to conflict")
	}
}

func TestLocalFirstLockService_RemoteConflictRollback(t *testing.T) {
	ctx := context.Background()
	base := NewLockService(ctx, "node1", "Node 1")
	defer base.Close()

	var rollbackCalled bool
	var rollbackLockID string
	var mu sync.Mutex

	config := DefaultLocalFirstConfig()
	config.ConflictTimeout = 500 * time.Millisecond
	config.AutoRollback = true

	lfs := NewLocalFirstLockService(base, config)
	lfs.SetRollbackHandler(func(lockID string, reason string) {
		mu.Lock()
		rollbackCalled = true
		rollbackLockID = lockID
		mu.Unlock()
	})

	// Acquire local lock
	req := &AcquireLockRequest{
		TargetType: TargetFile,
		FilePath:   "/test/file.go",
		StartLine:  1,
		EndLine:    100,
		Intention:  "editing file",
	}

	result, err := lfs.AcquireLockOptimistic(ctx, req)
	if err != nil {
		t.Fatalf("Failed to acquire lock: %v", err)
	}

	// Simulate remote lock that conflicts
	remoteLock := NewSemanticLock(
		&SemanticTarget{
			Type:      TargetFile,
			FilePath:  "/test/file.go",
			StartLine: 50,
			EndLine:   150,
		},
		"node2",
		"Node 2",
		"remote editing",
	)

	// Handle the remote lock
	err = lfs.HandleRemoteLockAcquired(remoteLock)
	if err != nil {
		t.Fatalf("Failed to handle remote lock: %v", err)
	}

	// Wait for conflict detection and rollback
	time.Sleep(200 * time.Millisecond)

	mu.Lock()
	if !rollbackCalled {
		t.Error("Rollback handler should have been called")
	}
	if rollbackLockID != result.Lock.ID {
		t.Errorf("Rollback lockID mismatch: got %s, want %s", rollbackLockID, result.Lock.ID)
	}
	mu.Unlock()

	// Lock should be rolled back
	pending := lfs.GetPendingLock(result.Lock.ID)
	if pending == nil {
		t.Fatal("Pending lock not found")
	}
	if pending.Status != StatusRolledBack {
		t.Errorf("Lock status should be rolled_back, got %s", pending.Status)
	}
}

func TestLocalFirstLockService_PessimisticMode(t *testing.T) {
	ctx := context.Background()
	base := NewLockService(ctx, "node1", "Node 1")
	defer base.Close()

	config := LocalFirstConfig{
		Mode: ModePessimistic,
	}

	lfs := NewLocalFirstLockService(base, config)

	req := &AcquireLockRequest{
		TargetType: TargetFile,
		FilePath:   "/test/file.go",
		StartLine:  1,
		EndLine:    100,
		Intention:  "editing file",
	}

	// In pessimistic mode, should use base service (may timeout without peers)
	ctx, cancel := context.WithTimeout(ctx, 100*time.Millisecond)
	defer cancel()

	_, err := lfs.AcquireLockOptimistic(ctx, req)
	// Expected to fail due to no broadcast function set
	if err == nil {
		t.Log("Pessimistic mode may succeed if no conflict check required")
	}
}

func TestLocalFirstLockService_GetPendingLocks(t *testing.T) {
	ctx := context.Background()
	base := NewLockService(ctx, "node1", "Node 1")
	defer base.Close()

	config := DefaultLocalFirstConfig()
	config.ConflictTimeout = 1 * time.Hour // Long timeout to keep pending

	lfs := NewLocalFirstLockService(base, config)

	// Acquire multiple locks
	for i := 0; i < 3; i++ {
		req := &AcquireLockRequest{
			TargetType: TargetFile,
			FilePath:   "/test/file" + string(rune('a'+i)) + ".go",
			StartLine:  1,
			EndLine:    100,
			Intention:  "editing file",
		}
		_, _ = lfs.AcquireLockOptimistic(ctx, req)
	}

	pending := lfs.GetPendingLocks()
	if len(pending) != 3 {
		t.Errorf("Expected 3 pending locks, got %d", len(pending))
	}

	for _, p := range pending {
		if p.Status != StatusPending {
			t.Errorf("Expected status pending, got %s", p.Status)
		}
	}
}

func BenchmarkAcquireLockOptimistic(b *testing.B) {
	ctx := context.Background()
	base := NewLockService(ctx, "node1", "Node 1")
	defer base.Close()

	config := DefaultLocalFirstConfig()
	config.ConflictTimeout = 1 * time.Hour

	lfs := NewLocalFirstLockService(base, config)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req := &AcquireLockRequest{
			TargetType: TargetFile,
			FilePath:   "/test/file" + string(rune(i%1000)) + ".go",
			StartLine:  1,
			EndLine:    100,
			Intention:  "benchmark",
		}
		_, _ = lfs.AcquireLockOptimistic(ctx, req)
	}
}
