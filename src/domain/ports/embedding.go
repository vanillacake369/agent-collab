package ports

import "context"

// EmbeddingService is the port for embedding generation operations.
// Infrastructure layer provides the concrete implementation.
type EmbeddingService interface {
	// Embed generates an embedding for a single text.
	Embed(ctx context.Context, text string) ([]float32, error)

	// EmbedBatch generates embeddings for multiple texts.
	EmbedBatch(ctx context.Context, texts []string) ([][]float32, error)

	// Dimension returns the embedding dimension.
	Dimension() int

	// Model returns the model name.
	Model() string
}
