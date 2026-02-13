package embedding

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

// OllamaProvider implements embedding using local Ollama.
type OllamaProvider struct {
	config *ProviderConfig
	client *http.Client
}

// NewOllamaProvider creates a new Ollama embedding provider.
func NewOllamaProvider(cfg *ProviderConfig) *OllamaProvider {
	if cfg.BaseURL == "" {
		cfg.BaseURL = os.Getenv("OLLAMA_HOST")
		if cfg.BaseURL == "" {
			cfg.BaseURL = "http://localhost:11434"
		}
	}
	if cfg.Model == "" {
		cfg.Model = "nomic-embed-text"
	}
	if cfg.Dimension == 0 {
		cfg.Dimension = 768
	}

	return &OllamaProvider{
		config: cfg,
		client: &http.Client{
			Timeout: 60 * time.Second, // Local models may be slower
		},
	}
}

func (p *OllamaProvider) Name() Provider {
	return ProviderOllama
}

func (p *OllamaProvider) Dimension() int {
	return p.config.Dimension
}

func (p *OllamaProvider) Model() string {
	return p.config.Model
}

func (p *OllamaProvider) SupportsModel(model string) bool {
	// Ollama supports many models, check common embedding models
	embeddingModels := []string{
		"nomic-embed-text",
		"mxbai-embed-large",
		"all-minilm",
		"snowflake-arctic-embed",
	}
	for _, m := range embeddingModels {
		if strings.HasPrefix(model, m) {
			return true
		}
	}
	// Ollama can use any model for embeddings
	return true
}

type ollamaEmbeddingRequest struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
}

type ollamaEmbeddingResponse struct {
	Embedding []float32 `json:"embedding"`
}

func (p *OllamaProvider) Embed(ctx context.Context, texts []string) ([][]float32, int, error) {
	embeddings := make([][]float32, len(texts))
	totalTokens := 0

	// Ollama processes one text at a time
	for i, text := range texts {
		embedding, tokens, err := p.embedSingle(ctx, text)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to embed text %d: %w", i, err)
		}
		embeddings[i] = embedding
		totalTokens += tokens
	}

	return embeddings, totalTokens, nil
}

func (p *OllamaProvider) embedSingle(ctx context.Context, text string) ([]float32, int, error) {
	reqBody := ollamaEmbeddingRequest{
		Model:  p.config.Model,
		Prompt: text,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to marshal request: %w", err)
	}

	url := p.config.BaseURL + "/api/embeddings"
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(jsonData))
	if err != nil {
		return nil, 0, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := p.client.Do(req) // #nosec G704 - URL is from trusted embedding config
	if err != nil {
		return nil, 0, fmt.Errorf("request failed (is Ollama running?): %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, 0, fmt.Errorf("API error: %d - %s", resp.StatusCode, string(body))
	}

	var embResp ollamaEmbeddingResponse
	if err := json.NewDecoder(resp.Body).Decode(&embResp); err != nil {
		return nil, 0, fmt.Errorf("failed to decode response: %w", err)
	}

	// Estimate tokens
	tokens := len(text) / 4

	return embResp.Embedding, tokens, nil
}
