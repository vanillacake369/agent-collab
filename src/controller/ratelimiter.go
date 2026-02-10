package controller

import (
	"sync"
	"time"
)

// ExponentialRateLimiter implements exponential backoff rate limiting.
type ExponentialRateLimiter struct {
	mu       sync.RWMutex
	failures map[string]int
	baseDel  time.Duration
	maxDelay time.Duration
}

// NewDefaultRateLimiter creates a rate limiter with sensible defaults.
func NewDefaultRateLimiter() RateLimiter {
	return NewExponentialRateLimiter(5*time.Millisecond, 1000*time.Second)
}

// NewExponentialRateLimiter creates a new exponential rate limiter.
func NewExponentialRateLimiter(baseDelay, maxDelay time.Duration) *ExponentialRateLimiter {
	return &ExponentialRateLimiter{
		failures: make(map[string]int),
		baseDel:  baseDelay,
		maxDelay: maxDelay,
	}
}

// When returns the delay before an item should be requeued.
func (r *ExponentialRateLimiter) When(item string) time.Duration {
	r.mu.Lock()
	defer r.mu.Unlock()

	exp := r.failures[item]
	r.failures[item]++

	// Calculate exponential backoff
	backoff := r.baseDel * time.Duration(1<<exp)
	if backoff > r.maxDelay {
		backoff = r.maxDelay
	}

	return backoff
}

// Forget indicates that an item is finished being retried.
func (r *ExponentialRateLimiter) Forget(item string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.failures, item)
}

// NumRequeues returns the number of times an item has been requeued.
func (r *ExponentialRateLimiter) NumRequeues(item string) int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.failures[item]
}

// ItemExponentialRateLimiter allows per-item rate limiting with defaults.
type ItemExponentialRateLimiter struct {
	*ExponentialRateLimiter
}

// NewItemExponentialRateLimiter creates a new item-level rate limiter.
func NewItemExponentialRateLimiter(baseDelay, maxDelay time.Duration) *ItemExponentialRateLimiter {
	return &ItemExponentialRateLimiter{
		ExponentialRateLimiter: NewExponentialRateLimiter(baseDelay, maxDelay),
	}
}

// MaxOfRateLimiter returns the max delay from multiple rate limiters.
type MaxOfRateLimiter struct {
	limiters []RateLimiter
}

// NewMaxOfRateLimiter creates a rate limiter that uses the max of multiple limiters.
func NewMaxOfRateLimiter(limiters ...RateLimiter) *MaxOfRateLimiter {
	return &MaxOfRateLimiter{limiters: limiters}
}

// When returns the maximum delay from all limiters.
func (r *MaxOfRateLimiter) When(item string) time.Duration {
	var max time.Duration
	for _, limiter := range r.limiters {
		delay := limiter.When(item)
		if delay > max {
			max = delay
		}
	}
	return max
}

// Forget forgets the item from all limiters.
func (r *MaxOfRateLimiter) Forget(item string) {
	for _, limiter := range r.limiters {
		limiter.Forget(item)
	}
}

// NumRequeues returns the max requeues from all limiters.
func (r *MaxOfRateLimiter) NumRequeues(item string) int {
	var max int
	for _, limiter := range r.limiters {
		n := limiter.NumRequeues(item)
		if n > max {
			max = n
		}
	}
	return max
}
