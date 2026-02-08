package lock

import "errors"

// Sentinel errors for the lock package.
var (
	// ErrLockNotFound indicates the requested lock was not found.
	ErrLockNotFound = errors.New("lock not found")

	// ErrLockExpired indicates the lock has expired.
	ErrLockExpired = errors.New("lock expired")

	// ErrLockConflict indicates a lock conflict exists.
	ErrLockConflict = errors.New("lock conflict")

	// ErrNotLockHolder indicates the caller is not the lock holder.
	ErrNotLockHolder = errors.New("not lock holder")

	// ErrMaxRenewalsExceeded indicates the maximum renewal count was exceeded.
	ErrMaxRenewalsExceeded = errors.New("max renewals exceeded")

	// ErrInvalidTarget indicates the target is invalid.
	ErrInvalidTarget = errors.New("invalid target")

	// ErrNegotiationFailed indicates the negotiation failed.
	ErrNegotiationFailed = errors.New("negotiation failed")

	// ErrHumanInterventionRequired indicates human intervention is required.
	ErrHumanInterventionRequired = errors.New("human intervention required")

	// ErrSessionNotFound indicates the negotiation session was not found.
	ErrSessionNotFound = errors.New("session not found")

	// ErrIntentNotFound indicates the lock intent was not found.
	ErrIntentNotFound = errors.New("intent not found")
)
