package badger

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/dgraph-io/badger/v4"

	"agent-collab/internal/infrastructure/storage"
)

const (
	// Key prefix for audit log entries
	// Format: audit:{timestamp_ns}:{lockID}
	prefixAudit = "audit:"

	// Default buffer size for async writes
	defaultAuditBufferSize = 1000

	// Batch flush interval
	auditFlushInterval = time.Second
)

// AuditStore implements storage.AuditStore using BadgerDB.
// Uses async buffered writes to avoid impacting lock performance.
type AuditStore struct {
	db      *badger.DB
	buffer  chan *storage.AuditEvent
	done    chan struct{}
	wg      sync.WaitGroup
	bufSize int
}

// NewAuditStore creates a new BadgerDB-backed audit store.
func NewAuditStore(db *badger.DB, bufferSize int) *AuditStore {
	if bufferSize <= 0 {
		bufferSize = defaultAuditBufferSize
	}

	store := &AuditStore{
		db:      db,
		buffer:  make(chan *storage.AuditEvent, bufferSize),
		done:    make(chan struct{}),
		bufSize: bufferSize,
	}

	// Start async writer
	store.wg.Add(1)
	go store.writer()

	return store
}

// auditKey generates the key for an audit event.
// Format: audit:{timestamp_ns}:{lockID}
func (s *AuditStore) auditKey(event *storage.AuditEvent) []byte {
	ts := event.Timestamp.UnixNano()
	return []byte(fmt.Sprintf("%s%020d:%s", prefixAudit, ts, event.LockID))
}

// LogAsync queues an audit event for asynchronous persistence.
// Returns immediately without waiting for the write to complete.
func (s *AuditStore) LogAsync(event *storage.AuditEvent) error {
	select {
	case s.buffer <- event:
		return nil
	default:
		// Buffer full, log but don't block
		return fmt.Errorf("audit buffer full, event dropped")
	}
}

// writer runs in a goroutine and batches writes to BadgerDB.
func (s *AuditStore) writer() {
	defer s.wg.Done()

	batch := make([]*storage.AuditEvent, 0, 100)
	ticker := time.NewTicker(auditFlushInterval)
	defer ticker.Stop()

	flush := func() {
		if len(batch) == 0 {
			return
		}

		wb := s.db.NewWriteBatch()
		for _, event := range batch {
			data, err := json.Marshal(event)
			if err != nil {
				continue
			}
			wb.Set(s.auditKey(event), data)
		}
		wb.Flush()
		batch = batch[:0]
	}

	for {
		select {
		case event := <-s.buffer:
			batch = append(batch, event)
			if len(batch) >= 100 {
				flush()
			}
		case <-ticker.C:
			flush()
		case <-s.done:
			// Drain remaining events
			close(s.buffer)
			for event := range s.buffer {
				batch = append(batch, event)
			}
			flush()
			return
		}
	}
}

// Query retrieves audit events matching the given filter.
func (s *AuditStore) Query(filter storage.AuditFilter) ([]*storage.AuditEvent, error) {
	var events []*storage.AuditEvent

	// Set default time range if not specified
	startTime := filter.StartTime
	endTime := filter.EndTime
	if startTime.IsZero() {
		startTime = time.Unix(0, 0)
	}
	if endTime.IsZero() {
		endTime = time.Now().Add(time.Hour) // Include future events
	}

	startKey := []byte(fmt.Sprintf("%s%020d", prefixAudit, startTime.UnixNano()))
	endKey := []byte(fmt.Sprintf("%s%020d", prefixAudit, endTime.UnixNano()))

	err := s.db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		it := txn.NewIterator(opts)
		defer it.Close()

		count := 0
		for it.Seek(startKey); it.Valid(); it.Next() {
			// Check limit
			if filter.Limit > 0 && count >= filter.Limit {
				break
			}

			item := it.Item()

			// Check end time
			if bytes.Compare(item.Key(), endKey) > 0 {
				break
			}

			err := item.Value(func(val []byte) error {
				var event storage.AuditEvent
				if err := json.Unmarshal(val, &event); err != nil {
					return nil // Skip invalid entries
				}

				// Apply filters
				if filter.HolderID != "" && event.HolderID != filter.HolderID {
					return nil
				}
				if filter.Action != "" && event.Action != filter.Action {
					return nil
				}

				events = append(events, &event)
				count++
				return nil
			})
			if err != nil {
				return err
			}
		}
		return nil
	})

	return events, WrapError(err)
}

// QueryByLockID retrieves all audit events for a specific lock.
func (s *AuditStore) QueryByLockID(lockID string, limit int) ([]*storage.AuditEvent, error) {
	var events []*storage.AuditEvent

	err := s.db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		it := txn.NewIterator(opts)
		defer it.Close()

		count := 0
		for it.Seek([]byte(prefixAudit)); it.Valid(); it.Next() {
			if limit > 0 && count >= limit {
				break
			}

			item := it.Item()

			// Check if key contains lockID
			if !bytes.Contains(item.Key(), []byte(lockID)) {
				continue
			}

			err := item.Value(func(val []byte) error {
				var event storage.AuditEvent
				if err := json.Unmarshal(val, &event); err != nil {
					return nil
				}

				if event.LockID == lockID {
					events = append(events, &event)
					count++
				}
				return nil
			})
			if err != nil {
				return err
			}
		}
		return nil
	})

	return events, WrapError(err)
}

// Close flushes pending events and releases resources.
func (s *AuditStore) Close() error {
	close(s.done)
	s.wg.Wait()
	return nil
}

// Count returns the total number of audit events.
func (s *AuditStore) Count() (int64, error) {
	var count int64

	err := s.db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.PrefetchValues = false
		opts.Prefix = []byte(prefixAudit)

		it := txn.NewIterator(opts)
		defer it.Close()

		for it.Rewind(); it.Valid(); it.Next() {
			count++
		}
		return nil
	})

	return count, WrapError(err)
}

// Compact removes audit events older than the given time.
func (s *AuditStore) Compact(before time.Time) (int64, error) {
	var deleted int64
	endKey := []byte(fmt.Sprintf("%s%020d", prefixAudit, before.UnixNano()))

	var keysToDelete [][]byte

	err := s.db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.PrefetchValues = false
		it := txn.NewIterator(opts)
		defer it.Close()

		for it.Seek([]byte(prefixAudit)); it.Valid(); it.Next() {
			item := it.Item()
			key := item.Key()

			if bytes.Compare(key, endKey) >= 0 {
				break
			}

			keysToDelete = append(keysToDelete, append([]byte{}, key...))
		}
		return nil
	})

	if err != nil {
		return 0, WrapError(err)
	}

	if len(keysToDelete) > 0 {
		batch := s.db.NewWriteBatch()
		defer batch.Cancel()

		for _, key := range keysToDelete {
			if err := batch.Delete(key); err != nil {
				return 0, WrapError(err)
			}
		}

		if err := batch.Flush(); err != nil {
			return 0, WrapError(err)
		}

		deleted = int64(len(keysToDelete))
	}

	return deleted, nil
}
