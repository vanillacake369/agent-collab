package daemon

import (
	"encoding/json"
	"time"
)

// EventType represents the type of daemon event.
type EventType string

const (
	// Lock events
	EventLockAcquired EventType = "lock.acquired"
	EventLockReleased EventType = "lock.released"
	EventLockConflict EventType = "lock.conflict"
	EventLockExpired  EventType = "lock.expired"

	// Agent events
	EventAgentJoined EventType = "agent.joined"
	EventAgentLeft   EventType = "agent.left"

	// Context events
	EventContextUpdated EventType = "context.updated"
	EventContextSynced  EventType = "context.synced"

	// Peer events
	EventPeerConnected    EventType = "peer.connected"
	EventPeerDisconnected EventType = "peer.disconnected"

	// System events
	EventDaemonReady    EventType = "daemon.ready"
	EventDaemonShutdown EventType = "daemon.shutdown"
)

// Event is a daemon event that can be streamed to clients.
type Event struct {
	Type      EventType       `json:"type"`
	Timestamp time.Time       `json:"ts"`
	Data      json.RawMessage `json:"data,omitempty"`
}

// NewEvent creates a new event with the given type and data.
func NewEvent(eventType EventType, data any) Event {
	var rawData json.RawMessage
	if data != nil {
		rawData, _ = json.Marshal(data)
	}
	return Event{
		Type:      eventType,
		Timestamp: time.Now(),
		Data:      rawData,
	}
}

// LockEventData contains data for lock-related events.
type LockEventData struct {
	LockID    string `json:"lock_id,omitempty"`
	FilePath  string `json:"file_path"`
	StartLine int    `json:"start_line,omitempty"`
	EndLine   int    `json:"end_line,omitempty"`
	AgentID   string `json:"agent_id"`
	Intention string `json:"intention,omitempty"`
}

// LockConflictData contains data for lock conflict events.
type LockConflictData struct {
	FilePath    string `json:"file_path"`
	HolderID    string `json:"holder_id"`
	RequesterID string `json:"requester_id"`
	Intention   string `json:"intention,omitempty"`
}

// AgentEventData contains data for agent-related events.
type AgentEventData struct {
	AgentID  string `json:"agent_id"`
	Name     string `json:"name"`
	Provider string `json:"provider"`
}

// ContextEventData contains data for context-related events.
type ContextEventData struct {
	FilePath string `json:"file_path"`
	Content  string `json:"content,omitempty"`
	AgentID  string `json:"agent_id,omitempty"`
}

// PeerEventData contains data for peer-related events.
type PeerEventData struct {
	PeerID string `json:"peer_id"`
	Addr   string `json:"addr,omitempty"`
}
