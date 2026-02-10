package v1

import (
	"time"
)

// LockKind is the resource kind for Lock.
const LockKind = "Lock"

// LockPhase represents the current phase of a lock lifecycle.
type LockPhase string

const (
	// LockPhasePending indicates the lock request is waiting to be processed.
	LockPhasePending LockPhase = "Pending"

	// LockPhaseNegotiating indicates the lock is in negotiation with other agents.
	LockPhaseNegotiating LockPhase = "Negotiating"

	// LockPhaseActive indicates the lock is held and active.
	LockPhaseActive LockPhase = "Active"

	// LockPhaseReleasing indicates the lock is being released.
	LockPhaseReleasing LockPhase = "Releasing"

	// LockPhaseReleased indicates the lock has been released.
	LockPhaseReleased LockPhase = "Released"

	// LockPhaseExpired indicates the lock has expired due to TTL.
	LockPhaseExpired LockPhase = "Expired"

	// LockPhaseFailed indicates the lock acquisition failed.
	LockPhaseFailed LockPhase = "Failed"
)

// LockTargetType represents the type of lock target.
type LockTargetType string

const (
	// LockTargetTypeFile indicates a file-level lock.
	LockTargetTypeFile LockTargetType = "File"

	// LockTargetTypeFunction indicates a function-level lock.
	LockTargetTypeFunction LockTargetType = "Function"

	// LockTargetTypeLineRange indicates a line-range lock.
	LockTargetTypeLineRange LockTargetType = "LineRange"

	// LockTargetTypeSymbol indicates a symbol-level lock.
	LockTargetTypeSymbol LockTargetType = "Symbol"
)

// Lock represents a distributed lock resource in the system.
// Locks are used to coordinate access to shared resources (files, functions, etc.)
// across multiple agents.
type Lock struct {
	TypeMeta   `json:",inline"`
	ObjectMeta `json:"metadata"`

	// Spec describes the desired state of the lock.
	Spec LockSpec `json:"spec"`

	// Status describes the current state of the lock.
	Status LockStatus `json:"status,omitempty"`
}

// GetObjectMeta returns the ObjectMeta of the Lock.
func (l *Lock) GetObjectMeta() *ObjectMeta {
	return &l.ObjectMeta
}

// LockSpec defines the desired state of a Lock.
type LockSpec struct {
	// Target specifies what resource is being locked.
	Target LockTarget `json:"target"`

	// HolderID is the identifier of the agent requesting/holding the lock.
	HolderID string `json:"holderId"`

	// HolderName is a human-readable name for the holder (optional).
	HolderName string `json:"holderName,omitempty"`

	// Intention describes why the lock is being acquired.
	// This helps other agents understand the purpose.
	Intention string `json:"intention"`

	// TTL is the time-to-live for the lock.
	// The lock will automatically expire after this duration.
	TTL Duration `json:"ttl"`

	// Priority is the priority of this lock request.
	// Higher priority requests may preempt lower priority locks.
	Priority int32 `json:"priority,omitempty"`

	// Exclusive indicates if this is an exclusive lock.
	// Non-exclusive locks allow multiple readers.
	Exclusive bool `json:"exclusive"`
}

// LockTarget specifies the resource being locked.
type LockTarget struct {
	// Type is the type of lock target.
	Type LockTargetType `json:"type"`

	// FilePath is the path to the file being locked.
	FilePath string `json:"filePath"`

	// StartLine is the starting line number for line-range locks.
	StartLine int32 `json:"startLine,omitempty"`

	// EndLine is the ending line number for line-range locks.
	EndLine int32 `json:"endLine,omitempty"`

	// Symbol is the symbol name for symbol-level locks.
	Symbol string `json:"symbol,omitempty"`

	// FunctionName is the function name for function-level locks.
	FunctionName string `json:"functionName,omitempty"`
}

// LockStatus defines the observed state of a Lock.
type LockStatus struct {
	// Phase is the current phase of the lock lifecycle.
	Phase LockPhase `json:"phase"`

	// FencingToken is a monotonically increasing token for fencing.
	// Used to prevent split-brain scenarios.
	FencingToken uint64 `json:"fencingToken"`

	// AcquiredAt is the timestamp when the lock was acquired.
	AcquiredAt *time.Time `json:"acquiredAt,omitempty"`

	// ExpiresAt is the timestamp when the lock will expire.
	ExpiresAt *time.Time `json:"expiresAt,omitempty"`

	// LastRenewedAt is the timestamp when the lock was last renewed.
	LastRenewedAt *time.Time `json:"lastRenewedAt,omitempty"`

	// ConflictingLocks lists any locks that conflict with this one.
	ConflictingLocks []LockConflict `json:"conflictingLocks,omitempty"`

	// Conditions represent the current state of the lock.
	Conditions []Condition `json:"conditions,omitempty"`

	// Message provides additional information about the current state.
	Message string `json:"message,omitempty"`
}

// LockConflict describes a conflicting lock.
type LockConflict struct {
	// LockName is the name of the conflicting lock.
	LockName string `json:"lockName"`

	// HolderID is the ID of the agent holding the conflicting lock.
	HolderID string `json:"holderId"`

	// HolderName is the name of the agent holding the conflicting lock.
	HolderName string `json:"holderName,omitempty"`

	// Target is the target of the conflicting lock.
	Target LockTarget `json:"target"`
}

// LockList is a list of Lock resources.
type LockList struct {
	TypeMeta `json:",inline"`
	ListMeta `json:"metadata,omitempty"`

	// Items is the list of locks.
	Items []Lock `json:"items"`
}

// Lock condition types.
const (
	// LockConditionReady indicates the lock is ready and active.
	LockConditionReady = "Ready"

	// LockConditionConflict indicates there is a conflict with another lock.
	LockConditionConflict = "Conflict"

	// LockConditionExpiring indicates the lock is about to expire.
	LockConditionExpiring = "Expiring"

	// LockConditionNegotiating indicates the lock is in negotiation.
	LockConditionNegotiating = "Negotiating"
)

// NewLock creates a new Lock with default values.
func NewLock(name string, spec LockSpec) *Lock {
	return &Lock{
		TypeMeta: TypeMeta{
			Kind:       LockKind,
			APIVersion: GroupVersion,
		},
		ObjectMeta: ObjectMeta{
			Name:              name,
			CreationTimestamp: time.Now(),
		},
		Spec: spec,
		Status: LockStatus{
			Phase: LockPhasePending,
		},
	}
}

// IsActive returns true if the lock is currently active.
func (l *Lock) IsActive() bool {
	return l.Status.Phase == LockPhaseActive
}

// IsExpired returns true if the lock has expired.
func (l *Lock) IsExpired() bool {
	if l.Status.ExpiresAt == nil {
		return false
	}
	return time.Now().After(*l.Status.ExpiresAt)
}

// HasConflict returns true if there are conflicting locks.
func (l *Lock) HasConflict() bool {
	return len(l.Status.ConflictingLocks) > 0
}

// SetCondition updates or adds a condition.
func (l *Lock) SetCondition(condType string, status ConditionStatus, reason, message string) {
	now := time.Now()
	for i := range l.Status.Conditions {
		if l.Status.Conditions[i].Type == condType {
			if l.Status.Conditions[i].Status != status {
				l.Status.Conditions[i].LastTransitionTime = now
			}
			l.Status.Conditions[i].Status = status
			l.Status.Conditions[i].Reason = reason
			l.Status.Conditions[i].Message = message
			return
		}
	}
	l.Status.Conditions = append(l.Status.Conditions, Condition{
		Type:               condType,
		Status:             status,
		LastTransitionTime: now,
		Reason:             reason,
		Message:            message,
	})
}

// GetCondition returns the condition with the given type.
func (l *Lock) GetCondition(condType string) *Condition {
	for i := range l.Status.Conditions {
		if l.Status.Conditions[i].Type == condType {
			return &l.Status.Conditions[i]
		}
	}
	return nil
}
