package lock

import (
	"context"
	"sync"
	"time"
)

// CleanupInterval is the interval for expired lock cleanup.
const CleanupInterval = 10 * time.Second

// LockStore is a lock storage.
type LockStore struct {
	mu       sync.RWMutex
	locks    map[string]*SemanticLock // lockID -> lock
	byTarget map[string]string        // targetID -> lockID
	history  []*HistoryEntry          // recent lock history
	maxHistory int
	ctx      context.Context
	cancel   context.CancelFunc
}

// NewLockStore creates a new lock store.
func NewLockStore(ctx context.Context) *LockStore {
	ctx, cancel := context.WithCancel(ctx)
	store := &LockStore{
		locks:      make(map[string]*SemanticLock),
		byTarget:   make(map[string]string),
		history:    make([]*HistoryEntry, 0, 100),
		maxHistory: 100,
		ctx:        ctx,
		cancel:     cancel,
	}

	go store.cleanupExpired()

	return store
}

// Close stops the cleanup goroutine and releases resources.
func (s *LockStore) Close() error {
	s.cancel()
	return nil
}

// Add stores a lock.
func (s *LockStore) Add(lock *SemanticLock) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Check if target already has a lock
	targetID := lock.Target.ID()
	if existingID, exists := s.byTarget[targetID]; exists {
		if existingLock, ok := s.locks[existingID]; ok && !existingLock.IsExpired() {
			return ErrLockConflict
		}
	}

	s.locks[lock.ID] = lock
	s.byTarget[targetID] = lock.ID

	// Record history
	s.addHistory(&HistoryEntry{
		Timestamp:  time.Now(),
		Action:     "acquired",
		LockID:     lock.ID,
		HolderID:   lock.HolderID,
		HolderName: lock.HolderName,
		Target:     lock.Target.String(),
	})

	return nil
}

// Get retrieves a lock.
func (s *LockStore) Get(lockID string) (*SemanticLock, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	lock, exists := s.locks[lockID]
	if !exists {
		return nil, ErrLockNotFound
	}

	if lock.IsExpired() {
		return nil, ErrLockExpired
	}

	return lock, nil
}

// GetByTarget retrieves a lock by target.
func (s *LockStore) GetByTarget(target *SemanticTarget) (*SemanticLock, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	lockID, exists := s.byTarget[target.ID()]
	if !exists {
		return nil, ErrLockNotFound
	}

	lock, exists := s.locks[lockID]
	if !exists {
		return nil, ErrLockNotFound
	}

	if lock.IsExpired() {
		return nil, ErrLockExpired
	}

	return lock, nil
}

// Remove removes a lock.
func (s *LockStore) Remove(lockID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	lock, exists := s.locks[lockID]
	if !exists {
		return ErrLockNotFound
	}

	delete(s.locks, lockID)
	delete(s.byTarget, lock.Target.ID())

	// Record history
	s.addHistory(&HistoryEntry{
		Timestamp:  time.Now(),
		Action:     "released",
		LockID:     lock.ID,
		HolderID:   lock.HolderID,
		HolderName: lock.HolderName,
		Target:     lock.Target.String(),
	})

	return nil
}

// FindConflicts finds conflicting locks.
func (s *LockStore) FindConflicts(target *SemanticTarget) []*SemanticLock {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var conflicts []*SemanticLock

	for _, lock := range s.locks {
		if lock.IsExpired() {
			continue
		}

		if lock.Target.Overlaps(target) {
			conflicts = append(conflicts, lock)
		}
	}

	return conflicts
}

// List returns all active locks.
func (s *LockStore) List() []*SemanticLock {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []*SemanticLock

	for _, lock := range s.locks {
		if !lock.IsExpired() {
			result = append(result, lock)
		}
	}

	return result
}

// ListByHolder returns locks held by a specific holder.
func (s *LockStore) ListByHolder(holderID string) []*SemanticLock {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []*SemanticLock

	for _, lock := range s.locks {
		if !lock.IsExpired() && lock.HolderID == holderID {
			result = append(result, lock)
		}
	}

	return result
}

// Count returns the number of active locks.
func (s *LockStore) Count() int {
	s.mu.RLock()
	defer s.mu.RUnlock()

	count := 0
	for _, lock := range s.locks {
		if !lock.IsExpired() {
			count++
		}
	}

	return count
}

// cleanupExpired cleans up expired locks.
func (s *LockStore) cleanupExpired() {
	ticker := time.NewTicker(CleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			s.mu.Lock()
			for id, lock := range s.locks {
				if lock.IsExpired() {
					delete(s.locks, id)
					delete(s.byTarget, lock.Target.ID())
					// Record expiration in history
					s.history = append(s.history, &HistoryEntry{
						Timestamp:  time.Now(),
						Action:     "expired",
						LockID:     lock.ID,
						HolderID:   lock.HolderID,
						HolderName: lock.HolderName,
						Target:     lock.Target.String(),
					})
				}
			}
			s.mu.Unlock()
		}
	}
}

// addHistory adds an entry to the history (must be called with lock held).
func (s *LockStore) addHistory(entry *HistoryEntry) {
	s.history = append(s.history, entry)
	if len(s.history) > s.maxHistory {
		s.history = s.history[1:]
	}
}

// GetHistory returns recent lock history entries.
func (s *LockStore) GetHistory(limit int) []*HistoryEntry {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if limit <= 0 || limit > len(s.history) {
		limit = len(s.history)
	}

	// Return most recent entries
	start := len(s.history) - limit
	if start < 0 {
		start = 0
	}

	result := make([]*HistoryEntry, limit)
	copy(result, s.history[start:])

	// Reverse to show newest first
	for i, j := 0, len(result)-1; i < j; i, j = i+1, j-1 {
		result[i], result[j] = result[j], result[i]
	}

	return result
}
