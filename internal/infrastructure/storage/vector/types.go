package vector

import (
	"time"
)

// Dimension is the embedding vector dimension.
const DefaultDimension = 1536 // OpenAI text-embedding-3-small

// Document represents a stored document with its embedding.
type Document struct {
	ID         string         `json:"id"`
	Collection string         `json:"collection"`
	Content    string         `json:"content"`
	Embedding  []float32      `json:"embedding"`
	Metadata   map[string]any `json:"metadata"`
	FilePath   string         `json:"file_path,omitempty"`
	StartLine  int            `json:"start_line,omitempty"`
	EndLine    int            `json:"end_line,omitempty"`
	Language   string         `json:"language,omitempty"`
	SymbolType string         `json:"symbol_type,omitempty"` // function, class, method
	SymbolName string         `json:"symbol_name,omitempty"`
	Hash       string         `json:"hash"`
	CreatedAt  time.Time      `json:"created_at"`
	UpdatedAt  time.Time      `json:"updated_at"`
}

// SearchResult represents a search result with similarity score.
type SearchResult struct {
	Document *Document `json:"document"`
	Score    float32   `json:"score"` // Similarity score (0-1, higher is better)
	Distance float32   `json:"distance"`
}

// SearchOptions configures vector search behavior.
type SearchOptions struct {
	Collection string         `json:"collection,omitempty"`
	TopK       int            `json:"top_k"`
	MinScore   float32        `json:"min_score,omitempty"`
	Filters    map[string]any `json:"filters,omitempty"`
	FilePath   string         `json:"file_path,omitempty"`
	Language   string         `json:"language,omitempty"`
}

// DefaultSearchOptions returns default search options.
func DefaultSearchOptions() *SearchOptions {
	return &SearchOptions{
		TopK:     10,
		MinScore: 0.0,
	}
}

// CollectionStats holds statistics for a collection.
type CollectionStats struct {
	Name      string    `json:"name"`
	Count     int64     `json:"count"`
	Dimension int       `json:"dimension"`
	SizeBytes int64     `json:"size_bytes"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Store is the interface for vector storage backends.
type Store interface {
	// Collection management
	CreateCollection(name string, dimension int) error
	DeleteCollection(name string) error
	ListCollections() ([]string, error)
	GetCollectionStats(name string) (*CollectionStats, error)

	// Document operations
	Insert(doc *Document) error
	InsertBatch(docs []*Document) error
	Get(collection, id string) (*Document, error)
	Delete(collection, id string) error
	DeleteByFilter(collection string, filter map[string]any) (int64, error)

	// Search
	Search(embedding []float32, opts *SearchOptions) ([]*SearchResult, error)
	SearchByText(text string, opts *SearchOptions) ([]*SearchResult, error)

	// Maintenance
	Flush() error
	Close() error
}
