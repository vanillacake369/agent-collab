package event

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"time"
)

// EventType defines the type of cluster event.
type EventType string

const (
	EventTypeFileChange    EventType = "file_change"
	EventTypeLockAcquired  EventType = "lock_acquired"
	EventTypeLockReleased  EventType = "lock_released"
	EventTypeLockConflict  EventType = "lock_conflict"
	EventTypeContextShared EventType = "context_shared"
	EventTypeAgentJoined   EventType = "agent_joined"
	EventTypeAgentLeft     EventType = "agent_left"
	EventTypeWarning       EventType = "warning"
)

// EventStatus defines the lifecycle state of an event.
type EventStatus string

const (
	EventStatusActive    EventStatus = "active"
	EventStatusCompleted EventStatus = "completed"
	EventStatusArchived  EventStatus = "archived"
)

// Event represents a cluster event.
type Event struct {
	ID        string    `json:"id"`
	Type      EventType `json:"type"`
	Timestamp time.Time `json:"timestamp"`

	// Source agent information
	SourceID   string `json:"source_id"`
	SourceName string `json:"source_name"`

	// Target location (optional)
	FilePath  string `json:"file_path,omitempty"`
	LineStart int    `json:"line_start,omitempty"`
	LineEnd   int    `json:"line_end,omitempty"`

	// Event payload (type-specific data)
	Payload json.RawMessage `json:"payload,omitempty"`

	// Vector embedding for semantic search
	Embedding []float32 `json:"embedding,omitempty"`

	// Lifecycle management
	Status       EventStatus `json:"status,omitempty"`
	ExpiresAt    time.Time   `json:"expires_at,omitempty"`
	SupersededBy string      `json:"superseded_by,omitempty"` // ID of newer event that replaced this
}

// DefaultEventTTL is the default time-to-live for events.
const DefaultEventTTL = 24 * time.Hour

// NewEvent creates a new event with default values.
func NewEvent(eventType EventType, sourceID, sourceName string) *Event {
	now := time.Now()
	return &Event{
		ID:         generateEventID(),
		Type:       eventType,
		Timestamp:  now,
		SourceID:   sourceID,
		SourceName: sourceName,
		Status:     EventStatusActive,
		ExpiresAt:  now.Add(DefaultEventTTL),
	}
}

// IsExpired checks if the event has expired.
func (e *Event) IsExpired() bool {
	if e.ExpiresAt.IsZero() {
		return false
	}
	return time.Now().After(e.ExpiresAt)
}

// IsSuperseded checks if this event has been replaced by a newer one.
func (e *Event) IsSuperseded() bool {
	return e.SupersededBy != ""
}

// MarkCompleted marks the event as completed.
func (e *Event) MarkCompleted() {
	e.Status = EventStatusCompleted
}

// MarkArchived marks the event as archived.
func (e *Event) MarkArchived() {
	e.Status = EventStatusArchived
}

// SetPayload sets the payload from a struct.
func (e *Event) SetPayload(payload any) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	e.Payload = data
	return nil
}

// GetPayload unmarshals the payload into the provided struct.
func (e *Event) GetPayload(v any) error {
	if len(e.Payload) == 0 {
		return nil
	}
	return json.Unmarshal(e.Payload, v)
}

// generateEventID generates a unique event ID.
func generateEventID() string {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return "evt-" + hex.EncodeToString([]byte(time.Now().String()))[:12]
	}
	return "evt-" + hex.EncodeToString(b)
}

// FileChangePayload is the payload for file change events.
type FileChangePayload struct {
	ChangeType string   `json:"change_type"` // "modify", "create", "delete"
	Summary    string   `json:"summary"`
	Symbols    []string `json:"symbols,omitempty"`
	LinesDiff  int      `json:"lines_diff"`
}

// NewFileChangeEvent creates a new file change event.
func NewFileChangeEvent(sourceID, sourceName, filePath string, payload *FileChangePayload) *Event {
	event := NewEvent(EventTypeFileChange, sourceID, sourceName)
	event.FilePath = filePath
	_ = event.SetPayload(payload)
	return event
}

// LockPayload is the payload for lock events.
type LockPayload struct {
	LockID     string `json:"lock_id"`
	HolderID   string `json:"holder_id"`
	HolderName string `json:"holder_name"`
	Purpose    string `json:"purpose,omitempty"`
}

// NewLockAcquiredEvent creates a new lock acquired event.
func NewLockAcquiredEvent(sourceID, sourceName, filePath string, lineStart, lineEnd int, payload *LockPayload) *Event {
	event := NewEvent(EventTypeLockAcquired, sourceID, sourceName)
	event.FilePath = filePath
	event.LineStart = lineStart
	event.LineEnd = lineEnd
	_ = event.SetPayload(payload)
	return event
}

// NewLockReleasedEvent creates a new lock released event.
func NewLockReleasedEvent(sourceID, sourceName string, payload *LockPayload) *Event {
	event := NewEvent(EventTypeLockReleased, sourceID, sourceName)
	_ = event.SetPayload(payload)
	return event
}

// LockConflictPayload is the payload for lock conflict events.
type LockConflictPayload struct {
	RequestedLockID   string `json:"requested_lock_id"`
	ConflictingLockID string `json:"conflicting_lock_id"`
	OverlapType       string `json:"overlap_type"` // "full", "partial", "contains"
	ConflictingHolder string `json:"conflicting_holder"`
}

// NewLockConflictEvent creates a new lock conflict event.
func NewLockConflictEvent(sourceID, sourceName, filePath string, payload *LockConflictPayload) *Event {
	event := NewEvent(EventTypeLockConflict, sourceID, sourceName)
	event.FilePath = filePath
	_ = event.SetPayload(payload)
	return event
}

// ContextSharedPayload is the payload for context shared events.
type ContextSharedPayload struct {
	Content  string            `json:"content"`
	Metadata map[string]string `json:"metadata,omitempty"`
}

// NewContextSharedEvent creates a new context shared event.
func NewContextSharedEvent(sourceID, sourceName, filePath string, payload *ContextSharedPayload) *Event {
	event := NewEvent(EventTypeContextShared, sourceID, sourceName)
	event.FilePath = filePath
	_ = event.SetPayload(payload)
	return event
}

// AgentPayload is the payload for agent join/leave events.
type AgentPayload struct {
	AgentID   string   `json:"agent_id"`
	AgentName string   `json:"agent_name"`
	Provider  string   `json:"provider,omitempty"`
	Interests []string `json:"interests,omitempty"`
}

// NewAgentJoinedEvent creates a new agent joined event.
func NewAgentJoinedEvent(sourceID, sourceName string, payload *AgentPayload) *Event {
	event := NewEvent(EventTypeAgentJoined, sourceID, sourceName)
	_ = event.SetPayload(payload)
	return event
}

// NewAgentLeftEvent creates a new agent left event.
func NewAgentLeftEvent(sourceID, sourceName string, payload *AgentPayload) *Event {
	event := NewEvent(EventTypeAgentLeft, sourceID, sourceName)
	_ = event.SetPayload(payload)
	return event
}

// WarningPayload is the payload for warning events.
type WarningPayload struct {
	Level   string `json:"level"` // "info", "warning", "error"
	Message string `json:"message"`
	Details string `json:"details,omitempty"`
}

// NewWarningEvent creates a new warning event.
func NewWarningEvent(sourceID, sourceName string, payload *WarningPayload) *Event {
	event := NewEvent(EventTypeWarning, sourceID, sourceName)
	_ = event.SetPayload(payload)
	return event
}

// EventFilter is used to filter events when querying.
type EventFilter struct {
	Types      []EventType `json:"types,omitempty"`
	Since      time.Time   `json:"since,omitempty"`
	FilePath   string      `json:"file_path,omitempty"`
	SourceID   string      `json:"source_id,omitempty"`
	Limit      int         `json:"limit,omitempty"`
	IncludeAll bool        `json:"include_all,omitempty"` // Ignore interest filtering
}

// DefaultEventFilter returns a filter with default values.
func DefaultEventFilter() *EventFilter {
	return &EventFilter{
		Limit: 50,
	}
}
