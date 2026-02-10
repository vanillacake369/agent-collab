package embedding

import (
	"context"
	"fmt"
	"os"
	"strings"
)

// Provider represents an embedding provider.
type Provider string

const (
	ProviderOpenAI    Provider = "openai"
	ProviderAnthropic Provider = "anthropic"
	ProviderGoogle    Provider = "google"
	ProviderOllama    Provider = "ollama"
	ProviderMock      Provider = "mock"
)

// EmbeddingProvider is the interface for embedding providers.
type EmbeddingProvider interface {
	// Name returns the provider name.
	Name() Provider

	// Embed generates embeddings for the given texts.
	Embed(ctx context.Context, texts []string) ([][]float32, int, error)

	// Dimension returns the embedding dimension.
	Dimension() int

	// Model returns the model name.
	Model() string

	// SupportsModel returns true if the provider supports the given model.
	SupportsModel(model string) bool
}

// ProviderConfig contains configuration for an embedding provider.
type ProviderConfig struct {
	Provider  Provider `json:"provider"`
	APIKey    string   `json:"-"` // Don't serialize
	BaseURL   string   `json:"base_url,omitempty"`
	Model     string   `json:"model"`
	Dimension int      `json:"dimension"`
}

// ProviderRegistry manages available embedding providers.
type ProviderRegistry struct {
	providers map[Provider]EmbeddingProvider
	configs   map[Provider]*ProviderConfig
}

// NewProviderRegistry creates a new provider registry.
func NewProviderRegistry() *ProviderRegistry {
	return &ProviderRegistry{
		providers: make(map[Provider]EmbeddingProvider),
		configs:   make(map[Provider]*ProviderConfig),
	}
}

// Register adds a provider to the registry.
func (r *ProviderRegistry) Register(provider EmbeddingProvider) {
	r.providers[provider.Name()] = provider
}

// Get returns a provider by name.
func (r *ProviderRegistry) Get(name Provider) (EmbeddingProvider, bool) {
	p, ok := r.providers[name]
	return p, ok
}

// List returns all registered providers.
func (r *ProviderRegistry) List() []Provider {
	providers := make([]Provider, 0, len(r.providers))
	for name := range r.providers {
		providers = append(providers, name)
	}
	return providers
}

// DefaultProviderConfigs returns default configurations for known providers.
func DefaultProviderConfigs() map[Provider]*ProviderConfig {
	return map[Provider]*ProviderConfig{
		ProviderOpenAI: {
			Provider:  ProviderOpenAI,
			BaseURL:   "https://api.openai.com/v1",
			Model:     "text-embedding-3-small",
			Dimension: 1536,
		},
		ProviderAnthropic: {
			Provider:  ProviderAnthropic,
			BaseURL:   "https://api.anthropic.com/v1",
			Model:     "claude-3-haiku-20240307", // Anthropic uses Voyage for embeddings
			Dimension: 1024,
		},
		ProviderGoogle: {
			Provider:  ProviderGoogle,
			BaseURL:   "https://generativelanguage.googleapis.com/v1beta",
			Model:     "text-embedding-004",
			Dimension: 768,
		},
		ProviderOllama: {
			Provider:  ProviderOllama,
			BaseURL:   "http://localhost:11434",
			Model:     "nomic-embed-text",
			Dimension: 768,
		},
		ProviderMock: {
			Provider:  ProviderMock,
			Model:     "mock-embedding",
			Dimension: 1536,
		},
	}
}

// GetAPIKeyEnvVar returns the environment variable name for a provider's API key.
func GetAPIKeyEnvVar(provider Provider) string {
	switch provider {
	case ProviderOpenAI:
		return "OPENAI_API_KEY"
	case ProviderAnthropic:
		return "ANTHROPIC_API_KEY"
	case ProviderGoogle:
		return "GOOGLE_API_KEY"
	case ProviderOllama:
		return "" // Local, no API key needed
	default:
		return strings.ToUpper(string(provider)) + "_API_KEY"
	}
}

// GetAPIKeyFromEnv gets the API key for a provider from environment.
func GetAPIKeyFromEnv(provider Provider) string {
	envVar := GetAPIKeyEnvVar(provider)
	if envVar == "" {
		return ""
	}
	return os.Getenv(envVar)
}

// DetectAvailableProvider detects which provider is available based on environment.
func DetectAvailableProvider() Provider {
	// Check in order of preference
	if os.Getenv("OPENAI_API_KEY") != "" {
		return ProviderOpenAI
	}
	if os.Getenv("ANTHROPIC_API_KEY") != "" {
		return ProviderAnthropic
	}
	if os.Getenv("GOOGLE_API_KEY") != "" {
		return ProviderGoogle
	}
	// Check if Ollama is available (localhost)
	if os.Getenv("OLLAMA_HOST") != "" || isOllamaRunning() {
		return ProviderOllama
	}
	return ProviderMock
}

// isOllamaRunning checks if Ollama is running locally.
func isOllamaRunning() bool {
	// Simple check - could be improved
	_, err := os.Stat("/usr/local/bin/ollama")
	if err == nil {
		return true
	}
	_, err = os.Stat("/usr/bin/ollama")
	return err == nil
}

// CreateProvider creates a provider from configuration.
func CreateProvider(cfg *ProviderConfig) (EmbeddingProvider, error) {
	// Get API key from environment if not set
	if cfg.APIKey == "" {
		cfg.APIKey = GetAPIKeyFromEnv(cfg.Provider)
	}

	switch cfg.Provider {
	case ProviderOpenAI:
		return NewOpenAIProvider(cfg), nil
	case ProviderAnthropic:
		return NewAnthropicProvider(cfg), nil
	case ProviderGoogle:
		return NewGoogleProvider(cfg), nil
	case ProviderOllama:
		return NewOllamaProvider(cfg), nil
	case ProviderMock:
		return NewMockProvider(cfg), nil
	default:
		return nil, fmt.Errorf("unknown provider: %s", cfg.Provider)
	}
}
