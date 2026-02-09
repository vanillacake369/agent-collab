package metrics

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"agent-collab/internal/domain/token"
)

// Store persists token usage metrics to disk.
type Store struct {
	mu       sync.RWMutex
	dataDir  string
	records  []*token.UsageRecord
	maxInMem int
}

// NewStore creates a new metrics store.
func NewStore(dataDir string) (*Store, error) {
	metricsDir := filepath.Join(dataDir, "metrics")
	if err := os.MkdirAll(metricsDir, 0750); err != nil {
		return nil, fmt.Errorf("failed to create metrics dir: %w", err)
	}

	return &Store{
		dataDir:  metricsDir,
		records:  make([]*token.UsageRecord, 0, 1000),
		maxInMem: 1000,
	}, nil
}

// Save persists a usage record.
func (s *Store) Save(record *token.UsageRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Add to in-memory buffer
	s.records = append(s.records, record)

	// Flush to disk periodically
	if len(s.records) >= s.maxInMem {
		return s.flush()
	}

	return nil
}

// flush writes buffered records to disk.
func (s *Store) flush() error {
	if len(s.records) == 0 {
		return nil
	}

	// Create daily file
	now := time.Now()
	filename := fmt.Sprintf("usage_%s.jsonl", now.Format("2006-01-02"))
	path := filepath.Join(s.dataDir, filename)

	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return fmt.Errorf("failed to open metrics file: %w", err)
	}
	defer f.Close()

	for _, record := range s.records {
		data, err := json.Marshal(record)
		if err != nil {
			continue
		}
		f.Write(data)
		f.Write([]byte("\n"))
	}

	s.records = s.records[:0]
	return nil
}

// Flush forces a flush of buffered records.
func (s *Store) Flush() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.flush()
}

// LoadDay loads usage records for a specific day.
func (s *Store) LoadDay(date time.Time) ([]*token.UsageRecord, error) {
	filename := fmt.Sprintf("usage_%s.jsonl", date.Format("2006-01-02"))
	path := filepath.Join(s.dataDir, filename)

	// #nosec G304 - path is constructed from s.dataDir (app data directory) and a date-based filename
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var records []*token.UsageRecord
	lines := splitLines(data)
	for _, line := range lines {
		if len(line) == 0 {
			continue
		}
		var record token.UsageRecord
		if err := json.Unmarshal(line, &record); err != nil {
			continue
		}
		records = append(records, &record)
	}

	return records, nil
}

// LoadRange loads usage records for a date range.
func (s *Store) LoadRange(start, end time.Time) ([]*token.UsageRecord, error) {
	var allRecords []*token.UsageRecord

	for d := start; !d.After(end); d = d.AddDate(0, 0, 1) {
		records, err := s.LoadDay(d)
		if err != nil {
			continue
		}
		allRecords = append(allRecords, records...)
	}

	return allRecords, nil
}

// AggregateDay returns aggregated metrics for a day.
func (s *Store) AggregateDay(date time.Time) (*DailyAggregate, error) {
	records, err := s.LoadDay(date)
	if err != nil {
		return nil, err
	}

	agg := &DailyAggregate{
		Date:       date,
		ByCategory: make(map[token.UsageCategory]int64),
		ByModel:    make(map[string]int64),
	}

	for _, r := range records {
		agg.TotalTokens += r.Tokens
		agg.ByCategory[r.Category] += r.Tokens
		agg.ByModel[r.Model] += r.Tokens
		agg.RecordCount++
	}

	agg.EstimatedCost = token.EstimateCost(agg.TotalTokens, "")

	return agg, nil
}

// DailyAggregate holds aggregated metrics for a day.
type DailyAggregate struct {
	Date          time.Time                     `json:"date"`
	TotalTokens   int64                         `json:"total_tokens"`
	RecordCount   int                           `json:"record_count"`
	ByCategory    map[token.UsageCategory]int64 `json:"by_category"`
	ByModel       map[string]int64              `json:"by_model"`
	EstimatedCost float64                       `json:"estimated_cost"`
}

// Close flushes any pending records and closes the store.
func (s *Store) Close() error {
	return s.Flush()
}

// Cleanup removes old metric files.
func (s *Store) Cleanup(retentionDays int) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	cutoff := time.Now().AddDate(0, 0, -retentionDays)

	entries, err := os.ReadDir(s.dataDir)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		// Parse date from filename
		name := entry.Name()
		if len(name) < 15 || name[:6] != "usage_" {
			continue
		}

		dateStr := name[6:16] // "usage_2024-01-15.jsonl" -> "2024-01-15"
		fileDate, err := time.Parse("2006-01-02", dateStr)
		if err != nil {
			continue
		}

		if fileDate.Before(cutoff) {
			os.Remove(filepath.Join(s.dataDir, name))
		}
	}

	return nil
}

// splitLines splits byte data into lines.
func splitLines(data []byte) [][]byte {
	var lines [][]byte
	start := 0

	for i, b := range data {
		if b == '\n' {
			if i > start {
				lines = append(lines, data[start:i])
			}
			start = i + 1
		}
	}

	if start < len(data) {
		lines = append(lines, data[start:])
	}

	return lines
}
