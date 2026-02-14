package interest

import (
	"crypto/rand"
	"encoding/hex"
	"time"
)

// InterestLevel defines the notification level for an interest.
type InterestLevel int

const (
	// InterestLevelAll receives all change notifications.
	InterestLevelAll InterestLevel = iota

	// InterestLevelDirect receives only direct pattern matches (no dependencies).
	InterestLevelDirect

	// InterestLevelLocksOnly receives only lock conflict notifications.
	InterestLevelLocksOnly

	// InterestLevelNone receives no notifications (search only).
	InterestLevelNone
)

// String returns the string representation of InterestLevel.
func (l InterestLevel) String() string {
	switch l {
	case InterestLevelAll:
		return "all"
	case InterestLevelDirect:
		return "direct"
	case InterestLevelLocksOnly:
		return "locks_only"
	case InterestLevelNone:
		return "none"
	default:
		return "unknown"
	}
}

// ParseInterestLevel parses a string to InterestLevel.
func ParseInterestLevel(s string) InterestLevel {
	switch s {
	case "all":
		return InterestLevelAll
	case "direct":
		return InterestLevelDirect
	case "locks_only":
		return InterestLevelLocksOnly
	case "none":
		return InterestLevelNone
	default:
		return InterestLevelAll
	}
}

// Interest defines an agent's area of interest.
type Interest struct {
	ID        string            `json:"id"`
	AgentID   string            `json:"agent_id"`
	AgentName string            `json:"agent_name"`

	// Patterns are glob patterns for matching file paths.
	// Examples: ["proj-a/**", "proj-b/src/*.go"]
	Patterns []string `json:"patterns"`

	// TrackDependencies enables automatic dependency tracking.
	TrackDependencies bool `json:"track_dependencies"`

	// Level controls notification filtering.
	Level InterestLevel `json:"level"`

	// ExpiresAt is the TTL for automatic cleanup.
	ExpiresAt time.Time `json:"expires_at"`

	// CreatedAt is the creation timestamp.
	CreatedAt time.Time `json:"created_at"`

	// Metadata stores additional information.
	Metadata map[string]string `json:"metadata,omitempty"`

	// Remote indicates if this interest was received from another node.
	Remote bool `json:"remote,omitempty"`

	// NodeID is the originating node ID for remote interests.
	NodeID string `json:"node_id,omitempty"`
}

// NewInterest creates a new interest with default values.
func NewInterest(agentID, agentName string, patterns []string) *Interest {
	return &Interest{
		ID:                generateInterestID(),
		AgentID:           agentID,
		AgentName:         agentName,
		Patterns:          patterns,
		TrackDependencies: false,
		Level:             InterestLevelAll,
		ExpiresAt:         time.Now().Add(24 * time.Hour),
		CreatedAt:         time.Now(),
		Metadata:          make(map[string]string),
	}
}

// IsExpired checks if the interest has expired.
func (i *Interest) IsExpired() bool {
	if i.ExpiresAt.IsZero() {
		return false
	}
	return time.Now().After(i.ExpiresAt)
}

// SetTTL sets the time-to-live for the interest.
func (i *Interest) SetTTL(ttl time.Duration) {
	i.ExpiresAt = time.Now().Add(ttl)
}

// Renew extends the expiration time by the default TTL.
func (i *Interest) Renew() {
	i.ExpiresAt = time.Now().Add(24 * time.Hour)
}

// generateInterestID generates a unique interest ID.
func generateInterestID() string {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return "int-" + hex.EncodeToString([]byte(time.Now().String()))[:12]
	}
	return "int-" + hex.EncodeToString(b)
}

// MatchType indicates how an interest matched an event.
type MatchType string

const (
	// MatchTypeDirect means the pattern directly matched the file path.
	MatchTypeDirect MatchType = "direct"

	// MatchTypeDependency means the file is a dependency of a matched file.
	MatchTypeDependency MatchType = "dependency"

	// MatchTypeProximity means the file is in the same directory.
	MatchTypeProximity MatchType = "proximity"
)

// InterestMatch represents a match between an interest and an event.
type InterestMatch struct {
	Interest    *Interest `json:"interest"`
	MatchType   MatchType `json:"match_type"`
	MatchedPath string    `json:"matched_path"`
	Relevance   float32   `json:"relevance"` // 0.0 ~ 1.0
}

// NewInterestMatch creates a new interest match.
func NewInterestMatch(interest *Interest, matchType MatchType, matchedPath string) *InterestMatch {
	relevance := float32(1.0)
	switch matchType {
	case MatchTypeDirect:
		relevance = 1.0
	case MatchTypeDependency:
		relevance = 0.8
	case MatchTypeProximity:
		relevance = 0.5
	}

	return &InterestMatch{
		Interest:    interest,
		MatchType:   matchType,
		MatchedPath: matchedPath,
		Relevance:   relevance,
	}
}

// RegisterInterestRequest is a request to register an interest.
type RegisterInterestRequest struct {
	Patterns          []string `json:"patterns"`
	TrackDependencies bool     `json:"track_dependencies"`
	Level             string   `json:"level"`
	TTL               string   `json:"ttl,omitempty"`
}

// UnregisterInterestRequest is a request to unregister an interest.
type UnregisterInterestRequest struct {
	InterestID string `json:"interest_id,omitempty"`
	All        bool   `json:"all,omitempty"`
}

// ChangeType defines the type of interest change.
type ChangeType string

const (
	// ChangeTypeAdded indicates a new interest was registered.
	ChangeTypeAdded ChangeType = "added"

	// ChangeTypeRemoved indicates an interest was unregistered.
	ChangeTypeRemoved ChangeType = "removed"

	// ChangeTypeUpdated indicates an interest was updated.
	ChangeTypeUpdated ChangeType = "updated"
)

// InterestChange represents a change to an interest.
type InterestChange struct {
	Type     ChangeType `json:"type"`
	Interest *Interest  `json:"interest"`
}
