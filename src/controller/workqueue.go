package controller

import (
	"sync"
	"time"
)

// WorkQueue is a rate-limited work queue for controller reconciliation.
type WorkQueue struct {
	mu          sync.Mutex
	cond        *sync.Cond
	items       map[string]struct{} // Set of items in queue
	queue       []string            // FIFO queue
	dirty       map[string]struct{} // Items that need processing
	processing  map[string]struct{} // Items currently being processed
	delayed     map[string]time.Time
	rateLimiter RateLimiter
	closed      bool
}

// NewWorkQueue creates a new work queue.
func NewWorkQueue(rateLimiter RateLimiter) *WorkQueue {
	q := &WorkQueue{
		items:       make(map[string]struct{}),
		queue:       make([]string, 0),
		dirty:       make(map[string]struct{}),
		processing:  make(map[string]struct{}),
		delayed:     make(map[string]time.Time),
		rateLimiter: rateLimiter,
	}
	q.cond = sync.NewCond(&q.mu)
	return q
}

// Add adds an item to the queue.
func (q *WorkQueue) Add(item string) {
	q.mu.Lock()
	defer q.mu.Unlock()

	if q.closed {
		return
	}

	q.dirty[item] = struct{}{}

	if _, exists := q.processing[item]; exists {
		// Already processing, will be requeued when done
		return
	}

	if _, exists := q.items[item]; exists {
		// Already in queue
		return
	}

	q.items[item] = struct{}{}
	q.queue = append(q.queue, item)
	q.cond.Signal()
}

// AddAfter adds an item to the queue after a delay.
func (q *WorkQueue) AddAfter(item string, delay time.Duration) {
	if delay <= 0 {
		q.Add(item)
		return
	}

	q.mu.Lock()
	if q.closed {
		q.mu.Unlock()
		return
	}

	when := time.Now().Add(delay)
	if existing, exists := q.delayed[item]; exists && existing.Before(when) {
		// Already scheduled for earlier
		q.mu.Unlock()
		return
	}
	q.delayed[item] = when
	q.mu.Unlock()

	// Start a timer to add the item
	time.AfterFunc(delay, func() {
		q.mu.Lock()
		defer q.mu.Unlock()

		if q.closed {
			return
		}

		scheduledTime, exists := q.delayed[item]
		if !exists || !scheduledTime.Equal(when) {
			// Timer was superseded
			return
		}
		delete(q.delayed, item)

		q.dirty[item] = struct{}{}
		if _, inProcessing := q.processing[item]; inProcessing {
			return
		}
		if _, inQueue := q.items[item]; inQueue {
			return
		}

		q.items[item] = struct{}{}
		q.queue = append(q.queue, item)
		q.cond.Signal()
	})
}

// AddRateLimited adds an item to the queue using the rate limiter.
func (q *WorkQueue) AddRateLimited(item string) {
	delay := q.rateLimiter.When(item)
	q.AddAfter(item, delay)
}

// Get blocks until an item is available or the queue is shut down.
func (q *WorkQueue) Get() (string, bool) {
	q.mu.Lock()
	defer q.mu.Unlock()

	for len(q.queue) == 0 && !q.closed {
		q.cond.Wait()
	}

	if q.closed && len(q.queue) == 0 {
		return "", true
	}

	item := q.queue[0]
	q.queue = q.queue[1:]
	delete(q.items, item)
	q.processing[item] = struct{}{}

	return item, false
}

// Done marks an item as done processing.
func (q *WorkQueue) Done(item string) {
	q.mu.Lock()
	defer q.mu.Unlock()

	delete(q.processing, item)

	if _, exists := q.dirty[item]; exists {
		delete(q.dirty, item)
		if _, inQueue := q.items[item]; !inQueue {
			q.items[item] = struct{}{}
			q.queue = append(q.queue, item)
			q.cond.Signal()
		}
	}
}

// Forget indicates that an item should not be retried.
func (q *WorkQueue) Forget(item string) {
	q.rateLimiter.Forget(item)
}

// NumRequeues returns the number of times an item has been requeued.
func (q *WorkQueue) NumRequeues(item string) int {
	return q.rateLimiter.NumRequeues(item)
}

// Len returns the number of items in the queue.
func (q *WorkQueue) Len() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return len(q.queue)
}

// ShutDown shuts down the queue.
func (q *WorkQueue) ShutDown() {
	q.mu.Lock()
	defer q.mu.Unlock()

	q.closed = true
	q.cond.Broadcast()
}

// ShutDownWithDrain shuts down and drains the queue.
func (q *WorkQueue) ShutDownWithDrain() {
	q.ShutDown()
	// Wait for processing to complete could be added here
}

// ShuttingDown returns true if the queue is shutting down.
func (q *WorkQueue) ShuttingDown() bool {
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.closed
}
