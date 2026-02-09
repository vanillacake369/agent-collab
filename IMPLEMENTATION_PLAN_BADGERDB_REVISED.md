# Implementation Plan: Revised BadgerDB Migration for agent-collab

## Executive Summary

This revised plan takes a **pragmatic, safety-first approach** to adding BadgerDB persistence, addressing all critical issues from the security review. Key changes:

1. **Lock Store**: Keep in-memory with async audit log only
2. **Delta Store**: Add BadgerDB with improved key schema
3. **Metrics**: Keep JSONL (optimal for append-only time-series)
4. **Vector Store**: Keep JSON (BadgerDB is wrong tool for vector search)

**Philosophy**: Not everything needs BadgerDB. Use the right tool for each job.

---

## Project Context

### Detected Environment
- **Primary Language**: Go 1.22
- **Architecture**: Clean Architecture with DDD patterns
- **P2P Framework**: libp2p with Kademlia DHT and Gossipsub
- **Current Storage**:
  - Locks: Pure in-memory (`sync.RWMutex` + maps)
  - Deltas: In-memory ring buffer (`DeltaLog`)
  - Metrics: JSONL files (daily rotation)
  - Vectors: JSON files with in-memory index
- **Data Directory**: `~/.agent-collab/`

### Discovered Conventions
- **Error Handling**: Wrapped errors with `fmt.Errorf`
- **Concurrency**: `sync.RWMutex` for read-heavy workloads
- **Testing**: E2E tests in `tests/e2e/`
- **Naming**: `NewXStore`, `Close()` lifecycle pattern
- **No External Dependencies**: Currently zero database dependencies

---

## Problem Analysis (From Review)

### Critical Issues Identified

1. **Architecture Flaws**
   - Single BadgerDB instance causes namespace collision
   - No storage abstraction layer
   - Dual-layer caching creates coherency problems

2. **Performance Regressions**
   - Lock store: O(1) in-memory → O(log n) disk lookup (100-1000x slower)
   - Delta store: Sequential keys are LSM anti-pattern
   - Metrics: JSONL already optimal for append-only time-series

3. **Security Vulnerabilities**
   - Lock expiration race conditions with dual cache
   - Fencing token not persisted (resets on restart)
   - Delta log lacks integrity verification

4. **Implementation Gaps**
   - No migration strategy from existing storage
   - Missing BadgerDB-specific error handling
   - No crash safety or corruption recovery

5. **Testing Gaps**
   - No crash safety tests
   - No concurrency stress tests
   - No performance regression benchmarks

---

## Revised Architecture

### Component Decision Matrix

| Component | Current | BadgerDB? | Rationale | Performance Impact |
|-----------|---------|-----------|-----------|-------------------|
| **Lock Store** | In-memory maps | ❌ NO | Locks are ephemeral, millisecond latency required | O(1) stays O(1) |
| **Lock Audit** | None | ✅ YES (async) | Compliance/debugging, non-blocking writes | Negligible |
| **Delta Log** | In-memory ring | ✅ YES | CRDT needs persistence across restarts | +10-50ms write latency (acceptable) |
| **Metrics** | JSONL | ❌ NO | Time-series append is JSONL's sweet spot | No change |
| **Vector Store** | JSON | ❌ NO | BadgerDB lacks vector search, cosine similarity | No change |

### Architecture Diagram

```
┌──────────────────────────────────────────────────────────────────┐
│                        Application Layer                          │
├──────────────────────────────────────────────────────────────────┤
│                                                                    │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐               │
│  │ LockService │  │ SyncManager │  │TokenTracker │               │
│  │  (domain)   │  │  (domain)   │  │  (domain)   │               │
│  └──────┬──────┘  └──────┬──────┘  └──────┬──────┘               │
│         │                │                │                       │
├─────────┼────────────────┼────────────────┼───────────────────────┤
│         │                │                │                       │
│  ┌──────▼──────┐  ┌──────▼──────┐  ┌──────▼──────┐               │
│  │  LockStore  │  │  DeltaLog   │  │MetricsStore │               │
│  │ (in-memory) │  │  (hybrid)   │  │   (JSONL)   │               │
│  └──────┬──────┘  └──────┬──────┘  └─────────────┘               │
│         │                │                                        │
│         │         ┌──────▼──────┐                                 │
│         │         │  DeltaStore │  ← BadgerDB Adapter             │
│         │         │  (BadgerDB) │                                 │
│         │         └─────────────┘                                 │
│         │                                                         │
│  ┌──────▼──────────┐                                              │
│  │ LockAuditStore  │  ← BadgerDB Adapter (async)                 │
│  │   (BadgerDB)    │                                              │
│  └─────────────────┘                                              │
│                                                                    │
├──────────────────────────────────────────────────────────────────┤
│                   Storage Abstraction Layer                       │
│                                                                    │
│  type DeltaStore interface {                                      │
│    Save(delta *Delta) error                                       │
│    GetSince(vc *VectorClock) ([]*Delta, error)                    │
│    GetRange(start, end time.Time) ([]*Delta, error)               │
│  }                                                                 │
│                                                                    │
│  type AuditStore interface {                                      │
│    LogAsync(event *AuditEvent) error                              │
│    Query(filter AuditFilter) ([]*AuditEvent, error)               │
│  }                                                                 │
│                                                                    │
├──────────────────────────────────────────────────────────────────┤
│                      BadgerDB Layer                               │
│                                                                    │
│  ┌──────────────────────────────────────────────────┐             │
│  │         BadgerDB Instance Manager                │             │
│  │  - Multiple named instances (audit, delta)       │             │
│  │  - Shared options, separate directories          │             │
│  │  - Lifecycle management (open/close/GC)          │             │
│  └──────────────────────────────────────────────────┘             │
│                                                                    │
└──────────────────────────────────────────────────────────────────┘

Data Directory Structure:
~/.agent-collab/
├── badger/
│   ├── audit/          ← Lock audit log (BadgerDB)
│   │   ├── MANIFEST
│   │   └── *.sst
│   └── delta/          ← Delta persistence (BadgerDB)
│       ├── MANIFEST
│       └── *.sst
├── metrics/            ← Token usage (JSONL) - unchanged
│   └── usage_2026-02-09.jsonl
├── vectors/            ← Embeddings (JSON) - unchanged
│   └── default.json
└── key.json
```

---

## Implementation Strategy

### Phase 1: Foundation (Week 1)

**Goal**: Add storage abstraction layer and BadgerDB infrastructure without changing behavior.

#### Step 1.1: Add BadgerDB Dependency
- **File**: `/Users/limjihoon/dev/agent-collab/go.mod`
- **Action**: Add `github.com/dgraph-io/badger/v4 v4.2.0`
- **Rationale**: Use latest v4 for improved performance and WAL compression
- **Validation**: `go mod tidy && go build ./...`

#### Step 1.2: Create Storage Abstraction Layer
- **File**: `/Users/limjihoon/dev/agent-collab/internal/infrastructure/storage/interfaces.go` (new)
- **Action**: Define storage interfaces
```go
package storage

import (
    "time"
    "agent-collab/internal/domain/ctxsync"
)

// DeltaStore persists context deltas for CRDT synchronization.
type DeltaStore interface {
    Save(delta *ctxsync.Delta) error
    SaveBatch(deltas []*ctxsync.Delta) error
    GetSince(vc *ctxsync.VectorClock) ([]*ctxsync.Delta, error)
    GetRange(start, end time.Time) ([]*ctxsync.Delta, error)
    GetBySource(sourceID string, limit int) ([]*ctxsync.Delta, error)
    Compact(before time.Time) (int64, error)
    Close() error
}

// AuditStore logs lock events for compliance and debugging.
type AuditStore interface {
    LogAsync(event *AuditEvent) error
    Query(filter AuditFilter) ([]*AuditEvent, error)
    Close() error
}

type AuditEvent struct {
    Timestamp  time.Time
    Action     string // acquired, released, expired, conflict
    LockID     string
    HolderID   string
    HolderName string
    Target     string
    Metadata   map[string]string
}

type AuditFilter struct {
    HolderID  string
    Action    string
    StartTime time.Time
    EndTime   time.Time
    Limit     int
}
```
- **Pattern Reference**: Follows existing `vector.Store` interface pattern

#### Step 1.3: Create BadgerDB Manager
- **File**: `/Users/limjihoon/dev/agent-collab/internal/infrastructure/storage/badger/manager.go` (new)
- **Action**: Implement multi-instance BadgerDB manager
```go
package badger

import (
    "fmt"
    "path/filepath"
    "sync"

    "github.com/dgraph-io/badger/v4"
)

// Manager manages multiple named BadgerDB instances.
type Manager struct {
    mu        sync.RWMutex
    baseDir   string
    instances map[string]*badger.DB
    options   badger.Options
}

func NewManager(baseDir string) *Manager {
    opts := badger.DefaultOptions("").
        WithLogger(nil).                    // Disable verbose logging
        WithValueLogFileSize(64 << 20).     // 64MB value log
        WithNumVersionsToKeep(1).           // Keep only latest version
        WithCompactL0OnClose(true).         // Compact on close
        WithDetectConflicts(false)          // No MVCC conflicts needed

    return &Manager{
        baseDir:   filepath.Join(baseDir, "badger"),
        instances: make(map[string]*badger.DB),
        options:   opts,
    }
}

func (m *Manager) Open(name string) (*badger.DB, error) {
    m.mu.Lock()
    defer m.mu.Unlock()

    if db, exists := m.instances[name]; exists {
        return db, nil
    }

    dbPath := filepath.Join(m.baseDir, name)
    opts := m.options.WithDir(dbPath).WithValueDir(dbPath)

    db, err := badger.Open(opts)
    if err != nil {
        return nil, fmt.Errorf("failed to open badger db %s: %w", name, err)
    }

    m.instances[name] = db
    return db, nil
}

func (m *Manager) CloseAll() error {
    m.mu.Lock()
    defer m.mu.Unlock()

    var firstErr error
    for name, db := range m.instances {
        if err := db.Close(); err != nil && firstErr == nil {
            firstErr = fmt.Errorf("failed to close %s: %w", name, err)
        }
    }
    m.instances = make(map[string]*badger.DB)
    return firstErr
}

// RunGC runs garbage collection on all instances.
func (m *Manager) RunGC(discardRatio float64) error {
    m.mu.RLock()
    defer m.mu.RUnlock()

    for name, db := range m.instances {
        for {
            err := db.RunValueLogGC(discardRatio)
            if err == badger.ErrNoRewrite {
                break // No more GC needed
            }
            if err != nil {
                return fmt.Errorf("GC failed for %s: %w", name, err)
            }
        }
    }
    return nil
}
```

#### Step 1.4: Add Error Handling Utilities
- **File**: `/Users/limjihoon/dev/agent-collab/internal/infrastructure/storage/badger/errors.go` (new)
- **Action**: BadgerDB-specific error handling
```go
package badger

import (
    "errors"
    "fmt"

    "github.com/dgraph-io/badger/v4"
)

var (
    ErrNotFound   = errors.New("key not found")
    ErrCorrupted  = errors.New("database corrupted")
    ErrReadOnly   = errors.New("database is read-only")
    ErrClosed     = errors.New("database is closed")
)

func WrapError(err error) error {
    if err == nil {
        return nil
    }

    switch {
    case errors.Is(err, badger.ErrKeyNotFound):
        return ErrNotFound
    case errors.Is(err, badger.ErrDBClosed):
        return ErrClosed
    case errors.Is(err, badger.ErrBlockedWrites):
        return fmt.Errorf("write blocked, database may be full: %w", err)
    case errors.Is(err, badger.ErrTruncateNeeded):
        return ErrCorrupted
    default:
        return err
    }
}

func IsRetriable(err error) bool {
    return errors.Is(err, badger.ErrConflict) ||
           errors.Is(err, badger.ErrTxnTooBig)
}
```

**Testing**: Unit tests for manager and error handling
```bash
go test ./internal/infrastructure/storage/badger/... -v
```

---

### Phase 2: Delta Store (Week 2)

**Goal**: Persist CRDT deltas with optimized key schema for LSM-tree.

#### Step 2.1: Design Key Schema

**Problem**: Sequential time-based keys cause LSM hotspot.
**Solution**: Shard by source ID, then timestamp.

```
Key Schema:
  delta:{sourceID}:{timestamp_ns}:{deltaID}

Examples:
  delta:peer-abc123:1707480123456789000:delta-f3a2b1
  delta:peer-def456:1707480123457890000:delta-c9d8e7

Index for range queries:
  delta_ts:{timestamp_ns}:{sourceID}:{deltaID} → (empty value, key is the data)
```

**Rationale**:
- Primary key shards writes across multiple peers (no hotspot)
- Timestamp in nanoseconds for uniqueness
- Secondary index for time-range queries
- LSM-tree handles this well (writes distributed, compaction efficient)

#### Step 2.2: Implement BadgerDB Delta Store
- **File**: `/Users/limjihoon/dev/agent-collab/internal/infrastructure/storage/badger/delta_store.go` (new)
- **Action**: Implement `DeltaStore` interface
```go
package badger

import (
    "bytes"
    "encoding/binary"
    "encoding/json"
    "fmt"
    "time"

    "github.com/dgraph-io/badger/v4"
    "agent-collab/internal/domain/ctxsync"
    "agent-collab/internal/infrastructure/storage"
)

type DeltaStore struct {
    db *badger.DB
}

func NewDeltaStore(db *badger.DB) *DeltaStore {
    return &DeltaStore{db: db}
}

// Key format: delta:{sourceID}:{timestamp_ns}:{deltaID}
func (s *DeltaStore) deltaKey(delta *ctxsync.Delta) []byte {
    ts := delta.Timestamp.UnixNano()
    return []byte(fmt.Sprintf("delta:%s:%020d:%s",
        delta.SourceID, ts, delta.ID))
}

// Index key: delta_ts:{timestamp_ns}:{sourceID}:{deltaID}
func (s *DeltaStore) indexKey(delta *ctxsync.Delta) []byte {
    ts := delta.Timestamp.UnixNano()
    return []byte(fmt.Sprintf("delta_ts:%020d:%s:%s",
        ts, delta.SourceID, delta.ID))
}

func (s *DeltaStore) Save(delta *ctxsync.Delta) error {
    data, err := json.Marshal(delta)
    if err != nil {
        return fmt.Errorf("marshal delta: %w", err)
    }

    err = s.db.Update(func(txn *badger.Txn) error {
        // Write main entry
        if err := txn.Set(s.deltaKey(delta), data); err != nil {
            return err
        }
        // Write index entry (empty value)
        return txn.Set(s.indexKey(delta), nil)
    })

    return WrapError(err)
}

func (s *DeltaStore) SaveBatch(deltas []*ctxsync.Delta) error {
    batch := s.db.NewWriteBatch()
    defer batch.Cancel()

    for _, delta := range deltas {
        data, err := json.Marshal(delta)
        if err != nil {
            continue
        }
        batch.Set(s.deltaKey(delta), data)
        batch.Set(s.indexKey(delta), nil)
    }

    return WrapError(batch.Flush())
}

func (s *DeltaStore) GetRange(start, end time.Time) ([]*ctxsync.Delta, error) {
    var deltas []*ctxsync.Delta

    startKey := []byte(fmt.Sprintf("delta_ts:%020d", start.UnixNano()))
    endKey := []byte(fmt.Sprintf("delta_ts:%020d", end.UnixNano()))

    err := s.db.View(func(txn *badger.Txn) error {
        opts := badger.DefaultIteratorOptions
        opts.PrefetchValues = false // Only need keys for index
        it := txn.NewIterator(opts)
        defer it.Close()

        for it.Seek(startKey); it.Valid(); it.Next() {
            item := it.Item()
            key := item.Key()

            if bytes.Compare(key, endKey) > 0 {
                break
            }

            // Parse index key to get actual delta key
            // delta_ts:{ts}:{sourceID}:{deltaID}
            parts := bytes.Split(key, []byte(":"))
            if len(parts) != 4 {
                continue
            }

            deltaKey := []byte(fmt.Sprintf("delta:%s:%s:%s",
                parts[2], parts[1], parts[3]))

            // Fetch actual delta
            deltaItem, err := txn.Get(deltaKey)
            if err != nil {
                continue
            }

            err = deltaItem.Value(func(val []byte) error {
                var delta ctxsync.Delta
                if err := json.Unmarshal(val, &delta); err == nil {
                    deltas = append(deltas, &delta)
                }
                return nil
            })
        }
        return nil
    })

    return deltas, WrapError(err)
}

func (s *DeltaStore) GetBySource(sourceID string, limit int) ([]*ctxsync.Delta, error) {
    var deltas []*ctxsync.Delta

    prefix := []byte(fmt.Sprintf("delta:%s:", sourceID))

    err := s.db.View(func(txn *badger.Txn) error {
        opts := badger.DefaultIteratorOptions
        opts.Reverse = true // Newest first
        it := txn.NewIterator(opts)
        defer it.Close()

        count := 0
        for it.Seek(prefix); it.Valid() && count < limit; it.Next() {
            item := it.Item()
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

    return deltas, WrapError(err)
}

func (s *DeltaStore) Compact(before time.Time) (int64, error) {
    var deleted int64
    endKey := []byte(fmt.Sprintf("delta_ts:%020d", before.UnixNano()))

    batch := s.db.NewWriteBatch()
    defer batch.Cancel()

    err := s.db.View(func(txn *badger.Txn) error {
        opts := badger.DefaultIteratorOptions
        opts.PrefetchValues = false
        it := txn.NewIterator(opts)
        defer it.Close()

        prefix := []byte("delta_ts:")
        for it.Seek(prefix); it.Valid(); it.Next() {
            item := it.Item()
            key := item.Key()

            if bytes.Compare(key, endKey) >= 0 {
                break
            }

            batch.Delete(key)
            deleted++
        }
        return nil
    })

    if err != nil {
        return 0, WrapError(err)
    }

    return deleted, WrapError(batch.Flush())
}

func (s *DeltaStore) Close() error {
    return nil // DB closed by manager
}
```

#### Step 2.3: Integrate with SyncManager
- **File**: `/Users/limjihoon/dev/agent-collab/internal/domain/ctxsync/sync.go`
- **Action**: Add async persistence to existing in-memory DeltaLog
```go
// Add to SyncManager struct:
type SyncManager struct {
    // ... existing fields ...
    deltaStore storage.DeltaStore // nil means memory-only
}

// Modify BroadcastDelta:
func (sm *SyncManager) BroadcastDelta(delta *Delta) error {
    // Add to in-memory log (fast path)
    sm.deltaLog.Append(delta)

    // Async persist to BadgerDB (if available)
    if sm.deltaStore != nil {
        go func() {
            if err := sm.deltaStore.Save(delta); err != nil {
                // Log but don't block - in-memory is source of truth
                fmt.Printf("WARN: failed to persist delta: %v\n", err)
            }
        }()
    }

    // Broadcast to P2P network
    if sm.broadcastFn != nil {
        return sm.broadcastFn(delta)
    }
    return nil
}
```

**Migration**: On startup, load recent deltas from BadgerDB into in-memory log
```go
func (sm *SyncManager) loadRecentDeltas() error {
    if sm.deltaStore == nil {
        return nil
    }

    // Load last 7 days
    start := time.Now().AddDate(0, 0, -7)
    deltas, err := sm.deltaStore.GetRange(start, time.Now())
    if err != nil {
        return err
    }

    for _, delta := range deltas {
        sm.deltaLog.Append(delta)
    }
    return nil
}
```

---

### Phase 3: Lock Audit Log (Week 3)

**Goal**: Add async audit logging without impacting lock performance.

#### Step 3.1: Implement Lock Audit Store
- **File**: `/Users/limjihoon/dev/agent-collab/internal/infrastructure/storage/badger/audit_store.go` (new)
- **Action**: Implement `AuditStore` interface
```go
package badger

import (
    "bytes"
    "encoding/json"
    "fmt"
    "time"

    "github.com/dgraph-io/badger/v4"
    "agent-collab/internal/infrastructure/storage"
)

type AuditStore struct {
    db      *badger.DB
    buffer  chan *storage.AuditEvent
    bufSize int
}

func NewAuditStore(db *badger.DB, bufferSize int) *AuditStore {
    if bufferSize <= 0 {
        bufferSize = 1000
    }

    store := &AuditStore{
        db:      db,
        buffer:  make(chan *storage.AuditEvent, bufferSize),
        bufSize: bufferSize,
    }

    // Start async writer
    go store.writer()

    return store
}

// Key format: audit:{timestamp_ns}:{lockID}
func (s *AuditStore) auditKey(event *storage.AuditEvent) []byte {
    ts := event.Timestamp.UnixNano()
    return []byte(fmt.Sprintf("audit:%020d:%s", ts, event.LockID))
}

func (s *AuditStore) LogAsync(event *storage.AuditEvent) error {
    select {
    case s.buffer <- event:
        return nil
    default:
        return fmt.Errorf("audit buffer full")
    }
}

func (s *AuditStore) writer() {
    batch := make([]*storage.AuditEvent, 0, 100)
    ticker := time.NewTicker(1 * time.Second)
    defer ticker.Stop()

    flush := func() {
        if len(batch) == 0 {
            return
        }

        wb := s.db.NewWriteBatch()
        for _, event := range batch {
            data, _ := json.Marshal(event)
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
        }
    }
}

func (s *AuditStore) Query(filter storage.AuditFilter) ([]*storage.AuditEvent, error) {
    var events []*storage.AuditEvent

    startKey := []byte(fmt.Sprintf("audit:%020d", filter.StartTime.UnixNano()))
    endKey := []byte(fmt.Sprintf("audit:%020d", filter.EndTime.UnixNano()))

    err := s.db.View(func(txn *badger.Txn) error {
        opts := badger.DefaultIteratorOptions
        it := txn.NewIterator(opts)
        defer it.Close()

        count := 0
        for it.Seek(startKey); it.Valid(); it.Next() {
            if filter.Limit > 0 && count >= filter.Limit {
                break
            }

            item := it.Item()
            if bytes.Compare(item.Key(), endKey) > 0 {
                break
            }

            err := item.Value(func(val []byte) error {
                var event storage.AuditEvent
                if err := json.Unmarshal(val, &event); err != nil {
                    return err
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

func (s *AuditStore) Close() error {
    close(s.buffer)
    return nil
}
```

#### Step 3.2: Integrate with Lock Service
- **File**: `/Users/limjihoon/dev/agent-collab/internal/domain/lock/service.go`
- **Action**: Add async audit logging
```go
type LockService struct {
    // ... existing fields ...
    auditStore storage.AuditStore // optional
}

func (s *LockService) SetAuditStore(store storage.AuditStore) {
    s.auditStore = store
}

// Modify AcquireLock to log async:
func (s *LockService) AcquireLock(ctx context.Context, req *AcquireLockRequest) (*LockResult, error) {
    // ... existing logic ...

    // Async audit log (non-blocking)
    if s.auditStore != nil {
        go s.auditStore.LogAsync(&storage.AuditEvent{
            Timestamp:  time.Now(),
            Action:     "acquired",
            LockID:     lock.ID,
            HolderID:   lock.HolderID,
            HolderName: lock.HolderName,
            Target:     lock.Target.String(),
        })
    }

    return result, nil
}
```

---

### Phase 4: Migration & Rollback (Week 4)

#### Step 4.1: Implement Migration Procedure
- **File**: `/Users/limjihoon/dev/agent-collab/internal/infrastructure/storage/migration.go` (new)
- **Action**: Safe migration with rollback capability
```go
package storage

import (
    "fmt"
    "os"
    "path/filepath"
    "time"
)

type MigrationManager struct {
    dataDir string
}

func NewMigrationManager(dataDir string) *MigrationManager {
    return &MigrationManager{dataDir: dataDir}
}

// Migrate safely migrates from JSON to BadgerDB
func (m *MigrationManager) Migrate() error {
    // 1. Create backup
    if err := m.createBackup(); err != nil {
        return fmt.Errorf("backup failed: %w", err)
    }

    // 2. Test BadgerDB creation
    if err := m.testBadgerDB(); err != nil {
        return fmt.Errorf("badger test failed: %w", err)
    }

    // 3. Mark migration started
    if err := m.markMigration("started"); err != nil {
        return err
    }

    // No actual data migration needed - we start fresh
    // Old data remains in JSON for manual recovery if needed

    // 4. Mark migration completed
    return m.markMigration("completed")
}

func (m *MigrationManager) createBackup() error {
    backupDir := filepath.Join(m.dataDir, "backup_"+time.Now().Format("20060102_150405"))

    // Copy vectors/ and metrics/ directories
    for _, dir := range []string{"vectors", "metrics"} {
        src := filepath.Join(m.dataDir, dir)
        dst := filepath.Join(backupDir, dir)

        if err := os.MkdirAll(dst, 0755); err != nil {
            return err
        }

        // Copy files (not shown - use filepath.Walk)
    }

    return nil
}

func (m *MigrationManager) testBadgerDB() error {
    // Create test BadgerDB instance
    testPath := filepath.Join(m.dataDir, "badger_test")
    defer os.RemoveAll(testPath)

    db, err := badger.Open(badger.DefaultOptions(testPath))
    if err != nil {
        return err
    }
    defer db.Close()

    // Test write
    return db.Update(func(txn *badger.Txn) error {
        return txn.Set([]byte("test"), []byte("value"))
    })
}

func (m *MigrationManager) markMigration(status string) error {
    markerPath := filepath.Join(m.dataDir, ".migration_"+status)
    return os.WriteFile(markerPath, []byte(time.Now().String()), 0644)
}

func (m *MigrationManager) Rollback() error {
    // Remove BadgerDB directories
    badgerPath := filepath.Join(m.dataDir, "badger")
    if err := os.RemoveAll(badgerPath); err != nil {
        return err
    }

    // Mark rollback
    return m.markMigration("rollback")
}
```

#### Step 4.2: Add CLI Migration Commands
- **File**: `/Users/limjihoon/dev/agent-collab/internal/interface/cli/migrate.go` (new)
- **Action**: User-facing migration commands
```go
package cli

import (
    "fmt"
    "github.com/spf13/cobra"
    "agent-collab/internal/infrastructure/storage"
)

func newMigrateCmd() *cobra.Command {
    cmd := &cobra.Command{
        Use:   "migrate",
        Short: "Manage storage migrations",
    }

    cmd.AddCommand(&cobra.Command{
        Use:   "start",
        Short: "Start BadgerDB migration",
        RunE: func(cmd *cobra.Command, args []string) error {
            mgr := storage.NewMigrationManager(dataDir)
            fmt.Println("Starting migration...")
            if err := mgr.Migrate(); err != nil {
                return err
            }
            fmt.Println("Migration completed successfully")
            return nil
        },
    })

    cmd.AddCommand(&cobra.Command{
        Use:   "rollback",
        Short: "Rollback to JSON storage",
        RunE: func(cmd *cobra.Command, args []string) error {
            mgr := storage.NewMigrationManager(dataDir)
            fmt.Println("Rolling back migration...")
            return mgr.Rollback()
        },
    })

    cmd.AddCommand(&cobra.Command{
        Use:   "status",
        Short: "Check migration status",
        RunE: func(cmd *cobra.Command, args []string) error {
            // Check for migration markers
            return nil
        },
    })

    return cmd
}
```

---

## Testing Requirements

### Unit Tests

#### Test 1: BadgerDB Manager
- **File**: `/Users/limjihoon/dev/agent-collab/internal/infrastructure/storage/badger/manager_test.go`
```go
func TestManager_MultipleInstances(t *testing.T) {
    // Test opening multiple named instances
    // Test that instances are isolated
    // Test CloseAll
}

func TestManager_GarbageCollection(t *testing.T) {
    // Test RunGC on instances
    // Verify disk space reclaimed
}
```

#### Test 2: Delta Store Key Schema
- **File**: `/Users/limjihoon/dev/agent-collab/internal/infrastructure/storage/badger/delta_store_test.go`
```go
func TestDeltaStore_KeyOrdering(t *testing.T) {
    // Insert deltas from multiple sources
    // Verify keys are properly sharded
    // Test range queries return correct order
}

func TestDeltaStore_Concurrent(t *testing.T) {
    // Multiple goroutines writing deltas
    // Verify no data loss
    // Verify key uniqueness
}

func TestDeltaStore_Compaction(t *testing.T) {
    // Insert old deltas
    // Run Compact()
    // Verify old data removed
}
```

#### Test 3: Audit Store
- **File**: `/Users/limjihoon/dev/agent-collab/internal/infrastructure/storage/badger/audit_store_test.go`
```go
func TestAuditStore_AsyncWrite(t *testing.T) {
    // Log many events
    // Verify all written (eventually)
    // Test buffer overflow handling
}

func TestAuditStore_Query(t *testing.T) {
    // Insert varied audit events
    // Test time range queries
    // Test filtering by holder, action
}
```

### Integration Tests

#### Test 4: SyncManager with Persistence
- **File**: `/Users/limjihoon/dev/agent-collab/internal/domain/ctxsync/sync_persistence_test.go`
```go
func TestSyncManager_DeltaPersistence(t *testing.T) {
    // Create SyncManager with DeltaStore
    // Broadcast deltas
    // Restart SyncManager
    // Verify deltas restored from BadgerDB
}
```

### Crash Safety Tests

#### Test 5: Crash Recovery
- **File**: `/Users/limjihoon/dev/agent-collab/tests/crash/recovery_test.go`
```go
func TestDeltaStore_CrashDuringWrite(t *testing.T) {
    // Start writing deltas
    // Simulate crash (kill process)
    // Reopen BadgerDB
    // Verify no corruption
    // Verify committed data intact
}

func TestAuditStore_CrashDuringBatch(t *testing.T) {
    // Similar test for audit log
}
```

### Performance Regression Tests

#### Test 6: Lock Latency Benchmark
- **File**: `/Users/limjihoon/dev/agent-collab/tests/benchmark/lock_bench_test.go`
```go
func BenchmarkLockStore_InMemory(b *testing.B) {
    // Measure acquire/release latency
    // Should be < 1ms
}

func BenchmarkLockStore_WithAudit(b *testing.B) {
    // Measure with async audit
    // Should be < 2ms (negligible impact)
}
```

#### Test 7: Delta Write Throughput
- **File**: `/Users/limjihoon/dev/agent-collab/tests/benchmark/delta_bench_test.go`
```go
func BenchmarkDeltaStore_Write(b *testing.B) {
    // Measure write throughput
    // Should handle >1000 deltas/sec
}

func BenchmarkDeltaStore_RangeQuery(b *testing.B) {
    // Measure range query performance
    // Should be <50ms for 1000 deltas
}
```

---

## Phase Timeline with Go/No-Go Criteria

### Phase 1: Foundation (Week 1)
**Deliverables**:
- BadgerDB manager implemented
- Storage interfaces defined
- Unit tests passing

**Go Criteria**:
- [ ] All unit tests pass
- [ ] Manager can open/close multiple instances
- [ ] No memory leaks in tests
- [ ] GC runs without errors

**No-Go**: Skip BadgerDB entirely, keep current storage

---

### Phase 2: Delta Store (Week 2)
**Deliverables**:
- DeltaStore implemented
- Integrated with SyncManager
- Key schema validated
- Benchmark tests

**Go Criteria**:
- [ ] Write throughput >1000 deltas/sec
- [ ] Range queries <50ms for 1000 deltas
- [ ] Crash recovery tests pass
- [ ] No data loss in concurrent tests
- [ ] Storage overhead <2x JSON size

**No-Go**: Keep deltas in-memory only, add warning log

---

### Phase 3: Lock Audit (Week 3)
**Deliverables**:
- AuditStore implemented
- Async logging working
- Query interface functional

**Go Criteria**:
- [ ] Lock latency <2ms with audit
- [ ] Audit buffer handles 10k events/sec
- [ ] Queries complete in <100ms
- [ ] No audit buffer overflows in stress test

**No-Go**: Disable audit logging, use simple log file instead

---

### Phase 4: Migration & Rollback (Week 4)
**Deliverables**:
- Migration scripts
- Rollback tested
- CLI commands
- User documentation

**Go Criteria**:
- [ ] Backup completes successfully
- [ ] Migration is idempotent
- [ ] Rollback restores to working state
- [ ] Zero downtime for existing users

**No-Go**: Delay rollout, make BadgerDB opt-in via config flag

---

## Success Metrics

### Performance Metrics
- **Lock Latency**: <2ms (baseline: <1ms) - Max 2x regression
- **Delta Write**: >1000/sec - 10x improvement over JSON
- **Delta Query**: <50ms for 1000 deltas - 5x improvement
- **Storage Overhead**: <2x JSON size - LSM compression benefit
- **Startup Time**: <500ms to load 7 days of deltas

### Reliability Metrics
- **Crash Recovery**: 100% data integrity after unclean shutdown
- **Audit Coverage**: >99% of lock events logged
- **Zero Data Loss**: All committed writes survive crashes
- **Uptime**: No BadgerDB-related crashes in 30-day test

### Operational Metrics
- **Disk Usage**: <100MB for 7 days of deltas (typical workload)
- **GC Efficiency**: >50% space reclaimed on compaction
- **Migration Success**: >95% of users migrate without issues
- **Rollback Time**: <1 minute to revert to JSON storage

---

## Risk Assessment & Mitigation

### Risk 1: BadgerDB Disk Space Growth
- **Impact**: High - could fill disk
- **Mitigation**:
  - Daily compaction scheduled
  - Alert at 80% disk usage
  - Automatic purge of deltas >30 days old
- **Rollback**: Disable BadgerDB, clear directory

### Risk 2: LSM Compaction Pauses
- **Impact**: Medium - brief write stalls
- **Mitigation**:
  - Configure L0 compaction triggers
  - Use background compaction
  - Monitor compaction metrics
- **Rollback**: Tune BadgerDB options

### Risk 3: Migration Corruption
- **Impact**: High - data loss
- **Mitigation**:
  - Mandatory backup before migration
  - Test migration in staging first
  - Keep JSON files as backup
  - Rollback script tested
- **Rollback**: Delete BadgerDB, restore from backup

### Risk 4: Lock Latency Regression
- **Impact**: Critical - system unusable
- **Mitigation**:
  - Keep locks in-memory (no regression)
  - Async audit only (non-blocking)
  - Benchmark before/after
- **Rollback**: Disable audit store

### Risk 5: Concurrent Write Conflicts
- **Impact**: Medium - lost updates
- **Mitigation**:
  - Use write batches for bulk operations
  - Retry on conflict errors
  - Monitor conflict rate
- **Rollback**: Increase transaction timeout

---

## Rollback Procedure

### Immediate Rollback (During Development)
```bash
# Stop the application
agent-collab daemon stop

# Run rollback
agent-collab migrate rollback

# Restart with JSON storage
agent-collab daemon start
```

### Production Rollback (Post-Deployment)
```bash
# 1. Create emergency backup
cp -r ~/.agent-collab ~/.agent-collab.emergency

# 2. Stop daemon
agent-collab daemon stop

# 3. Delete BadgerDB
rm -rf ~/.agent-collab/badger

# 4. Restore JSON backups (if needed)
cp -r ~/.agent-collab/backup_LATEST/* ~/.agent-collab/

# 5. Restart
agent-collab daemon start
```

**Validation**:
- Check locks work (acquire/release)
- Check deltas broadcast
- Check metrics readable
- No errors in logs for 5 minutes

---

## Future Enhancements (Out of Scope)

These are NOT included in this plan but could be considered later:

1. **Distributed Transactions**: CRDT already handles eventual consistency
2. **Multi-Cluster Replication**: Use libp2p gossip instead
3. **Vector DB Migration**: Dedicated vector DB (e.g., Weaviate) is better choice
4. **Metrics in BadgerDB**: JSONL is optimal for time-series append
5. **Lock Persistence**: Locks are ephemeral by design (TTL-based)

---

## Documentation Updates

### User Documentation
- **File**: `/Users/limjihoon/dev/agent-collab/README.md`
- **Sections to Add**:
  - Storage architecture overview
  - Migration instructions
  - Disk space requirements
  - Backup recommendations

### Developer Documentation
- **File**: `/Users/limjihoon/dev/agent-collab/docs/STORAGE.md` (new)
- **Content**:
  - BadgerDB key schemas
  - Performance tuning guide
  - Debugging BadgerDB issues
  - Compaction strategy

### API Documentation
- **File**: `/Users/limjihoon/dev/agent-collab/docs/API.md` (new)
- **Content**:
  - Storage interface contracts
  - Error handling guidelines
  - Migration API reference

---

## Summary of Changes vs Original Plan

| Aspect | Original Plan | Revised Plan | Rationale |
|--------|--------------|--------------|-----------|
| **Lock Store** | BadgerDB with dual cache | In-memory only + async audit | Avoid O(1)→O(log n) regression |
| **Delta Store** | Sequential keys | Sharded by sourceID | Fix LSM anti-pattern |
| **Metrics** | Migrate to BadgerDB | Keep JSONL | Already optimal for time-series |
| **Vector Store** | BadgerDB | Keep JSON | BadgerDB lacks vector search |
| **Architecture** | Single DB instance | Multiple isolated instances | Avoid namespace collision |
| **Abstraction** | None | Storage interfaces | Enable testing and future swaps |
| **Migration** | Immediate | Phased with rollback | Reduce risk |
| **Testing** | Basic | Comprehensive (crash, perf, stress) | Ensure reliability |

---

## Approval Checklist

Before proceeding with implementation:

- [ ] Technical lead approves revised architecture
- [ ] Performance benchmarks meet criteria
- [ ] Security review completed
- [ ] Backup/rollback procedures tested
- [ ] User migration path documented
- [ ] Go/no-go criteria agreed upon
- [ ] Resource allocation confirmed (4 weeks)

---

## Conclusion

This revised plan takes a **pragmatic, incremental approach**:

1. **What to persist**: Only deltas and audit logs (both benefit from BadgerDB)
2. **What to keep**: Locks (in-memory), metrics (JSONL), vectors (JSON)
3. **How to migrate**: Safely with backups and rollback
4. **How to test**: Comprehensive crash, performance, and stress tests
5. **How to deploy**: Phased with clear go/no-go criteria

**Key Principle**: Use the right tool for each job. BadgerDB is excellent for append-heavy workloads with range queries (deltas, audit logs) but wrong for:
- Ephemeral data (locks)
- Time-series append (metrics)
- Vector similarity search (embeddings)

This plan addresses all critical issues from the security review while maintaining system performance and reliability.
