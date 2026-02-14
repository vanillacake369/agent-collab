package vector

import (
	"agent-collab/src/domain/ports"
)

// PortsAdapter wraps a vector.Store to implement ports.VectorStore interface.
// This is used to bridge the infrastructure implementation with the domain port.
type PortsAdapter struct {
	store Store
}

// NewPortsAdapter creates a new adapter that wraps a vector.Store.
func NewPortsAdapter(store Store) ports.VectorStore {
	return &PortsAdapter{store: store}
}

// Collection management

func (a *PortsAdapter) CreateCollection(name string, dimension int) error {
	return a.store.CreateCollection(name, dimension)
}

func (a *PortsAdapter) DeleteCollection(name string) error {
	return a.store.DeleteCollection(name)
}

func (a *PortsAdapter) ListCollections() ([]string, error) {
	return a.store.ListCollections()
}

func (a *PortsAdapter) GetCollectionStats(name string) (*ports.VectorCollectionStats, error) {
	stats, err := a.store.GetCollectionStats(name)
	if err != nil {
		return nil, err
	}
	return &ports.VectorCollectionStats{
		Name:      stats.Name,
		Count:     stats.Count,
		Dimension: stats.Dimension,
		SizeBytes: stats.SizeBytes,
		CreatedAt: stats.CreatedAt,
		UpdatedAt: stats.UpdatedAt,
	}, nil
}

// Document operations

func (a *PortsAdapter) Insert(doc *ports.VectorDocument) error {
	return a.store.Insert(toInfraDocument(doc))
}

func (a *PortsAdapter) InsertBatch(docs []*ports.VectorDocument) error {
	infraDocs := make([]*Document, len(docs))
	for i, doc := range docs {
		infraDocs[i] = toInfraDocument(doc)
	}
	return a.store.InsertBatch(infraDocs)
}

func (a *PortsAdapter) Get(collection, id string) (*ports.VectorDocument, error) {
	doc, err := a.store.Get(collection, id)
	if err != nil {
		return nil, err
	}
	if doc == nil {
		return nil, nil
	}
	return toPortsDocument(doc), nil
}

func (a *PortsAdapter) Delete(collection, id string) error {
	return a.store.Delete(collection, id)
}

func (a *PortsAdapter) DeleteByFilter(collection string, filter map[string]any) (int64, error) {
	return a.store.DeleteByFilter(collection, filter)
}

// Search

func (a *PortsAdapter) Search(embedding []float32, opts *ports.VectorSearchOptions) ([]*ports.VectorSearchResult, error) {
	results, err := a.store.Search(embedding, toInfraSearchOptions(opts))
	if err != nil {
		return nil, err
	}
	return toPortsSearchResults(results), nil
}

func (a *PortsAdapter) SearchByText(text string, opts *ports.VectorSearchOptions) ([]*ports.VectorSearchResult, error) {
	results, err := a.store.SearchByText(text, toInfraSearchOptions(opts))
	if err != nil {
		return nil, err
	}
	return toPortsSearchResults(results), nil
}

// Maintenance

func (a *PortsAdapter) Flush() error {
	return a.store.Flush()
}

func (a *PortsAdapter) Close() error {
	return a.store.Close()
}

// Conversion helpers

func toInfraDocument(doc *ports.VectorDocument) *Document {
	if doc == nil {
		return nil
	}
	return &Document{
		ID:         doc.ID,
		Collection: doc.Collection,
		Content:    doc.Content,
		Embedding:  doc.Embedding,
		Metadata:   doc.Metadata,
		FilePath:   doc.FilePath,
		StartLine:  doc.StartLine,
		EndLine:    doc.EndLine,
		Language:   doc.Language,
		SymbolType: doc.SymbolType,
		SymbolName: doc.SymbolName,
		Hash:       doc.Hash,
		CreatedAt:  doc.CreatedAt,
		UpdatedAt:  doc.UpdatedAt,
	}
}

func toPortsDocument(doc *Document) *ports.VectorDocument {
	if doc == nil {
		return nil
	}
	return &ports.VectorDocument{
		ID:         doc.ID,
		Collection: doc.Collection,
		Content:    doc.Content,
		Embedding:  doc.Embedding,
		Metadata:   doc.Metadata,
		FilePath:   doc.FilePath,
		StartLine:  doc.StartLine,
		EndLine:    doc.EndLine,
		Language:   doc.Language,
		SymbolType: doc.SymbolType,
		SymbolName: doc.SymbolName,
		Hash:       doc.Hash,
		CreatedAt:  doc.CreatedAt,
		UpdatedAt:  doc.UpdatedAt,
	}
}

func toInfraSearchOptions(opts *ports.VectorSearchOptions) *SearchOptions {
	if opts == nil {
		return nil
	}
	return &SearchOptions{
		Collection: opts.Collection,
		TopK:       opts.TopK,
		MinScore:   opts.MinScore,
		Filters:    opts.Filters,
		FilePath:   opts.FilePath,
		Language:   opts.Language,
	}
}

func toPortsSearchResults(results []*SearchResult) []*ports.VectorSearchResult {
	portsResults := make([]*ports.VectorSearchResult, len(results))
	for i, r := range results {
		portsResults[i] = &ports.VectorSearchResult{
			Document: toPortsDocument(r.Document),
			Score:    r.Score,
			Distance: r.Distance,
		}
	}
	return portsResults
}
