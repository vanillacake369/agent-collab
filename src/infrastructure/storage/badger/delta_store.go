package badger

import (
	"bytes"
	"encoding/json"
	"fmt"
	"time"

	"github.com/dgraph-io/badger/v4"

	"agent-collab/src/domain/ctxsync"
)

const (
	// Key prefixes for delta storage
	// Primary key: delta:{sourceID}:{timestamp_ns}:{deltaID}
	// Index key:   delta_ts:{timestamp_ns}:{sourceID}:{deltaID}
	prefixDelta   = "delta:"
	prefixDeltaTS = "delta_ts:"
)

// DeltaStore implements storage.DeltaStore using BadgerDB.
// Uses sharded keys by sourceID to prevent LSM hotspots.
type DeltaStore struct {
	db *badger.DB
}

// NewDeltaStore creates a new BadgerDB-backed delta store.
func NewDeltaStore(db *badger.DB) *DeltaStore {
	return &DeltaStore{db: db}
}

// deltaKey generates the primary key for a delta.
// Format: delta:{sourceID}:{timestamp_ns}:{deltaID}
// Sharding by sourceID distributes writes across the key space.
func (s *DeltaStore) deltaKey(delta *ctxsync.Delta) []byte {
	ts := delta.Timestamp.UnixNano()
	return []byte(fmt.Sprintf("%s%s:%020d:%s",
		prefixDelta, delta.SourceID, ts, delta.ID))
}

// indexKey generates the time-ordered index key for a delta.
// Format: delta_ts:{timestamp_ns}:{sourceID}:{deltaID}
// Enables efficient range queries by time.
func (s *DeltaStore) indexKey(delta *ctxsync.Delta) []byte {
	ts := delta.Timestamp.UnixNano()
	return []byte(fmt.Sprintf("%s%020d:%s:%s",
		prefixDeltaTS, ts, delta.SourceID, delta.ID))
}

// Save persists a single delta to storage.
func (s *DeltaStore) Save(delta *ctxsync.Delta) error {
	data, err := json.Marshal(delta)
	if err != nil {
		return fmt.Errorf("marshal delta: %w", err)
	}

	err = s.db.Update(func(txn *badger.Txn) error {
		// Write primary entry
		if err := txn.Set(s.deltaKey(delta), data); err != nil {
			return err
		}
		// Write time-ordered index entry (empty value, key contains all info)
		return txn.Set(s.indexKey(delta), nil)
	})

	return WrapError(err)
}

// SaveBatch persists multiple deltas in a single transaction.
func (s *DeltaStore) SaveBatch(deltas []*ctxsync.Delta) error {
	if len(deltas) == 0 {
		return nil
	}

	batch := s.db.NewWriteBatch()
	defer batch.Cancel()

	for _, delta := range deltas {
		data, err := json.Marshal(delta)
		if err != nil {
			continue // Skip invalid deltas
		}
		if err := batch.Set(s.deltaKey(delta), data); err != nil {
			return WrapError(err)
		}
		if err := batch.Set(s.indexKey(delta), nil); err != nil {
			return WrapError(err)
		}
	}

	return WrapError(batch.Flush())
}

// GetSince retrieves all deltas that happened after or concurrent with
// the given vector clock.
func (s *DeltaStore) GetSince(vc *ctxsync.VectorClock) ([]*ctxsync.Delta, error) {
	var deltas []*ctxsync.Delta

	err := s.db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.Prefix = []byte(prefixDelta)

		it := txn.NewIterator(opts)
		defer it.Close()

		for it.Rewind(); it.Valid(); it.Next() {
			item := it.Item()

			// Skip index keys (they start with delta_ts:)
			if bytes.HasPrefix(item.Key(), []byte(prefixDeltaTS)) {
				continue
			}

			err := item.Value(func(val []byte) error {
				var delta ctxsync.Delta
				if err := json.Unmarshal(val, &delta); err != nil {
					return nil // Skip invalid entries
				}

				// Filter by vector clock: include if happens after or concurrent
				if delta.VectorClock != nil && vc != nil {
					cmp := delta.VectorClock.Compare(vc)
					if cmp == HappensAfter || cmp == Concurrent {
						deltas = append(deltas, &delta)
					}
				} else {
					// Include if no vector clock comparison possible
					deltas = append(deltas, &delta)
				}
				return nil
			})
			if err != nil {
				return err
			}
		}
		return nil
	})

	return deltas, WrapError(err)
}

// VectorClock comparison constants (mirror ctxsync package)
const (
	HappensBefore = -1
	Equal         = 0
	HappensAfter  = 1
	Concurrent    = 2
)

// GetRange retrieves all deltas within the given time range.
func (s *DeltaStore) GetRange(start, end time.Time) ([]*ctxsync.Delta, error) {
	var deltas []*ctxsync.Delta

	startKey := []byte(fmt.Sprintf("%s%020d", prefixDeltaTS, start.UnixNano()))
	endKey := []byte(fmt.Sprintf("%s%020d", prefixDeltaTS, end.UnixNano()))

	err := s.db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.PrefetchValues = false // Index keys have no values
		it := txn.NewIterator(opts)
		defer it.Close()

		for it.Seek(startKey); it.Valid(); it.Next() {
			item := it.Item()
			key := item.Key()

			// Stop if past end time
			if bytes.Compare(key, endKey) > 0 {
				break
			}

			// Parse index key to reconstruct delta key
			// Format: delta_ts:{ts}:{sourceID}:{deltaID}
			keyStr := string(key)
			if len(keyStr) <= len(prefixDeltaTS) {
				continue
			}

			parts := bytes.Split(key[len(prefixDeltaTS):], []byte(":"))
			if len(parts) != 3 {
				continue
			}

			ts := parts[0]
			sourceID := parts[1]
			deltaID := parts[2]

			// Construct primary key
			primaryKey := []byte(fmt.Sprintf("%s%s:%s:%s",
				prefixDelta, sourceID, ts, deltaID))

			// Fetch actual delta
			deltaItem, err := txn.Get(primaryKey)
			if err != nil {
				continue // Skip if not found
			}

			err = deltaItem.Value(func(val []byte) error {
				var delta ctxsync.Delta
				if err := json.Unmarshal(val, &delta); err == nil {
					deltas = append(deltas, &delta)
				}
				return nil
			})
			if err != nil {
				continue
			}
		}
		return nil
	})

	return deltas, WrapError(err)
}

// GetBySource retrieves the most recent deltas from a specific source.
func (s *DeltaStore) GetBySource(sourceID string, limit int) ([]*ctxsync.Delta, error) {
	var deltas []*ctxsync.Delta

	prefix := []byte(fmt.Sprintf("%s%s:", prefixDelta, sourceID))

	err := s.db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.Reverse = true // Newest first
		it := txn.NewIterator(opts)
		defer it.Close()

		count := 0
		// Seek to end of prefix range
		seekKey := append(prefix, 0xFF)
		for it.Seek(seekKey); it.Valid() && count < limit; it.Next() {
			item := it.Item()

			// Check if still within prefix
			if !bytes.HasPrefix(item.Key(), prefix) {
				break
			}

			err := item.Value(func(val []byte) error {
				var delta ctxsync.Delta
				if err := json.Unmarshal(val, &delta); err == nil {
					deltas = append(deltas, &delta)
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

	// Reverse to get chronological order
	for i, j := 0, len(deltas)-1; i < j; i, j = i+1, j-1 {
		deltas[i], deltas[j] = deltas[j], deltas[i]
	}

	return deltas, WrapError(err)
}

// Compact removes deltas older than the given time.
// Returns the number of deleted entries.
func (s *DeltaStore) Compact(before time.Time) (int64, error) {
	var deleted int64
	endKey := []byte(fmt.Sprintf("%s%020d", prefixDeltaTS, before.UnixNano()))

	// Collect keys to delete
	var keysToDelete [][]byte

	err := s.db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.PrefetchValues = false
		it := txn.NewIterator(opts)
		defer it.Close()

		prefix := []byte(prefixDeltaTS)
		for it.Seek(prefix); it.Valid(); it.Next() {
			item := it.Item()
			key := item.Key()

			// Stop if past cutoff time
			if bytes.Compare(key, endKey) >= 0 {
				break
			}

			// Parse index key
			parts := bytes.Split(key[len(prefixDeltaTS):], []byte(":"))
			if len(parts) != 3 {
				continue
			}

			ts := parts[0]
			sourceID := parts[1]
			deltaID := parts[2]

			// Add index key and primary key to delete list
			keysToDelete = append(keysToDelete, append([]byte{}, key...))
			primaryKey := []byte(fmt.Sprintf("%s%s:%s:%s",
				prefixDelta, sourceID, ts, deltaID))
			keysToDelete = append(keysToDelete, primaryKey)
		}
		return nil
	})

	if err != nil {
		return 0, WrapError(err)
	}

	// Delete in batch
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

		deleted = int64(len(keysToDelete) / 2) // Each delta has 2 keys
	}

	return deleted, nil
}

// Close releases any resources held by the store.
// Note: The BadgerDB instance is managed by Manager and closed separately.
func (s *DeltaStore) Close() error {
	return nil
}

// Count returns the total number of deltas in the store.
func (s *DeltaStore) Count() (int64, error) {
	var count int64

	err := s.db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.PrefetchValues = false
		opts.Prefix = []byte(prefixDeltaTS) // Count index keys

		it := txn.NewIterator(opts)
		defer it.Close()

		for it.Rewind(); it.Valid(); it.Next() {
			count++
		}
		return nil
	})

	return count, WrapError(err)
}
