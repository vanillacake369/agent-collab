package embedding

import (
	"context"

	"agent-collab/src/domain/ports"
)

// PortsAdapter wraps an embedding.Service to implement ports.EmbeddingService interface.
type PortsAdapter struct {
	service *Service
}

// NewPortsAdapter creates a new adapter that wraps an embedding.Service.
func NewPortsAdapter(service *Service) ports.EmbeddingService {
	return &PortsAdapter{service: service}
}

// Embed generates an embedding for a single text.
func (a *PortsAdapter) Embed(ctx context.Context, text string) ([]float32, error) {
	return a.service.Embed(ctx, text)
}

// EmbedBatch generates embeddings for multiple texts.
func (a *PortsAdapter) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	return a.service.EmbedBatch(ctx, texts)
}

// Dimension returns the embedding dimension.
func (a *PortsAdapter) Dimension() int {
	return a.service.Dimension()
}

// Model returns the model name.
func (a *PortsAdapter) Model() string {
	return a.service.Model()
}
