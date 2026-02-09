package agent

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// Registry manages connected agents.
type Registry struct {
	mu     sync.RWMutex
	agents map[string]*ConnectedAgent // agentID -> agent
	byPeer map[string]string          // peerID -> agentID
	byCap  map[Capability][]string    // capability -> []agentID

	// Callbacks
	onConnect    func(*ConnectedAgent)
	onDisconnect func(*ConnectedAgent)
	onChange     func(*ConnectedAgent)

	ctx    context.Context
	cancel context.CancelFunc
}

// NewRegistry creates a new agent registry.
func NewRegistry(ctx context.Context) *Registry {
	ctx, cancel := context.WithCancel(ctx)
	r := &Registry{
		agents: make(map[string]*ConnectedAgent),
		byPeer: make(map[string]string),
		byCap:  make(map[Capability][]string),
		ctx:    ctx,
		cancel: cancel,
	}

	go r.cleanupLoop()

	return r
}

// Close stops the registry.
func (r *Registry) Close() error {
	r.cancel()
	return nil
}

// Register adds or updates an agent in the registry.
func (r *Registry) Register(agent *ConnectedAgent) error {
	if agent.Info.ID == "" {
		return fmt.Errorf("agent ID is required")
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	existing := r.agents[agent.Info.ID]
	isNew := existing == nil

	// Update timestamps
	now := time.Now()
	if isNew {
		agent.ConnectedAt = now
	}
	agent.LastSeenAt = now
	agent.Status = StatusOnline

	// Store agent
	r.agents[agent.Info.ID] = agent

	// Update peer index
	if agent.PeerID != "" {
		r.byPeer[agent.PeerID] = agent.Info.ID
	}

	// Update capability index
	for _, cap := range agent.Info.Capabilities {
		if !r.hasCapability(cap, agent.Info.ID) {
			r.byCap[cap] = append(r.byCap[cap], agent.Info.ID)
		}
	}

	// Callbacks
	if isNew && r.onConnect != nil {
		go r.onConnect(agent)
	} else if !isNew && r.onChange != nil {
		go r.onChange(agent)
	}

	return nil
}

// Unregister removes an agent from the registry.
func (r *Registry) Unregister(agentID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	agent, exists := r.agents[agentID]
	if !exists {
		return fmt.Errorf("agent not found: %s", agentID)
	}

	// Remove from indices
	delete(r.agents, agentID)
	if agent.PeerID != "" {
		delete(r.byPeer, agent.PeerID)
	}

	// Remove from capability index
	for cap, ids := range r.byCap {
		r.byCap[cap] = removeFromSlice(ids, agentID)
	}

	// Callback
	if r.onDisconnect != nil {
		go r.onDisconnect(agent)
	}

	return nil
}

// Get returns an agent by ID.
func (r *Registry) Get(agentID string) (*ConnectedAgent, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	agent, exists := r.agents[agentID]
	return agent, exists
}

// GetByPeer returns an agent by peer ID.
func (r *Registry) GetByPeer(peerID string) (*ConnectedAgent, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	agentID, exists := r.byPeer[peerID]
	if !exists {
		return nil, false
	}

	agent, exists := r.agents[agentID]
	return agent, exists
}

// List returns all connected agents.
func (r *Registry) List() []*ConnectedAgent {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]*ConnectedAgent, 0, len(r.agents))
	for _, agent := range r.agents {
		result = append(result, agent)
	}
	return result
}

// ListByProvider returns agents by provider.
func (r *Registry) ListByProvider(provider Provider) []*ConnectedAgent {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var result []*ConnectedAgent
	for _, agent := range r.agents {
		if agent.Info.Provider == provider {
			result = append(result, agent)
		}
	}
	return result
}

// ListByCapability returns agents with a specific capability.
func (r *Registry) ListByCapability(cap Capability) []*ConnectedAgent {
	r.mu.RLock()
	defer r.mu.RUnlock()

	ids := r.byCap[cap]
	result := make([]*ConnectedAgent, 0, len(ids))
	for _, id := range ids {
		if agent, exists := r.agents[id]; exists {
			result = append(result, agent)
		}
	}
	return result
}

// FindBestAgent finds the best available agent for a capability.
func (r *Registry) FindBestAgent(cap Capability, preferProvider Provider) (*ConnectedAgent, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	ids := r.byCap[cap]
	if len(ids) == 0 {
		return nil, fmt.Errorf("no agent with capability: %s", cap)
	}

	var best *ConnectedAgent
	for _, id := range ids {
		agent, exists := r.agents[id]
		if !exists || agent.Status != StatusOnline {
			continue
		}

		// Prefer the specified provider
		if preferProvider != "" && agent.Info.Provider == preferProvider {
			return agent, nil
		}

		// Otherwise, pick the first available
		if best == nil {
			best = agent
		}
	}

	if best == nil {
		return nil, fmt.Errorf("no online agent with capability: %s", cap)
	}

	return best, nil
}

// UpdateStatus updates an agent's status.
func (r *Registry) UpdateStatus(agentID string, status AgentStatus) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	agent, exists := r.agents[agentID]
	if !exists {
		return fmt.Errorf("agent not found: %s", agentID)
	}

	agent.Status = status
	agent.LastSeenAt = time.Now()

	if r.onChange != nil {
		go r.onChange(agent)
	}

	return nil
}

// Heartbeat updates the last seen time for an agent.
func (r *Registry) Heartbeat(agentID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	agent, exists := r.agents[agentID]
	if !exists {
		return fmt.Errorf("agent not found: %s", agentID)
	}

	agent.LastSeenAt = time.Now()
	if agent.Status == StatusOffline {
		agent.Status = StatusOnline
	}

	return nil
}

// RecordUsage records token usage for an agent.
func (r *Registry) RecordUsage(agentID string, tokens int64) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if agent, exists := r.agents[agentID]; exists {
		agent.TokensUsed += tokens
		agent.RequestCount++
	}
}

// Count returns the number of connected agents.
func (r *Registry) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.agents)
}

// CountByStatus returns counts by status.
func (r *Registry) CountByStatus() map[AgentStatus]int {
	r.mu.RLock()
	defer r.mu.RUnlock()

	counts := make(map[AgentStatus]int)
	for _, agent := range r.agents {
		counts[agent.Status]++
	}
	return counts
}

// SetOnConnect sets the callback for agent connection.
func (r *Registry) SetOnConnect(fn func(*ConnectedAgent)) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.onConnect = fn
}

// SetOnDisconnect sets the callback for agent disconnection.
func (r *Registry) SetOnDisconnect(fn func(*ConnectedAgent)) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.onDisconnect = fn
}

// SetOnChange sets the callback for agent changes.
func (r *Registry) SetOnChange(fn func(*ConnectedAgent)) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.onChange = fn
}

// cleanupLoop marks agents as offline if they haven't been seen recently.
func (r *Registry) cleanupLoop() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-r.ctx.Done():
			return
		case <-ticker.C:
			r.cleanupStaleAgents()
		}
	}
}

func (r *Registry) cleanupStaleAgents() {
	r.mu.Lock()
	defer r.mu.Unlock()

	threshold := time.Now().Add(-60 * time.Second)
	for _, agent := range r.agents {
		if agent.LastSeenAt.Before(threshold) && agent.Status == StatusOnline {
			agent.Status = StatusOffline
			if r.onChange != nil {
				go r.onChange(agent)
			}
		}
	}
}

func (r *Registry) hasCapability(cap Capability, agentID string) bool {
	for _, id := range r.byCap[cap] {
		if id == agentID {
			return true
		}
	}
	return false
}

func removeFromSlice(slice []string, item string) []string {
	result := make([]string, 0, len(slice))
	for _, s := range slice {
		if s != item {
			result = append(result, s)
		}
	}
	return result
}
