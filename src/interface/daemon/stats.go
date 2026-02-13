package daemon

import (
	"encoding/json"
	"net/http"
	"strconv"
)

// TokenUsageResponse represents token usage statistics.
type TokenUsageResponse struct {
	TokensToday   int64   `json:"tokens_today"`
	TokensWeek    int64   `json:"tokens_week"`
	TokensMonth   int64   `json:"tokens_month"`
	TokensPerHour float64 `json:"tokens_per_hour"`
	CostToday     float64 `json:"cost_today"`
	CostWeek      float64 `json:"cost_week"`
	CostMonth     float64 `json:"cost_month"`
	DailyLimit    int64   `json:"daily_limit"`
	UsagePercent  float64 `json:"usage_percent"`
	Provider      string  `json:"provider,omitempty"`
	Model         string  `json:"model,omitempty"`
}

// ContextStatsResponse represents context statistics.
type ContextStatsResponse struct {
	TotalDocuments  int64             `json:"total_documents"`
	TotalEmbeddings int64             `json:"total_embeddings"`
	SharedContexts  int64             `json:"shared_contexts"`
	WatchedFiles    int               `json:"watched_files"`
	PendingDeltas   int               `json:"pending_deltas"`
	Collections     []CollectionStats `json:"collections,omitempty"`
	RecentActivity  []ContextActivity `json:"recent_activity,omitempty"`
}

// CollectionStats represents stats for a single collection.
type CollectionStats struct {
	Name      string `json:"name"`
	Count     int64  `json:"count"`
	Dimension int    `json:"dimension"`
}

// ContextActivity represents recent context activity.
type ContextActivity struct {
	Timestamp string `json:"timestamp"`
	Type      string `json:"type"`
	FilePath  string `json:"file_path,omitempty"`
	Source    string `json:"source,omitempty"`
}

// handleTokenUsage handles the /tokens/usage endpoint.
func (s *Server) handleTokenUsage(w http.ResponseWriter, _ *http.Request) {
	tokenTracker := s.app.TokenTracker()
	if tokenTracker == nil {
		json.NewEncoder(w).Encode(TokenUsageResponse{})
		return
	}

	metrics := tokenTracker.GetMetrics()

	resp := TokenUsageResponse{
		TokensToday:   metrics.TokensToday,
		TokensWeek:    metrics.TokensWeek,
		TokensMonth:   metrics.TokensMonth,
		TokensPerHour: metrics.TokensPerHour,
		CostToday:     metrics.CostToday,
		CostWeek:      metrics.CostWeek,
		CostMonth:     metrics.CostMonth,
		DailyLimit:    metrics.DailyLimit,
		UsagePercent:  metrics.UsagePercent(),
	}

	// Add provider info if embedding service is available
	embedService := s.app.EmbeddingService()
	if embedService != nil {
		resp.Provider = string(embedService.Provider())
		resp.Model = embedService.Model()
	}

	json.NewEncoder(w).Encode(resp)
}

// handleContextStats handles the /context/stats endpoint.
func (s *Server) handleContextStats(w http.ResponseWriter, r *http.Request) {
	resp := ContextStatsResponse{
		Collections:    []CollectionStats{},
		RecentActivity: []ContextActivity{},
	}

	// Get vector store stats
	vectorStore := s.app.VectorStore()
	if vectorStore != nil {
		if stats, err := vectorStore.GetCollectionStats("default"); err == nil {
			resp.TotalDocuments = stats.Count
			resp.TotalEmbeddings = stats.Count
			resp.Collections = append(resp.Collections, CollectionStats{
				Name:      stats.Name,
				Count:     stats.Count,
				Dimension: stats.Dimension,
			})
		}
	}

	// Get sync manager stats
	syncManager := s.app.SyncManager()
	if syncManager != nil {
		syncStats := syncManager.GetStats()
		resp.WatchedFiles = syncStats.WatchedFiles
		resp.PendingDeltas = syncStats.TotalDeltas
	}

	// Get recent events for activity
	limit := 10
	if l := r.URL.Query().Get("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 {
			limit = parsed
		}
	}

	events := s.eventBus.GetEventsByType(EventContextUpdated, limit)
	for _, e := range events {
		activity := ContextActivity{
			Timestamp: e.Timestamp.Format("2006-01-02T15:04:05Z07:00"),
			Type:      string(e.Type),
		}
		// Event data is stored as json.RawMessage, so we need to decode if present
		resp.RecentActivity = append(resp.RecentActivity, activity)
	}

	// Count shared contexts from events
	sharedEvents := s.eventBus.GetEventsByType(EventContextUpdated, 100)
	resp.SharedContexts = int64(len(sharedEvents))

	json.NewEncoder(w).Encode(resp)
}
