package lock

import (
	"sync"
	"time"
)

// RateLimiter implements a per-peer rate limiter using token bucket algorithm.
type RateLimiter struct {
	mu      sync.RWMutex
	buckets map[string]*tokenBucket
	rate    float64       // tokens per second
	burst   int           // maximum tokens
	cleanup time.Duration // cleanup interval for idle buckets
}

// tokenBucket represents a token bucket for a single peer.
type tokenBucket struct {
	tokens     float64
	lastUpdate time.Time
}

// RateLimitConfig configures the rate limiter.
type RateLimitConfig struct {
	Rate            float64       // requests per second per peer
	Burst           int           // maximum burst size
	CleanupInterval time.Duration // interval to clean up idle buckets
}

// DefaultRateLimitConfig returns default rate limit configuration.
func DefaultRateLimitConfig() *RateLimitConfig {
	return &RateLimitConfig{
		Rate:            10.0,            // 10 requests per second
		Burst:           20,              // burst of 20
		CleanupInterval: 5 * time.Minute, // cleanup every 5 minutes
	}
}

// NewRateLimiter creates a new rate limiter.
func NewRateLimiter(config *RateLimitConfig) *RateLimiter {
	if config == nil {
		config = DefaultRateLimitConfig()
	}

	rl := &RateLimiter{
		buckets: make(map[string]*tokenBucket),
		rate:    config.Rate,
		burst:   config.Burst,
		cleanup: config.CleanupInterval,
	}

	return rl
}

// Allow checks if a request from the given peer is allowed.
// Returns true if allowed, false if rate limited.
func (rl *RateLimiter) Allow(peerID string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	bucket, exists := rl.buckets[peerID]

	if !exists {
		// New peer, create bucket with full tokens
		rl.buckets[peerID] = &tokenBucket{
			tokens:     float64(rl.burst) - 1, // consume one token
			lastUpdate: now,
		}
		return true
	}

	// Refill tokens based on elapsed time
	elapsed := now.Sub(bucket.lastUpdate).Seconds()
	bucket.tokens += elapsed * rl.rate
	if bucket.tokens > float64(rl.burst) {
		bucket.tokens = float64(rl.burst)
	}
	bucket.lastUpdate = now

	// Try to consume a token
	if bucket.tokens >= 1 {
		bucket.tokens--
		return true
	}

	return false
}

// AllowN checks if n requests from the given peer are allowed.
func (rl *RateLimiter) AllowN(peerID string, n int) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	bucket, exists := rl.buckets[peerID]

	if !exists {
		if n > rl.burst {
			return false
		}
		rl.buckets[peerID] = &tokenBucket{
			tokens:     float64(rl.burst - n),
			lastUpdate: now,
		}
		return true
	}

	// Refill tokens
	elapsed := now.Sub(bucket.lastUpdate).Seconds()
	bucket.tokens += elapsed * rl.rate
	if bucket.tokens > float64(rl.burst) {
		bucket.tokens = float64(rl.burst)
	}
	bucket.lastUpdate = now

	// Try to consume tokens
	if bucket.tokens >= float64(n) {
		bucket.tokens -= float64(n)
		return true
	}

	return false
}

// Reset resets the rate limit for a peer.
func (rl *RateLimiter) Reset(peerID string) {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	delete(rl.buckets, peerID)
}

// Cleanup removes idle buckets that haven't been used recently.
func (rl *RateLimiter) Cleanup() {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	threshold := time.Now().Add(-rl.cleanup)
	for peerID, bucket := range rl.buckets {
		if bucket.lastUpdate.Before(threshold) {
			delete(rl.buckets, peerID)
		}
	}
}

// Stats returns current rate limiter statistics.
func (rl *RateLimiter) Stats() RateLimiterStats {
	rl.mu.RLock()
	defer rl.mu.RUnlock()

	return RateLimiterStats{
		ActivePeers: len(rl.buckets),
		Rate:        rl.rate,
		Burst:       rl.burst,
	}
}

// RateLimiterStats holds rate limiter statistics.
type RateLimiterStats struct {
	ActivePeers int     `json:"active_peers"`
	Rate        float64 `json:"rate"`
	Burst       int     `json:"burst"`
}
