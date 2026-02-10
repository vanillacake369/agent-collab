package cohesion

import (
	"context"
	"testing"
	"time"

	"agent-collab/src/infrastructure/embedding"
	"agent-collab/src/infrastructure/storage/vector"
)

// mockVectorStore implements vector.Store for testing.
type mockVectorStore struct {
	documents []*vector.Document
}

func newMockVectorStore() *mockVectorStore {
	return &mockVectorStore{
		documents: make([]*vector.Document, 0),
	}
}

func (m *mockVectorStore) CreateCollection(name string, dimension int) error { return nil }
func (m *mockVectorStore) DeleteCollection(name string) error                { return nil }
func (m *mockVectorStore) ListCollections() ([]string, error)                { return []string{"default"}, nil }
func (m *mockVectorStore) GetCollectionStats(name string) (*vector.CollectionStats, error) {
	return &vector.CollectionStats{Name: name}, nil
}

func (m *mockVectorStore) Insert(doc *vector.Document) error {
	doc.ID = "doc-" + doc.Content[:min(8, len(doc.Content))]
	doc.CreatedAt = time.Now()
	m.documents = append(m.documents, doc)
	return nil
}

func (m *mockVectorStore) InsertBatch(docs []*vector.Document) error {
	for _, doc := range docs {
		m.Insert(doc)
	}
	return nil
}

func (m *mockVectorStore) Get(collection, id string) (*vector.Document, error) {
	for _, doc := range m.documents {
		if doc.ID == id {
			return doc, nil
		}
	}
	return nil, nil
}

func (m *mockVectorStore) Delete(collection, id string) error { return nil }
func (m *mockVectorStore) DeleteByFilter(collection string, filter map[string]any) (int64, error) {
	return 0, nil
}

func (m *mockVectorStore) Search(emb []float32, opts *vector.SearchOptions) ([]*vector.SearchResult, error) {
	results := make([]*vector.SearchResult, 0)
	for _, doc := range m.documents {
		// Simple mock: return all documents with fake similarity scores
		results = append(results, &vector.SearchResult{
			Document: doc,
			Score:    0.8, // Default high similarity
		})
	}
	if opts != nil && opts.TopK > 0 && len(results) > opts.TopK {
		results = results[:opts.TopK]
	}
	return results, nil
}

func (m *mockVectorStore) SearchByText(text string, opts *vector.SearchOptions) ([]*vector.SearchResult, error) {
	return m.Search(nil, opts)
}

func (m *mockVectorStore) Flush() error { return nil }
func (m *mockVectorStore) Close() error { return nil }

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// TestCheckBefore_NoPreviousContext tests cohesion check when no context exists.
func TestCheckBefore_NoPreviousContext(t *testing.T) {
	store := newMockVectorStore()
	embedService := embedding.NewService(&embedding.Config{
		Provider: embedding.ProviderMock,
	})

	checker := NewChecker(store, embedService)

	result, err := checker.Check(context.Background(), &CheckRequest{
		Type:      CheckTypeBefore,
		Intention: "Implement JWT authentication",
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Verdict != VerdictCohesive {
		t.Errorf("expected VerdictCohesive, got %s", result.Verdict)
	}

	if len(result.RelatedContexts) != 0 {
		t.Errorf("expected no related contexts, got %d", len(result.RelatedContexts))
	}
}

// TestCheckBefore_WithRelatedContext tests cohesion check when related context exists.
func TestCheckBefore_WithRelatedContext(t *testing.T) {
	store := newMockVectorStore()
	embedService := embedding.NewService(&embedding.Config{
		Provider: embedding.ProviderMock,
	})

	// Add existing context
	store.Insert(&vector.Document{
		Content:  "Implemented JWT token validation with expiry checking",
		FilePath: "auth/handler.go",
		Metadata: map[string]any{"agent": "Agent-A"},
	})

	checker := NewChecker(store, embedService)

	result, err := checker.Check(context.Background(), &CheckRequest{
		Type:      CheckTypeBefore,
		Intention: "Add JWT refresh token support",
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result.RelatedContexts) == 0 {
		t.Error("expected related contexts, got none")
	}

	// Should be cohesive since we're extending JWT, not conflicting
	if result.Verdict != VerdictCohesive {
		t.Logf("verdict: %s, message: %s", result.Verdict, result.Message)
	}
}

// TestCheckBefore_ConflictingApproach tests detection of conflicting approaches.
func TestCheckBefore_ConflictingApproach(t *testing.T) {
	store := newMockVectorStore()
	embedService := embedding.NewService(&embedding.Config{
		Provider: embedding.ProviderMock,
	})

	// Add existing context about JWT
	store.Insert(&vector.Document{
		Content:  "Implemented JWT-based stateless authentication",
		FilePath: "auth/handler.go",
	})

	checker := NewChecker(store, embedService)

	result, err := checker.Check(context.Background(), &CheckRequest{
		Type:      CheckTypeBefore,
		Intention: "Switch to session-based authentication instead of JWT",
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should detect potential conflict
	if result.Verdict != VerdictConflict {
		t.Errorf("expected VerdictConflict, got %s. Message: %s", result.Verdict, result.Message)
	}

	if len(result.PotentialConflicts) == 0 {
		t.Error("expected potential conflicts, got none")
	}

	if len(result.Suggestions) == 0 {
		t.Error("expected suggestions, got none")
	}
}

// TestCheckAfter_WithResult tests after-work cohesion check.
func TestCheckAfter_WithResult(t *testing.T) {
	store := newMockVectorStore()
	embedService := embedding.NewService(&embedding.Config{
		Provider: embedding.ProviderMock,
	})

	// Add existing context
	store.Insert(&vector.Document{
		Content:  "API uses REST endpoints with JSON responses",
		FilePath: "api/routes.go",
	})

	checker := NewChecker(store, embedService)

	result, err := checker.Check(context.Background(), &CheckRequest{
		Type:         CheckTypeAfter,
		Result:       "Added new REST endpoint for user profile",
		FilesChanged: []string{"api/routes.go", "api/handlers/profile.go"},
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should be cohesive since we're following REST pattern
	if result.Verdict != VerdictCohesive {
		t.Logf("verdict: %s, message: %s", result.Verdict, result.Message)
	}
}

// TestCheckAfter_ConflictingChange tests detection of conflicting changes.
func TestCheckAfter_ConflictingChange(t *testing.T) {
	store := newMockVectorStore()
	embedService := embedding.NewService(&embedding.Config{
		Provider: embedding.ProviderMock,
	})

	// Add existing context about REST API
	store.Insert(&vector.Document{
		Content:  "All APIs follow REST principles with JSON",
		FilePath: "api/routes.go",
	})

	checker := NewChecker(store, embedService)

	result, err := checker.Check(context.Background(), &CheckRequest{
		Type:         CheckTypeAfter,
		Result:       "Replaced REST API with GraphQL endpoint",
		FilesChanged: []string{"api/graphql.go"},
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should detect conflict (REST -> GraphQL)
	if result.Verdict != VerdictConflict {
		t.Errorf("expected VerdictConflict, got %s", result.Verdict)
	}
}

// TestCheck_InvalidRequest tests error handling for invalid requests.
func TestCheck_InvalidRequest(t *testing.T) {
	store := newMockVectorStore()
	embedService := embedding.NewService(&embedding.Config{
		Provider: embedding.ProviderMock,
	})

	checker := NewChecker(store, embedService)

	// Nil request
	_, err := checker.Check(context.Background(), nil)
	if err == nil {
		t.Error("expected error for nil request")
	}

	// Before without intention
	_, err = checker.Check(context.Background(), &CheckRequest{
		Type: CheckTypeBefore,
	})
	if err == nil {
		t.Error("expected error for before check without intention")
	}

	// After without result
	_, err = checker.Check(context.Background(), &CheckRequest{
		Type: CheckTypeAfter,
	})
	if err == nil {
		t.Error("expected error for after check without result")
	}

	// Unknown type
	_, err = checker.Check(context.Background(), &CheckRequest{
		Type: "unknown",
	})
	if err == nil {
		t.Error("expected error for unknown check type")
	}
}

// TestConflictIndicators tests detection of conflict indicator keywords.
func TestConflictIndicators(t *testing.T) {
	checker := &Checker{}

	testCases := []struct {
		query    string
		expected bool
	}{
		{"replace jwt with session", true},
		{"switch to graphql instead of rest", true},
		{"migrate from sql to nosql", true},
		{"add new feature", false},
		{"implement logging", false},
		{"JWT 대신 세션으로 변경", true},
		{"새 기능 추가", false},
	}

	for _, tc := range testCases {
		result := checker.hasConflictIndicators(tc.query)
		if result != tc.expected {
			t.Errorf("hasConflictIndicators(%q) = %v, expected %v", tc.query, result, tc.expected)
		}
	}
}
