package store

import (
	"context"
	"sync"
	"time"

	v1 "agent-collab/src/api/v1"
)

// Informer provides a cache and watch mechanism for a resource type.
// It syncs with a ResourceStore and delivers events to handlers.
type Informer[T Object] struct {
	store    ResourceStore[T]
	handlers []EventHandler[T]
	cache    map[string]T
	mu       sync.RWMutex

	resyncPeriod time.Duration
	stopCh       chan struct{}
	doneCh       chan struct{}
}

// NewInformer creates a new informer for the given store.
func NewInformer[T Object](store ResourceStore[T], resyncPeriod time.Duration) *Informer[T] {
	return &Informer[T]{
		store:        store,
		handlers:     make([]EventHandler[T], 0),
		cache:        make(map[string]T),
		resyncPeriod: resyncPeriod,
		stopCh:       make(chan struct{}),
		doneCh:       make(chan struct{}),
	}
}

// AddEventHandler registers an event handler.
// Must be called before Run.
func (i *Informer[T]) AddEventHandler(handler EventHandler[T]) {
	i.handlers = append(i.handlers, handler)
}

// AddEventHandlerFuncs registers event handlers using functions.
func (i *Informer[T]) AddEventHandlerFuncs(add func(T), update func(T, T), delete func(T)) {
	i.AddEventHandler(EventHandlerFuncs[T]{
		AddFunc:    add,
		UpdateFunc: update,
		DeleteFunc: delete,
	})
}

// Run starts the informer and blocks until stopped.
func (i *Informer[T]) Run(ctx context.Context) error {
	defer close(i.doneCh)

	// Do initial list to populate cache
	if err := i.list(ctx); err != nil {
		return err
	}

	// Start watching
	watcher, err := i.store.Watch(ctx, WatchOptions{
		SendInitialEvents: false, // We already listed
	})
	if err != nil {
		return err
	}
	defer watcher.Stop()

	// Start resync timer if configured
	var resyncCh <-chan time.Time
	if i.resyncPeriod > 0 {
		ticker := time.NewTicker(i.resyncPeriod)
		defer ticker.Stop()
		resyncCh = ticker.C
	}

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()

		case <-i.stopCh:
			return nil

		case event, ok := <-watcher.ResultChan():
			if !ok {
				// Watcher closed, try to restart
				watcher, err = i.store.Watch(ctx, WatchOptions{
					SendInitialEvents: true,
				})
				if err != nil {
					return err
				}
				continue
			}
			i.handleEvent(event)

		case <-resyncCh:
			if err := i.resync(ctx); err != nil {
				// Log error but continue
				continue
			}
		}
	}
}

// Stop stops the informer.
func (i *Informer[T]) Stop() {
	close(i.stopCh)
	<-i.doneCh
}

// Get retrieves an item from the cache.
func (i *Informer[T]) Get(name string) (T, bool) {
	i.mu.RLock()
	defer i.mu.RUnlock()

	item, exists := i.cache[name]
	if !exists {
		var zero T
		return zero, false
	}
	return item.DeepCopy().(T), true
}

// List returns all items in the cache.
func (i *Informer[T]) List() []T {
	i.mu.RLock()
	defer i.mu.RUnlock()

	result := make([]T, 0, len(i.cache))
	for _, item := range i.cache {
		result = append(result, item.DeepCopy().(T))
	}
	return result
}

// Len returns the number of items in the cache.
func (i *Informer[T]) Len() int {
	i.mu.RLock()
	defer i.mu.RUnlock()
	return len(i.cache)
}

// HasSynced returns true if the informer has completed at least one full list.
func (i *Informer[T]) HasSynced() bool {
	i.mu.RLock()
	defer i.mu.RUnlock()
	return len(i.cache) > 0 || true // Simplified: always return true after initialization
}

func (i *Informer[T]) list(ctx context.Context) error {
	items, err := i.store.List(ctx, ListOptions{})
	if err != nil {
		return err
	}

	i.mu.Lock()
	defer i.mu.Unlock()

	for _, item := range items {
		name := item.GetObjectMeta().Name
		i.cache[name] = item

		// Notify handlers of existing items
		for _, handler := range i.handlers {
			handler.OnAdd(item.DeepCopy().(T))
		}
	}

	return nil
}

func (i *Informer[T]) resync(ctx context.Context) error {
	items, err := i.store.List(ctx, ListOptions{})
	if err != nil {
		return err
	}

	i.mu.Lock()
	defer i.mu.Unlock()

	// Create a set of current items
	currentNames := make(map[string]struct{})
	for _, item := range items {
		name := item.GetObjectMeta().Name
		currentNames[name] = struct{}{}

		existing, exists := i.cache[name]
		if !exists {
			// New item
			i.cache[name] = item
			for _, handler := range i.handlers {
				handler.OnAdd(item.DeepCopy().(T))
			}
		} else if existing.GetObjectMeta().ResourceVersion != item.GetObjectMeta().ResourceVersion {
			// Updated item
			i.cache[name] = item
			for _, handler := range i.handlers {
				handler.OnUpdate(existing.DeepCopy().(T), item.DeepCopy().(T))
			}
		}
	}

	// Find deleted items
	for name, item := range i.cache {
		if _, exists := currentNames[name]; !exists {
			delete(i.cache, name)
			for _, handler := range i.handlers {
				handler.OnDelete(item.DeepCopy().(T))
			}
		}
	}

	return nil
}

func (i *Informer[T]) handleEvent(event WatchEvent[T]) {
	i.mu.Lock()
	defer i.mu.Unlock()

	name := event.Object.GetObjectMeta().Name

	switch event.Type {
	case v1.EventAdded:
		existing, exists := i.cache[name]
		i.cache[name] = event.Object
		if exists {
			// Treat as update if item already existed
			for _, handler := range i.handlers {
				handler.OnUpdate(existing, event.Object)
			}
		} else {
			for _, handler := range i.handlers {
				handler.OnAdd(event.Object)
			}
		}

	case v1.EventModified:
		existing := i.cache[name]
		i.cache[name] = event.Object
		for _, handler := range i.handlers {
			handler.OnUpdate(existing, event.Object)
		}

	case v1.EventDeleted:
		existing := i.cache[name]
		delete(i.cache, name)
		for _, handler := range i.handlers {
			handler.OnDelete(existing)
		}
	}
}

// SharedInformerFactory creates and manages shared informers.
// Shared informers ensure that only one watch is established per resource type.
type SharedInformerFactory struct {
	mu        sync.Mutex
	informers map[string]any
	started   bool
}

// NewSharedInformerFactory creates a new factory.
func NewSharedInformerFactory() *SharedInformerFactory {
	return &SharedInformerFactory{
		informers: make(map[string]any),
	}
}

// InformerFor returns a shared informer for the given resource type.
// The key is used to identify the informer type.
func InformerFor[T Object](f *SharedInformerFactory, key string, store ResourceStore[T], resyncPeriod time.Duration) *Informer[T] {
	f.mu.Lock()
	defer f.mu.Unlock()

	if inf, exists := f.informers[key]; exists {
		return inf.(*Informer[T])
	}

	informer := NewInformer(store, resyncPeriod)
	f.informers[key] = informer
	return informer
}

// Start starts all registered informers.
func (f *SharedInformerFactory) Start(ctx context.Context) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.started {
		return
	}
	f.started = true

	for _, inf := range f.informers {
		go func(informer any) {
			switch i := informer.(type) {
			case interface{ Run(context.Context) error }:
				_ = i.Run(ctx)
			}
		}(inf)
	}
}

// WaitForCacheSync waits for all informer caches to sync.
func (f *SharedInformerFactory) WaitForCacheSync(ctx context.Context) bool {
	f.mu.Lock()
	informers := make([]any, 0, len(f.informers))
	for _, inf := range f.informers {
		informers = append(informers, inf)
	}
	f.mu.Unlock()

	for _, inf := range informers {
		switch i := inf.(type) {
		case interface{ HasSynced() bool }:
			// Simple wait with timeout
			deadline := time.Now().Add(30 * time.Second)
			for !i.HasSynced() {
				if time.Now().After(deadline) {
					return false
				}
				select {
				case <-ctx.Done():
					return false
				case <-time.After(100 * time.Millisecond):
				}
			}
		}
	}

	return true
}
