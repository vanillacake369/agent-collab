package daemon

import (
	"time"

	"agent-collab/internal/domain/agent"
	"agent-collab/internal/domain/lock"
)

// Request/Response types for daemon RPC

// StatusResponse contains the daemon status.
type StatusResponse struct {
	Running           bool      `json:"running"`
	PID               int       `json:"pid"`
	StartedAt         time.Time `json:"started_at"`
	ProjectName       string    `json:"project_name"`
	NodeID            string    `json:"node_id"`
	PeerCount         int       `json:"peer_count"`
	LockCount         int       `json:"lock_count"`
	AgentCount        int       `json:"agent_count"`
	EmbeddingProvider string    `json:"embedding_provider"`
	EventSubscribers  int       `json:"event_subscribers"`
}

// LockRequest is a request to acquire a lock.
type LockRequest struct {
	FilePath  string `json:"file_path"`
	StartLine int    `json:"start_line"`
	EndLine   int    `json:"end_line"`
	Intention string `json:"intention"`
}

// LockResponse is the response to a lock request.
type LockResponse struct {
	Success bool   `json:"success"`
	LockID  string `json:"lock_id,omitempty"`
	Error   string `json:"error,omitempty"`
}

// ReleaseLockRequest is a request to release a lock.
type ReleaseLockRequest struct {
	LockID string `json:"lock_id"`
}

// ListLocksResponse contains the list of active locks.
type ListLocksResponse struct {
	Locks []*lock.SemanticLock `json:"locks"`
}

// EmbedRequest is a request to generate embeddings.
type EmbedRequest struct {
	Text string `json:"text"`
}

// EmbedResponse is the response with embeddings.
type EmbedResponse struct {
	Embedding []float32 `json:"embedding"`
	Dimension int       `json:"dimension"`
	Provider  string    `json:"provider"`
	Model     string    `json:"model"`
}

// SearchRequest is a request to search similar content.
type SearchRequest struct {
	Query string `json:"query"`
	Limit int    `json:"limit"`
}

// SearchResult is a single search result.
type SearchResult struct {
	ID       string         `json:"id"`
	Content  string         `json:"content"`
	Score    float32        `json:"score"`
	Metadata map[string]any `json:"metadata,omitempty"`
}

// SearchResponse contains search results.
type SearchResponse struct {
	Results []SearchResult `json:"results"`
}

// ListAgentsResponse contains connected agents.
type ListAgentsResponse struct {
	Agents []*agent.ConnectedAgent `json:"agents"`
}

// WatchFileRequest is a request to watch a file for changes.
type WatchFileRequest struct {
	FilePath string `json:"file_path"`
}

// GenericResponse is a generic success/error response.
type GenericResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message,omitempty"`
	Error   string `json:"error,omitempty"`
}

// InitRequest is a request to initialize a cluster.
type InitRequest struct {
	ProjectName string `json:"project_name"`
}

// InitResponse is the response after initialization.
type InitResponse struct {
	Success     bool   `json:"success"`
	ProjectName string `json:"project_name"`
	NodeID      string `json:"node_id"`
	InviteToken string `json:"invite_token"`
	Error       string `json:"error,omitempty"`
}

// JoinRequest is a request to join a cluster.
type JoinRequest struct {
	Token string `json:"token"`
}

// JoinResponse is the response after joining.
type JoinResponse struct {
	Success        bool   `json:"success"`
	ProjectName    string `json:"project_name"`
	NodeID         string `json:"node_id"`
	ConnectedPeers int    `json:"connected_peers"`
	Error          string `json:"error,omitempty"`
}

// PeerInfo contains information about a connected peer.
type PeerInfo struct {
	ID        string   `json:"id"`
	Addresses []string `json:"addresses"`
	Latency   int64    `json:"latency_ms"`
	Connected bool     `json:"connected"`
}

// ListPeersResponse contains the list of connected peers.
type ListPeersResponse struct {
	Peers []PeerInfo `json:"peers"`
}
