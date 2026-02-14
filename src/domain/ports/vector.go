// Package ports defines interfaces for external dependencies.
// This follows the Ports and Adapters (Hexagonal) architecture pattern.
// Domain layer defines the ports (interfaces), infrastructure implements them.
package ports

import "time"

// VectorDocument represents a stored document with its embedding.
type VectorDocument struct {
	ID         string
	Collection string
	Content    string
	Embedding  []float32
	Metadata   map[string]any
	FilePath   string
	StartLine  int
	EndLine    int
	Language   string
	SymbolType string // function, class, method
	SymbolName string
	Hash       string
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

// VectorSearchResult represents a search result with similarity score.
type VectorSearchResult struct {
	Document *VectorDocument
	Score    float32 // Similarity score (0-1, higher is better)
	Distance float32
}

// VectorSearchOptions configures vector search behavior.
type VectorSearchOptions struct {
	Collection string
	TopK       int
	MinScore   float32
	Filters    map[string]any
	FilePath   string
	Language   string
}

// VectorCollectionStats holds statistics for a collection.
type VectorCollectionStats struct {
	Name      string
	Count     int64
	Dimension int
	SizeBytes int64
	CreatedAt time.Time
	UpdatedAt time.Time
}

// VectorWriter is the port for writing documents to vector storage.
type VectorWriter interface {
	Insert(doc *VectorDocument) error
	InsertBatch(docs []*VectorDocument) error
}

// VectorSearcher is the port for searching vector storage.
type VectorSearcher interface {
	Search(embedding []float32, opts *VectorSearchOptions) ([]*VectorSearchResult, error)
	SearchByText(text string, opts *VectorSearchOptions) ([]*VectorSearchResult, error)
}

// VectorReader is the port for reading documents from vector storage.
type VectorReader interface {
	Get(collection, id string) (*VectorDocument, error)
}

// VectorCollectionManager is the port for managing vector collections.
type VectorCollectionManager interface {
	CreateCollection(name string, dimension int) error
	DeleteCollection(name string) error
	ListCollections() ([]string, error)
	GetCollectionStats(name string) (*VectorCollectionStats, error)
}

// VectorMaintainer is the port for maintenance operations.
type VectorMaintainer interface {
	Flush() error
	Close() error
}

// VectorDeleter is the port for deleting documents.
type VectorDeleter interface {
	Delete(collection, id string) error
	DeleteByFilter(collection string, filter map[string]any) (int64, error)
}

// VectorStore is the full port for vector storage operations.
// Use the smaller interfaces when only specific operations are needed.
type VectorStore interface {
	VectorWriter
	VectorSearcher
	VectorReader
	VectorCollectionManager
	VectorMaintainer
	VectorDeleter
}
