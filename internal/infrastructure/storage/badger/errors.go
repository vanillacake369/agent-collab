package badger

import (
	"errors"
	"fmt"

	"github.com/dgraph-io/badger/v4"
)

// Storage errors that wrap BadgerDB-specific errors.
var (
	ErrNotFound  = errors.New("key not found")
	ErrCorrupted = errors.New("database corrupted")
	ErrReadOnly  = errors.New("database is read-only")
	ErrClosed    = errors.New("database is closed")
	ErrConflict  = errors.New("transaction conflict")
	ErrTxnTooBig = errors.New("transaction too big")
)

// WrapError converts BadgerDB errors to storage errors.
func WrapError(err error) error {
	if err == nil {
		return nil
	}

	switch {
	case errors.Is(err, badger.ErrKeyNotFound):
		return ErrNotFound
	case errors.Is(err, badger.ErrDBClosed):
		return ErrClosed
	case errors.Is(err, badger.ErrConflict):
		return ErrConflict
	case errors.Is(err, badger.ErrTxnTooBig):
		return ErrTxnTooBig
	case errors.Is(err, badger.ErrBlockedWrites):
		return fmt.Errorf("write blocked, database may be full: %w", err)
	case errors.Is(err, badger.ErrTruncateNeeded):
		return ErrCorrupted
	default:
		return err
	}
}

// IsNotFound returns true if the error indicates a missing key.
func IsNotFound(err error) bool {
	return errors.Is(err, ErrNotFound) || errors.Is(err, badger.ErrKeyNotFound)
}

// IsRetriable returns true if the operation can be safely retried.
func IsRetriable(err error) bool {
	return errors.Is(err, ErrConflict) || errors.Is(err, badger.ErrConflict)
}

// IsFatal returns true if the error indicates a non-recoverable state.
func IsFatal(err error) bool {
	return errors.Is(err, ErrCorrupted) ||
		errors.Is(err, ErrClosed) ||
		errors.Is(err, badger.ErrTruncateNeeded)
}
