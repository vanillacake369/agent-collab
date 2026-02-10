package libp2p

import (
	"encoding/json"
	"sync"
	"time"
)

// StateSnapshot represents a point-in-time snapshot of the system state
type StateSnapshot struct {
	// Metadata
	Version     int       `json:"version"`
	NodeID      string    `json:"node_id"`
	Timestamp   time.Time `json:"timestamp"`
	SequenceNum uint64    `json:"sequence_num"`

	// Vector clock for causal ordering
	VectorClock map[string]uint64 `json:"vector_clock"`

	// Active locks
	Locks []LockSnapshot `json:"locks"`

	// Content references (CIDs)
	ContentCIDs []string `json:"content_cids"`

	// Peer information
	Peers []PeerSnapshot `json:"peers"`
}

// LockSnapshot is a serializable lock state
type LockSnapshot struct {
	ID         string    `json:"id"`
	HolderID   string    `json:"holder_id"`
	HolderName string    `json:"holder_name"`
	TargetType string    `json:"target_type"`
	FilePath   string    `json:"file_path"`
	StartLine  int       `json:"start_line"`
	EndLine    int       `json:"end_line"`
	Intention  string    `json:"intention"`
	AcquiredAt time.Time `json:"acquired_at"`
	ExpiresAt  time.Time `json:"expires_at"`
}

// PeerSnapshot is a serializable peer state
type PeerSnapshot struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	LastSeen    time.Time `json:"last_seen"`
	VectorClock uint64    `json:"vector_clock"`
}

// SnapshotManager manages state snapshots for sync and recovery
type SnapshotManager struct {
	mu sync.RWMutex

	nodeID      string
	sequenceNum uint64
	vectorClock map[string]uint64

	// Snapshot storage
	snapshots    map[uint64]*StateSnapshot
	maxSnapshots int
	latestSeqNum uint64

	// Callbacks for getting current state
	getLocks func() []LockSnapshot
	getCIDs  func() []string
	getPeers func() []PeerSnapshot
}

// SnapshotManagerConfig configures the snapshot manager
type SnapshotManagerConfig struct {
	NodeID       string
	MaxSnapshots int
}

// NewSnapshotManager creates a new snapshot manager
func NewSnapshotManager(config SnapshotManagerConfig) *SnapshotManager {
	if config.MaxSnapshots == 0 {
		config.MaxSnapshots = 10
	}

	return &SnapshotManager{
		nodeID:       config.NodeID,
		vectorClock:  make(map[string]uint64),
		snapshots:    make(map[uint64]*StateSnapshot),
		maxSnapshots: config.MaxSnapshots,
	}
}

// SetLockProvider sets the callback to get current locks
func (sm *SnapshotManager) SetLockProvider(fn func() []LockSnapshot) {
	sm.mu.Lock()
	sm.getLocks = fn
	sm.mu.Unlock()
}

// SetCIDProvider sets the callback to get current content CIDs
func (sm *SnapshotManager) SetCIDProvider(fn func() []string) {
	sm.mu.Lock()
	sm.getCIDs = fn
	sm.mu.Unlock()
}

// SetPeerProvider sets the callback to get current peers
func (sm *SnapshotManager) SetPeerProvider(fn func() []PeerSnapshot) {
	sm.mu.Lock()
	sm.getPeers = fn
	sm.mu.Unlock()
}

// IncrementClock increments the vector clock for this node
func (sm *SnapshotManager) IncrementClock() uint64 {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	sm.vectorClock[sm.nodeID]++
	return sm.vectorClock[sm.nodeID]
}

// UpdateClock updates the vector clock from a received message
func (sm *SnapshotManager) UpdateClock(peerID string, peerClock uint64) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if peerClock > sm.vectorClock[peerID] {
		sm.vectorClock[peerID] = peerClock
	}
}

// GetVectorClock returns a copy of the current vector clock
func (sm *SnapshotManager) GetVectorClock() map[string]uint64 {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	result := make(map[string]uint64, len(sm.vectorClock))
	for k, v := range sm.vectorClock {
		result[k] = v
	}
	return result
}

// CreateSnapshot creates a new state snapshot
func (sm *SnapshotManager) CreateSnapshot() *StateSnapshot {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	sm.sequenceNum++

	snapshot := &StateSnapshot{
		Version:     1,
		NodeID:      sm.nodeID,
		Timestamp:   time.Now(),
		SequenceNum: sm.sequenceNum,
		VectorClock: make(map[string]uint64),
	}

	// Copy vector clock
	for k, v := range sm.vectorClock {
		snapshot.VectorClock[k] = v
	}

	// Get current state from providers
	if sm.getLocks != nil {
		snapshot.Locks = sm.getLocks()
	}
	if sm.getCIDs != nil {
		snapshot.ContentCIDs = sm.getCIDs()
	}
	if sm.getPeers != nil {
		snapshot.Peers = sm.getPeers()
	}

	// Store snapshot
	sm.snapshots[sm.sequenceNum] = snapshot
	sm.latestSeqNum = sm.sequenceNum

	// Cleanup old snapshots
	sm.cleanupOldSnapshots()

	return snapshot
}

// GetSnapshot returns a snapshot by sequence number
func (sm *SnapshotManager) GetSnapshot(seqNum uint64) *StateSnapshot {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return sm.snapshots[seqNum]
}

// GetLatestSnapshot returns the most recent snapshot
func (sm *SnapshotManager) GetLatestSnapshot() *StateSnapshot {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return sm.snapshots[sm.latestSeqNum]
}

// GetSnapshotsSince returns all snapshots since a given sequence number
func (sm *SnapshotManager) GetSnapshotsSince(seqNum uint64) []*StateSnapshot {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	var result []*StateSnapshot
	for seq, snap := range sm.snapshots {
		if seq > seqNum {
			result = append(result, snap)
		}
	}

	// Sort by sequence number
	for i := 0; i < len(result)-1; i++ {
		for j := i + 1; j < len(result); j++ {
			if result[i].SequenceNum > result[j].SequenceNum {
				result[i], result[j] = result[j], result[i]
			}
		}
	}

	return result
}

// cleanupOldSnapshots removes old snapshots beyond the limit
func (sm *SnapshotManager) cleanupOldSnapshots() {
	if len(sm.snapshots) <= sm.maxSnapshots {
		return
	}

	// Find and remove oldest snapshots
	var seqNums []uint64
	for seq := range sm.snapshots {
		seqNums = append(seqNums, seq)
	}

	// Sort ascending
	for i := 0; i < len(seqNums)-1; i++ {
		for j := i + 1; j < len(seqNums); j++ {
			if seqNums[i] > seqNums[j] {
				seqNums[i], seqNums[j] = seqNums[j], seqNums[i]
			}
		}
	}

	// Remove oldest
	toRemove := len(sm.snapshots) - sm.maxSnapshots
	for i := 0; i < toRemove && i < len(seqNums); i++ {
		delete(sm.snapshots, seqNums[i])
	}
}

// ComputeDelta computes the difference between two snapshots
func ComputeDelta(old, new *StateSnapshot) *SnapshotDelta {
	delta := &SnapshotDelta{
		FromSeqNum: old.SequenceNum,
		ToSeqNum:   new.SequenceNum,
		Timestamp:  new.Timestamp,
	}

	// Compute lock changes
	oldLocks := make(map[string]LockSnapshot)
	for _, l := range old.Locks {
		oldLocks[l.ID] = l
	}

	for _, l := range new.Locks {
		if _, exists := oldLocks[l.ID]; !exists {
			delta.LocksAdded = append(delta.LocksAdded, l)
		}
		delete(oldLocks, l.ID)
	}

	for id := range oldLocks {
		delta.LocksRemoved = append(delta.LocksRemoved, id)
	}

	// Compute CID changes
	oldCIDs := make(map[string]bool)
	for _, cid := range old.ContentCIDs {
		oldCIDs[cid] = true
	}

	for _, cid := range new.ContentCIDs {
		if !oldCIDs[cid] {
			delta.CIDsAdded = append(delta.CIDsAdded, cid)
		}
		delete(oldCIDs, cid)
	}

	for cid := range oldCIDs {
		delta.CIDsRemoved = append(delta.CIDsRemoved, cid)
	}

	// Compute vector clock updates
	delta.VectorClockUpdates = make(map[string]uint64)
	for nodeID, clock := range new.VectorClock {
		if oldClock, exists := old.VectorClock[nodeID]; !exists || clock > oldClock {
			delta.VectorClockUpdates[nodeID] = clock
		}
	}

	return delta
}

// SnapshotDelta represents the difference between two snapshots
type SnapshotDelta struct {
	FromSeqNum         uint64            `json:"from_seq_num"`
	ToSeqNum           uint64            `json:"to_seq_num"`
	Timestamp          time.Time         `json:"timestamp"`
	LocksAdded         []LockSnapshot    `json:"locks_added,omitempty"`
	LocksRemoved       []string          `json:"locks_removed,omitempty"`
	CIDsAdded          []string          `json:"cids_added,omitempty"`
	CIDsRemoved        []string          `json:"cids_removed,omitempty"`
	VectorClockUpdates map[string]uint64 `json:"vector_clock_updates,omitempty"`
}

// IsEmpty returns true if the delta has no changes
func (d *SnapshotDelta) IsEmpty() bool {
	return len(d.LocksAdded) == 0 &&
		len(d.LocksRemoved) == 0 &&
		len(d.CIDsAdded) == 0 &&
		len(d.CIDsRemoved) == 0 &&
		len(d.VectorClockUpdates) == 0
}

// Serialize serializes a snapshot to JSON
func (s *StateSnapshot) Serialize() ([]byte, error) {
	return json.Marshal(s)
}

// DeserializeSnapshot deserializes a snapshot from JSON
func DeserializeSnapshot(data []byte) (*StateSnapshot, error) {
	var snapshot StateSnapshot
	if err := json.Unmarshal(data, &snapshot); err != nil {
		return nil, err
	}
	return &snapshot, nil
}

// PartitionRecovery handles recovery after a network partition
type PartitionRecovery struct {
	mu sync.Mutex

	// Local state
	localSnapshot *StateSnapshot

	// Remote state received
	remoteSnapshots map[string]*StateSnapshot

	// Resolution callbacks
	onLockConflict  func(local, remote LockSnapshot) LockSnapshot
	onMergeComplete func(*StateSnapshot)
}

// NewPartitionRecovery creates a new partition recovery handler
func NewPartitionRecovery(localSnapshot *StateSnapshot) *PartitionRecovery {
	return &PartitionRecovery{
		localSnapshot:   localSnapshot,
		remoteSnapshots: make(map[string]*StateSnapshot),
	}
}

// SetLockConflictResolver sets the callback for resolving lock conflicts
func (pr *PartitionRecovery) SetLockConflictResolver(fn func(local, remote LockSnapshot) LockSnapshot) {
	pr.mu.Lock()
	pr.onLockConflict = fn
	pr.mu.Unlock()
}

// SetMergeCompleteCallback sets the callback when merge is complete
func (pr *PartitionRecovery) SetMergeCompleteCallback(fn func(*StateSnapshot)) {
	pr.mu.Lock()
	pr.onMergeComplete = fn
	pr.mu.Unlock()
}

// AddRemoteSnapshot adds a snapshot received from a remote peer
func (pr *PartitionRecovery) AddRemoteSnapshot(nodeID string, snapshot *StateSnapshot) {
	pr.mu.Lock()
	pr.remoteSnapshots[nodeID] = snapshot
	pr.mu.Unlock()
}

// Merge merges all snapshots and returns the reconciled state
func (pr *PartitionRecovery) Merge() *StateSnapshot {
	pr.mu.Lock()
	defer pr.mu.Unlock()

	merged := &StateSnapshot{
		Version:     1,
		NodeID:      pr.localSnapshot.NodeID,
		Timestamp:   time.Now(),
		VectorClock: make(map[string]uint64),
		Locks:       make([]LockSnapshot, 0),
		ContentCIDs: make([]string, 0),
		Peers:       make([]PeerSnapshot, 0),
	}

	// Merge vector clocks (take max of each)
	for k, v := range pr.localSnapshot.VectorClock {
		merged.VectorClock[k] = v
	}
	for _, remote := range pr.remoteSnapshots {
		for k, v := range remote.VectorClock {
			if v > merged.VectorClock[k] {
				merged.VectorClock[k] = v
			}
		}
	}

	// Merge locks (resolve conflicts)
	lockMap := make(map[string]LockSnapshot)
	for _, l := range pr.localSnapshot.Locks {
		lockMap[l.ID] = l
	}
	for _, remote := range pr.remoteSnapshots {
		for _, l := range remote.Locks {
			if existing, exists := lockMap[l.ID]; exists {
				// Conflict - use resolver or default to most recent
				if pr.onLockConflict != nil {
					lockMap[l.ID] = pr.onLockConflict(existing, l)
				} else if l.AcquiredAt.After(existing.AcquiredAt) {
					lockMap[l.ID] = l
				}
			} else {
				lockMap[l.ID] = l
			}
		}
	}
	for _, l := range lockMap {
		merged.Locks = append(merged.Locks, l)
	}

	// Merge CIDs (union)
	cidSet := make(map[string]bool)
	for _, cid := range pr.localSnapshot.ContentCIDs {
		cidSet[cid] = true
	}
	for _, remote := range pr.remoteSnapshots {
		for _, cid := range remote.ContentCIDs {
			cidSet[cid] = true
		}
	}
	for cid := range cidSet {
		merged.ContentCIDs = append(merged.ContentCIDs, cid)
	}

	// Merge peers (most recent info)
	peerMap := make(map[string]PeerSnapshot)
	for _, p := range pr.localSnapshot.Peers {
		peerMap[p.ID] = p
	}
	for _, remote := range pr.remoteSnapshots {
		for _, p := range remote.Peers {
			if existing, exists := peerMap[p.ID]; !exists || p.LastSeen.After(existing.LastSeen) {
				peerMap[p.ID] = p
			}
		}
	}
	for _, p := range peerMap {
		merged.Peers = append(merged.Peers, p)
	}

	// Call completion callback
	if pr.onMergeComplete != nil {
		pr.onMergeComplete(merged)
	}

	return merged
}
