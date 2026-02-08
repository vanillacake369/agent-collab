package embedding

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// GoogleProvider implements embedding using Google AI API.
type GoogleProvider struct {
	config *ProviderConfig
	client *http.Client
}

// NewGoogleProvider creates a new Google embedding provider.
func NewGoogleProvider(cfg *ProviderConfig) *GoogleProvider {
	if cfg.BaseURL == "" {
		cfg.BaseURL = "https://generativelanguage.googleapis.com/v1beta"
	}
	if cfg.Model == "" {
		cfg.Model = "text-embedding-004"
	}
	if cfg.Dimension == 0 {
		cfg.Dimension = 768
	}

	return &GoogleProvider{
		config: cfg,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (p *GoogleProvider) Name() Provider {
	return ProviderGoogle
}

func (p *GoogleProvider) Dimension() int {
	return p.config.Dimension
}

func (p *GoogleProvider) Model() string {
	return p.config.Model
}

func (p *GoogleProvider) SupportsModel(model string) bool {
	supportedModels := []string{
		"text-embedding-004",
		"embedding-001",
		"text-embedding-preview-0409",
	}
	for _, m := range supportedModels {
		if m == model {
			return true
		}
	}
	return strings.HasPrefix(model, "text-embedding") || strings.HasPrefix(model, "embedding")
}

type googleEmbedRequest struct {
	Model   string `json:"model"`
	Content struct {
		Parts []struct {
			Text string `json:"text"`
		} `json:"parts"`
	} `json:"content"`
}

type googleBatchEmbedRequest struct {
	Requests []googleEmbedRequest `json:"requests"`
}

type googleEmbedResponse struct {
	Embedding struct {
		Values []float32 `json:"values"`
	} `json:"embedding"`
}

type googleBatchEmbedResponse struct {
	Embeddings []struct {
		Values []float32 `json:"values"`
	} `json:"embeddings"`
}

func (p *GoogleProvider) Embed(ctx context.Context, texts []string) ([][]float32, int, error) {
	if p.config.APIKey == "" {
		return nil, 0, fmt.Errorf("google API key not set (set GOOGLE_API_KEY environment variable)")
	}

	// Build batch request
	requests := make([]googleEmbedRequest, len(texts))
	for i, text := range texts {
		requests[i] = googleEmbedRequest{
			Model: "models/" + p.config.Model,
		}
		requests[i].Content.Parts = []struct {
			Text string `json:"text"`
		}{{Text: text}}
	}

	reqBody := googleBatchEmbedRequest{Requests: requests}
	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to marshal request: %w", err)
	}

	url := fmt.Sprintf("%s/models/%s:batchEmbedContents?key=%s",
		p.config.BaseURL, p.config.Model, p.config.APIKey)

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(jsonData))
	if err != nil {
		return nil, 0, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, 0, fmt.Errorf("API error: %d - %s", resp.StatusCode, string(body))
	}

	var embResp googleBatchEmbedResponse
	if err := json.NewDecoder(resp.Body).Decode(&embResp); err != nil {
		return nil, 0, fmt.Errorf("failed to decode response: %w", err)
	}

	embeddings := make([][]float32, len(texts))
	for i, emb := range embResp.Embeddings {
		if i < len(embeddings) {
			embeddings[i] = emb.Values
		}
	}

	// Estimate token usage (Google doesn't always return this)
	totalTokens := 0
	for _, text := range texts {
		totalTokens += len(text) / 4
	}

	return embeddings, totalTokens, nil
}
