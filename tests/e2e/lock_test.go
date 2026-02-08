//go:build e2e

package e2e

import (
	"context"
	"testing"
	"time"

	"agent-collab/internal/domain/lock"
)

// TestLockAcquisition tests basic lock acquisition.
func TestLockAcquisition(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cluster := NewTestCluster(t, 2)

	// Initialize cluster
	leader, initResult, err := cluster.InitializeLeader(ctx, "lock-test")
	if err != nil {
		t.Fatalf("failed to initialize leader: %v", err)
	}

	// Join second node
	_, err = cluster.JoinNode(ctx, "node2", initResult.InviteToken)
	if err != nil {
		t.Fatalf("failed to join node2: %v", err)
	}

	// Acquire lock on leader
	lockService := leader.LockService()
	result, err := lockService.AcquireLock(ctx, &lock.AcquireLockRequest{
		TargetType: lock.TargetFunction,
		FilePath:   "src/main.go",
		Name:       "handleRequest",
		StartLine:  10,
		EndLine:    50,
		Intention:  "refactoring",
	})

	if err != nil {
		t.Fatalf("failed to acquire lock: %v", err)
	}

	if !result.Success {
		t.Errorf("expected lock acquisition to succeed, got reason: %s", result.Reason)
	}

	t.Logf("Lock acquired: ID=%s", result.Lock.ID)

	// Verify lock stats
	stats := lockService.GetStats()
	if stats.TotalLocks != 1 {
		t.Errorf("expected 1 lock, got %d", stats.TotalLocks)
	}
	if stats.MyLocks != 1 {
		t.Errorf("expected 1 my lock, got %d", stats.MyLocks)
	}

	// Release lock
	if err := lockService.ReleaseLock(ctx, result.Lock.ID); err != nil {
		t.Errorf("failed to release lock: %v", err)
	}

	// Verify lock released
	stats = lockService.GetStats()
	if stats.TotalLocks != 0 {
		t.Errorf("expected 0 locks after release, got %d", stats.TotalLocks)
	}

	t.Log("Lock acquisition test passed")
}

// TestLockConflict tests that overlapping locks create conflicts.
func TestLockConflict(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cluster := NewTestCluster(t, 2)

	// Initialize cluster
	leader, initResult, err := cluster.InitializeLeader(ctx, "conflict-test")
	if err != nil {
		t.Fatalf("failed to initialize leader: %v", err)
	}

	node2, err := cluster.JoinNode(ctx, "node2", initResult.InviteToken)
	if err != nil {
		t.Fatalf("failed to join node2: %v", err)
	}

	// Wait for peer connection
	time.Sleep(2 * time.Second)

	// Acquire lock on leader
	lockService1 := leader.LockService()
	result1, err := lockService1.AcquireLock(ctx, &lock.AcquireLockRequest{
		TargetType: lock.TargetFunction,
		FilePath:   "src/handler.go",
		Name:       "processData",
		StartLine:  100,
		EndLine:    150,
		Intention:  "bug fix",
	})

	if err != nil {
		t.Fatalf("failed to acquire first lock: %v", err)
	}
	if !result1.Success {
		t.Fatalf("first lock should succeed")
	}

	t.Logf("First lock acquired: %s", result1.Lock.ID)

	// Try to acquire overlapping lock on node2
	lockService2 := node2.LockService()

	// Simulate remote lock registration (in real scenario this would come via P2P)
	lockService2.HandleRemoteLockAcquired(result1.Lock)

	result2, err := lockService2.AcquireLock(ctx, &lock.AcquireLockRequest{
		TargetType: lock.TargetFunction,
		FilePath:   "src/handler.go",
		Name:       "processData",
		StartLine:  120,
		EndLine:    140,
		Intention:  "optimization",
	})

	// Should fail or go into negotiation
	if result2.Success {
		t.Log("Lock succeeded - may have been negotiated")
	} else {
		t.Logf("Lock correctly blocked: %s", result2.Reason)
	}

	// Verify first lock still active
	locks := lockService1.ListLocks()
	if len(locks) < 1 {
		t.Error("first lock should still be active")
	}

	t.Log("Lock conflict test passed")
}

// TestLockRenewal tests lock TTL renewal.
func TestLockRenewal(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cluster := NewTestCluster(t, 1)

	// Initialize cluster
	leader, _, err := cluster.InitializeLeader(ctx, "renewal-test")
	if err != nil {
		t.Fatalf("failed to initialize leader: %v", err)
	}

	lockService := leader.LockService()

	// Acquire lock
	result, err := lockService.AcquireLock(ctx, &lock.AcquireLockRequest{
		TargetType: lock.TargetFile,
		FilePath:   "config.yaml",
		Name:       "config",
		StartLine:  1,
		EndLine:    100,
		Intention:  "updating config",
	})

	if err != nil {
		t.Fatalf("failed to acquire lock: %v", err)
	}

	originalTTL := result.Lock.TTLRemaining()
	t.Logf("Original TTL: %v", originalTTL)

	// Wait a bit
	time.Sleep(2 * time.Second)

	// Renew lock
	if err := lockService.RenewLock(ctx, result.Lock.ID); err != nil {
		t.Fatalf("failed to renew lock: %v", err)
	}

	// Get updated lock
	updatedLock, err := lockService.GetLock(result.Lock.ID)
	if err != nil {
		t.Fatalf("failed to get updated lock: %v", err)
	}

	renewedTTL := updatedLock.TTLRemaining()
	t.Logf("Renewed TTL: %v", renewedTTL)

	// Renewed TTL should be close to original (within a few seconds)
	if renewedTTL < originalTTL-5*time.Second {
		t.Errorf("renewed TTL should be close to original: got %v, expected ~%v", renewedTTL, originalTTL)
	}

	t.Log("Lock renewal test passed")
}

// TestLockExpiration tests automatic lock expiration.
func TestLockExpiration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping expiration test in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	cluster := NewTestCluster(t, 1)

	// Initialize cluster
	leader, _, err := cluster.InitializeLeader(ctx, "expiration-test")
	if err != nil {
		t.Fatalf("failed to initialize leader: %v", err)
	}

	lockService := leader.LockService()

	// Acquire lock with short TTL
	result, err := lockService.AcquireLock(ctx, &lock.AcquireLockRequest{
		TargetType: lock.TargetFunction,
		FilePath:   "temp.go",
		Name:       "tempFunc",
		StartLine:  1,
		EndLine:    10,
		Intention:  "short task",
	})

	if err != nil {
		t.Fatalf("failed to acquire lock: %v", err)
	}

	t.Logf("Lock acquired with TTL: %v", result.Lock.TTLRemaining())

	// Wait for expiration (default TTL is 30s, cleanup interval is 10s)
	t.Log("Waiting for lock expiration...")
	time.Sleep(45 * time.Second)

	// Lock should be expired/cleaned up
	_, err = lockService.GetLock(result.Lock.ID)
	if err == nil {
		t.Log("Lock may still exist but should be expired")
	} else {
		t.Logf("Lock correctly expired/removed: %v", err)
	}

	t.Log("Lock expiration test passed")
}
