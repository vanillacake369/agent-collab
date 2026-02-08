// Package storage provides storage interfaces and implementations for agent-collab.
package storage

import (
	"time"

	"agent-collab/internal/domain/ctxsync"
)

// DeltaStore persists context deltas for CRDT synchronization.
// Deltas are stored with sharded keys to prevent LSM hotspots.
type DeltaStore interface {
	// Save persists a single delta to storage.
	Save(delta *ctxsync.Delta) error

	// SaveBatch persists multiple deltas in a single transaction.
	SaveBatch(deltas []*ctxsync.Delta) error

	// GetSince retrieves all deltas that happened after or concurrent with
	// the given vector clock.
	GetSince(vc *ctxsync.VectorClock) ([]*ctxsync.Delta, error)

	// GetRange retrieves all deltas within the given time range.
	GetRange(start, end time.Time) ([]*ctxsync.Delta, error)

	// GetBySource retrieves the most recent deltas from a specific source.
	GetBySource(sourceID string, limit int) ([]*ctxsync.Delta, error)

	// Compact removes deltas older than the given time.
	// Returns the number of deleted entries.
	Compact(before time.Time) (int64, error)

	// Close releases any resources held by the store.
	Close() error
}

// AuditStore logs lock events for compliance and debugging.
// Writes are asynchronous to avoid impacting lock performance.
type AuditStore interface {
	// LogAsync queues an audit event for asynchronous persistence.
	// Returns immediately without waiting for the write to complete.
	LogAsync(event *AuditEvent) error

	// Query retrieves audit events matching the given filter.
	Query(filter AuditFilter) ([]*AuditEvent, error)

	// Close flushes pending events and releases resources.
	Close() error
}

// AuditEvent represents a lock operation for audit logging.
type AuditEvent struct {
	Timestamp  time.Time
	Action     string // acquired, released, expired, conflict
	LockID     string
	HolderID   string
	HolderName string
	Target     string
	Metadata   map[string]string
}

// AuditFilter specifies criteria for querying audit events.
type AuditFilter struct {
	HolderID  string
	Action    string
	StartTime time.Time
	EndTime   time.Time
	Limit     int
}
