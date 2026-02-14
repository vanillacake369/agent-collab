package event

import "errors"

var (
	// ErrVectorStoreNotConfigured is returned when vector store is not configured.
	ErrVectorStoreNotConfigured = errors.New("vector store not configured")

	// ErrEventNotFound is returned when event is not found.
	ErrEventNotFound = errors.New("event not found")

	// ErrInvalidEventType is returned when event type is invalid.
	ErrInvalidEventType = errors.New("invalid event type")

	// ErrBroadcastNotConfigured is returned when broadcast function is not set.
	ErrBroadcastNotConfigured = errors.New("broadcast function not configured")
)
