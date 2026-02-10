package libp2p

import (
	"testing"
	"time"
)

func TestSnapshotManager_CreateSnapshot(t *testing.T) {
	sm := NewSnapshotManager(SnapshotManagerConfig{
		NodeID:       "node1",
		MaxSnapshots: 5,
	})

	sm.SetLockProvider(func() []LockSnapshot {
		return []LockSnapshot{
			{ID: "lock1", HolderID: "node1", FilePath: "/test.go"},
		}
	})

	sm.SetCIDProvider(func() []string {
		return []string{"sha256-abc123", "sha256-def456"}
	})

	snapshot := sm.CreateSnapshot()

	if snapshot == nil {
		t.Fatal("Snapshot should not be nil")
	}

	if snapshot.NodeID != "node1" {
		t.Errorf("NodeID mismatch: got %s", snapshot.NodeID)
	}

	if snapshot.SequenceNum != 1 {
		t.Errorf("SequenceNum should be 1, got %d", snapshot.SequenceNum)
	}

	if len(snapshot.Locks) != 1 {
		t.Errorf("Expected 1 lock, got %d", len(snapshot.Locks))
	}

	if len(snapshot.ContentCIDs) != 2 {
		t.Errorf("Expected 2 CIDs, got %d", len(snapshot.ContentCIDs))
	}
}

func TestSnapshotManager_VectorClock(t *testing.T) {
	sm := NewSnapshotManager(SnapshotManagerConfig{
		NodeID: "node1",
	})

	// Increment local clock
	v1 := sm.IncrementClock()
	if v1 != 1 {
		t.Errorf("First increment should return 1, got %d", v1)
	}

	v2 := sm.IncrementClock()
	if v2 != 2 {
		t.Errorf("Second increment should return 2, got %d", v2)
	}

	// Update from peer
	sm.UpdateClock("node2", 5)
	clock := sm.GetVectorClock()

	if clock["node1"] != 2 {
		t.Errorf("node1 clock should be 2, got %d", clock["node1"])
	}
	if clock["node2"] != 5 {
		t.Errorf("node2 clock should be 5, got %d", clock["node2"])
	}

	// Stale update should be ignored
	sm.UpdateClock("node2", 3)
	clock = sm.GetVectorClock()
	if clock["node2"] != 5 {
		t.Errorf("node2 clock should still be 5, got %d", clock["node2"])
	}
}

func TestSnapshotManager_GetSnapshots(t *testing.T) {
	sm := NewSnapshotManager(SnapshotManagerConfig{
		NodeID:       "node1",
		MaxSnapshots: 5,
	})

	// Create multiple snapshots
	for i := 0; i < 3; i++ {
		sm.CreateSnapshot()
	}

	// Get specific snapshot
	snap := sm.GetSnapshot(2)
	if snap == nil {
		t.Fatal("Snapshot 2 should exist")
	}
	if snap.SequenceNum != 2 {
		t.Errorf("SequenceNum mismatch")
	}

	// Get latest
	latest := sm.GetLatestSnapshot()
	if latest.SequenceNum != 3 {
		t.Errorf("Latest should be 3, got %d", latest.SequenceNum)
	}

	// Get since sequence number
	since := sm.GetSnapshotsSince(1)
	if len(since) != 2 {
		t.Errorf("Expected 2 snapshots since seq 1, got %d", len(since))
	}
}

func TestSnapshotManager_MaxSnapshots(t *testing.T) {
	sm := NewSnapshotManager(SnapshotManagerConfig{
		NodeID:       "node1",
		MaxSnapshots: 3,
	})

	// Create more than max
	for i := 0; i < 5; i++ {
		sm.CreateSnapshot()
	}

	// Should have max snapshots
	sm.mu.RLock()
	count := len(sm.snapshots)
	sm.mu.RUnlock()

	if count != 3 {
		t.Errorf("Should have 3 snapshots, got %d", count)
	}

	// Oldest should be removed
	if sm.GetSnapshot(1) != nil {
		t.Error("Snapshot 1 should have been removed")
	}
	if sm.GetSnapshot(2) != nil {
		t.Error("Snapshot 2 should have been removed")
	}
	if sm.GetSnapshot(5) == nil {
		t.Error("Snapshot 5 should still exist")
	}
}

func TestComputeDelta(t *testing.T) {
	old := &StateSnapshot{
		SequenceNum: 1,
		VectorClock: map[string]uint64{"node1": 1},
		Locks: []LockSnapshot{
			{ID: "lock1", HolderID: "node1"},
			{ID: "lock2", HolderID: "node2"},
		},
		ContentCIDs: []string{"cid1", "cid2"},
	}

	new := &StateSnapshot{
		SequenceNum: 2,
		Timestamp:   time.Now(),
		VectorClock: map[string]uint64{"node1": 2, "node2": 1},
		Locks: []LockSnapshot{
			{ID: "lock2", HolderID: "node2"},
			{ID: "lock3", HolderID: "node1"},
		},
		ContentCIDs: []string{"cid2", "cid3"},
	}

	delta := ComputeDelta(old, new)

	// Check lock changes
	if len(delta.LocksAdded) != 1 {
		t.Errorf("Expected 1 lock added, got %d", len(delta.LocksAdded))
	}
	if len(delta.LocksRemoved) != 1 {
		t.Errorf("Expected 1 lock removed, got %d", len(delta.LocksRemoved))
	}

	// Check CID changes
	if len(delta.CIDsAdded) != 1 {
		t.Errorf("Expected 1 CID added, got %d", len(delta.CIDsAdded))
	}
	if len(delta.CIDsRemoved) != 1 {
		t.Errorf("Expected 1 CID removed, got %d", len(delta.CIDsRemoved))
	}

	// Check vector clock updates
	if delta.VectorClockUpdates["node1"] != 2 {
		t.Errorf("node1 clock should be updated to 2")
	}
	if delta.VectorClockUpdates["node2"] != 1 {
		t.Errorf("node2 clock should be updated to 1")
	}
}

func TestSnapshotDelta_IsEmpty(t *testing.T) {
	emptyDelta := &SnapshotDelta{}
	if !emptyDelta.IsEmpty() {
		t.Error("Empty delta should return true for IsEmpty")
	}

	nonEmptyDelta := &SnapshotDelta{
		LocksAdded: []LockSnapshot{{ID: "lock1"}},
	}
	if nonEmptyDelta.IsEmpty() {
		t.Error("Non-empty delta should return false for IsEmpty")
	}
}

func TestStateSnapshot_SerializeDeserialize(t *testing.T) {
	original := &StateSnapshot{
		Version:     1,
		NodeID:      "node1",
		Timestamp:   time.Now().UTC().Truncate(time.Millisecond),
		SequenceNum: 42,
		VectorClock: map[string]uint64{"node1": 10, "node2": 5},
		Locks: []LockSnapshot{
			{ID: "lock1", HolderID: "node1", FilePath: "/test.go"},
		},
		ContentCIDs: []string{"cid1", "cid2"},
	}

	data, err := original.Serialize()
	if err != nil {
		t.Fatalf("Serialize failed: %v", err)
	}

	restored, err := DeserializeSnapshot(data)
	if err != nil {
		t.Fatalf("Deserialize failed: %v", err)
	}

	if restored.NodeID != original.NodeID {
		t.Errorf("NodeID mismatch")
	}
	if restored.SequenceNum != original.SequenceNum {
		t.Errorf("SequenceNum mismatch")
	}
	if len(restored.Locks) != len(original.Locks) {
		t.Errorf("Locks count mismatch")
	}
}

func TestPartitionRecovery_Merge(t *testing.T) {
	localSnapshot := &StateSnapshot{
		NodeID:      "node1",
		VectorClock: map[string]uint64{"node1": 5},
		Locks: []LockSnapshot{
			{ID: "lock1", HolderID: "node1", AcquiredAt: time.Now().Add(-time.Hour)},
		},
		ContentCIDs: []string{"cid1", "cid2"},
		Peers: []PeerSnapshot{
			{ID: "node2", LastSeen: time.Now().Add(-time.Minute)},
		},
	}

	remoteSnapshot := &StateSnapshot{
		NodeID:      "node2",
		VectorClock: map[string]uint64{"node2": 7},
		Locks: []LockSnapshot{
			{ID: "lock2", HolderID: "node2", AcquiredAt: time.Now()},
		},
		ContentCIDs: []string{"cid2", "cid3"},
		Peers: []PeerSnapshot{
			{ID: "node1", LastSeen: time.Now()},
		},
	}

	pr := NewPartitionRecovery(localSnapshot)
	pr.AddRemoteSnapshot("node2", remoteSnapshot)

	merged := pr.Merge()

	// Check vector clock merge
	if merged.VectorClock["node1"] != 5 {
		t.Errorf("node1 clock should be 5, got %d", merged.VectorClock["node1"])
	}
	if merged.VectorClock["node2"] != 7 {
		t.Errorf("node2 clock should be 7, got %d", merged.VectorClock["node2"])
	}

	// Check locks merge
	if len(merged.Locks) != 2 {
		t.Errorf("Should have 2 locks, got %d", len(merged.Locks))
	}

	// Check CIDs merge (union)
	if len(merged.ContentCIDs) != 3 {
		t.Errorf("Should have 3 CIDs, got %d", len(merged.ContentCIDs))
	}

	// Check peers merge
	if len(merged.Peers) != 2 {
		t.Errorf("Should have 2 peers, got %d", len(merged.Peers))
	}
}

func TestPartitionRecovery_LockConflict(t *testing.T) {
	localLock := LockSnapshot{
		ID:         "conflicting-lock",
		HolderID:   "node1",
		AcquiredAt: time.Now().Add(-time.Hour),
	}

	remoteLock := LockSnapshot{
		ID:         "conflicting-lock",
		HolderID:   "node2",
		AcquiredAt: time.Now(),
	}

	localSnapshot := &StateSnapshot{
		Locks: []LockSnapshot{localLock},
	}

	remoteSnapshot := &StateSnapshot{
		Locks: []LockSnapshot{remoteLock},
	}

	pr := NewPartitionRecovery(localSnapshot)
	pr.AddRemoteSnapshot("node2", remoteSnapshot)

	// Custom resolver: always prefer node1
	pr.SetLockConflictResolver(func(local, remote LockSnapshot) LockSnapshot {
		if local.HolderID == "node1" {
			return local
		}
		return remote
	})

	merged := pr.Merge()

	if len(merged.Locks) != 1 {
		t.Fatalf("Expected 1 lock, got %d", len(merged.Locks))
	}

	if merged.Locks[0].HolderID != "node1" {
		t.Errorf("Custom resolver should prefer node1, got %s", merged.Locks[0].HolderID)
	}
}
