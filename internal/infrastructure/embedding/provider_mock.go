package embedding

import (
	"context"
	"crypto/sha256"
)

// MockProvider implements mock embedding for testing.
type MockProvider struct {
	config *ProviderConfig
}

// NewMockProvider creates a new mock embedding provider.
func NewMockProvider(cfg *ProviderConfig) *MockProvider {
	if cfg.Model == "" {
		cfg.Model = "mock-embedding"
	}
	if cfg.Dimension == 0 {
		cfg.Dimension = 1536
	}

	return &MockProvider{
		config: cfg,
	}
}

func (p *MockProvider) Name() Provider {
	return ProviderMock
}

func (p *MockProvider) Dimension() int {
	return p.config.Dimension
}

func (p *MockProvider) Model() string {
	return p.config.Model
}

func (p *MockProvider) SupportsModel(model string) bool {
	return true // Mock supports any model
}

func (p *MockProvider) Embed(ctx context.Context, texts []string) ([][]float32, int, error) {
	embeddings := make([][]float32, len(texts))
	totalTokens := 0

	for i, text := range texts {
		// Generate deterministic mock embedding based on text hash
		hash := sha256.Sum256([]byte(text))
		embedding := make([]float32, p.config.Dimension)
		for j := 0; j < p.config.Dimension; j++ {
			embedding[j] = float32(hash[j%32]) / 255.0
		}
		embeddings[i] = embedding
		totalTokens += len(text) / 4 // Rough token estimate
	}

	return embeddings, totalTokens, nil
}
