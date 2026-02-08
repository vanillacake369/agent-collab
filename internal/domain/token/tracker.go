package token

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"sync"
	"time"
)

// Tracker tracks token usage across the application.
type Tracker struct {
	mu sync.RWMutex

	nodeID   string
	nodeName string

	// Current metrics
	metrics *UsageMetrics

	// Recent records (ring buffer)
	records     []*UsageRecord
	recordsHead int
	maxRecords  int

	// Hourly aggregation
	currentHour *HourlyBucket

	// Persistence callback
	persistFn func(*UsageRecord) error

	// Background cleanup
	ctx    context.Context
	cancel context.CancelFunc
}

// NewTracker creates a new token usage tracker.
func NewTracker(nodeID, nodeName string) *Tracker {
	ctx, cancel := context.WithCancel(context.Background())

	t := &Tracker{
		nodeID:     nodeID,
		nodeName:   nodeName,
		metrics:    NewUsageMetrics(),
		records:    make([]*UsageRecord, 1000),
		maxRecords: 1000,
		ctx:        ctx,
		cancel:     cancel,
	}

	// Initialize current hour bucket
	t.currentHour = &HourlyBucket{
		Hour:       truncateToHour(time.Now()),
		ByCategory: make(map[UsageCategory]int64),
	}

	go t.aggregationLoop()

	return t
}

// Close stops the tracker and releases resources.
func (t *Tracker) Close() error {
	t.cancel()
	return nil
}

// SetPersistFn sets the persistence callback function.
func (t *Tracker) SetPersistFn(fn func(*UsageRecord) error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.persistFn = fn
}

// Record records a token usage event.
func (t *Tracker) Record(category UsageCategory, tokens int64, model string, metadata map[string]any) error {
	record := &UsageRecord{
		ID:        generateRecordID(),
		Category:  category,
		Tokens:    tokens,
		Model:     model,
		Timestamp: time.Now(),
		Metadata:  metadata,
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	// Add to ring buffer
	t.records[t.recordsHead] = record
	t.recordsHead = (t.recordsHead + 1) % t.maxRecords

	// Update metrics
	t.metrics.TokensToday += tokens
	t.metrics.TokensWeek += tokens
	t.metrics.TokensMonth += tokens
	t.metrics.ByCategory[category] += tokens
	t.metrics.LastUpdated = time.Now()

	// Update costs
	cost := EstimateCost(tokens, model)
	t.metrics.CostToday += cost
	t.metrics.CostWeek += cost
	t.metrics.CostMonth += cost

	// Update hourly bucket
	now := time.Now()
	hourStart := truncateToHour(now)
	if !t.currentHour.Hour.Equal(hourStart) {
		// Rotate to new hour
		t.metrics.HourlyData = append(t.metrics.HourlyData, t.currentHour)
		if len(t.metrics.HourlyData) > 24 {
			t.metrics.HourlyData = t.metrics.HourlyData[1:]
		}
		t.currentHour = &HourlyBucket{
			Hour:       hourStart,
			ByCategory: make(map[UsageCategory]int64),
		}
	}
	t.currentHour.Total += tokens
	t.currentHour.ByCategory[category] += tokens

	// Calculate tokens per hour rate
	t.updateTokenRate()

	// Persist if callback is set
	if t.persistFn != nil {
		go t.persistFn(record)
	}

	return nil
}

// RecordEmbedding is a convenience method for recording embedding token usage.
func (t *Tracker) RecordEmbedding(tokens int64, model string) error {
	return t.Record(CategoryEmbedding, tokens, model, nil)
}

// RecordSync is a convenience method for recording sync token usage.
func (t *Tracker) RecordSync(tokens int64, model string) error {
	return t.Record(CategorySync, tokens, model, nil)
}

// RecordNegotiation is a convenience method for recording negotiation token usage.
func (t *Tracker) RecordNegotiation(tokens int64, model string) error {
	return t.Record(CategoryNegotiation, tokens, model, nil)
}

// RecordQuery is a convenience method for recording query token usage.
func (t *Tracker) RecordQuery(tokens int64, model string) error {
	return t.Record(CategoryQuery, tokens, model, nil)
}

// GetMetrics returns a copy of current metrics.
func (t *Tracker) GetMetrics() *UsageMetrics {
	t.mu.RLock()
	defer t.mu.RUnlock()

	// Create a copy
	copy := &UsageMetrics{
		TokensToday:   t.metrics.TokensToday,
		TokensWeek:    t.metrics.TokensWeek,
		TokensMonth:   t.metrics.TokensMonth,
		CostToday:     t.metrics.CostToday,
		CostWeek:      t.metrics.CostWeek,
		CostMonth:     t.metrics.CostMonth,
		TokensPerHour: t.metrics.TokensPerHour,
		DailyLimit:    t.metrics.DailyLimit,
		LastUpdated:   t.metrics.LastUpdated,
		ByCategory:    make(map[UsageCategory]int64),
		HourlyData:    make([]*HourlyBucket, len(t.metrics.HourlyData)),
	}

	for k, v := range t.metrics.ByCategory {
		copy.ByCategory[k] = v
	}

	for i, bucket := range t.metrics.HourlyData {
		bucketCopy := &HourlyBucket{
			Hour:       bucket.Hour,
			Total:      bucket.Total,
			ByCategory: make(map[UsageCategory]int64),
		}
		for k, v := range bucket.ByCategory {
			bucketCopy.ByCategory[k] = v
		}
		copy.HourlyData[i] = bucketCopy
	}

	return copy
}

// GetRecentRecords returns recent usage records.
func (t *Tracker) GetRecentRecords(count int) []*UsageRecord {
	t.mu.RLock()
	defer t.mu.RUnlock()

	if count > t.maxRecords {
		count = t.maxRecords
	}

	result := make([]*UsageRecord, 0, count)
	for i := 0; i < count; i++ {
		idx := (t.recordsHead - 1 - i + t.maxRecords) % t.maxRecords
		if t.records[idx] != nil {
			result = append(result, t.records[idx])
		}
	}

	return result
}

// SetDailyLimit sets the daily token limit.
func (t *Tracker) SetDailyLimit(limit int64) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.metrics.DailyLimit = limit
}

// Reset resets daily/weekly/monthly counters based on time.
func (t *Tracker) Reset(period string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	switch period {
	case "day":
		t.metrics.TokensToday = 0
		t.metrics.CostToday = 0
		t.metrics.ByCategory = make(map[UsageCategory]int64)
	case "week":
		t.metrics.TokensWeek = 0
		t.metrics.CostWeek = 0
	case "month":
		t.metrics.TokensMonth = 0
		t.metrics.CostMonth = 0
	case "all":
		t.metrics = NewUsageMetrics()
	}
}

// updateTokenRate calculates tokens per hour based on recent activity.
func (t *Tracker) updateTokenRate() {
	if len(t.metrics.HourlyData) == 0 {
		t.metrics.TokensPerHour = float64(t.currentHour.Total)
		return
	}

	// Calculate average over last few hours
	totalTokens := t.currentHour.Total
	hours := 1.0

	for i := len(t.metrics.HourlyData) - 1; i >= 0 && hours < 4; i-- {
		totalTokens += t.metrics.HourlyData[i].Total
		hours++
	}

	t.metrics.TokensPerHour = float64(totalTokens) / hours
}

// aggregationLoop runs background aggregation tasks.
func (t *Tracker) aggregationLoop() {
	dayTicker := time.NewTicker(time.Hour)
	defer dayTicker.Stop()

	for {
		select {
		case <-t.ctx.Done():
			return
		case now := <-dayTicker.C:
			t.mu.Lock()

			// Reset daily counters at midnight
			if now.Hour() == 0 {
				t.metrics.TokensToday = 0
				t.metrics.CostToday = 0
				t.metrics.ByCategory = make(map[UsageCategory]int64)

				// Reset weekly on Monday
				if now.Weekday() == time.Monday {
					t.metrics.TokensWeek = 0
					t.metrics.CostWeek = 0
				}

				// Reset monthly on 1st
				if now.Day() == 1 {
					t.metrics.TokensMonth = 0
					t.metrics.CostMonth = 0
				}
			}

			t.mu.Unlock()
		}
	}
}

// truncateToHour truncates a time to the start of its hour.
func truncateToHour(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), t.Hour(), 0, 0, 0, t.Location())
}

// generateRecordID generates a unique record ID.
func generateRecordID() string {
	bytes := make([]byte, 8)
	rand.Read(bytes)
	return "rec-" + hex.EncodeToString(bytes)
}
