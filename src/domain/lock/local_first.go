package lock

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// LockMode defines the lock acquisition mode
type LockMode string

const (
	// ModePessimistic waits for distributed consensus before acquiring
	ModePessimistic LockMode = "pessimistic"
	// ModeOptimistic acquires locally first, broadcasts asynchronously
	ModeOptimistic LockMode = "optimistic"
)

// LocalFirstConfig configures the local-first lock behavior
type LocalFirstConfig struct {
	// Mode determines lock acquisition strategy
	Mode LockMode
	// ConflictTimeout is how long to wait for remote conflict detection
	ConflictTimeout time.Duration
	// AutoRollback automatically rolls back on conflict
	AutoRollback bool
	// ConflictCallback is called when a conflict is detected
	ConflictCallback func(lock *SemanticLock, conflictWith *SemanticLock)
}

// DefaultLocalFirstConfig returns the default local-first configuration
func DefaultLocalFirstConfig() LocalFirstConfig {
	return LocalFirstConfig{
		Mode:            ModeOptimistic,
		ConflictTimeout: 5 * time.Second,
		AutoRollback:    true,
	}
}

// LocalFirstLockService wraps LockService with local-first semantics
type LocalFirstLockService struct {
	*LockService
	config LocalFirstConfig

	mu              sync.RWMutex
	pendingLocks    map[string]*PendingLock
	conflictChan    chan *LockConflict
	broadcastFn     func(msg any) error
	rollbackHandler func(lockID string, reason string)
}

// PendingLock represents a lock that was acquired locally but not yet confirmed
type PendingLock struct {
	Lock          *SemanticLock
	AcquiredAt    time.Time
	ConfirmedAt   *time.Time
	ConflictsWith []*SemanticLock
	Status        PendingStatus
}

// PendingStatus is the status of a pending lock
type PendingStatus string

const (
	StatusPending    PendingStatus = "pending"
	StatusConfirmed  PendingStatus = "confirmed"
	StatusConflict   PendingStatus = "conflict"
	StatusRolledBack PendingStatus = "rolled_back"
)

// NewLocalFirstLockService creates a new local-first lock service
func NewLocalFirstLockService(base *LockService, config LocalFirstConfig) *LocalFirstLockService {
	lfs := &LocalFirstLockService{
		LockService:  base,
		config:       config,
		pendingLocks: make(map[string]*PendingLock),
		conflictChan: make(chan *LockConflict, 100),
	}

	// Start conflict monitor
	go lfs.monitorConflicts()

	return lfs
}

// SetBroadcastFn sets the broadcast function for remote sync
func (s *LocalFirstLockService) SetBroadcastFn(fn func(msg any) error) {
	s.mu.Lock()
	s.broadcastFn = fn
	s.mu.Unlock()
	// Also set on base service
	s.LockService.SetBroadcastFn(fn)
}

// SetRollbackHandler sets the handler called when a lock is rolled back
func (s *LocalFirstLockService) SetRollbackHandler(fn func(lockID string, reason string)) {
	s.mu.Lock()
	s.rollbackHandler = fn
	s.mu.Unlock()
}

// AcquireLockOptimistic acquires a lock with local-first semantics
// Returns immediately after local acquisition, broadcasts asynchronously
func (s *LocalFirstLockService) AcquireLockOptimistic(ctx context.Context, req *AcquireLockRequest) (*LockResult, error) {
	if s.config.Mode == ModePessimistic {
		// Fall back to standard behavior
		return s.AcquireLock(ctx, req)
	}

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

	// Check for local conflicts first (< 1ms)
	localConflicts := s.store.FindConflicts(target)
	if len(localConflicts) > 0 {
		return &LockResult{
			Success: false,
			Reason:  fmt.Sprintf("conflict with existing lock: %s", localConflicts[0].ID),
		}, ErrLockConflict
	}

	// Create and store lock locally (< 1ms)
	lock := NewSemanticLock(target, s.nodeID, s.nodeName, req.Intention)
	if err := s.store.Add(lock); err != nil {
		return &LockResult{
			Success: false,
			Reason:  err.Error(),
		}, err
	}

	// Track as pending
	pending := &PendingLock{
		Lock:       lock,
		AcquiredAt: time.Now(),
		Status:     StatusPending,
	}
	s.mu.Lock()
	s.pendingLocks[lock.ID] = pending
	s.mu.Unlock()

	// Broadcast asynchronously
	go s.broadcastLockAcquired(lock)

	// Start conflict detection in background
	go s.detectConflicts(lock)

	return &LockResult{
		Success: true,
		Lock:    lock,
		Reason:  "lock acquired (pending confirmation)",
	}, nil
}

// broadcastLockAcquired broadcasts the lock acquisition to peers
func (s *LocalFirstLockService) broadcastLockAcquired(lock *SemanticLock) {
	s.mu.RLock()
	fn := s.broadcastFn
	s.mu.RUnlock()

	if fn == nil {
		return
	}

	msg := struct {
		Type string        `json:"type"`
		Lock *SemanticLock `json:"lock"`
	}{
		Type: "lock_acquired",
		Lock: lock,
	}

	if err := fn(msg); err != nil {
		// Log error but don't fail - lock is still valid locally
		fmt.Printf("Warning: failed to broadcast lock: %v\n", err)
	}
}

// detectConflicts monitors for conflicts after optimistic acquisition
func (s *LocalFirstLockService) detectConflicts(lock *SemanticLock) {
	// Wait for conflict timeout
	timer := time.NewTimer(s.config.ConflictTimeout)
	defer timer.Stop()

	select {
	case <-timer.C:
		// No conflict detected within timeout - confirm lock
		s.confirmLock(lock.ID)
	case conflict := <-s.conflictChan:
		if conflict.ConflictingLock.ID == lock.ID || conflict.RequestedLock.ID == lock.ID {
			s.handleConflict(lock, conflict)
		}
	}
}

// confirmLock confirms a pending lock
func (s *LocalFirstLockService) confirmLock(lockID string) {
	s.mu.Lock()
	pending, ok := s.pendingLocks[lockID]
	if ok && pending.Status == StatusPending {
		now := time.Now()
		pending.ConfirmedAt = &now
		pending.Status = StatusConfirmed
	}
	s.mu.Unlock()
}

// handleConflict handles a detected conflict
func (s *LocalFirstLockService) handleConflict(lock *SemanticLock, conflict *LockConflict) {
	s.mu.Lock()
	pending, ok := s.pendingLocks[lock.ID]
	if !ok {
		s.mu.Unlock()
		return
	}

	pending.Status = StatusConflict
	pending.ConflictsWith = append(pending.ConflictsWith, conflict.ConflictingLock)
	s.mu.Unlock()

	// Call conflict callback
	if s.config.ConflictCallback != nil {
		s.config.ConflictCallback(lock, conflict.ConflictingLock)
	}

	// Auto-rollback if configured
	if s.config.AutoRollback {
		s.rollbackLock(lock.ID, "conflict detected with: "+conflict.ConflictingLock.ID)
	}
}

// rollbackLock rolls back a pending lock
func (s *LocalFirstLockService) rollbackLock(lockID string, reason string) {
	s.mu.Lock()
	pending, ok := s.pendingLocks[lockID]
	if ok {
		pending.Status = StatusRolledBack
	}
	handler := s.rollbackHandler
	s.mu.Unlock()

	// Remove from store
	_ = s.store.Remove(lockID)

	// Broadcast release
	s.mu.RLock()
	fn := s.broadcastFn
	s.mu.RUnlock()

	if fn != nil {
		msg := struct {
			Type   string `json:"type"`
			LockID string `json:"lock_id"`
		}{
			Type:   "lock_released",
			LockID: lockID,
		}
		_ = fn(msg)
	}

	// Notify handler
	if handler != nil {
		handler(lockID, reason)
	}
}

// monitorConflicts monitors for conflicts from remote peers
func (s *LocalFirstLockService) monitorConflicts() {
	// This is called by the base service when a conflict is detected
	s.SetConflictHandler(func(conflict *LockConflict) error {
		select {
		case s.conflictChan <- conflict:
		default:
			// Channel full, conflict will be handled by timeout
		}
		return nil
	})
}

// HandleRemoteLockAcquired handles a remote lock acquisition with conflict check
func (s *LocalFirstLockService) HandleRemoteLockAcquired(remoteLock *SemanticLock) error {
	// Check if this conflicts with any pending locks
	s.mu.RLock()
	for _, pending := range s.pendingLocks {
		if pending.Status == StatusPending && pending.Lock.Target.Overlaps(remoteLock.Target) {
			// Conflict detected - use NewLockConflict for proper initialization
			conflict := NewLockConflict(pending.Lock, remoteLock)
			select {
			case s.conflictChan <- conflict:
			default:
			}
		}
	}
	s.mu.RUnlock()

	// Store the remote lock
	return s.LockService.HandleRemoteLockAcquired(remoteLock)
}

// GetPendingLocks returns all pending locks
func (s *LocalFirstLockService) GetPendingLocks() []*PendingLock {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]*PendingLock, 0, len(s.pendingLocks))
	for _, p := range s.pendingLocks {
		result = append(result, p)
	}
	return result
}

// GetPendingLock returns a specific pending lock
func (s *LocalFirstLockService) GetPendingLock(lockID string) *PendingLock {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.pendingLocks[lockID]
}

// IsPending checks if a lock is still pending confirmation
func (s *LocalFirstLockService) IsPending(lockID string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	p, ok := s.pendingLocks[lockID]
	return ok && p.Status == StatusPending
}

// IsConfirmed checks if a lock has been confirmed
func (s *LocalFirstLockService) IsConfirmed(lockID string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	p, ok := s.pendingLocks[lockID]
	return ok && p.Status == StatusConfirmed
}
