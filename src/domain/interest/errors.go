package interest

import "errors"

var (
	// ErrNilInterest is returned when interest is nil.
	ErrNilInterest = errors.New("interest cannot be nil")

	// ErrEmptyAgentID is returned when agent ID is empty.
	ErrEmptyAgentID = errors.New("agent ID cannot be empty")

	// ErrEmptyPatterns is returned when patterns slice is empty.
	ErrEmptyPatterns = errors.New("patterns cannot be empty")

	// ErrInterestNotFound is returned when interest is not found.
	ErrInterestNotFound = errors.New("interest not found")

	// ErrInvalidPattern is returned when a pattern is invalid.
	ErrInvalidPattern = errors.New("invalid pattern")
)
