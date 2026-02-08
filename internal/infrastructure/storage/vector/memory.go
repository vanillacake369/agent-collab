package vector

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

// MemoryStore is an in-memory vector store with persistence.
type MemoryStore struct {
	mu          sync.RWMutex
	collections map[string]*collection
	dataDir     string
	dimension   int
	embedFn     func(text string) ([]float32, error)
}

type collection struct {
	Name      string               `json:"name"`
	Dimension int                  `json:"dimension"`
	Documents map[string]*Document `json:"documents"`
	CreatedAt time.Time            `json:"created_at"`
	UpdatedAt time.Time            `json:"updated_at"`
}

// NewMemoryStore creates a new in-memory vector store.
func NewMemoryStore(dataDir string, dimension int) (*MemoryStore, error) {
	if dimension <= 0 {
		dimension = DefaultDimension
	}

	vectorDir := filepath.Join(dataDir, "vectors")
	if err := os.MkdirAll(vectorDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create vector dir: %w", err)
	}

	store := &MemoryStore{
		collections: make(map[string]*collection),
		dataDir:     vectorDir,
		dimension:   dimension,
	}

	// Load existing collections
	if err := store.load(); err != nil {
		return nil, err
	}

	return store, nil
}

// SetEmbeddingFunction sets the function used for text-to-vector conversion.
func (s *MemoryStore) SetEmbeddingFunction(fn func(text string) ([]float32, error)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.embedFn = fn
}

// CreateCollection creates a new collection.
func (s *MemoryStore) CreateCollection(name string, dimension int) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.collections[name]; exists {
		return fmt.Errorf("collection already exists: %s", name)
	}

	if dimension <= 0 {
		dimension = s.dimension
	}

	s.collections[name] = &collection{
		Name:      name,
		Dimension: dimension,
		Documents: make(map[string]*Document),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	return s.persist()
}

// DeleteCollection deletes a collection.
func (s *MemoryStore) DeleteCollection(name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.collections[name]; !exists {
		return fmt.Errorf("collection not found: %s", name)
	}

	delete(s.collections, name)
	return s.persist()
}

// ListCollections returns all collection names.
func (s *MemoryStore) ListCollections() ([]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	names := make([]string, 0, len(s.collections))
	for name := range s.collections {
		names = append(names, name)
	}
	return names, nil
}

// GetCollectionStats returns statistics for a collection.
func (s *MemoryStore) GetCollectionStats(name string) (*CollectionStats, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	coll, exists := s.collections[name]
	if !exists {
		return nil, fmt.Errorf("collection not found: %s", name)
	}

	// Estimate size
	var sizeBytes int64
	for _, doc := range coll.Documents {
		sizeBytes += int64(len(doc.Content))
		sizeBytes += int64(len(doc.Embedding) * 4) // float32 = 4 bytes
	}

	return &CollectionStats{
		Name:      name,
		Count:     int64(len(coll.Documents)),
		Dimension: coll.Dimension,
		SizeBytes: sizeBytes,
		CreatedAt: coll.CreatedAt,
		UpdatedAt: coll.UpdatedAt,
	}, nil
}

// Insert inserts a document.
func (s *MemoryStore) Insert(doc *Document) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	collName := doc.Collection
	if collName == "" {
		collName = "default"
	}

	coll, exists := s.collections[collName]
	if !exists {
		// Auto-create collection
		coll = &collection{
			Name:      collName,
			Dimension: len(doc.Embedding),
			Documents: make(map[string]*Document),
			CreatedAt: time.Now(),
		}
		s.collections[collName] = coll
	}

	// Generate ID if not provided
	if doc.ID == "" {
		doc.ID = generateDocID(doc.Content)
	}

	// Set timestamps
	now := time.Now()
	if doc.CreatedAt.IsZero() {
		doc.CreatedAt = now
	}
	doc.UpdatedAt = now

	// Compute hash if not provided
	if doc.Hash == "" {
		doc.Hash = computeHash(doc.Content)
	}

	coll.Documents[doc.ID] = doc
	coll.UpdatedAt = now

	return nil
}

// InsertBatch inserts multiple documents.
func (s *MemoryStore) InsertBatch(docs []*Document) error {
	for _, doc := range docs {
		if err := s.Insert(doc); err != nil {
			return err
		}
	}
	return s.Flush()
}

// Get retrieves a document by ID.
func (s *MemoryStore) Get(collectionName, id string) (*Document, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	coll, exists := s.collections[collectionName]
	if !exists {
		return nil, fmt.Errorf("collection not found: %s", collectionName)
	}

	doc, exists := coll.Documents[id]
	if !exists {
		return nil, fmt.Errorf("document not found: %s", id)
	}

	return doc, nil
}

// Delete removes a document.
func (s *MemoryStore) Delete(collectionName, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	coll, exists := s.collections[collectionName]
	if !exists {
		return fmt.Errorf("collection not found: %s", collectionName)
	}

	delete(coll.Documents, id)
	coll.UpdatedAt = time.Now()

	return nil
}

// DeleteByFilter deletes documents matching a filter.
func (s *MemoryStore) DeleteByFilter(collectionName string, filter map[string]any) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	coll, exists := s.collections[collectionName]
	if !exists {
		return 0, fmt.Errorf("collection not found: %s", collectionName)
	}

	var deleted int64
	for id, doc := range coll.Documents {
		if matchesFilter(doc, filter) {
			delete(coll.Documents, id)
			deleted++
		}
	}

	coll.UpdatedAt = time.Now()
	return deleted, nil
}

// Search performs vector similarity search.
func (s *MemoryStore) Search(embedding []float32, opts *SearchOptions) ([]*SearchResult, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if opts == nil {
		opts = DefaultSearchOptions()
	}

	var results []*SearchResult

	// Determine which collections to search
	collectionsToSearch := make([]*collection, 0)
	if opts.Collection != "" {
		if coll, exists := s.collections[opts.Collection]; exists {
			collectionsToSearch = append(collectionsToSearch, coll)
		}
	} else {
		for _, coll := range s.collections {
			collectionsToSearch = append(collectionsToSearch, coll)
		}
	}

	// Search each collection
	for _, coll := range collectionsToSearch {
		for _, doc := range coll.Documents {
			// Apply filters
			if opts.FilePath != "" && doc.FilePath != opts.FilePath {
				continue
			}
			if opts.Language != "" && doc.Language != opts.Language {
				continue
			}
			if opts.Filters != nil && !matchesFilter(doc, opts.Filters) {
				continue
			}

			// Calculate similarity
			score := cosineSimilarity(embedding, doc.Embedding)
			if score < opts.MinScore {
				continue
			}

			results = append(results, &SearchResult{
				Document: doc,
				Score:    score,
				Distance: 1 - score,
			})
		}
	}

	// Sort by score descending
	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})

	// Limit to top-K
	if len(results) > opts.TopK {
		results = results[:opts.TopK]
	}

	return results, nil
}

// SearchByText searches using text (requires embedding function).
func (s *MemoryStore) SearchByText(text string, opts *SearchOptions) ([]*SearchResult, error) {
	s.mu.RLock()
	embedFn := s.embedFn
	s.mu.RUnlock()

	if embedFn == nil {
		return nil, fmt.Errorf("embedding function not set")
	}

	embedding, err := embedFn(text)
	if err != nil {
		return nil, fmt.Errorf("failed to generate embedding: %w", err)
	}

	return s.Search(embedding, opts)
}

// Flush persists data to disk.
func (s *MemoryStore) Flush() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.persist()
}

// Close closes the store.
func (s *MemoryStore) Close() error {
	return s.Flush()
}

// persist saves all collections to disk.
func (s *MemoryStore) persist() error {
	for name, coll := range s.collections {
		path := filepath.Join(s.dataDir, name+".json")
		data, err := json.MarshalIndent(coll, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal collection %s: %w", name, err)
		}
		if err := os.WriteFile(path, data, 0644); err != nil {
			return fmt.Errorf("failed to write collection %s: %w", name, err)
		}
	}
	return nil
}

// load reads all collections from disk.
func (s *MemoryStore) load() error {
	entries, err := os.ReadDir(s.dataDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}

		path := filepath.Join(s.dataDir, entry.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}

		var coll collection
		if err := json.Unmarshal(data, &coll); err != nil {
			continue
		}

		s.collections[coll.Name] = &coll
	}

	return nil
}

// cosineSimilarity calculates cosine similarity between two vectors.
func cosineSimilarity(a, b []float32) float32 {
	if len(a) != len(b) || len(a) == 0 {
		return 0
	}

	var dotProduct, normA, normB float64
	for i := range a {
		dotProduct += float64(a[i]) * float64(b[i])
		normA += float64(a[i]) * float64(a[i])
		normB += float64(b[i]) * float64(b[i])
	}

	if normA == 0 || normB == 0 {
		return 0
	}

	return float32(dotProduct / (math.Sqrt(normA) * math.Sqrt(normB)))
}

// matchesFilter checks if a document matches a filter.
func matchesFilter(doc *Document, filter map[string]any) bool {
	for key, value := range filter {
		switch key {
		case "file_path":
			if doc.FilePath != value {
				return false
			}
		case "language":
			if doc.Language != value {
				return false
			}
		case "symbol_type":
			if doc.SymbolType != value {
				return false
			}
		case "symbol_name":
			if doc.SymbolName != value {
				return false
			}
		default:
			// Check metadata
			if doc.Metadata != nil {
				if metaVal, exists := doc.Metadata[key]; exists {
					if metaVal != value {
						return false
					}
				} else {
					return false
				}
			}
		}
	}
	return true
}

// generateDocID generates a document ID from content.
func generateDocID(content string) string {
	hash := sha256.Sum256([]byte(content))
	return "doc-" + hex.EncodeToString(hash[:8])
}

// computeHash computes a hash of content.
func computeHash(content string) string {
	hash := sha256.Sum256([]byte(content))
	return hex.EncodeToString(hash[:16])
}
