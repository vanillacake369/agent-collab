// Package memory provides an in-memory implementation of the ResourceStore.
package memory

import (
	"context"
	"fmt"
	"strconv"
	"sync"
	"sync/atomic"

	v1 "agent-collab/src/api/v1"
	"agent-collab/src/store"
)

// Store is a thread-safe in-memory resource store with watch support.
type Store[T store.Object] struct {
	mu sync.RWMutex

	// items maps resource names to resources
	items map[string]T

	// resourceVersion is the current version, incremented on each mutation
	resourceVersion uint64

	// watchers is the set of active watchers
	watchers  map[*watcher[T]]struct{}
	watcherMu sync.Mutex

	// indexes for fast lookups
	indexes   map[string]store.IndexKeyFunc[T]
	indexData map[string]map[string][]string // indexName -> indexValue -> []resourceName

	// closed indicates if the store has been closed
	closed atomic.Bool
}

// New creates a new in-memory store.
func New[T store.Object]() *Store[T] {
	return &Store[T]{
		items:     make(map[string]T),
		watchers:  make(map[*watcher[T]]struct{}),
		indexes:   make(map[string]store.IndexKeyFunc[T]),
		indexData: make(map[string]map[string][]string),
	}
}

// Create stores a new resource.
func (s *Store[T]) Create(ctx context.Context, obj T) error {
	if s.closed.Load() {
		return store.ErrStoreClosed
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	name := obj.GetObjectMeta().Name
	if _, exists := s.items[name]; exists {
		return store.ErrAlreadyExists
	}

	// Set resource version
	s.resourceVersion++
	obj.GetObjectMeta().ResourceVersion = strconv.FormatUint(s.resourceVersion, 10)

	// Store a deep copy
	copy := obj.DeepCopy().(T)
	s.items[name] = copy

	// Update indexes
	s.updateIndexes(name, copy)

	// Notify watchers
	s.notifyWatchers(store.WatchEvent[T]{
		Type:   v1.EventAdded,
		Object: copy.DeepCopy().(T),
	})

	return nil
}

// Update modifies an existing resource.
func (s *Store[T]) Update(ctx context.Context, obj T) error {
	if s.closed.Load() {
		return store.ErrStoreClosed
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	name := obj.GetObjectMeta().Name
	existing, exists := s.items[name]
	if !exists {
		return store.ErrNotFound
	}

	// Check for version conflict if specified
	incomingVersion := obj.GetObjectMeta().ResourceVersion
	if incomingVersion != "" && incomingVersion != existing.GetObjectMeta().ResourceVersion {
		return store.ErrConflict
	}

	// Remove old index entries
	s.removeFromIndexes(name, existing)

	// Set new resource version
	s.resourceVersion++
	obj.GetObjectMeta().ResourceVersion = strconv.FormatUint(s.resourceVersion, 10)

	// Store a deep copy
	copy := obj.DeepCopy().(T)
	s.items[name] = copy

	// Update indexes
	s.updateIndexes(name, copy)

	// Notify watchers
	s.notifyWatchers(store.WatchEvent[T]{
		Type:   v1.EventModified,
		Object: copy.DeepCopy().(T),
	})

	return nil
}

// Delete removes a resource by name.
func (s *Store[T]) Delete(ctx context.Context, name string) error {
	if s.closed.Load() {
		return store.ErrStoreClosed
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	existing, exists := s.items[name]
	if !exists {
		return store.ErrNotFound
	}

	// Remove from indexes
	s.removeFromIndexes(name, existing)

	// Delete the item
	delete(s.items, name)

	// Increment version even on delete
	s.resourceVersion++

	// Notify watchers
	s.notifyWatchers(store.WatchEvent[T]{
		Type:   v1.EventDeleted,
		Object: existing.DeepCopy().(T),
	})

	return nil
}

// Get retrieves a resource by name.
func (s *Store[T]) Get(ctx context.Context, name string) (T, error) {
	if s.closed.Load() {
		var zero T
		return zero, store.ErrStoreClosed
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	item, exists := s.items[name]
	if !exists {
		var zero T
		return zero, store.ErrNotFound
	}

	return item.DeepCopy().(T), nil
}

// List returns all resources matching the options.
func (s *Store[T]) List(ctx context.Context, opts store.ListOptions) ([]T, error) {
	if s.closed.Load() {
		return nil, store.ErrStoreClosed
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]T, 0, len(s.items))
	for _, item := range s.items {
		if s.matchesSelector(item, opts.LabelSelector) {
			result = append(result, item.DeepCopy().(T))
		}
		if opts.Limit > 0 && len(result) >= opts.Limit {
			break
		}
	}

	return result, nil
}

// Watch returns a watcher that delivers events for matching resources.
func (s *Store[T]) Watch(ctx context.Context, opts store.WatchOptions) (store.Watcher[T], error) {
	if s.closed.Load() {
		return nil, store.ErrStoreClosed
	}

	w := &watcher[T]{
		ch:       make(chan store.WatchEvent[T], 100),
		store:    s,
		selector: opts.LabelSelector,
	}

	s.watcherMu.Lock()
	s.watchers[w] = struct{}{}
	s.watcherMu.Unlock()

	// Send initial events if requested
	if opts.SendInitialEvents {
		s.mu.RLock()
		for _, item := range s.items {
			if s.matchesSelector(item, opts.LabelSelector) {
				select {
				case w.ch <- store.WatchEvent[T]{
					Type:   v1.EventAdded,
					Object: item.DeepCopy().(T),
				}:
				case <-ctx.Done():
					s.mu.RUnlock()
					w.Stop()
					return nil, ctx.Err()
				}
			}
		}
		s.mu.RUnlock()
	}

	// Handle context cancellation
	go func() {
		<-ctx.Done()
		w.Stop()
	}()

	return w, nil
}

// Close shuts down the store.
func (s *Store[T]) Close() error {
	if s.closed.Swap(true) {
		return nil // Already closed
	}

	s.watcherMu.Lock()
	for w := range s.watchers {
		close(w.ch)
		delete(s.watchers, w)
	}
	s.watcherMu.Unlock()

	return nil
}

// AddIndexer adds a new index.
func (s *Store[T]) AddIndexer(name string, keyFunc store.IndexKeyFunc[T]) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.indexes[name]; exists {
		return fmt.Errorf("index %q already exists", name)
	}

	s.indexes[name] = keyFunc
	s.indexData[name] = make(map[string][]string)

	// Index existing items
	for resourceName, item := range s.items {
		keys := keyFunc(item)
		for _, key := range keys {
			s.indexData[name][key] = append(s.indexData[name][key], resourceName)
		}
	}

	return nil
}

// ByIndex returns all objects that match the given index value.
func (s *Store[T]) ByIndex(indexName, indexValue string) ([]T, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	indexValues, exists := s.indexData[indexName]
	if !exists {
		return nil, fmt.Errorf("index %q not found", indexName)
	}

	names := indexValues[indexValue]
	result := make([]T, 0, len(names))
	for _, name := range names {
		if item, exists := s.items[name]; exists {
			result = append(result, item.DeepCopy().(T))
		}
	}

	return result, nil
}

// IndexKeys returns all index keys for the given index name.
func (s *Store[T]) IndexKeys(indexName string) ([]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	indexValues, exists := s.indexData[indexName]
	if !exists {
		return nil, fmt.Errorf("index %q not found", indexName)
	}

	keys := make([]string, 0, len(indexValues))
	for key := range indexValues {
		keys = append(keys, key)
	}

	return keys, nil
}

// GetResourceVersion returns the current resource version.
func (s *Store[T]) GetResourceVersion() string {
	return strconv.FormatUint(atomic.LoadUint64(&s.resourceVersion), 10)
}

// Len returns the number of items in the store.
func (s *Store[T]) Len() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.items)
}

// Helper methods

func (s *Store[T]) matchesSelector(obj T, selector map[string]string) bool {
	if len(selector) == 0 {
		return true
	}

	labels := obj.GetObjectMeta().Labels
	for k, v := range selector {
		if labels[k] != v {
			return false
		}
	}
	return true
}

func (s *Store[T]) updateIndexes(name string, obj T) {
	for indexName, keyFunc := range s.indexes {
		keys := keyFunc(obj)
		for _, key := range keys {
			s.indexData[indexName][key] = append(s.indexData[indexName][key], name)
		}
	}
}

func (s *Store[T]) removeFromIndexes(name string, obj T) {
	for indexName, keyFunc := range s.indexes {
		keys := keyFunc(obj)
		for _, key := range keys {
			names := s.indexData[indexName][key]
			for i, n := range names {
				if n == name {
					s.indexData[indexName][key] = append(names[:i], names[i+1:]...)
					break
				}
			}
		}
	}
}

func (s *Store[T]) notifyWatchers(event store.WatchEvent[T]) {
	s.watcherMu.Lock()
	defer s.watcherMu.Unlock()

	for w := range s.watchers {
		if s.matchesSelector(event.Object, w.selector) {
			select {
			case w.ch <- event:
			default:
				// Channel full, skip event
				// In production, might want to close the watcher
			}
		}
	}
}

// watcher implements the Watcher interface.
type watcher[T store.Object] struct {
	ch       chan store.WatchEvent[T]
	store    *Store[T]
	selector map[string]string
	stopped  atomic.Bool
}

func (w *watcher[T]) ResultChan() <-chan store.WatchEvent[T] {
	return w.ch
}

func (w *watcher[T]) Stop() {
	if w.stopped.Swap(true) {
		return // Already stopped
	}

	w.store.watcherMu.Lock()
	delete(w.store.watchers, w)
	w.store.watcherMu.Unlock()

	// Close channel only if it hasn't been closed by store.Close()
	select {
	case <-w.ch:
		// Channel already closed
	default:
		close(w.ch)
	}
}
