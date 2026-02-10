package lock

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"
)

// RecoveryManager handles lock recovery after network partitions.
type RecoveryManager struct {
	mu sync.RWMutex

	store  *LockStore
	nodeID string

	// Recovery state
	inRecovery     bool
	recoveryStart  time.Time
	partitionStart time.Time

	// Pending reconciliation
	remoteLocks map[string]*SemanticLock // Locks from other nodes during partition

	// Callbacks
	onConflict  func(*RecoveryConflict) error
	broadcastFn func(msg any) error
}

// RecoveryConflict represents a conflict discovered during recovery.
type RecoveryConflict struct {
	LocalLock  *SemanticLock  `json:"local_lock"`
	RemoteLock *SemanticLock  `json:"remote_lock"`
	Resolution ResolutionType `json:"resolution"`
	Reason     string         `json:"reason"`
}

// RecoveryResult holds the result of recovery process.
type RecoveryResult struct {
	LocksReconciled    int                 `json:"locks_reconciled"`
	LocksRemoved       int                 `json:"locks_removed"`
	ConflictsResolved  int                 `json:"conflicts_resolved"`
	ConflictsEscalated int                 `json:"conflicts_escalated"`
	Duration           time.Duration       `json:"duration"`
	Conflicts          []*RecoveryConflict `json:"conflicts,omitempty"`
}

// NewRecoveryManager creates a new recovery manager.
func NewRecoveryManager(store *LockStore, nodeID string) *RecoveryManager {
	return &RecoveryManager{
		store:       store,
		nodeID:      nodeID,
		remoteLocks: make(map[string]*SemanticLock),
	}
}

// SetConflictHandler sets the handler for recovery conflicts.
func (rm *RecoveryManager) SetConflictHandler(handler func(*RecoveryConflict) error) {
	rm.mu.Lock()
	defer rm.mu.Unlock()
	rm.onConflict = handler
}

// SetBroadcastFn sets the broadcast function.
func (rm *RecoveryManager) SetBroadcastFn(fn func(msg any) error) {
	rm.mu.Lock()
	defer rm.mu.Unlock()
	rm.broadcastFn = fn
}

// StartRecovery begins the recovery process after detecting partition heal.
func (rm *RecoveryManager) StartRecovery(ctx context.Context, partitionStart time.Time) error {
	rm.mu.Lock()
	if rm.inRecovery {
		rm.mu.Unlock()
		return fmt.Errorf("recovery already in progress")
	}
	rm.inRecovery = true
	rm.recoveryStart = time.Now()
	rm.partitionStart = partitionStart
	rm.remoteLocks = make(map[string]*SemanticLock)
	rm.mu.Unlock()

	// Request lock state from all peers
	if rm.broadcastFn != nil {
		msg := &LockStateRequest{
			RequestorID: rm.nodeID,
			Since:       partitionStart,
			Timestamp:   time.Now(),
		}
		rm.broadcastFn(msg)
	}

	return nil
}

// ReceiveLockState receives lock state from a peer during recovery.
func (rm *RecoveryManager) ReceiveLockState(peerID string, locks []*SemanticLock) {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	if !rm.inRecovery {
		return
	}

	for _, lock := range locks {
		// Only track locks from other nodes
		if lock.HolderID != rm.nodeID {
			rm.remoteLocks[lock.ID] = lock
		}
	}
}

// FinishRecovery completes the recovery process and reconciles locks.
func (rm *RecoveryManager) FinishRecovery(ctx context.Context) (*RecoveryResult, error) {
	rm.mu.Lock()
	defer func() {
		rm.inRecovery = false
		rm.mu.Unlock()
	}()

	if !rm.inRecovery {
		return nil, fmt.Errorf("not in recovery mode")
	}

	result := &RecoveryResult{
		Conflicts: make([]*RecoveryConflict, 0),
	}

	startTime := time.Now()

	// Get local locks
	localLocks := rm.store.List()

	// Build map for quick lookup
	localLockMap := make(map[string]*SemanticLock)
	for _, lock := range localLocks {
		localLockMap[lock.ID] = lock
	}

	// Check each remote lock against local state
	for _, remoteLock := range rm.remoteLocks {
		// Skip expired locks
		if remoteLock.IsExpired() {
			continue
		}

		// Check if we have the same lock
		if localLock, exists := localLockMap[remoteLock.ID]; exists {
			// Same lock exists - compare fencing tokens
			if localLock.FencingToken != remoteLock.FencingToken {
				// Conflict: same lock ID but different state
				conflict := rm.resolveFencingConflict(localLock, remoteLock)
				result.Conflicts = append(result.Conflicts, conflict)
				if conflict.Resolution == ResolutionHumanNeeded {
					result.ConflictsEscalated++
				} else {
					result.ConflictsResolved++
				}
			}
			result.LocksReconciled++
			continue
		}

		// Remote lock doesn't exist locally - check for overlaps
		localConflicts := rm.store.FindConflicts(remoteLock.Target)
		if len(localConflicts) > 0 {
			for _, localLock := range localConflicts {
				conflict := rm.resolveOverlapConflict(localLock, remoteLock)
				result.Conflicts = append(result.Conflicts, conflict)
				if conflict.Resolution == ResolutionHumanNeeded {
					result.ConflictsEscalated++
				} else {
					result.ConflictsResolved++
				}
			}
		} else {
			// No conflict - add remote lock
			rm.store.Add(remoteLock)
			result.LocksReconciled++
		}
	}

	// Check for local locks that were released remotely
	for id, localLock := range localLockMap {
		if localLock.HolderID == rm.nodeID {
			continue // Skip our own locks
		}
		if _, exists := rm.remoteLocks[id]; !exists {
			// Remote lock was released during partition
			if localLock.AcquiredAt.Before(rm.partitionStart) {
				rm.store.Remove(id)
				result.LocksRemoved++
			}
		}
	}

	result.Duration = time.Since(startTime)

	// Notify about escalated conflicts
	if rm.onConflict != nil {
		for _, conflict := range result.Conflicts {
			rm.onConflict(conflict)
		}
	}

	return result, nil
}

// resolveFencingConflict resolves a conflict where the same lock has different fencing tokens.
func (rm *RecoveryManager) resolveFencingConflict(local, remote *SemanticLock) *RecoveryConflict {
	conflict := &RecoveryConflict{
		LocalLock:  local,
		RemoteLock: remote,
	}

	// Higher fencing token wins (more recent)
	if remote.FencingToken > local.FencingToken {
		// Remote is newer
		rm.store.Remove(local.ID)
		rm.store.Add(remote)
		conflict.Resolution = ResolutionApproved
		conflict.Reason = "Remote lock has higher fencing token"
	} else if local.FencingToken > remote.FencingToken {
		// Local is newer - keep local
		conflict.Resolution = ResolutionApproved
		conflict.Reason = "Local lock has higher fencing token"
	} else {
		// Same fencing token but different state - escalate
		conflict.Resolution = ResolutionHumanNeeded
		conflict.Reason = "Identical fencing tokens with different state"
	}

	return conflict
}

// resolveOverlapConflict resolves an overlap conflict between local and remote locks.
func (rm *RecoveryManager) resolveOverlapConflict(local, remote *SemanticLock) *RecoveryConflict {
	conflict := &RecoveryConflict{
		LocalLock:  local,
		RemoteLock: remote,
	}

	// Strategy: Older lock wins (was acquired first)
	if remote.AcquiredAt.Before(local.AcquiredAt) {
		// Remote was first
		if local.HolderID == rm.nodeID {
			// We need to release our lock
			rm.store.Remove(local.ID)
			rm.store.Add(remote)
			conflict.Resolution = ResolutionApproved
			conflict.Reason = "Remote lock was acquired first"
		} else {
			// Both are remote - escalate
			conflict.Resolution = ResolutionHumanNeeded
			conflict.Reason = "Overlapping remote locks acquired during partition"
		}
	} else if local.AcquiredAt.Before(remote.AcquiredAt) {
		// Local was first - keep local
		conflict.Resolution = ResolutionApproved
		conflict.Reason = "Local lock was acquired first"
	} else {
		// Same acquisition time - use fencing token as tiebreaker
		if remote.FencingToken > local.FencingToken {
			rm.store.Remove(local.ID)
			rm.store.Add(remote)
			conflict.Resolution = ResolutionApproved
			conflict.Reason = "Same acquisition time, remote has higher fencing token"
		} else if local.FencingToken >= remote.FencingToken {
			conflict.Resolution = ResolutionApproved
			conflict.Reason = "Same acquisition time, local has higher fencing token"
		} else {
			conflict.Resolution = ResolutionHumanNeeded
			conflict.Reason = "Cannot resolve overlap automatically"
		}
	}

	return conflict
}

// IsRecovering returns whether recovery is in progress.
func (rm *RecoveryManager) IsRecovering() bool {
	rm.mu.RLock()
	defer rm.mu.RUnlock()
	return rm.inRecovery
}

// LockStateRequest is sent to request lock state from peers.
type LockStateRequest struct {
	RequestorID string    `json:"requestor_id"`
	Since       time.Time `json:"since"`
	Timestamp   time.Time `json:"timestamp"`
}

// LockStateResponse is the response containing lock state.
type LockStateResponse struct {
	ResponderID string          `json:"responder_id"`
	Locks       []*SemanticLock `json:"locks"`
	Timestamp   time.Time       `json:"timestamp"`
}

// PrepareStateResponse prepares a response to a lock state request.
func (rm *RecoveryManager) PrepareStateResponse(req *LockStateRequest) *LockStateResponse {
	locks := rm.store.List()

	// Filter to locks acquired after the partition start
	filtered := make([]*SemanticLock, 0)
	for _, lock := range locks {
		if lock.AcquiredAt.After(req.Since) || !lock.IsExpired() {
			filtered = append(filtered, lock)
		}
	}

	// Sort by acquisition time for deterministic ordering
	sort.Slice(filtered, func(i, j int) bool {
		return filtered[i].AcquiredAt.Before(filtered[j].AcquiredAt)
	})

	return &LockStateResponse{
		ResponderID: rm.nodeID,
		Locks:       filtered,
		Timestamp:   time.Now(),
	}
}
