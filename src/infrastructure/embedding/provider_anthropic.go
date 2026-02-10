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

// AnthropicProvider implements embedding using Anthropic/Voyage API.
// Note: Anthropic partners with Voyage AI for embeddings.
type AnthropicProvider struct {
	config *ProviderConfig
	client *http.Client
}

// NewAnthropicProvider creates a new Anthropic embedding provider.
func NewAnthropicProvider(cfg *ProviderConfig) *AnthropicProvider {
	if cfg.BaseURL == "" {
		// Voyage AI is Anthropic's embedding partner
		cfg.BaseURL = "https://api.voyageai.com/v1"
	}
	if cfg.Model == "" {
		cfg.Model = "voyage-2"
	}
	if cfg.Dimension == 0 {
		cfg.Dimension = 1024
	}

	return &AnthropicProvider{
		config: cfg,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (p *AnthropicProvider) Name() Provider {
	return ProviderAnthropic
}

func (p *AnthropicProvider) Dimension() int {
	return p.config.Dimension
}

func (p *AnthropicProvider) Model() string {
	return p.config.Model
}

func (p *AnthropicProvider) SupportsModel(model string) bool {
	supportedModels := []string{
		"voyage-2",
		"voyage-large-2",
		"voyage-code-2",
		"voyage-lite-02-instruct",
	}
	for _, m := range supportedModels {
		if m == model {
			return true
		}
	}
	return strings.HasPrefix(model, "voyage")
}

type voyageEmbeddingRequest struct {
	Model     string   `json:"model"`
	Input     []string `json:"input"`
	InputType string   `json:"input_type,omitempty"` // "query" or "document"
}

type voyageEmbeddingResponse struct {
	Object string `json:"object"`
	Data   []struct {
		Object    string    `json:"object"`
		Index     int       `json:"index"`
		Embedding []float32 `json:"embedding"`
	} `json:"data"`
	Model string `json:"model"`
	Usage struct {
		TotalTokens int `json:"total_tokens"`
	} `json:"usage"`
}

func (p *AnthropicProvider) Embed(ctx context.Context, texts []string) ([][]float32, int, error) {
	if p.config.APIKey == "" {
		return nil, 0, fmt.Errorf("Anthropic/Voyage API key not set (set ANTHROPIC_API_KEY or VOYAGE_API_KEY)")
	}

	reqBody := voyageEmbeddingRequest{
		Model:     p.config.Model,
		Input:     texts,
		InputType: "document",
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to marshal request: %w", err)
	}

	url := p.config.BaseURL + "/embeddings"
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(jsonData))
	if err != nil {
		return nil, 0, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+p.config.APIKey)

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, 0, fmt.Errorf("API error: %d - %s", resp.StatusCode, string(body))
	}

	var embResp voyageEmbeddingResponse
	if err := json.NewDecoder(resp.Body).Decode(&embResp); err != nil {
		return nil, 0, fmt.Errorf("failed to decode response: %w", err)
	}

	embeddings := make([][]float32, len(texts))
	for _, item := range embResp.Data {
		if item.Index < len(embeddings) {
			embeddings[item.Index] = item.Embedding
		}
	}

	return embeddings, embResp.Usage.TotalTokens, nil
}
