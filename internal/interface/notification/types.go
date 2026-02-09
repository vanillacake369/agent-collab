package notification

import (
	"time"
)

// Priority represents notification priority level.
type Priority int

const (
	PriorityLow Priority = iota
	PriorityNormal
	PriorityHigh
	PriorityCritical
)

// String returns the string representation of priority.
func (p Priority) String() string {
	switch p {
	case PriorityLow:
		return "low"
	case PriorityNormal:
		return "normal"
	case PriorityHigh:
		return "high"
	case PriorityCritical:
		return "critical"
	default:
		return "unknown"
	}
}

// Category represents notification category.
type Category string

const (
	CategoryLockConflict Category = "lock_conflict"
	CategorySyncConflict Category = "sync_conflict"
	CategoryNegotiation  Category = "negotiation"
	CategoryPeerEvent    Category = "peer_event"
	CategorySystemAlert  Category = "system_alert"
)

// Notification represents a notification to be sent to the user.
type Notification struct {
	ID           string         `json:"id"`
	Category     Category       `json:"category"`
	Priority     Priority       `json:"priority"`
	Title        string         `json:"title"`
	Message      string         `json:"message"`
	Details      map[string]any `json:"details,omitempty"`
	Actions      []Action       `json:"actions,omitempty"`
	CreatedAt    time.Time      `json:"created_at"`
	ExpiresAt    time.Time      `json:"expires_at,omitempty"`
	Acknowledged bool           `json:"acknowledged"`
	Response     *Response      `json:"response,omitempty"`
}

// Action represents an action the user can take.
type Action struct {
	ID          string `json:"id"`
	Label       string `json:"label"`
	Description string `json:"description,omitempty"`
	IsDefault   bool   `json:"is_default,omitempty"`
	IsDangerous bool   `json:"is_dangerous,omitempty"`
}

// Response represents a user's response to a notification.
type Response struct {
	ActionID    string         `json:"action_id"`
	Comment     string         `json:"comment,omitempty"`
	Data        map[string]any `json:"data,omitempty"`
	RespondedAt time.Time      `json:"responded_at"`
}

// NotificationHandler is called when a notification needs to be sent.
type NotificationHandler func(*Notification) error

// ResponseHandler is called when a user responds to a notification.
type ResponseHandler func(*Notification, *Response) error

// LockConflictDetails holds details for lock conflict notifications.
type LockConflictDetails struct {
	RequestedLockID   string `json:"requested_lock_id"`
	ConflictingLockID string `json:"conflicting_lock_id"`
	FilePath          string `json:"file_path"`
	RequestedBy       string `json:"requested_by"`
	HeldBy            string `json:"held_by"`
	RequestedTarget   string `json:"requested_target"`
	ConflictingTarget string `json:"conflicting_target"`
	OverlapType       string `json:"overlap_type"`
}

// SyncConflictDetails holds details for sync conflict notifications.
type SyncConflictDetails struct {
	FilePath     string `json:"file_path"`
	LocalAgent   string `json:"local_agent"`
	RemoteAgent  string `json:"remote_agent"`
	LocalChange  string `json:"local_change"`
	RemoteChange string `json:"remote_change"`
}

// NegotiationDetails holds details for negotiation notifications.
type NegotiationDetails struct {
	SessionID      string   `json:"session_id"`
	Participants   []string `json:"participants"`
	Reason         string   `json:"reason"`
	TimeoutSeconds int      `json:"timeout_seconds"`
}
