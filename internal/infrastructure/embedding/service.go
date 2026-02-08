package embedding

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"sync"
	"time"

	"agent-collab/internal/domain/token"
)

// Provider represents an embedding provider.
type Provider string

const (
	ProviderOpenAI   Provider = "openai"
	ProviderLocal    Provider = "local"
	ProviderMock     Provider = "mock"
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
	return &Config{
		Provider:   ProviderOpenAI,
		Model:      "text-embedding-3-small",
		Dimension:  1536,
		BaseURL:    "https://api.openai.com/v1",
		Timeout:    30 * time.Second,
		BatchSize:  100,
		MaxRetries: 3,
	}
}

// Service generates embeddings for text content.
type Service struct {
	mu     sync.RWMutex
	config *Config
	client *http.Client
	cache  map[string][]float32 // content hash -> embedding

	// Token tracking
	tokenTracker *token.Tracker
}

// NewService creates a new embedding service.
func NewService(cfg *Config) *Service {
	if cfg == nil {
		cfg = DefaultConfig()
	}

	// Try to get API key from environment
	if cfg.APIKey == "" {
		cfg.APIKey = os.Getenv("OPENAI_API_KEY")
	}

	return &Service{
		config: cfg,
		client: &http.Client{
			Timeout: cfg.Timeout,
		},
		cache: make(map[string][]float32),
	}
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
	embeddings, tokensUsed, err := s.generateEmbeddings(ctx, []string{text})
	if err != nil {
		return nil, err
	}

	if len(embeddings) == 0 {
		return nil, fmt.Errorf("no embeddings returned")
	}

	// Record token usage
	if s.tokenTracker != nil && tokensUsed > 0 {
		s.tokenTracker.RecordEmbedding(int64(tokensUsed), s.config.Model)
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
	s.mu.RUnlock()

	if len(uncachedTexts) == 0 {
		return results, nil
	}

	// Generate embeddings for uncached texts in batches
	var totalTokens int
	for i := 0; i < len(uncachedTexts); i += s.config.BatchSize {
		end := i + s.config.BatchSize
		if end > len(uncachedTexts) {
			end = len(uncachedTexts)
		}

		batch := uncachedTexts[i:end]
		embeddings, tokensUsed, err := s.generateEmbeddings(ctx, batch)
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
	if s.tokenTracker != nil && totalTokens > 0 {
		s.tokenTracker.RecordEmbedding(int64(totalTokens), s.config.Model)
	}

	return results, nil
}

// generateEmbeddings calls the embedding API.
func (s *Service) generateEmbeddings(ctx context.Context, texts []string) ([][]float32, int, error) {
	switch s.config.Provider {
	case ProviderOpenAI:
		return s.openAIEmbed(ctx, texts)
	case ProviderMock:
		return s.mockEmbed(texts)
	case ProviderLocal:
		return s.localEmbed(ctx, texts)
	default:
		return nil, 0, fmt.Errorf("unknown provider: %s", s.config.Provider)
	}
}

// OpenAI embedding request/response types
type openAIEmbeddingRequest struct {
	Model string   `json:"model"`
	Input []string `json:"input"`
}

type openAIEmbeddingResponse struct {
	Object string `json:"object"`
	Data   []struct {
		Object    string    `json:"object"`
		Index     int       `json:"index"`
		Embedding []float32 `json:"embedding"`
	} `json:"data"`
	Model string `json:"model"`
	Usage struct {
		PromptTokens int `json:"prompt_tokens"`
		TotalTokens  int `json:"total_tokens"`
	} `json:"usage"`
}

// openAIEmbed generates embeddings using OpenAI API.
func (s *Service) openAIEmbed(ctx context.Context, texts []string) ([][]float32, int, error) {
	if s.config.APIKey == "" {
		return nil, 0, fmt.Errorf("OpenAI API key not set (set OPENAI_API_KEY environment variable)")
	}

	reqBody := openAIEmbeddingRequest{
		Model: s.config.Model,
		Input: texts,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to marshal request: %w", err)
	}

	url := s.config.BaseURL + "/embeddings"
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(jsonData))
	if err != nil {
		return nil, 0, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+s.config.APIKey)

	var resp *http.Response
	var lastErr error

	for attempt := 0; attempt <= s.config.MaxRetries; attempt++ {
		resp, err = s.client.Do(req)
		if err == nil && resp.StatusCode == http.StatusOK {
			break
		}
		lastErr = err
		if resp != nil {
			resp.Body.Close()
		}
		if attempt < s.config.MaxRetries {
			time.Sleep(time.Duration(attempt+1) * time.Second)
		}
	}

	if lastErr != nil && resp == nil {
		return nil, 0, fmt.Errorf("request failed after retries: %w", lastErr)
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, 0, fmt.Errorf("API error: %d - %s", resp.StatusCode, string(body))
	}

	defer resp.Body.Close()

	var embResp openAIEmbeddingResponse
	if err := json.NewDecoder(resp.Body).Decode(&embResp); err != nil {
		return nil, 0, fmt.Errorf("failed to decode response: %w", err)
	}

	// Extract embeddings in correct order
	embeddings := make([][]float32, len(texts))
	for _, item := range embResp.Data {
		if item.Index < len(embeddings) {
			embeddings[item.Index] = item.Embedding
		}
	}

	return embeddings, embResp.Usage.TotalTokens, nil
}

// mockEmbed generates mock embeddings for testing.
func (s *Service) mockEmbed(texts []string) ([][]float32, int, error) {
	embeddings := make([][]float32, len(texts))
	totalTokens := 0

	for i, text := range texts {
		// Generate deterministic mock embedding based on text hash
		hash := sha256.Sum256([]byte(text))
		embedding := make([]float32, s.config.Dimension)
		for j := 0; j < s.config.Dimension; j++ {
			embedding[j] = float32(hash[j%32]) / 255.0
		}
		embeddings[i] = embedding
		totalTokens += len(text) / 4 // Rough token estimate
	}

	return embeddings, totalTokens, nil
}

// localEmbed generates embeddings using a local model (placeholder).
func (s *Service) localEmbed(ctx context.Context, texts []string) ([][]float32, int, error) {
	// Placeholder for local embedding model integration
	// Could integrate with sentence-transformers, fastembed, etc.
	return nil, 0, fmt.Errorf("local embedding not implemented")
}

// Dimension returns the embedding dimension.
func (s *Service) Dimension() int {
	return s.config.Dimension
}

// Model returns the model name.
func (s *Service) Model() string {
	return s.config.Model
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
