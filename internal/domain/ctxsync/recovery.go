package ctxsync

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"
)

// RecoveryManager handles context synchronization recovery after network partitions.
type RecoveryManager struct {
	mu sync.RWMutex

	nodeID   string
	nodeName string
	deltaLog *DeltaLog
	peers    map[string]*PeerState

	// Recovery state
	inRecovery      bool
	recoveryStart   time.Time
	partitionStart  time.Time
	pendingReplays  map[string][]*Delta // peerID -> deltas to replay

	// Callbacks
	onConflict  func(*RecoveryConflict) error
	broadcastFn func(delta *Delta) error
}

// RecoveryConflict represents a conflict discovered during recovery.
type RecoveryConflict struct {
	LocalDelta  *Delta `json:"local_delta"`
	RemoteDelta *Delta `json:"remote_delta"`
	FilePath    string `json:"file_path"`
	Resolution  string `json:"resolution"`
	Reason      string `json:"reason"`
}

// RecoveryResult holds the result of context sync recovery.
type RecoveryResult struct {
	DeltasReplayed     int              `json:"deltas_replayed"`
	DeltasMerged       int              `json:"deltas_merged"`
	ConflictsFound     int              `json:"conflicts_found"`
	ConflictsResolved  int              `json:"conflicts_resolved"`
	ConflictsEscalated int              `json:"conflicts_escalated"`
	PeersReconciled    int              `json:"peers_reconciled"`
	Duration           time.Duration    `json:"duration"`
	Conflicts          []*RecoveryConflict `json:"conflicts,omitempty"`
}

// NewRecoveryManager creates a new context sync recovery manager.
func NewRecoveryManager(nodeID, nodeName string, deltaLog *DeltaLog, peers map[string]*PeerState) *RecoveryManager {
	return &RecoveryManager{
		nodeID:         nodeID,
		nodeName:       nodeName,
		deltaLog:       deltaLog,
		peers:          peers,
		pendingReplays: make(map[string][]*Delta),
	}
}

// SetConflictHandler sets the handler for recovery conflicts.
func (rm *RecoveryManager) SetConflictHandler(handler func(*RecoveryConflict) error) {
	rm.mu.Lock()
	defer rm.mu.Unlock()
	rm.onConflict = handler
}

// SetBroadcastFn sets the broadcast function.
func (rm *RecoveryManager) SetBroadcastFn(fn func(delta *Delta) error) {
	rm.mu.Lock()
	defer rm.mu.Unlock()
	rm.broadcastFn = fn
}

// StartRecovery initiates recovery after detecting partition heal.
func (rm *RecoveryManager) StartRecovery(ctx context.Context, partitionStart time.Time) error {
	rm.mu.Lock()
	if rm.inRecovery {
		rm.mu.Unlock()
		return fmt.Errorf("recovery already in progress")
	}
	rm.inRecovery = true
	rm.recoveryStart = time.Now()
	rm.partitionStart = partitionStart
	rm.pendingReplays = make(map[string][]*Delta)
	rm.mu.Unlock()

	// Request delta state from all peers
	if rm.broadcastFn != nil {
		syncReq := &SyncRequest{
			RequestorID:    rm.nodeID,
			RequestorName:  rm.nodeName,
			TargetID:       "", // Broadcast to all
			LastKnownClock: NewVectorClock(),
			Timestamp:      time.Now(),
		}
		// Convert to delta for broadcasting
		delta := &Delta{
			ID:          "sync-req-" + rm.nodeID,
			Type:        DeltaCustom,
			SourceID:    rm.nodeID,
			SourceName:  rm.nodeName,
			VectorClock: NewVectorClock(),
			Timestamp:   time.Now(),
			Payload: &DeltaPayload{
				CustomType: "sync_request",
				CustomData: map[string]any{
					"since":     partitionStart.Format(time.RFC3339),
					"requestor": syncReq.RequestorID,
				},
			},
		}
		rm.broadcastFn(delta)
	}

	return nil
}

// ReceiveDeltaReplay receives deltas from a peer for replay.
func (rm *RecoveryManager) ReceiveDeltaReplay(peerID string, deltas []*Delta) {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	if !rm.inRecovery {
		return
	}

	rm.pendingReplays[peerID] = append(rm.pendingReplays[peerID], deltas...)
}

// FinishRecovery completes recovery and reconciles deltas.
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

	// Collect all deltas to replay
	allDeltas := make([]*Delta, 0)
	for peerID, deltas := range rm.pendingReplays {
		allDeltas = append(allDeltas, deltas...)
		result.PeersReconciled++
		_ = peerID // Track peer
	}

	// Sort by vector clock for proper ordering
	sort.Slice(allDeltas, func(i, j int) bool {
		// Compare by timestamp first, then by source ID for determinism
		if allDeltas[i].Timestamp.Equal(allDeltas[j].Timestamp) {
			return allDeltas[i].SourceID < allDeltas[j].SourceID
		}
		return allDeltas[i].Timestamp.Before(allDeltas[j].Timestamp)
	})

	// Replay deltas
	fileChanges := make(map[string][]*Delta) // FilePath -> deltas affecting it

	for _, delta := range allDeltas {
		// Check if we already have this delta
		if _, exists := rm.deltaLog.Get(delta.ID); exists {
			result.DeltasMerged++
			continue
		}

		// Group file changes for conflict detection
		if delta.Type == DeltaFileChange && delta.Payload.FilePath != "" {
			fileChanges[delta.Payload.FilePath] = append(fileChanges[delta.Payload.FilePath], delta)
		}

		// Apply delta
		rm.deltaLog.Append(delta)
		result.DeltasReplayed++
	}

	// Detect and resolve conflicts
	for filePath, deltas := range fileChanges {
		if len(deltas) <= 1 {
			continue
		}

		// Check for concurrent modifications
		for i := 0; i < len(deltas); i++ {
			for j := i + 1; j < len(deltas); j++ {
				if deltas[i].VectorClock.IsConcurrent(deltas[j].VectorClock) {
					conflict := rm.resolveConflict(filePath, deltas[i], deltas[j])
					result.Conflicts = append(result.Conflicts, conflict)
					result.ConflictsFound++

					if conflict.Resolution == "escalated" {
						result.ConflictsEscalated++
					} else {
						result.ConflictsResolved++
					}
				}
			}
		}
	}

	result.Duration = time.Since(startTime)

	// Notify about conflicts
	if rm.onConflict != nil {
		for _, conflict := range result.Conflicts {
			rm.onConflict(conflict)
		}
	}

	return result, nil
}

// resolveConflict attempts to resolve a conflict between two concurrent deltas.
func (rm *RecoveryManager) resolveConflict(filePath string, delta1, delta2 *Delta) *RecoveryConflict {
	conflict := &RecoveryConflict{
		LocalDelta:  delta1,
		RemoteDelta: delta2,
		FilePath:    filePath,
	}

	// Determine which is local vs remote
	isLocal1 := delta1.SourceID == rm.nodeID
	isLocal2 := delta2.SourceID == rm.nodeID

	if isLocal1 && !isLocal2 {
		conflict.LocalDelta = delta1
		conflict.RemoteDelta = delta2
	} else if isLocal2 && !isLocal1 {
		conflict.LocalDelta = delta2
		conflict.RemoteDelta = delta1
	}

	// Resolution strategy: Last-Writer-Wins with vector clock tiebreaker
	if delta1.Timestamp.After(delta2.Timestamp) {
		conflict.Resolution = "keep_first"
		conflict.Reason = "Delta 1 has later timestamp"
	} else if delta2.Timestamp.After(delta1.Timestamp) {
		conflict.Resolution = "keep_second"
		conflict.Reason = "Delta 2 has later timestamp"
	} else {
		// Same timestamp - use source ID as tiebreaker
		if delta1.SourceID < delta2.SourceID {
			conflict.Resolution = "keep_first"
			conflict.Reason = "Same timestamp, source ID tiebreaker"
		} else if delta2.SourceID < delta1.SourceID {
			conflict.Resolution = "keep_second"
			conflict.Reason = "Same timestamp, source ID tiebreaker"
		} else {
			// Cannot resolve automatically
			conflict.Resolution = "escalated"
			conflict.Reason = "Cannot resolve automatically, human intervention required"
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

// GetMissedDeltas returns deltas that occurred after a given vector clock.
func (rm *RecoveryManager) GetMissedDeltas(since *VectorClock) []*Delta {
	return rm.deltaLog.GetSince(since)
}

// PrepareReplayResponse prepares deltas for replay to a peer.
func (rm *RecoveryManager) PrepareReplayResponse(since time.Time) []*Delta {
	deltas := rm.deltaLog.GetRecent(1000) // Get all recent deltas

	// Filter to deltas after the partition start
	result := make([]*Delta, 0)
	for _, delta := range deltas {
		if delta.Timestamp.After(since) {
			result = append(result, delta)
		}
	}

	// Sort by timestamp
	sort.Slice(result, func(i, j int) bool {
		return result[i].Timestamp.Before(result[j].Timestamp)
	})

	return result
}

// AntiEntropySync performs anti-entropy synchronization with a peer.
func (rm *RecoveryManager) AntiEntropySync(ctx context.Context, peerID string, peerClock *VectorClock) ([]*Delta, error) {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	// Find deltas that peer hasn't seen
	deltasToSend := rm.deltaLog.GetSince(peerClock)

	// Filter out deltas from the peer itself
	result := make([]*Delta, 0)
	for _, delta := range deltasToSend {
		if delta.SourceID != peerID {
			result = append(result, delta)
		}
	}

	return result, nil
}
