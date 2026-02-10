package cohesion

import (
	"context"
	"fmt"
	"strings"

	"agent-collab/src/infrastructure/embedding"
	"agent-collab/src/infrastructure/storage/vector"
)

// Thresholds for cohesion detection.
const (
	// SimilarityThresholdHigh indicates strong relevance.
	SimilarityThresholdHigh = 0.85

	// SimilarityThresholdMedium indicates moderate relevance.
	SimilarityThresholdMedium = 0.70

	// SimilarityThresholdLow indicates weak relevance.
	SimilarityThresholdLow = 0.55

	// DefaultSearchLimit is the default number of contexts to search.
	DefaultSearchLimit = 10
)

// ConflictIndicators are words that suggest potential conflicts.
var ConflictIndicators = []string{
	"instead", "replace", "remove", "delete", "change from", "migrate",
	"switch to", "convert", "deprecated", "obsolete", "no longer",
	"대신", "변경", "제거", "삭제", "전환", "마이그레이션",
}

// Checker performs cohesion checks against existing contexts.
type Checker struct {
	vectorStore  vector.Store
	embedService *embedding.Service
}

// NewChecker creates a new cohesion checker.
func NewChecker(vectorStore vector.Store, embedService *embedding.Service) *Checker {
	return &Checker{
		vectorStore:  vectorStore,
		embedService: embedService,
	}
}

// Check performs a cohesion check based on the request.
func (c *Checker) Check(ctx context.Context, req *CheckRequest) (*CheckResult, error) {
	if req == nil {
		return nil, fmt.Errorf("check request is nil")
	}

	switch req.Type {
	case CheckTypeBefore:
		return c.checkBefore(ctx, req)
	case CheckTypeAfter:
		return c.checkAfter(ctx, req)
	default:
		return nil, fmt.Errorf("unknown check type: %s", req.Type)
	}
}

// checkBefore checks if the intention aligns with existing contexts.
func (c *Checker) checkBefore(ctx context.Context, req *CheckRequest) (*CheckResult, error) {
	if req.Intention == "" {
		return nil, fmt.Errorf("intention is required for before check")
	}

	// Generate embedding for the intention
	intentionEmb, err := c.embedService.Embed(ctx, req.Intention)
	if err != nil {
		return nil, fmt.Errorf("failed to embed intention: %w", err)
	}

	// Search for related contexts
	results, err := c.vectorStore.Search(intentionEmb, &vector.SearchOptions{
		Collection: "default",
		TopK:       DefaultSearchLimit,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to search contexts: %w", err)
	}

	return c.analyzeResults(req.Intention, results)
}

// checkAfter checks if the result aligns with existing contexts.
func (c *Checker) checkAfter(ctx context.Context, req *CheckRequest) (*CheckResult, error) {
	if req.Result == "" {
		return nil, fmt.Errorf("result is required for after check")
	}

	// Generate embedding for the result
	resultEmb, err := c.embedService.Embed(ctx, req.Result)
	if err != nil {
		return nil, fmt.Errorf("failed to embed result: %w", err)
	}

	// Search for related contexts
	results, err := c.vectorStore.Search(resultEmb, &vector.SearchOptions{
		Collection: "default",
		TopK:       DefaultSearchLimit,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to search contexts: %w", err)
	}

	return c.analyzeResults(req.Result, results)
}

// analyzeResults analyzes search results and determines cohesion.
func (c *Checker) analyzeResults(query string, results []*vector.SearchResult) (*CheckResult, error) {
	result := &CheckResult{
		Verdict:         VerdictCohesive,
		Confidence:      1.0,
		RelatedContexts: make([]RelatedContext, 0),
		Suggestions:     make([]string, 0),
	}

	// No existing contexts - cohesive by default
	if len(results) == 0 {
		result.Message = "No related contexts found. Proceeding is safe."
		result.Suggestions = append(result.Suggestions,
			"Consider sharing context after completing this work")
		return result, nil
	}

	queryLower := strings.ToLower(query)
	hasConflictIndicator := c.hasConflictIndicators(queryLower)

	var potentialConflicts []ConflictInfo

	for _, sr := range results {
		if sr.Document == nil {
			continue
		}

		related := RelatedContext{
			ID:         sr.Document.ID,
			FilePath:   sr.Document.FilePath,
			Content:    sr.Document.Content,
			Similarity: sr.Score,
			CreatedAt:  sr.Document.CreatedAt,
		}

		// Extract agent from metadata if available
		if sr.Document.Metadata != nil {
			if agent, ok := sr.Document.Metadata["agent"].(string); ok {
				related.Agent = agent
			}
		}

		result.RelatedContexts = append(result.RelatedContexts, related)

		// Check for potential conflicts
		if conflict := c.detectConflict(query, sr, hasConflictIndicator); conflict != nil {
			potentialConflicts = append(potentialConflicts, *conflict)
		}
	}

	result.PotentialConflicts = potentialConflicts

	// Determine verdict based on conflicts
	if len(potentialConflicts) > 0 {
		result.Verdict = VerdictConflict

		// Calculate confidence based on highest conflict severity
		maxSeverity := "low"
		for _, conflict := range potentialConflicts {
			if conflict.Severity == "high" {
				maxSeverity = "high"
				break
			} else if conflict.Severity == "medium" && maxSeverity == "low" {
				maxSeverity = "medium"
			}
		}

		switch maxSeverity {
		case "high":
			result.Confidence = 0.9
		case "medium":
			result.Confidence = 0.7
		default:
			result.Confidence = 0.5
		}

		result.Message = fmt.Sprintf("Potential conflict detected with %d existing context(s)",
			len(potentialConflicts))

		// Add suggestions
		result.Suggestions = append(result.Suggestions,
			"Review the related contexts before proceeding",
			"Consider discussing with the team if this changes existing decisions",
			"Share context after completing work to inform other agents",
		)
	} else if len(result.RelatedContexts) > 0 {
		// Related but not conflicting
		highSimilarity := false
		for _, rc := range result.RelatedContexts {
			if rc.Similarity >= SimilarityThresholdHigh {
				highSimilarity = true
				break
			}
		}

		if highSimilarity {
			result.Message = "Related contexts found. Your work aligns with existing context."
			result.Confidence = 0.85
		} else {
			result.Message = "Some related contexts found. Proceeding appears safe."
			result.Confidence = 0.75
		}

		result.Suggestions = append(result.Suggestions,
			"Review related contexts for additional information",
		)
	}

	return result, nil
}

// hasConflictIndicators checks if the query contains conflict indicators.
func (c *Checker) hasConflictIndicators(query string) bool {
	for _, indicator := range ConflictIndicators {
		if strings.Contains(query, strings.ToLower(indicator)) {
			return true
		}
	}
	return false
}

// detectConflict checks if a search result potentially conflicts with the query.
func (c *Checker) detectConflict(query string, sr *vector.SearchResult, hasIndicator bool) *ConflictInfo {
	if sr.Document == nil {
		return nil
	}

	queryLower := strings.ToLower(query)
	contentLower := strings.ToLower(sr.Document.Content)

	// High similarity + conflict indicators = potential conflict
	if sr.Score >= SimilarityThresholdHigh && hasIndicator {
		return &ConflictInfo{
			Context: RelatedContext{
				ID:         sr.Document.ID,
				FilePath:   sr.Document.FilePath,
				Content:    sr.Document.Content,
				Similarity: sr.Score,
			},
			Reason:   "High similarity with change indicators detected",
			Severity: "high",
		}
	}

	// Check for opposing patterns
	opposingPatterns := []struct {
		pattern1 string
		pattern2 string
		reason   string
	}{
		{"jwt", "session", "Conflicting authentication approach"},
		{"session", "jwt", "Conflicting authentication approach"},
		{"rest", "graphql", "Conflicting API style"},
		{"graphql", "rest", "Conflicting API style"},
		{"sql", "nosql", "Conflicting database approach"},
		{"nosql", "sql", "Conflicting database approach"},
		{"monolith", "microservice", "Conflicting architecture"},
		{"microservice", "monolith", "Conflicting architecture"},
		{"sync", "async", "Conflicting execution model"},
		{"async", "sync", "Conflicting execution model"},
	}

	for _, op := range opposingPatterns {
		if strings.Contains(queryLower, op.pattern1) && strings.Contains(contentLower, op.pattern2) {
			severity := "medium"
			if sr.Score >= SimilarityThresholdMedium {
				severity = "high"
			}
			return &ConflictInfo{
				Context: RelatedContext{
					ID:         sr.Document.ID,
					FilePath:   sr.Document.FilePath,
					Content:    sr.Document.Content,
					Similarity: sr.Score,
				},
				Reason:   op.reason,
				Severity: severity,
			}
		}
	}

	// Medium-high similarity in same file area might need attention
	if sr.Score >= SimilarityThresholdMedium && hasIndicator {
		return &ConflictInfo{
			Context: RelatedContext{
				ID:         sr.Document.ID,
				FilePath:   sr.Document.FilePath,
				Content:    sr.Document.Content,
				Similarity: sr.Score,
			},
			Reason:   "Related context exists for this area",
			Severity: "low",
		}
	}

	return nil
}
