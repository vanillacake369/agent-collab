package controller

import (
	"context"
	"fmt"
	"runtime/debug"
	"sync"
	"time"

	"agent-collab/src/plugin"
	"agent-collab/src/store"
)

// Manager manages a set of controllers and their lifecycle.
type Manager struct {
	controllers []Controller
	network     plugin.NetworkPlugin
	stores      map[string]any

	mu      sync.RWMutex
	started bool
	errCh   chan error
}

// NewManager creates a new controller manager.
func NewManager() *Manager {
	return &Manager{
		controllers: make([]Controller, 0),
		stores:      make(map[string]any),
		errCh:       make(chan error, 10),
	}
}

// SetNetwork sets the network plugin for controllers.
func (m *Manager) SetNetwork(n plugin.NetworkPlugin) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.network = n
}

// GetNetwork returns the network plugin.
func (m *Manager) GetNetwork() plugin.NetworkPlugin {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.network
}

// AddStore registers a store with the manager.
func (m *Manager) AddStore(name string, s any) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.stores[name] = s
}

// GetStore retrieves a store by name.
func (m *Manager) GetStore(name string) any {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.stores[name]
}

// AddController adds a controller to the manager.
func (m *Manager) AddController(ctrl Controller) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.started {
		panic("cannot add controller after manager started")
	}
	m.controllers = append(m.controllers, ctrl)
}

// Start starts all controllers.
func (m *Manager) Start(ctx context.Context) error {
	m.mu.Lock()
	if m.started {
		m.mu.Unlock()
		return fmt.Errorf("manager already started")
	}
	m.started = true
	controllers := m.controllers
	m.mu.Unlock()

	var wg sync.WaitGroup
	for _, ctrl := range controllers {
		wg.Add(1)
		go func(c Controller) {
			defer wg.Done()
			if err := c.Start(ctx); err != nil {
				select {
				case m.errCh <- err:
				default:
				}
			}
		}(ctrl)
	}

	// Wait for context cancellation
	<-ctx.Done()

	// Wait for all controllers to stop
	wg.Wait()

	return ctx.Err()
}

// Errors returns a channel that receives controller errors.
func (m *Manager) Errors() <-chan error {
	return m.errCh
}

// GenericController is a generic controller implementation.
type GenericController[T store.Object] struct {
	name       string
	reconciler Reconciler
	store      store.ResourceStore[T]
	workqueue  *WorkQueue
	options    Options
	network    plugin.NetworkPlugin

	mu      sync.Mutex
	started bool
}

// NewGenericController creates a new generic controller.
func NewGenericController[T store.Object](
	name string,
	reconciler Reconciler,
	resourceStore store.ResourceStore[T],
	opts Options,
) *GenericController[T] {
	if opts.RateLimiter == nil {
		opts.RateLimiter = NewDefaultRateLimiter()
	}
	return &GenericController[T]{
		name:       name,
		reconciler: reconciler,
		store:      resourceStore,
		workqueue:  NewWorkQueue(opts.RateLimiter),
		options:    opts,
	}
}

// SetNetwork sets the network plugin for the controller.
func (c *GenericController[T]) SetNetwork(n plugin.NetworkPlugin) {
	c.network = n
}

// Start begins the reconciliation loop.
func (c *GenericController[T]) Start(ctx context.Context) error {
	c.mu.Lock()
	if c.started {
		c.mu.Unlock()
		return fmt.Errorf("controller %s already started", c.name)
	}
	c.started = true
	c.mu.Unlock()

	// Start watching the store
	watcher, err := c.store.Watch(ctx, store.WatchOptions{
		SendInitialEvents: true,
	})
	if err != nil {
		return fmt.Errorf("failed to watch store: %w", err)
	}

	// Process watch events
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case event, ok := <-watcher.ResultChan():
				if !ok {
					return
				}
				name := event.Object.GetObjectMeta().Name
				c.Enqueue(name)
			}
		}
	}()

	// Start workers
	var wg sync.WaitGroup
	for range c.options.MaxConcurrentReconciles {
		wg.Add(1)
		go func() {
			defer wg.Done()
			c.worker(ctx)
		}()
	}

	// Wait for context cancellation
	<-ctx.Done()
	c.workqueue.ShutDown()
	wg.Wait()

	return nil
}

// Enqueue adds a resource to the work queue.
func (c *GenericController[T]) Enqueue(name string) {
	c.workqueue.Add(name)
}

// EnqueueAfter adds a resource to the work queue after a delay.
func (c *GenericController[T]) EnqueueAfter(name string, delay time.Duration) {
	c.workqueue.AddAfter(name, delay)
}

func (c *GenericController[T]) worker(ctx context.Context) {
	for c.processNextItem(ctx) {
	}
}

func (c *GenericController[T]) processNextItem(ctx context.Context) bool {
	item, shutdown := c.workqueue.Get()
	if shutdown {
		return false
	}

	defer c.workqueue.Done(item)

	if c.options.RecoverPanic {
		defer func() {
			if r := recover(); r != nil {
				fmt.Printf("controller %s: panic during reconcile of %s: %v\n%s",
					c.name, item, r, debug.Stack())
				c.workqueue.AddRateLimited(item)
			}
		}()
	}

	result, err := c.reconciler.Reconcile(ctx, Request{Name: item})
	if err != nil {
		// Requeue with rate limiting on error
		c.workqueue.AddRateLimited(item)
		return true
	}

	// Forget the item (reset rate limiting)
	c.workqueue.Forget(item)

	// Requeue if requested
	if result.RequeueAfter > 0 {
		c.workqueue.AddAfter(item, result.RequeueAfter)
	} else if result.Requeue {
		c.workqueue.Add(item)
	}

	return true
}

// Verify GenericController implements Controller
var _ Controller = (*GenericController[store.Object])(nil)
