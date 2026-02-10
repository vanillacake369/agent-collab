package token

import (
	"sync"
	"time"
)

// UsageCategory represents the category of token usage.
type UsageCategory string

const (
	CategoryEmbedding   UsageCategory = "embedding"
	CategorySync        UsageCategory = "sync"
	CategoryNegotiation UsageCategory = "negotiation"
	CategoryQuery       UsageCategory = "query"
	CategoryOther       UsageCategory = "other"
)

// UsageRecord represents a single token usage event.
type UsageRecord struct {
	ID        string         `json:"id"`
	Category  UsageCategory  `json:"category"`
	Tokens    int64          `json:"tokens"`
	Model     string         `json:"model"`
	Timestamp time.Time      `json:"timestamp"`
	Metadata  map[string]any `json:"metadata,omitempty"`
}

// HourlyBucket aggregates usage for one hour.
type HourlyBucket struct {
	Hour       time.Time               `json:"hour"`
	Total      int64                   `json:"total"`
	ByCategory map[UsageCategory]int64 `json:"by_category"`
}

// UsageMetrics holds aggregated token usage statistics.
type UsageMetrics struct {
	mu sync.RWMutex

	// Current period totals
	TokensToday int64 `json:"tokens_today"`
	TokensWeek  int64 `json:"tokens_week"`
	TokensMonth int64 `json:"tokens_month"`

	// Breakdown by category
	ByCategory map[UsageCategory]int64 `json:"by_category"`

	// Hourly data for trends (last 24 hours)
	HourlyData []*HourlyBucket `json:"hourly_data"`

	// Cost estimates
	CostToday float64 `json:"cost_today"`
	CostWeek  float64 `json:"cost_week"`
	CostMonth float64 `json:"cost_month"`

	// Rate
	TokensPerHour float64 `json:"tokens_per_hour"`

	// Limits
	DailyLimit int64 `json:"daily_limit"`

	// Last updated
	LastUpdated time.Time `json:"last_updated"`
}

// NewUsageMetrics creates a new UsageMetrics instance.
func NewUsageMetrics() *UsageMetrics {
	return &UsageMetrics{
		ByCategory: make(map[UsageCategory]int64),
		HourlyData: make([]*HourlyBucket, 0, 24),
		DailyLimit: 200000, // Default 200K tokens per day
	}
}

// CategoryBreakdown represents usage breakdown for a category.
type CategoryBreakdown struct {
	Category UsageCategory `json:"category"`
	Tokens   int64         `json:"tokens"`
	Percent  float64       `json:"percent"`
	Cost     float64       `json:"cost"`
}

// GetBreakdown returns usage breakdown by category.
func (m *UsageMetrics) GetBreakdown() []CategoryBreakdown {
	m.mu.RLock()
	defer m.mu.RUnlock()

	total := m.TokensToday
	if total == 0 {
		total = 1 // Avoid division by zero
	}

	breakdown := make([]CategoryBreakdown, 0, len(m.ByCategory))
	for cat, tokens := range m.ByCategory {
		breakdown = append(breakdown, CategoryBreakdown{
			Category: cat,
			Tokens:   tokens,
			Percent:  float64(tokens) / float64(total) * 100,
			Cost:     EstimateCost(tokens, ""),
		})
	}

	return breakdown
}

// GetHourlyTrend returns hourly token counts for charting.
func (m *UsageMetrics) GetHourlyTrend() []float64 {
	m.mu.RLock()
	defer m.mu.RUnlock()

	data := make([]float64, len(m.HourlyData))
	for i, bucket := range m.HourlyData {
		data[i] = float64(bucket.Total)
	}
	return data
}

// UsagePercent returns the percentage of daily limit used.
func (m *UsageMetrics) UsagePercent() float64 {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.DailyLimit == 0 {
		return 0
	}
	return float64(m.TokensToday) / float64(m.DailyLimit) * 100
}
