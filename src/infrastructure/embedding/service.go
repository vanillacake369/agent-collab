package embedding

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sync"
	"time"

	"agent-collab/src/domain/token"
)

// Config holds embedding service configuration.
type Config struct {
	Provider   Provider      `json:"provider"`
	Model      string        `json:"model"`
	Dimension  int           `json:"dimension"`
	APIKey     string        `json:"-"` // Don't serialize API key
	BaseURL    string        `json:"base_url,omitempty"`
	Timeout    time.Duration `json:"timeout"`
	BatchSize  int           `json:"batch_size"`
	MaxRetries int           `json:"max_retries"`
}

// DefaultConfig returns default configuration.
func DefaultConfig() *Config {
	// Auto-detect available provider
	provider := DetectAvailableProvider()
	defaults := DefaultProviderConfigs()
	cfg := defaults[provider]

	return &Config{
		Provider:   provider,
		Model:      cfg.Model,
		Dimension:  cfg.Dimension,
		BaseURL:    cfg.BaseURL,
		APIKey:     GetAPIKeyFromEnv(provider),
		Timeout:    30 * time.Second,
		BatchSize:  100,
		MaxRetries: 3,
	}
}

// Service generates embeddings for text content.
type Service struct {
	mu       sync.RWMutex
	config   *Config
	provider EmbeddingProvider
	cache    map[string][]float32 // content hash -> embedding

	// Token tracking
	tokenTracker *token.Tracker
}

// NewService creates a new embedding service.
func NewService(cfg *Config) *Service {
	if cfg == nil {
		cfg = DefaultConfig()
	}

	// Get API key from environment if not set
	if cfg.APIKey == "" {
		cfg.APIKey = GetAPIKeyFromEnv(cfg.Provider)
	}

	// Create provider
	providerCfg := &ProviderConfig{
		Provider:  cfg.Provider,
		APIKey:    cfg.APIKey,
		BaseURL:   cfg.BaseURL,
		Model:     cfg.Model,
		Dimension: cfg.Dimension,
	}

	provider, err := CreateProvider(providerCfg)
	if err != nil {
		// Fall back to mock provider
		provider = NewMockProvider(providerCfg)
	}

	return &Service{
		config:   cfg,
		provider: provider,
		cache:    make(map[string][]float32),
	}
}

// NewServiceWithProvider creates a new embedding service with a specific provider.
func NewServiceWithProvider(provider EmbeddingProvider) *Service {
	return &Service{
		config: &Config{
			Provider:  provider.Name(),
			Model:     provider.Model(),
			Dimension: provider.Dimension(),
			BatchSize: 100,
		},
		provider: provider,
		cache:    make(map[string][]float32),
	}
}

// SetProvider changes the embedding provider.
func (s *Service) SetProvider(provider EmbeddingProvider) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.provider = provider
	s.config.Provider = provider.Name()
	s.config.Model = provider.Model()
	s.config.Dimension = provider.Dimension()
}

// GetProvider returns the current provider.
func (s *Service) GetProvider() EmbeddingProvider {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.provider
}

// SetTokenTracker sets the token tracker for usage monitoring.
func (s *Service) SetTokenTracker(tracker *token.Tracker) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.tokenTracker = tracker
}

// Embed generates an embedding for a single text.
func (s *Service) Embed(ctx context.Context, text string) ([]float32, error) {
	// Check cache
	hash := computeHash(text)
	s.mu.RLock()
	if cached, ok := s.cache[hash]; ok {
		s.mu.RUnlock()
		return cached, nil
	}
	s.mu.RUnlock()

	// Generate embedding
	s.mu.RLock()
	provider := s.provider
	model := s.config.Model
	s.mu.RUnlock()

	embeddings, tokensUsed, err := provider.Embed(ctx, []string{text})
	if err != nil {
		return nil, err
	}

	if len(embeddings) == 0 {
		return nil, fmt.Errorf("no embeddings returned")
	}

	// Record token usage
	s.mu.RLock()
	tracker := s.tokenTracker
	s.mu.RUnlock()

	if tracker != nil && tokensUsed > 0 {
		tracker.RecordEmbedding(int64(tokensUsed), model)
	}

	// Cache result
	s.mu.Lock()
	s.cache[hash] = embeddings[0]
	s.mu.Unlock()

	return embeddings[0], nil
}

// EmbedBatch generates embeddings for multiple texts.
func (s *Service) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return nil, nil
	}

	// Check cache for all texts
	results := make([][]float32, len(texts))
	uncached := make([]int, 0)
	uncachedTexts := make([]string, 0)

	s.mu.RLock()
	for i, text := range texts {
		hash := computeHash(text)
		if cached, ok := s.cache[hash]; ok {
			results[i] = cached
		} else {
			uncached = append(uncached, i)
			uncachedTexts = append(uncachedTexts, text)
		}
	}
	provider := s.provider
	model := s.config.Model
	batchSize := s.config.BatchSize
	s.mu.RUnlock()

	if len(uncachedTexts) == 0 {
		return results, nil
	}

	// Generate embeddings for uncached texts in batches
	var totalTokens int
	for i := 0; i < len(uncachedTexts); i += batchSize {
		end := i + batchSize
		if end > len(uncachedTexts) {
			end = len(uncachedTexts)
		}

		batch := uncachedTexts[i:end]
		embeddings, tokensUsed, err := provider.Embed(ctx, batch)
		if err != nil {
			return nil, err
		}

		totalTokens += tokensUsed

		// Fill in results and cache
		s.mu.Lock()
		for j, embedding := range embeddings {
			idx := uncached[i+j]
			results[idx] = embedding
			s.cache[computeHash(texts[idx])] = embedding
		}
		s.mu.Unlock()
	}

	// Record token usage
	s.mu.RLock()
	tracker := s.tokenTracker
	s.mu.RUnlock()

	if tracker != nil && totalTokens > 0 {
		tracker.RecordEmbedding(int64(totalTokens), model)
	}

	return results, nil
}

// Dimension returns the embedding dimension.
func (s *Service) Dimension() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.provider.Dimension()
}

// Model returns the model name.
func (s *Service) Model() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.provider.Model()
}

// Provider returns the current provider name.
func (s *Service) Provider() Provider {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.provider.Name()
}

// ClearCache clears the embedding cache.
func (s *Service) ClearCache() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cache = make(map[string][]float32)
}

// CacheSize returns the number of cached embeddings.
func (s *Service) CacheSize() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.cache)
}

// computeHash generates a hash for cache key.
func computeHash(text string) string {
	hash := sha256.Sum256([]byte(text))
	return hex.EncodeToString(hash[:])
}
