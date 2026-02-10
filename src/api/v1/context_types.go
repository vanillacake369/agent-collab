package v1

import (
	"time"
)

// ContextKind is the resource kind for Context.
const ContextKind = "Context"

// ContextPhase represents the current phase of a context lifecycle.
type ContextPhase string

const (
	// ContextPhasePending indicates the context is waiting to be synced.
	ContextPhasePending ContextPhase = "Pending"

	// ContextPhaseSyncing indicates the context is being synced.
	ContextPhaseSyncing ContextPhase = "Syncing"

	// ContextPhaseSynced indicates the context is fully synced.
	ContextPhaseSynced ContextPhase = "Synced"

	// ContextPhaseEmbedding indicates the context is being embedded.
	ContextPhaseEmbedding ContextPhase = "Embedding"

	// ContextPhaseReady indicates the context is synced and embedded.
	ContextPhaseReady ContextPhase = "Ready"

	// ContextPhaseFailed indicates the context sync/embedding failed.
	ContextPhaseFailed ContextPhase = "Failed"
)

// ContextType represents the type of context being shared.
type ContextType string

const (
	// ContextTypeFile indicates file content context.
	ContextTypeFile ContextType = "File"

	// ContextTypeDelta indicates a change/diff context.
	ContextTypeDelta ContextType = "Delta"

	// ContextTypeMessage indicates a message/note context.
	ContextTypeMessage ContextType = "Message"

	// ContextTypeSymbol indicates a code symbol context.
	ContextTypeSymbol ContextType = "Symbol"

	// ContextTypeDocument indicates a document context.
	ContextTypeDocument ContextType = "Document"
)

// Context represents shared context between agents.
// Context can be file changes, messages, or other information
// that agents need to share with each other.
type Context struct {
	TypeMeta   `json:",inline"`
	ObjectMeta `json:"metadata"`

	// Spec describes the context content and metadata.
	Spec ContextSpec `json:"spec"`

	// Status describes the current sync/embedding state.
	Status ContextStatus `json:"status,omitempty"`
}

// GetObjectMeta returns the ObjectMeta of the Context.
func (c *Context) GetObjectMeta() *ObjectMeta {
	return &c.ObjectMeta
}

// ContextSpec defines the content and metadata of shared context.
type ContextSpec struct {
	// Type is the type of context.
	Type ContextType `json:"type"`

	// SourceAgentID is the ID of the agent that created this context.
	SourceAgentID string `json:"sourceAgentId"`

	// SourceAgentName is the human-readable name of the source agent.
	SourceAgentName string `json:"sourceAgentName,omitempty"`

	// FilePath is the path to the file (for file-related context).
	FilePath string `json:"filePath,omitempty"`

	// Content is the actual context content.
	Content string `json:"content"`

	// Summary is a brief summary of the context.
	Summary string `json:"summary,omitempty"`

	// ContentHash is a hash of the content for deduplication.
	ContentHash string `json:"contentHash,omitempty"`

	// VectorClock tracks causal ordering across agents.
	VectorClock map[string]uint64 `json:"vectorClock,omitempty"`

	// Delta contains change information for delta-type contexts.
	Delta *ContextDelta `json:"delta,omitempty"`

	// TTL is the optional time-to-live for this context.
	TTL *Duration `json:"ttl,omitempty"`

	// Tags are optional tags for categorization.
	Tags []string `json:"tags,omitempty"`
}

// ContextDelta contains information about a change/diff.
type ContextDelta struct {
	// Operation is the type of change (add, modify, delete).
	Operation DeltaOperation `json:"operation"`

	// OldContent is the content before the change.
	OldContent string `json:"oldContent,omitempty"`

	// NewContent is the content after the change.
	NewContent string `json:"newContent,omitempty"`

	// StartLine is the starting line of the change.
	StartLine int32 `json:"startLine,omitempty"`

	// EndLine is the ending line of the change.
	EndLine int32 `json:"endLine,omitempty"`

	// Symbols lists the affected symbols.
	Symbols []string `json:"symbols,omitempty"`
}

// DeltaOperation represents the type of delta operation.
type DeltaOperation string

const (
	// DeltaOperationAdd indicates content was added.
	DeltaOperationAdd DeltaOperation = "Add"

	// DeltaOperationModify indicates content was modified.
	DeltaOperationModify DeltaOperation = "Modify"

	// DeltaOperationDelete indicates content was deleted.
	DeltaOperationDelete DeltaOperation = "Delete"
)

// ContextStatus defines the observed state of a Context.
type ContextStatus struct {
	// Phase is the current phase of the context lifecycle.
	Phase ContextPhase `json:"phase"`

	// SyncedTo lists the agents this context has been synced to.
	SyncedTo []SyncTarget `json:"syncedTo,omitempty"`

	// Embedding contains embedding information if the context was embedded.
	Embedding *EmbeddingInfo `json:"embedding,omitempty"`

	// Conditions represent the current state.
	Conditions []Condition `json:"conditions,omitempty"`

	// LastSyncTime is when the context was last synced.
	LastSyncTime *time.Time `json:"lastSyncTime,omitempty"`

	// Message provides additional information.
	Message string `json:"message,omitempty"`
}

// SyncTarget describes an agent that received the context.
type SyncTarget struct {
	// AgentID is the ID of the agent.
	AgentID string `json:"agentId"`

	// AgentName is the name of the agent.
	AgentName string `json:"agentName,omitempty"`

	// SyncedAt is when the context was synced to this agent.
	SyncedAt time.Time `json:"syncedAt"`

	// Acknowledged indicates if the agent acknowledged receipt.
	Acknowledged bool `json:"acknowledged"`
}

// EmbeddingInfo contains information about the context embedding.
type EmbeddingInfo struct {
	// Provider is the embedding provider used.
	Provider string `json:"provider"`

	// Model is the embedding model used.
	Model string `json:"model"`

	// Dimensions is the number of dimensions in the embedding.
	Dimensions int32 `json:"dimensions"`

	// EmbeddedAt is when the embedding was created.
	EmbeddedAt time.Time `json:"embeddedAt"`

	// CollectionID is the ID of the vector collection.
	CollectionID string `json:"collectionId,omitempty"`

	// DocumentID is the ID of the document in the vector store.
	DocumentID string `json:"documentId,omitempty"`
}

// ContextList is a list of Context resources.
type ContextList struct {
	TypeMeta `json:",inline"`
	ListMeta `json:"metadata,omitempty"`

	// Items is the list of contexts.
	Items []Context `json:"items"`
}

// Context condition types.
const (
	// ContextConditionSynced indicates the context is synced.
	ContextConditionSynced = "Synced"

	// ContextConditionEmbedded indicates the context is embedded.
	ContextConditionEmbedded = "Embedded"

	// ContextConditionConflict indicates a sync conflict.
	ContextConditionConflict = "Conflict"

	// ContextConditionReady indicates the context is ready for use.
	ContextConditionReady = "Ready"
)

// NewContext creates a new Context with default values.
func NewContext(name string, spec ContextSpec) *Context {
	return &Context{
		TypeMeta: TypeMeta{
			Kind:       ContextKind,
			APIVersion: GroupVersion,
		},
		ObjectMeta: ObjectMeta{
			Name:              name,
			CreationTimestamp: time.Now(),
		},
		Spec: spec,
		Status: ContextStatus{
			Phase: ContextPhasePending,
		},
	}
}

// IsSynced returns true if the context is synced.
func (c *Context) IsSynced() bool {
	return c.Status.Phase == ContextPhaseSynced || c.Status.Phase == ContextPhaseReady
}

// IsEmbedded returns true if the context has been embedded.
func (c *Context) IsEmbedded() bool {
	return c.Status.Embedding != nil
}

// IsReady returns true if the context is fully processed.
func (c *Context) IsReady() bool {
	return c.Status.Phase == ContextPhaseReady
}

// SetCondition updates or adds a condition.
func (c *Context) SetCondition(condType string, status ConditionStatus, reason, message string) {
	now := time.Now()
	for i := range c.Status.Conditions {
		if c.Status.Conditions[i].Type == condType {
			if c.Status.Conditions[i].Status != status {
				c.Status.Conditions[i].LastTransitionTime = now
			}
			c.Status.Conditions[i].Status = status
			c.Status.Conditions[i].Reason = reason
			c.Status.Conditions[i].Message = message
			return
		}
	}
	c.Status.Conditions = append(c.Status.Conditions, Condition{
		Type:               condType,
		Status:             status,
		LastTransitionTime: now,
		Reason:             reason,
		Message:            message,
	})
}

// GetCondition returns the condition with the given type.
func (c *Context) GetCondition(condType string) *Condition {
	for i := range c.Status.Conditions {
		if c.Status.Conditions[i].Type == condType {
			return &c.Status.Conditions[i]
		}
	}
	return nil
}
