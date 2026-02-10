// Package embedding provides an EmbeddingPlugin implementation.
package embedding

import (
	"context"
	"fmt"
	"sync"

	infraEmbed "agent-collab/src/infrastructure/embedding"
	"agent-collab/src/plugin"
)

// Plugin wraps the embedding Service as an EmbeddingPlugin.
type Plugin struct {
	service *infraEmbed.Service
	config  *infraEmbed.Config

	ready   bool
	readyMu sync.RWMutex
}

// Config holds the plugin configuration.
type Config = infraEmbed.Config

// NewPlugin creates a new embedding plugin.
func NewPlugin(cfg *Config) *Plugin {
	if cfg == nil {
		cfg = infraEmbed.DefaultConfig()
	}
	return &Plugin{
		config: cfg,
	}
}

// Name returns the plugin name.
func (p *Plugin) Name() string {
	return "embedding." + string(p.config.Provider)
}

// Start initializes and starts the embedding service.
func (p *Plugin) Start(ctx context.Context) error {
	p.service = infraEmbed.NewService(p.config)

	// Test the service with a simple embed
	_, err := p.service.Embed(ctx, "test")
	if err != nil {
		// Non-fatal, service might still work
		fmt.Printf("warning: embedding test failed: %v\n", err)
	}

	p.setReady(true)
	return nil
}

// Stop gracefully stops the embedding service.
func (p *Plugin) Stop() error {
	p.setReady(false)
	// Embedding service doesn't need explicit cleanup
	return nil
}

// Ready returns true if the plugin is ready.
func (p *Plugin) Ready() bool {
	p.readyMu.RLock()
	defer p.readyMu.RUnlock()
	return p.ready
}

func (p *Plugin) setReady(ready bool) {
	p.readyMu.Lock()
	defer p.readyMu.Unlock()
	p.ready = ready
}

// Embed generates an embedding for the given text.
func (p *Plugin) Embed(ctx context.Context, text string) ([]float32, error) {
	if p.service == nil {
		return nil, fmt.Errorf("service not started")
	}
	return p.service.Embed(ctx, text)
}

// EmbedBatch generates embeddings for multiple texts.
func (p *Plugin) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	if p.service == nil {
		return nil, fmt.Errorf("service not started")
	}
	return p.service.EmbedBatch(ctx, texts)
}

// Dimension returns the embedding dimension.
func (p *Plugin) Dimension() int {
	return p.config.Dimension
}

// Model returns the model name being used.
func (p *Plugin) Model() string {
	return p.config.Model
}

// Provider returns the provider name.
func (p *Plugin) Provider() string {
	return string(p.config.Provider)
}

// Service returns the underlying embedding Service for advanced use cases.
// This should be used sparingly as it breaks the abstraction.
func (p *Plugin) Service() *infraEmbed.Service {
	return p.service
}

// Verify Plugin implements EmbeddingPlugin interface
var _ plugin.EmbeddingPlugin = (*Plugin)(nil)
