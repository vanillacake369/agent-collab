package lock

import (
	"context"
	"fmt"
	"time"
)

// LockService is the lock service.
type LockService struct {
	store      *LockStore
	negotiator *LockNegotiator
	nodeID     string
	nodeName   string
}

// NewLockService creates a new lock service.
func NewLockService(ctx context.Context, nodeID, nodeName string) *LockService {
	store := NewLockStore(ctx)
	negotiator := NewLockNegotiator(ctx, store)

	return &LockService{
		store:      store,
		negotiator: negotiator,
		nodeID:     nodeID,
		nodeName:   nodeName,
	}
}

// Close stops background goroutines and releases resources.
func (s *LockService) Close() error {
	if err := s.negotiator.Close(); err != nil {
		return err
	}
	return s.store.Close()
}

// SetBroadcastFn sets the broadcast function.
func (s *LockService) SetBroadcastFn(fn func(msg any) error) {
	s.negotiator.SetBroadcastFn(fn)
}

// SetConflictHandler sets the conflict handler.
func (s *LockService) SetConflictHandler(handler func(*LockConflict) error) {
	s.negotiator.SetConflictHandler(handler)
}

// SetEscalateHandler sets the escalation handler.
func (s *LockService) SetEscalateHandler(handler func(*NegotiationSession) error) {
	s.negotiator.SetEscalateHandler(handler)
}

// AcquireLock acquires a lock.
func (s *LockService) AcquireLock(ctx context.Context, req *AcquireLockRequest) (*LockResult, error) {
	target, err := NewSemanticTarget(
		req.TargetType,
		req.FilePath,
		req.Name,
		req.StartLine,
		req.EndLine,
	)
	if err != nil {
		return &LockResult{
			Success: false,
			Reason:  err.Error(),
		}, err
	}

	lock := NewSemanticLock(target, s.nodeID, s.nodeName, req.Intention)

	// Phase 1: Announce intent
	intent, err := s.negotiator.AnnounceIntent(ctx, lock)
	if err != nil {
		return &LockResult{
			Success: false,
			Reason:  err.Error(),
		}, err
	}

	// Phase 2: Acquire lock
	result, err := s.negotiator.AcquireLock(ctx, intent.ID)
	if err != nil {
		return result, err
	}

	return result, nil
}

// ReleaseLock releases a lock.
func (s *LockService) ReleaseLock(ctx context.Context, lockID string) error {
	return s.negotiator.ReleaseLock(ctx, lockID, s.nodeID)
}

// RenewLock renews a lock.
func (s *LockService) RenewLock(ctx context.Context, lockID string) error {
	lock, err := s.store.Get(lockID)
	if err != nil {
		return err
	}

	if lock.HolderID != s.nodeID {
		return ErrNotLockHolder
	}

	return lock.Renew()
}

// RenewLockWithTTL renews a lock with specified TTL.
func (s *LockService) RenewLockWithTTL(ctx context.Context, lockID string, ttl time.Duration) error {
	lock, err := s.store.Get(lockID)
	if err != nil {
		return err
	}

	if lock.HolderID != s.nodeID {
		return ErrNotLockHolder
	}

	return lock.RenewWithTTL(ttl)
}

// GetLock retrieves a lock.
func (s *LockService) GetLock(lockID string) (*SemanticLock, error) {
	return s.store.Get(lockID)
}

// GetLockByTarget retrieves a lock by target.
func (s *LockService) GetLockByTarget(target *SemanticTarget) (*SemanticLock, error) {
	return s.store.GetByTarget(target)
}

// FindConflicts finds conflicting locks.
func (s *LockService) FindConflicts(target *SemanticTarget) []*SemanticLock {
	return s.store.FindConflicts(target)
}

// ListLocks returns all active locks.
func (s *LockService) ListLocks() []*SemanticLock {
	return s.store.List()
}

// ListMyLocks returns locks owned by this node.
func (s *LockService) ListMyLocks() []*SemanticLock {
	return s.store.ListByHolder(s.nodeID)
}

// ListLocksByHolder returns locks owned by a specific holder.
func (s *LockService) ListLocksByHolder(holderID string) []*SemanticLock {
	return s.store.ListByHolder(holderID)
}

// Count returns the number of active locks.
func (s *LockService) Count() int {
	return s.store.Count()
}

// Negotiate negotiates a conflict.
func (s *LockService) Negotiate(ctx context.Context, sessionID string, proposal *NegotiationProposal) (*NegotiationResult, error) {
	return s.negotiator.Negotiate(ctx, sessionID, proposal)
}

// Vote votes on lock acquisition.
func (s *LockService) Vote(ctx context.Context, sessionID string, approve bool, reason string) error {
	vote := &Vote{
		VoterID:   s.nodeID,
		VoterName: s.nodeName,
		Approve:   approve,
		Reason:    reason,
		Timestamp: time.Now(),
	}

	return s.negotiator.Vote(ctx, sessionID, vote)
}

// GetNegotiationSession retrieves a negotiation session.
func (s *LockService) GetNegotiationSession(sessionID string) (*NegotiationSession, error) {
	return s.negotiator.GetSession(sessionID)
}

// ListActiveNegotiations lists active negotiation sessions.
func (s *LockService) ListActiveNegotiations() []*NegotiationSession {
	return s.negotiator.ListActiveSessions()
}

// HandleRemoteLockIntent handles a remote lock intent.
func (s *LockService) HandleRemoteLockIntent(intent *LockIntent) error {
	conflicts := s.store.FindConflicts(intent.Lock.Target)
	if len(conflicts) > 0 {
		// Notify if conflict with my lock
		for _, conflict := range conflicts {
			if conflict.HolderID == s.nodeID {
				return fmt.Errorf("conflict with my lock: %s", conflict.ID)
			}
		}
	}
	return nil
}

// HandleRemoteLockAcquired handles a remote lock acquisition.
func (s *LockService) HandleRemoteLockAcquired(lock *SemanticLock) error {
	// Store remote lock info (read-only)
	return s.store.Add(lock)
}

// HandleRemoteLockReleased handles a remote lock release.
func (s *LockService) HandleRemoteLockReleased(lockID string) error {
	lock, err := s.store.Get(lockID)
	if err != nil {
		return nil // Already removed
	}

	// Only remove if not my lock
	if lock.HolderID != s.nodeID {
		return s.store.Remove(lockID)
	}

	return nil
}

// GetStats returns lock statistics.
func (s *LockService) GetStats() *LockStats {
	locks := s.store.List()
	myLocks := s.store.ListByHolder(s.nodeID)
	sessions := s.negotiator.ListActiveSessions()

	var totalTTL time.Duration
	for _, lock := range locks {
		totalTTL += lock.TTLRemaining()
	}

	avgTTL := time.Duration(0)
	if len(locks) > 0 {
		avgTTL = totalTTL / time.Duration(len(locks))
	}

	return &LockStats{
		TotalLocks:         len(locks),
		MyLocks:            len(myLocks),
		ActiveNegotiations: len(sessions),
		AverageTTL:         avgTTL,
	}
}

// AcquireLockRequest is a lock acquisition request.
type AcquireLockRequest struct {
	TargetType TargetType `json:"target_type"`
	FilePath   string     `json:"file_path"`
	Name       string     `json:"name"`
	StartLine  int        `json:"start_line"`
	EndLine    int        `json:"end_line"`
	Intention  string     `json:"intention"`
}

// LockStats is lock statistics.
type LockStats struct {
	TotalLocks         int           `json:"total_locks"`
	MyLocks            int           `json:"my_locks"`
	ActiveNegotiations int           `json:"active_negotiations"`
	AverageTTL         time.Duration `json:"average_ttl"`
}

// HistoryEntry is a lock history entry.
type HistoryEntry struct {
	Timestamp  time.Time `json:"timestamp"`
	Action     string    `json:"action"` // acquired, released, conflict, expired
	LockID     string    `json:"lock_id"`
	HolderID   string    `json:"holder_id"`
	HolderName string    `json:"holder_name"`
	Target     string    `json:"target"`
}

// GetHistory returns recent lock history.
func (s *LockService) GetHistory(limit int) []*HistoryEntry {
	return s.store.GetHistory(limit)
}
