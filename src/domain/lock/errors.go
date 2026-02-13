package lock

import (
	"errors"
	"fmt"

	pkgerrors "agent-collab/src/pkg/errors"
)

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

	// ErrRateLimited indicates the request was rate limited.
	ErrRateLimited = errors.New("rate limited: too many requests")
)

// LockError represents a lock-related error with context and category.
type LockError struct {
	Code     string
	Message  string
	category pkgerrors.Category
	LockID   string
	FilePath string
	Cause    error
}

func (e *LockError) Error() string {
	if e.LockID != "" && e.FilePath != "" {
		return fmt.Sprintf("%s: %s (lock: %s, file: %s)", e.Code, e.Message, e.LockID, e.FilePath)
	}
	if e.LockID != "" {
		return fmt.Sprintf("%s: %s (lock: %s)", e.Code, e.Message, e.LockID)
	}
	if e.FilePath != "" {
		return fmt.Sprintf("%s: %s (file: %s)", e.Code, e.Message, e.FilePath)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

// Category returns the error category for handling decisions.
func (e *LockError) Category() pkgerrors.Category {
	return e.category
}

// Unwrap returns the underlying cause.
func (e *LockError) Unwrap() error {
	return e.Cause
}

// WithLockID returns a copy with the lock ID set.
func (e *LockError) WithLockID(lockID string) *LockError {
	return &LockError{
		Code:     e.Code,
		Message:  e.Message,
		category: e.category,
		LockID:   lockID,
		FilePath: e.FilePath,
		Cause:    e.Cause,
	}
}

// WithFilePath returns a copy with the file path set.
func (e *LockError) WithFilePath(filePath string) *LockError {
	return &LockError{
		Code:     e.Code,
		Message:  e.Message,
		category: e.category,
		LockID:   e.LockID,
		FilePath: filePath,
		Cause:    e.Cause,
	}
}

// WithCause returns a copy with the cause set.
func (e *LockError) WithCause(cause error) *LockError {
	return &LockError{
		Code:     e.Code,
		Message:  e.Message,
		category: e.category,
		LockID:   e.LockID,
		FilePath: e.FilePath,
		Cause:    cause,
	}
}

// Categorized lock errors for type-safe error handling.
var (
	// ErrLockConflictCategorized is a retryable lock conflict error.
	ErrLockConflictCategorized = &LockError{
		Code:     "LOCK_CONFLICT",
		Message:  "resource already locked by another agent",
		category: pkgerrors.CategoryRetryable,
	}

	// ErrLockNotFoundCategorized is a permanent lock not found error.
	ErrLockNotFoundCategorized = &LockError{
		Code:     "LOCK_NOT_FOUND",
		Message:  "lock does not exist",
		category: pkgerrors.CategoryPermanent,
	}

	// ErrLockExpiredCategorized is a retryable lock expired error.
	ErrLockExpiredCategorized = &LockError{
		Code:     "LOCK_EXPIRED",
		Message:  "lock has expired",
		category: pkgerrors.CategoryRetryable,
	}

	// ErrInvalidTargetCategorized is a validation error for invalid targets.
	ErrInvalidTargetCategorized = &LockError{
		Code:     "INVALID_TARGET",
		Message:  "lock target is invalid",
		category: pkgerrors.CategoryValidation,
	}

	// ErrInvalidHolderID is a validation error for empty holder ID.
	ErrInvalidHolderID = &LockError{
		Code:     "INVALID_HOLDER_ID",
		Message:  "holder ID cannot be empty",
		category: pkgerrors.CategoryValidation,
	}
)

// ValidationError represents input validation failures for lock operations.
type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("lock validation error: %s %s", e.Field, e.Message)
}

// Category returns the validation category.
func (e *ValidationError) Category() pkgerrors.Category {
	return pkgerrors.CategoryValidation
}

// NewValidationError creates a new lock validation error.
func NewValidationError(field, message string) *ValidationError {
	return &ValidationError{
		Field:   field,
		Message: message,
	}
}
