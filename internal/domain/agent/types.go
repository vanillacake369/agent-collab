package agent

import (
	"time"
)

// Provider represents an AI model provider.
type Provider string

const (
	ProviderOpenAI    Provider = "openai"
	ProviderAnthropic Provider = "anthropic"
	ProviderGoogle    Provider = "google"
	ProviderOllama    Provider = "ollama"
	ProviderCustom    Provider = "custom"
)

// Capability represents what an agent can do.
type Capability string

const (
	CapabilityEmbedding  Capability = "embedding"
	CapabilityCompletion Capability = "completion"
	CapabilityCodeEdit   Capability = "code_edit"
	CapabilityCodeReview Capability = "code_review"
	CapabilityChat       Capability = "chat"
	CapabilityToolUse    Capability = "tool_use"
	CapabilityVision     Capability = "vision"
)

// AgentInfo contains metadata about an agent.
type AgentInfo struct {
	ID           string         `json:"id"`
	Name         string         `json:"name"`
	Provider     Provider       `json:"provider"`
	Model        string         `json:"model"`
	Version      string         `json:"version"`
	Capabilities []Capability   `json:"capabilities"`
	Metadata     map[string]any `json:"metadata,omitempty"`
}

// AgentStatus represents the current status of an agent.
type AgentStatus string

const (
	StatusOnline     AgentStatus = "online"
	StatusOffline    AgentStatus = "offline"
	StatusBusy       AgentStatus = "busy"
	StatusError      AgentStatus = "error"
	StatusConnecting AgentStatus = "connecting"
)

// ConnectedAgent represents an agent connected to the cluster.
type ConnectedAgent struct {
	Info         AgentInfo   `json:"info"`
	PeerID       string      `json:"peer_id"`
	Status       AgentStatus `json:"status"`
	ConnectedAt  time.Time   `json:"connected_at"`
	LastSeenAt   time.Time   `json:"last_seen_at"`
	TokensUsed   int64       `json:"tokens_used"`
	RequestCount int64       `json:"request_count"`
}

// AgentMessage is a message exchanged between agents.
type AgentMessage struct {
	ID        string         `json:"id"`
	From      string         `json:"from"` // Agent ID
	To        string         `json:"to"`   // Agent ID or "*" for broadcast
	Type      MessageType    `json:"type"`
	Payload   map[string]any `json:"payload"`
	Timestamp time.Time      `json:"timestamp"`
}

// MessageType represents the type of agent message.
type MessageType string

const (
	MessageTypeHandshake    MessageType = "handshake"
	MessageTypeHeartbeat    MessageType = "heartbeat"
	MessageTypeRequest      MessageType = "request"
	MessageTypeResponse     MessageType = "response"
	MessageTypeNotification MessageType = "notification"
	MessageTypeError        MessageType = "error"
)

// ProviderConfig contains configuration for a specific provider.
type ProviderConfig struct {
	Provider    Provider          `json:"provider"`
	APIKey      string            `json:"api_key,omitempty"`
	BaseURL     string            `json:"base_url,omitempty"`
	Model       string            `json:"model,omitempty"`
	MaxTokens   int               `json:"max_tokens,omitempty"`
	Temperature float64           `json:"temperature,omitempty"`
	Timeout     time.Duration     `json:"timeout,omitempty"`
	Headers     map[string]string `json:"headers,omitempty"`
	Options     map[string]any    `json:"options,omitempty"`
}

// DefaultProviderConfigs returns default configurations for known providers.
func DefaultProviderConfigs() map[Provider]*ProviderConfig {
	return map[Provider]*ProviderConfig{
		ProviderOpenAI: {
			Provider: ProviderOpenAI,
			BaseURL:  "https://api.openai.com/v1",
			Model:    "gpt-4",
			Timeout:  30 * time.Second,
		},
		ProviderAnthropic: {
			Provider: ProviderAnthropic,
			BaseURL:  "https://api.anthropic.com/v1",
			Model:    "claude-sonnet-4-20250514",
			Timeout:  30 * time.Second,
		},
		ProviderGoogle: {
			Provider: ProviderGoogle,
			BaseURL:  "https://generativelanguage.googleapis.com/v1beta",
			Model:    "gemini-pro",
			Timeout:  30 * time.Second,
		},
		ProviderOllama: {
			Provider: ProviderOllama,
			BaseURL:  "http://localhost:11434",
			Model:    "llama2",
			Timeout:  60 * time.Second,
		},
	}
}
