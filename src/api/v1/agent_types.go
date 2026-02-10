package v1

import (
	"time"
)

// AgentKind is the resource kind for Agent.
const AgentKind = "Agent"

// AgentPhase represents the current phase of an agent lifecycle.
type AgentPhase string

const (
	// AgentPhasePending indicates the agent is initializing.
	AgentPhasePending AgentPhase = "Pending"

	// AgentPhaseConnecting indicates the agent is connecting to the cluster.
	AgentPhaseConnecting AgentPhase = "Connecting"

	// AgentPhaseOnline indicates the agent is online and ready.
	AgentPhaseOnline AgentPhase = "Online"

	// AgentPhaseBusy indicates the agent is busy with a task.
	AgentPhaseBusy AgentPhase = "Busy"

	// AgentPhaseOffline indicates the agent is offline.
	AgentPhaseOffline AgentPhase = "Offline"

	// AgentPhaseError indicates the agent is in an error state.
	AgentPhaseError AgentPhase = "Error"

	// AgentPhaseTerminating indicates the agent is shutting down.
	AgentPhaseTerminating AgentPhase = "Terminating"
)

// AgentProvider represents the AI provider for an agent.
type AgentProvider string

const (
	// AgentProviderOpenAI represents OpenAI.
	AgentProviderOpenAI AgentProvider = "OpenAI"

	// AgentProviderAnthropic represents Anthropic.
	AgentProviderAnthropic AgentProvider = "Anthropic"

	// AgentProviderGoogle represents Google (Gemini).
	AgentProviderGoogle AgentProvider = "Google"

	// AgentProviderOllama represents local Ollama.
	AgentProviderOllama AgentProvider = "Ollama"

	// AgentProviderCustom represents a custom provider.
	AgentProviderCustom AgentProvider = "Custom"
)

// AgentCapability represents a capability of an agent.
type AgentCapability string

const (
	// AgentCapabilityEmbedding indicates embedding capability.
	AgentCapabilityEmbedding AgentCapability = "Embedding"

	// AgentCapabilityCompletion indicates text completion capability.
	AgentCapabilityCompletion AgentCapability = "Completion"

	// AgentCapabilityCodeEdit indicates code editing capability.
	AgentCapabilityCodeEdit AgentCapability = "CodeEdit"

	// AgentCapabilityCodeReview indicates code review capability.
	AgentCapabilityCodeReview AgentCapability = "CodeReview"

	// AgentCapabilityChat indicates chat capability.
	AgentCapabilityChat AgentCapability = "Chat"

	// AgentCapabilityToolUse indicates tool use capability.
	AgentCapabilityToolUse AgentCapability = "ToolUse"

	// AgentCapabilityVision indicates vision capability.
	AgentCapabilityVision AgentCapability = "Vision"
)

// Agent represents an AI agent in the collaboration cluster.
type Agent struct {
	TypeMeta   `json:",inline"`
	ObjectMeta `json:"metadata"`

	// Spec describes the desired state of the agent.
	Spec AgentSpec `json:"spec"`

	// Status describes the current state of the agent.
	Status AgentStatus `json:"status,omitempty"`
}

// GetObjectMeta returns the ObjectMeta of the Agent.
func (a *Agent) GetObjectMeta() *ObjectMeta {
	return &a.ObjectMeta
}

// AgentSpec defines the desired state of an Agent.
type AgentSpec struct {
	// Provider is the AI provider for this agent.
	Provider AgentProvider `json:"provider"`

	// Model is the specific model being used.
	Model string `json:"model,omitempty"`

	// Capabilities lists the capabilities of this agent.
	Capabilities []AgentCapability `json:"capabilities,omitempty"`

	// PeerID is the libp2p peer ID for this agent.
	PeerID string `json:"peerId"`

	// DisplayName is a human-readable name for the agent.
	DisplayName string `json:"displayName,omitempty"`

	// Description is a description of the agent's purpose.
	Description string `json:"description,omitempty"`

	// HeartbeatInterval is the interval between heartbeats.
	HeartbeatInterval Duration `json:"heartbeatInterval,omitempty"`

	// Config contains provider-specific configuration.
	Config map[string]string `json:"config,omitempty"`

	// MaxConcurrentTasks is the maximum number of concurrent tasks.
	MaxConcurrentTasks int32 `json:"maxConcurrentTasks,omitempty"`
}

// AgentStatus defines the observed state of an Agent.
type AgentStatus struct {
	// Phase is the current phase of the agent lifecycle.
	Phase AgentPhase `json:"phase"`

	// LastHeartbeat is the time of the last heartbeat.
	LastHeartbeat *time.Time `json:"lastHeartbeat,omitempty"`

	// LastSeenAt is when the agent was last seen.
	LastSeenAt *time.Time `json:"lastSeenAt,omitempty"`

	// ConnectedAt is when the agent connected.
	ConnectedAt *time.Time `json:"connectedAt,omitempty"`

	// CurrentTask describes the current task if the agent is busy.
	CurrentTask *AgentTask `json:"currentTask,omitempty"`

	// ActiveLocks lists locks held by this agent.
	ActiveLocks []string `json:"activeLocks,omitempty"`

	// TokenUsage tracks token usage for this agent.
	TokenUsage *TokenUsage `json:"tokenUsage,omitempty"`

	// NetworkInfo contains network-related information.
	NetworkInfo *NetworkInfo `json:"networkInfo,omitempty"`

	// Conditions represent the current state.
	Conditions []Condition `json:"conditions,omitempty"`

	// Message provides additional information.
	Message string `json:"message,omitempty"`
}

// AgentTask describes a task an agent is working on.
type AgentTask struct {
	// TaskID is the unique identifier of the task.
	TaskID string `json:"taskId"`

	// Description is a brief description of the task.
	Description string `json:"description,omitempty"`

	// StartedAt is when the task started.
	StartedAt time.Time `json:"startedAt"`

	// Progress is the progress percentage (0-100).
	Progress int32 `json:"progress,omitempty"`
}

// TokenUsage tracks token consumption.
type TokenUsage struct {
	// InputTokens is the number of input tokens used.
	InputTokens int64 `json:"inputTokens"`

	// OutputTokens is the number of output tokens used.
	OutputTokens int64 `json:"outputTokens"`

	// TotalTokens is the total number of tokens used.
	TotalTokens int64 `json:"totalTokens"`

	// EstimatedCost is the estimated cost in USD.
	EstimatedCost float64 `json:"estimatedCost,omitempty"`

	// LastResetAt is when the usage was last reset.
	LastResetAt *time.Time `json:"lastResetAt,omitempty"`
}

// NetworkInfo contains network-related information about an agent.
type NetworkInfo struct {
	// Addresses are the multiaddresses where the agent can be reached.
	Addresses []string `json:"addresses,omitempty"`

	// Latency is the round-trip latency to this agent.
	Latency *Duration `json:"latency,omitempty"`

	// Bandwidth is the available bandwidth to this agent.
	Bandwidth int64 `json:"bandwidth,omitempty"`

	// LastPingAt is when the agent was last pinged.
	LastPingAt *time.Time `json:"lastPingAt,omitempty"`
}

// AgentList is a list of Agent resources.
type AgentList struct {
	TypeMeta `json:",inline"`
	ListMeta `json:"metadata,omitempty"`

	// Items is the list of agents.
	Items []Agent `json:"items"`
}

// Agent condition types.
const (
	// AgentConditionReady indicates the agent is ready.
	AgentConditionReady = "Ready"

	// AgentConditionHealthy indicates the agent is healthy.
	AgentConditionHealthy = "Healthy"

	// AgentConditionConnected indicates the agent is connected to the cluster.
	AgentConditionConnected = "Connected"

	// AgentConditionAvailable indicates the agent is available for tasks.
	AgentConditionAvailable = "Available"
)

// NewAgent creates a new Agent with default values.
func NewAgent(name string, spec AgentSpec) *Agent {
	return &Agent{
		TypeMeta: TypeMeta{
			Kind:       AgentKind,
			APIVersion: GroupVersion,
		},
		ObjectMeta: ObjectMeta{
			Name:              name,
			CreationTimestamp: time.Now(),
		},
		Spec: spec,
		Status: AgentStatus{
			Phase: AgentPhasePending,
		},
	}
}

// IsOnline returns true if the agent is online.
func (a *Agent) IsOnline() bool {
	return a.Status.Phase == AgentPhaseOnline
}

// IsBusy returns true if the agent is busy.
func (a *Agent) IsBusy() bool {
	return a.Status.Phase == AgentPhaseBusy
}

// IsAvailable returns true if the agent is available for tasks.
func (a *Agent) IsAvailable() bool {
	return a.Status.Phase == AgentPhaseOnline && a.Status.CurrentTask == nil
}

// HasCapability returns true if the agent has the given capability.
func (a *Agent) HasCapability(cap AgentCapability) bool {
	for _, c := range a.Spec.Capabilities {
		if c == cap {
			return true
		}
	}
	return false
}

// SetCondition updates or adds a condition.
func (a *Agent) SetCondition(condType string, status ConditionStatus, reason, message string) {
	now := time.Now()
	for i := range a.Status.Conditions {
		if a.Status.Conditions[i].Type == condType {
			if a.Status.Conditions[i].Status != status {
				a.Status.Conditions[i].LastTransitionTime = now
			}
			a.Status.Conditions[i].Status = status
			a.Status.Conditions[i].Reason = reason
			a.Status.Conditions[i].Message = message
			return
		}
	}
	a.Status.Conditions = append(a.Status.Conditions, Condition{
		Type:               condType,
		Status:             status,
		LastTransitionTime: now,
		Reason:             reason,
		Message:            message,
	})
}

// GetCondition returns the condition with the given type.
func (a *Agent) GetCondition(condType string) *Condition {
	for i := range a.Status.Conditions {
		if a.Status.Conditions[i].Type == condType {
			return &a.Status.Conditions[i]
		}
	}
	return nil
}
