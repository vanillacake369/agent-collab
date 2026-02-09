package lock

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// NegotiationState is the negotiation state.
type NegotiationState string

const (
	StateIntentAnnounced NegotiationState = "intent_announced"
	StateWaitingVotes    NegotiationState = "waiting_votes"
	StateAcquiring       NegotiationState = "acquiring"
	StateAcquired        NegotiationState = "acquired"
	StateRejected        NegotiationState = "rejected"
	StateNegotiating     NegotiationState = "negotiating"
	StateEscalated       NegotiationState = "escalated"
)

// Timeout constants for negotiation.
const (
	IntentTimeout            = 5 * time.Second
	VoteTimeout              = 10 * time.Second
	NegotiationTimeout       = 30 * time.Second
	SessionCleanupInterval   = 30 * time.Second
	ResolvedSessionRetention = time.Hour
)

// Vote is a vote for lock acquisition.
type Vote struct {
	VoterID   string    `json:"voter_id"`
	VoterName string    `json:"voter_name"`
	Approve   bool      `json:"approve"`
	Reason    string    `json:"reason"`
	Timestamp time.Time `json:"timestamp"`
}

// NegotiationSession is a negotiation session.
type NegotiationSession struct {
	ID              string             `json:"id"`
	RequestedLock   *SemanticLock      `json:"requested_lock"`
	ConflictingLock *SemanticLock      `json:"conflicting_lock"`
	State           NegotiationState   `json:"state"`
	Votes           map[string]*Vote   `json:"votes"`
	RequiredVotes   int                `json:"required_votes"`
	StartedAt       time.Time          `json:"started_at"`
	ExpiresAt       time.Time          `json:"expires_at"`
	Resolution      *NegotiationResult `json:"resolution,omitempty"`
}

// NegotiationResult is the negotiation result.
type NegotiationResult struct {
	Success        bool           `json:"success"`
	WinnerLock     *SemanticLock  `json:"winner_lock,omitempty"`
	LoserLock      *SemanticLock  `json:"loser_lock,omitempty"`
	ResolutionType ResolutionType `json:"resolution_type"`
	Message        string         `json:"message"`
	ResolvedAt     time.Time      `json:"resolved_at"`
}

// ResolutionType is the resolution type.
type ResolutionType string

const (
	ResolutionApproved    ResolutionType = "approved"
	ResolutionRejected    ResolutionType = "rejected"
	ResolutionNegotiated  ResolutionType = "negotiated"
	ResolutionTimedOut    ResolutionType = "timed_out"
	ResolutionHumanNeeded ResolutionType = "human_needed"
)

// LockNegotiator is the lock negotiator.
type LockNegotiator struct {
	mu          sync.RWMutex
	store       *LockStore
	sessions    map[string]*NegotiationSession
	intentQueue map[string]*LockIntent
	ctx         context.Context
	cancel      context.CancelFunc

	// Rate limiting
	rateLimiter *RateLimiter

	// Callbacks
	onConflict  func(*LockConflict) error
	onEscalate  func(*NegotiationSession) error
	broadcastFn func(msg any) error
}

// LockIntent is a lock acquisition intent.
type LockIntent struct {
	ID           string          `json:"id"`
	Lock         *SemanticLock   `json:"lock"`
	AnnouncedAt  time.Time       `json:"announced_at"`
	ExpiresAt    time.Time       `json:"expires_at"`
	Acknowledged map[string]bool `json:"acknowledged"`
}

// NewLockNegotiator creates a new lock negotiator.
func NewLockNegotiator(ctx context.Context, store *LockStore) *LockNegotiator {
	ctx, cancel := context.WithCancel(ctx)
	n := &LockNegotiator{
		store:       store,
		sessions:    make(map[string]*NegotiationSession),
		intentQueue: make(map[string]*LockIntent),
		ctx:         ctx,
		cancel:      cancel,
		rateLimiter: NewRateLimiter(DefaultRateLimitConfig()),
	}

	go n.cleanupExpiredSessions()

	return n
}

// NewLockNegotiatorWithConfig creates a new lock negotiator with custom rate limit config.
func NewLockNegotiatorWithConfig(ctx context.Context, store *LockStore, rlConfig *RateLimitConfig) *LockNegotiator {
	ctx, cancel := context.WithCancel(ctx)
	n := &LockNegotiator{
		store:       store,
		sessions:    make(map[string]*NegotiationSession),
		intentQueue: make(map[string]*LockIntent),
		ctx:         ctx,
		cancel:      cancel,
		rateLimiter: NewRateLimiter(rlConfig),
	}

	go n.cleanupExpiredSessions()

	return n
}

// Close stops the cleanup goroutine and releases resources.
func (n *LockNegotiator) Close() error {
	n.cancel()
	return nil
}

// SetConflictHandler sets the conflict handler.
func (n *LockNegotiator) SetConflictHandler(handler func(*LockConflict) error) {
	n.onConflict = handler
}

// SetEscalateHandler sets the escalation handler.
func (n *LockNegotiator) SetEscalateHandler(handler func(*NegotiationSession) error) {
	n.onEscalate = handler
}

// SetBroadcastFn sets the broadcast function.
func (n *LockNegotiator) SetBroadcastFn(fn func(msg any) error) {
	n.broadcastFn = fn
}

// AnnounceIntent announces lock acquisition intent (Phase 1).
func (n *LockNegotiator) AnnounceIntent(ctx context.Context, lock *SemanticLock) (*LockIntent, error) {
	// Rate limit check before acquiring lock
	if !n.rateLimiter.Allow(lock.HolderID) {
		return nil, ErrRateLimited
	}

	n.mu.Lock()
	defer n.mu.Unlock()

	// Check for conflicts
	conflicts := n.store.FindConflicts(lock.Target)
	if len(conflicts) > 0 {
		// Start negotiation for conflicts
		for _, conflicting := range conflicts {
			conflict := NewLockConflict(lock, conflicting)
			if n.onConflict != nil {
				if err := n.onConflict(conflict); err != nil {
					return nil, fmt.Errorf("conflict handler failed: %w", err)
				}
			}
		}

		// Start negotiation session for first conflict
		session := n.startNegotiationSession(lock, conflicts[0])
		return nil, fmt.Errorf("conflict detected, negotiation session started: %s", session.ID)
	}

	// Register intent
	now := time.Now()
	intent := &LockIntent{
		ID:           lock.ID,
		Lock:         lock,
		AnnouncedAt:  now,
		ExpiresAt:    now.Add(IntentTimeout),
		Acknowledged: make(map[string]bool),
	}

	n.intentQueue[intent.ID] = intent

	// Broadcast
	if n.broadcastFn != nil {
		if err := n.broadcastFn(IntentMessage{
			Type:   "lock_intent",
			Intent: intent,
		}); err != nil {
			// Log error but don't fail
			fmt.Printf("broadcast intent failed: %v\n", err)
		}
	}

	return intent, nil
}

// AcquireLock acquires a lock (Phase 2).
func (n *LockNegotiator) AcquireLock(ctx context.Context, intentID string) (*LockResult, error) {
	n.mu.Lock()
	defer n.mu.Unlock()

	intent, exists := n.intentQueue[intentID]
	if !exists {
		return &LockResult{
			Success: false,
			Reason:  "intent not found",
		}, ErrLockNotFound
	}

	// Check intent expiration
	if time.Now().After(intent.ExpiresAt) {
		delete(n.intentQueue, intentID)
		return &LockResult{
			Success: false,
			Reason:  "intent expired",
		}, ErrLockExpired
	}

	// Check for conflicts again (new locks may have been acquired after intent announcement)
	conflicts := n.store.FindConflicts(intent.Lock.Target)
	if len(conflicts) > 0 {
		delete(n.intentQueue, intentID)
		return &LockResult{
			Success: false,
			Reason:  fmt.Sprintf("conflict with %s", conflicts[0].HolderName),
		}, ErrLockConflict
	}

	// Acquire lock
	if err := n.store.Add(intent.Lock); err != nil {
		delete(n.intentQueue, intentID)
		return &LockResult{
			Success: false,
			Reason:  err.Error(),
		}, err
	}

	delete(n.intentQueue, intentID)

	// Broadcast
	if n.broadcastFn != nil {
		if err := n.broadcastFn(AcquireMessage{
			Type: "lock_acquired",
			Lock: intent.Lock,
		}); err != nil {
			fmt.Printf("broadcast acquire failed: %v\n", err)
		}
	}

	return &LockResult{
		Success: true,
		Lock:    intent.Lock,
		Reason:  "lock acquired successfully",
	}, nil
}

// ReleaseLock releases a lock (Phase 3).
func (n *LockNegotiator) ReleaseLock(ctx context.Context, lockID, holderID string) error {
	n.mu.Lock()
	defer n.mu.Unlock()

	lock, err := n.store.Get(lockID)
	if err != nil {
		return err
	}

	if lock.HolderID != holderID {
		return ErrNotLockHolder
	}

	if err := n.store.Remove(lockID); err != nil {
		return err
	}

	// Broadcast
	if n.broadcastFn != nil {
		if err := n.broadcastFn(ReleaseMessage{
			Type:   "lock_released",
			LockID: lockID,
		}); err != nil {
			fmt.Printf("broadcast release failed: %v\n", err)
		}
	}

	return nil
}

// Negotiate negotiates a conflict.
func (n *LockNegotiator) Negotiate(ctx context.Context, sessionID string, proposal *NegotiationProposal) (*NegotiationResult, error) {
	n.mu.Lock()
	defer n.mu.Unlock()

	session, exists := n.sessions[sessionID]
	if !exists {
		return nil, fmt.Errorf("session not found: %s", sessionID)
	}

	if time.Now().After(session.ExpiresAt) {
		session.State = StateEscalated
		result := &NegotiationResult{
			Success:        false,
			ResolutionType: ResolutionTimedOut,
			Message:        "negotiation timed out",
			ResolvedAt:     time.Now(),
		}
		session.Resolution = result

		if n.onEscalate != nil {
			n.onEscalate(session)
		}

		return result, ErrNegotiationFailed
	}

	switch proposal.Type {
	case ProposalYield:
		return n.handleYieldProposal(session, proposal)
	case ProposalSplit:
		return n.handleSplitProposal(session, proposal)
	case ProposalPriority:
		return n.handlePriorityProposal(session)
	case ProposalEscalate:
		return n.handleEscalateProposal(session, proposal)
	default:
		return nil, fmt.Errorf("unknown proposal type: %s", proposal.Type)
	}
}

// Vote votes on lock acquisition.
func (n *LockNegotiator) Vote(ctx context.Context, sessionID string, vote *Vote) error {
	n.mu.Lock()
	defer n.mu.Unlock()

	session, exists := n.sessions[sessionID]
	if !exists {
		return fmt.Errorf("session not found: %s", sessionID)
	}

	session.Votes[vote.VoterID] = vote

	// Check if voting is complete
	if len(session.Votes) >= session.RequiredVotes {
		n.resolveByVotes(session)
	}

	return nil
}

// GetSession retrieves a negotiation session.
func (n *LockNegotiator) GetSession(sessionID string) (*NegotiationSession, error) {
	n.mu.RLock()
	defer n.mu.RUnlock()

	session, exists := n.sessions[sessionID]
	if !exists {
		return nil, fmt.Errorf("session not found: %s", sessionID)
	}

	return session, nil
}

// ListActiveSessions lists active negotiation sessions.
func (n *LockNegotiator) ListActiveSessions() []*NegotiationSession {
	n.mu.RLock()
	defer n.mu.RUnlock()

	var sessions []*NegotiationSession
	for _, session := range n.sessions {
		if session.Resolution == nil {
			sessions = append(sessions, session)
		}
	}

	return sessions
}

// startNegotiationSession starts a negotiation session.
func (n *LockNegotiator) startNegotiationSession(requested, conflicting *SemanticLock) *NegotiationSession {
	now := time.Now()
	session := &NegotiationSession{
		ID:              fmt.Sprintf("neg-%s", requested.ID[5:]),
		RequestedLock:   requested,
		ConflictingLock: conflicting,
		State:           StateNegotiating,
		Votes:           make(map[string]*Vote),
		RequiredVotes:   2, // Default: both parties
		StartedAt:       now,
		ExpiresAt:       now.Add(NegotiationTimeout),
	}

	n.sessions[session.ID] = session

	return session
}

// handleYieldProposal handles a yield proposal.
func (n *LockNegotiator) handleYieldProposal(session *NegotiationSession, proposal *NegotiationProposal) (*NegotiationResult, error) {
	var winner, loser *SemanticLock

	if proposal.YielderID == session.RequestedLock.HolderID {
		winner = session.ConflictingLock
		loser = session.RequestedLock
	} else {
		winner = session.RequestedLock
		loser = session.ConflictingLock

		// Remove existing lock and add new lock
		n.store.Remove(session.ConflictingLock.ID)
		n.store.Add(session.RequestedLock)
	}

	result := &NegotiationResult{
		Success:        true,
		WinnerLock:     winner,
		LoserLock:      loser,
		ResolutionType: ResolutionNegotiated,
		Message:        fmt.Sprintf("%s yielded to %s", loser.HolderName, winner.HolderName),
		ResolvedAt:     time.Now(),
	}

	session.State = StateAcquired
	session.Resolution = result

	return result, nil
}

// handleSplitProposal handles a split proposal.
func (n *LockNegotiator) handleSplitProposal(session *NegotiationSession, proposal *NegotiationProposal) (*NegotiationResult, error) {
	// Split the target so each party gets their own region
	// This implementation assumes line-range based splitting

	if proposal.SplitPoint <= session.RequestedLock.Target.StartLine ||
		proposal.SplitPoint >= session.RequestedLock.Target.EndLine {
		return nil, fmt.Errorf("invalid split point: %d", proposal.SplitPoint)
	}

	// First part: conflicting lock keeps
	session.ConflictingLock.Target.EndLine = proposal.SplitPoint - 1

	// Second part: requested lock acquires
	session.RequestedLock.Target.StartLine = proposal.SplitPoint
	n.store.Add(session.RequestedLock)

	result := &NegotiationResult{
		Success:        true,
		WinnerLock:     session.RequestedLock,
		LoserLock:      session.ConflictingLock,
		ResolutionType: ResolutionNegotiated,
		Message:        fmt.Sprintf("split at line %d", proposal.SplitPoint),
		ResolvedAt:     time.Now(),
	}

	session.State = StateAcquired
	session.Resolution = result

	return result, nil
}

// handlePriorityProposal handles a priority proposal.
func (n *LockNegotiator) handlePriorityProposal(session *NegotiationSession) (*NegotiationResult, error) {
	// Priority based on fencing token
	var winner, loser *SemanticLock

	if session.RequestedLock.FencingToken > session.ConflictingLock.FencingToken {
		winner = session.RequestedLock
		loser = session.ConflictingLock
		n.store.Remove(session.ConflictingLock.ID)
		n.store.Add(session.RequestedLock)
	} else {
		winner = session.ConflictingLock
		loser = session.RequestedLock
	}

	result := &NegotiationResult{
		Success:        true,
		WinnerLock:     winner,
		LoserLock:      loser,
		ResolutionType: ResolutionNegotiated,
		Message:        fmt.Sprintf("priority: fencing token %d > %d", winner.FencingToken, loser.FencingToken),
		ResolvedAt:     time.Now(),
	}

	session.State = StateAcquired
	session.Resolution = result

	return result, nil
}

// handleEscalateProposal handles an escalation proposal.
func (n *LockNegotiator) handleEscalateProposal(session *NegotiationSession, proposal *NegotiationProposal) (*NegotiationResult, error) {
	session.State = StateEscalated

	result := &NegotiationResult{
		Success:        false,
		ResolutionType: ResolutionHumanNeeded,
		Message:        fmt.Sprintf("escalated: %s", proposal.EscalateReason),
		ResolvedAt:     time.Now(),
	}

	session.Resolution = result

	if n.onEscalate != nil {
		n.onEscalate(session)
	}

	return result, ErrHumanInterventionRequired
}

// resolveByVotes resolves by votes.
func (n *LockNegotiator) resolveByVotes(session *NegotiationSession) {
	approves := 0
	for _, vote := range session.Votes {
		if vote.Approve {
			approves++
		}
	}

	var result *NegotiationResult
	if approves > len(session.Votes)/2 {
		// Approved by majority
		n.store.Remove(session.ConflictingLock.ID)
		n.store.Add(session.RequestedLock)

		result = &NegotiationResult{
			Success:        true,
			WinnerLock:     session.RequestedLock,
			LoserLock:      session.ConflictingLock,
			ResolutionType: ResolutionApproved,
			Message:        fmt.Sprintf("approved by vote: %d/%d", approves, len(session.Votes)),
			ResolvedAt:     time.Now(),
		}
		session.State = StateAcquired
	} else {
		result = &NegotiationResult{
			Success:        false,
			WinnerLock:     session.ConflictingLock,
			LoserLock:      session.RequestedLock,
			ResolutionType: ResolutionRejected,
			Message:        fmt.Sprintf("rejected by vote: %d/%d", approves, len(session.Votes)),
			ResolvedAt:     time.Now(),
		}
		session.State = StateRejected
	}

	session.Resolution = result
}

// cleanupExpiredSessions cleans up expired sessions.
func (n *LockNegotiator) cleanupExpiredSessions() {
	ticker := time.NewTicker(SessionCleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-n.ctx.Done():
			return
		case <-ticker.C:
			n.mu.Lock()
			now := time.Now()

			// Cleanup expired intents
			for id, intent := range n.intentQueue {
				if now.After(intent.ExpiresAt) {
					delete(n.intentQueue, id)
				}
			}

			// Cleanup resolved sessions (delete after retention period)
			for id, session := range n.sessions {
				if session.Resolution != nil && now.Sub(session.Resolution.ResolvedAt) > ResolvedSessionRetention {
					delete(n.sessions, id)
				}
			}

			n.mu.Unlock()
		}
	}
}

// NegotiationProposal is a negotiation proposal.
type NegotiationProposal struct {
	Type           ProposalType `json:"type"`
	YielderID      string       `json:"yielder_id,omitempty"`
	SplitPoint     int          `json:"split_point,omitempty"`
	EscalateReason string       `json:"escalate_reason,omitempty"`
}

// ProposalType is the proposal type.
type ProposalType string

const (
	ProposalYield    ProposalType = "yield"
	ProposalSplit    ProposalType = "split"
	ProposalPriority ProposalType = "priority"
	ProposalEscalate ProposalType = "escalate"
)

// IntentMessage is an intent message.
type IntentMessage struct {
	Type   string      `json:"type"`
	Intent *LockIntent `json:"intent"`
}

// AcquireMessage is an acquire message.
type AcquireMessage struct {
	Type string        `json:"type"`
	Lock *SemanticLock `json:"lock"`
}

// ReleaseMessage is a release message.
type ReleaseMessage struct {
	Type   string `json:"type"`
	LockID string `json:"lock_id"`
}
