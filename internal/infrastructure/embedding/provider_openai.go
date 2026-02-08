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

// OpenAIProvider implements embedding using OpenAI API.
type OpenAIProvider struct {
	config *ProviderConfig
	client *http.Client
}

// NewOpenAIProvider creates a new OpenAI embedding provider.
func NewOpenAIProvider(cfg *ProviderConfig) *OpenAIProvider {
	if cfg.BaseURL == "" {
		cfg.BaseURL = "https://api.openai.com/v1"
	}
	if cfg.Model == "" {
		cfg.Model = "text-embedding-3-small"
	}
	if cfg.Dimension == 0 {
		cfg.Dimension = 1536
	}

	return &OpenAIProvider{
		config: cfg,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (p *OpenAIProvider) Name() Provider {
	return ProviderOpenAI
}

func (p *OpenAIProvider) Dimension() int {
	return p.config.Dimension
}

func (p *OpenAIProvider) Model() string {
	return p.config.Model
}

func (p *OpenAIProvider) SupportsModel(model string) bool {
	supportedModels := []string{
		"text-embedding-3-small",
		"text-embedding-3-large",
		"text-embedding-ada-002",
	}
	for _, m := range supportedModels {
		if m == model {
			return true
		}
	}
	return strings.HasPrefix(model, "text-embedding")
}

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

func (p *OpenAIProvider) Embed(ctx context.Context, texts []string) ([][]float32, int, error) {
	if p.config.APIKey == "" {
		return nil, 0, fmt.Errorf("OpenAI API key not set (set OPENAI_API_KEY environment variable)")
	}

	reqBody := openAIEmbeddingRequest{
		Model: p.config.Model,
		Input: texts,
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

	var embResp openAIEmbeddingResponse
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
