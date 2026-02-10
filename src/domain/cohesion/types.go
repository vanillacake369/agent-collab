package cohesion

import "time"

// CheckType represents the type of cohesion check.
type CheckType string

const (
	// CheckTypeBefore is used before starting work to check intention alignment.
	CheckTypeBefore CheckType = "before"
	// CheckTypeAfter is used after completing work to check result alignment.
	CheckTypeAfter CheckType = "after"
)

// Verdict represents the cohesion check verdict.
type Verdict string

const (
	// VerdictCohesive means the intention/result aligns with existing context.
	VerdictCohesive Verdict = "cohesive"
	// VerdictConflict means potential conflict detected with existing context.
	VerdictConflict Verdict = "conflict"
	// VerdictUncertain means unable to determine, needs human judgment.
	VerdictUncertain Verdict = "uncertain"
)

// CheckRequest represents a cohesion check request.
type CheckRequest struct {
	// Type is the check type (before/after).
	Type CheckType `json:"type"`

	// Intention is used for "before" checks - what the agent intends to do.
	Intention string `json:"intention,omitempty"`

	// Result is used for "after" checks - what the agent has done.
	Result string `json:"result,omitempty"`

	// FilesChanged is used for "after" checks - list of modified files.
	FilesChanged []string `json:"files_changed,omitempty"`

	// Scope limits the search to specific file paths or patterns.
	Scope []string `json:"scope,omitempty"`
}

// CheckResult represents the cohesion check result.
type CheckResult struct {
	// Verdict is the overall cohesion verdict.
	Verdict Verdict `json:"verdict"`

	// Confidence is the confidence score (0.0 to 1.0).
	Confidence float32 `json:"confidence"`

	// RelatedContexts are existing contexts that are related to the check.
	RelatedContexts []RelatedContext `json:"related_contexts"`

	// PotentialConflicts are contexts that may conflict.
	PotentialConflicts []ConflictInfo `json:"potential_conflicts,omitempty"`

	// Suggestions are actionable suggestions for the agent.
	Suggestions []string `json:"suggestions,omitempty"`

	// Message is a human-readable summary.
	Message string `json:"message"`
}

// RelatedContext represents a related context found during check.
type RelatedContext struct {
	// ID is the context document ID.
	ID string `json:"id"`

	// Agent is the agent that shared this context.
	Agent string `json:"agent,omitempty"`

	// FilePath is the file path associated with the context.
	FilePath string `json:"file_path,omitempty"`

	// Content is the context content.
	Content string `json:"content"`

	// Similarity is the similarity score (0.0 to 1.0).
	Similarity float32 `json:"similarity"`

	// CreatedAt is when the context was created.
	CreatedAt time.Time `json:"created_at,omitempty"`
}

// ConflictInfo represents a potential conflict.
type ConflictInfo struct {
	// Context is the conflicting context.
	Context RelatedContext `json:"context"`

	// Reason explains why this might be a conflict.
	Reason string `json:"reason"`

	// Severity indicates conflict severity (low, medium, high).
	Severity string `json:"severity"`
}
