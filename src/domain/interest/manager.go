package interest

import (
	"path/filepath"
	"sync"
)

// ChangeListener is a callback function for interest changes.
type ChangeListener func(change InterestChange)

// Manager manages agent interests and performs matching.
type Manager struct {
	mu        sync.RWMutex
	interests map[string]*Interest           // interestID -> Interest
	byAgent   map[string]map[string]struct{} // agentID -> set of interestIDs

	// Pattern cache for performance
	patternCache *patternCache

	// Change listeners for notifications
	listeners []ChangeListener
}

// NewManager creates a new interest manager.
func NewManager() *Manager {
	return &Manager{
		interests:    make(map[string]*Interest),
		byAgent:      make(map[string]map[string]struct{}),
		patternCache: newPatternCache(),
	}
}

// Register registers a new interest.
func (m *Manager) Register(interest *Interest) error {
	if interest == nil {
		return ErrNilInterest
	}
	if interest.AgentID == "" {
		return ErrEmptyAgentID
	}
	if len(interest.Patterns) == 0 {
		return ErrEmptyPatterns
	}

	m.mu.Lock()

	// Generate ID if not set
	if interest.ID == "" {
		interest.ID = generateInterestID()
	}

	// Store interest
	m.interests[interest.ID] = interest

	// Index by agent
	if _, exists := m.byAgent[interest.AgentID]; !exists {
		m.byAgent[interest.AgentID] = make(map[string]struct{})
	}
	m.byAgent[interest.AgentID][interest.ID] = struct{}{}

	// Cache patterns
	m.patternCache.add(interest.ID, interest.Patterns)

	m.mu.Unlock()

	// Notify listeners (after releasing lock)
	m.notifyUnlocked(ChangeTypeAdded, interest)

	return nil
}

// Unregister removes an interest.
func (m *Manager) Unregister(interestID string) error {
	m.mu.Lock()

	interest, exists := m.interests[interestID]
	if !exists {
		m.mu.Unlock()
		return ErrInterestNotFound
	}

	// Remove from agent index
	if agentInterests, ok := m.byAgent[interest.AgentID]; ok {
		delete(agentInterests, interestID)
		if len(agentInterests) == 0 {
			delete(m.byAgent, interest.AgentID)
		}
	}

	// Remove from pattern cache
	m.patternCache.remove(interestID)

	// Remove interest
	delete(m.interests, interestID)

	m.mu.Unlock()

	// Notify listeners (after releasing lock)
	m.notifyUnlocked(ChangeTypeRemoved, interest)

	return nil
}

// UnregisterAgent removes all interests for an agent.
func (m *Manager) UnregisterAgent(agentID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	interestIDs, exists := m.byAgent[agentID]
	if !exists {
		return nil
	}

	for interestID := range interestIDs {
		m.patternCache.remove(interestID)
		delete(m.interests, interestID)
	}

	delete(m.byAgent, agentID)

	return nil
}

// Get retrieves an interest by ID.
func (m *Manager) Get(interestID string) (*Interest, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	interest, exists := m.interests[interestID]
	if !exists {
		return nil, ErrInterestNotFound
	}

	return interest, nil
}

// GetAgentInterests returns all interests for an agent.
func (m *Manager) GetAgentInterests(agentID string) []*Interest {
	m.mu.RLock()
	defer m.mu.RUnlock()

	interestIDs, exists := m.byAgent[agentID]
	if !exists {
		return nil
	}

	interests := make([]*Interest, 0, len(interestIDs))
	for interestID := range interestIDs {
		if interest, ok := m.interests[interestID]; ok {
			interests = append(interests, interest)
		}
	}

	return interests
}

// Match finds all interests that match a file path.
func (m *Manager) Match(filePath string) []InterestMatch {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var matches []InterestMatch

	for _, interest := range m.interests {
		// Skip expired interests
		if interest.IsExpired() {
			continue
		}

		// Check direct pattern match
		if matched, pattern := m.matchPatterns(interest.Patterns, filePath); matched {
			matches = append(matches, *NewInterestMatch(interest, MatchTypeDirect, pattern))
			continue
		}

		// Check proximity match (same directory)
		if interest.Level == InterestLevelAll {
			if matched, pattern := m.matchProximity(interest.Patterns, filePath); matched {
				matches = append(matches, *NewInterestMatch(interest, MatchTypeProximity, pattern))
			}
		}
	}

	return matches
}

// MatchWithDependencies matches file path including dependency tracking.
// This is a placeholder for future dependency graph integration.
func (m *Manager) MatchWithDependencies(filePath string, dependencies []string) []InterestMatch {
	matches := m.Match(filePath)

	m.mu.RLock()
	defer m.mu.RUnlock()

	// Check if any interest patterns match the dependencies
	for _, interest := range m.interests {
		if interest.IsExpired() || !interest.TrackDependencies {
			continue
		}

		// Skip if already matched directly
		alreadyMatched := false
		for _, match := range matches {
			if match.Interest.ID == interest.ID {
				alreadyMatched = true
				break
			}
		}
		if alreadyMatched {
			continue
		}

		// Check dependency match
		for _, dep := range dependencies {
			if matched, _ := m.matchPatterns(interest.Patterns, dep); matched {
				matches = append(matches, *NewInterestMatch(interest, MatchTypeDependency, filePath))
				break
			}
		}
	}

	return matches
}

// matchPatterns checks if any pattern matches the file path.
func (m *Manager) matchPatterns(patterns []string, filePath string) (bool, string) {
	for _, pattern := range patterns {
		matched, err := filepath.Match(pattern, filePath)
		if err == nil && matched {
			return true, pattern
		}

		// Try matching with ** support (recursive)
		if matchGlobstar(pattern, filePath) {
			return true, pattern
		}
	}
	return false, ""
}

// matchProximity checks if the file is in a directory covered by patterns.
func (m *Manager) matchProximity(patterns []string, filePath string) (bool, string) {
	fileDir := filepath.Dir(filePath)

	for _, pattern := range patterns {
		patternDir := filepath.Dir(pattern)
		if patternDir == fileDir {
			return true, pattern
		}
	}
	return false, ""
}

// matchGlobstar handles ** patterns for recursive matching.
func matchGlobstar(pattern, path string) bool {
	// Simple implementation: check if pattern contains **
	for i := 0; i < len(pattern)-1; i++ {
		if pattern[i] == '*' && pattern[i+1] == '*' {
			// Split pattern at **
			prefix := pattern[:i]
			suffix := pattern[i+2:]

			// Remove leading / from suffix if present
			if len(suffix) > 0 && suffix[0] == '/' {
				suffix = suffix[1:]
			}

			// Check if path starts with prefix
			if len(prefix) > 0 && !matchPrefix(prefix, path) {
				return false
			}

			// If no suffix, prefix match is enough
			if len(suffix) == 0 {
				return true
			}

			// Check if any suffix of path matches the pattern suffix
			pathParts := splitPath(path)
			for j := 0; j < len(pathParts); j++ {
				remainingPath := joinPath(pathParts[j:])
				if matched, _ := filepath.Match(suffix, remainingPath); matched {
					return true
				}
			}
		}
	}
	return false
}

// matchPrefix checks if path starts with prefix.
func matchPrefix(prefix, path string) bool {
	if len(prefix) == 0 {
		return true
	}
	if len(path) < len(prefix) {
		return false
	}
	return path[:len(prefix)] == prefix
}

// splitPath splits a path into components.
func splitPath(path string) []string {
	var parts []string
	for path != "" {
		dir, file := filepath.Split(path)
		if file != "" {
			parts = append([]string{file}, parts...)
		}
		if dir == "" || dir == "/" {
			break
		}
		path = filepath.Clean(dir)
	}
	return parts
}

// joinPath joins path components.
func joinPath(parts []string) string {
	return filepath.Join(parts...)
}

// List returns all registered interests.
func (m *Manager) List() []*Interest {
	m.mu.RLock()
	defer m.mu.RUnlock()

	interests := make([]*Interest, 0, len(m.interests))
	for _, interest := range m.interests {
		interests = append(interests, interest)
	}
	return interests
}

// Count returns the number of registered interests.
func (m *Manager) Count() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.interests)
}

// CleanupExpired removes expired interests.
func (m *Manager) CleanupExpired() int {
	m.mu.Lock()
	defer m.mu.Unlock()

	var expired []string
	for id, interest := range m.interests {
		if interest.IsExpired() {
			expired = append(expired, id)
		}
	}

	for _, id := range expired {
		interest := m.interests[id]

		// Remove from agent index
		if agentInterests, ok := m.byAgent[interest.AgentID]; ok {
			delete(agentInterests, id)
			if len(agentInterests) == 0 {
				delete(m.byAgent, interest.AgentID)
			}
		}

		// Remove from cache and map
		m.patternCache.remove(id)
		delete(m.interests, id)
	}

	return len(expired)
}

// patternCache caches compiled patterns for performance.
type patternCache struct {
	mu       sync.RWMutex
	patterns map[string][]string // interestID -> patterns
}

func newPatternCache() *patternCache {
	return &patternCache{
		patterns: make(map[string][]string),
	}
}

func (c *patternCache) add(interestID string, patterns []string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.patterns[interestID] = patterns
}

func (c *patternCache) remove(interestID string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.patterns, interestID)
}

// OnChange registers a listener for interest changes.
func (m *Manager) OnChange(listener ChangeListener) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.listeners = append(m.listeners, listener)
}

// notifyUnlocked notifies all listeners of a change (must be called without holding mu).
func (m *Manager) notifyUnlocked(changeType ChangeType, interest *Interest) {
	// Copy listeners to avoid holding lock during callback
	m.mu.RLock()
	listeners := make([]ChangeListener, len(m.listeners))
	copy(listeners, m.listeners)
	m.mu.RUnlock()

	change := InterestChange{
		Type:     changeType,
		Interest: interest,
	}

	for _, listener := range listeners {
		listener(change)
	}
}

// Snapshot returns a copy of all interests for synchronization.
func (m *Manager) Snapshot() []*Interest {
	m.mu.RLock()
	defer m.mu.RUnlock()

	snapshot := make([]*Interest, 0, len(m.interests))
	for _, interest := range m.interests {
		// Create a copy to avoid mutation
		copy := *interest
		snapshot = append(snapshot, &copy)
	}
	return snapshot
}

// MergeRemote merges remote interests into the local manager.
// Remote interests are marked with Remote=true and won't trigger notifications.
func (m *Manager) MergeRemote(remoteInterests []*Interest) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, remote := range remoteInterests {
		// Skip if already exists
		if _, exists := m.interests[remote.ID]; exists {
			continue
		}

		// Ensure it's marked as remote
		remote.Remote = true

		// Store interest
		m.interests[remote.ID] = remote

		// Index by agent
		if _, exists := m.byAgent[remote.AgentID]; !exists {
			m.byAgent[remote.AgentID] = make(map[string]struct{})
		}
		m.byAgent[remote.AgentID][remote.ID] = struct{}{}

		// Cache patterns
		m.patternCache.add(remote.ID, remote.Patterns)
	}
}

// GetRemoteInterests returns all remote interests.
func (m *Manager) GetRemoteInterests() []*Interest {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var remote []*Interest
	for _, interest := range m.interests {
		if interest.Remote {
			remote = append(remote, interest)
		}
	}
	return remote
}

// ClearRemote removes all remote interests.
func (m *Manager) ClearRemote() int {
	m.mu.Lock()
	defer m.mu.Unlock()

	var toRemove []string
	for id, interest := range m.interests {
		if interest.Remote {
			toRemove = append(toRemove, id)
		}
	}

	for _, id := range toRemove {
		interest := m.interests[id]

		// Remove from agent index
		if agentInterests, ok := m.byAgent[interest.AgentID]; ok {
			delete(agentInterests, id)
			if len(agentInterests) == 0 {
				delete(m.byAgent, interest.AgentID)
			}
		}

		// Remove from cache and map
		m.patternCache.remove(id)
		delete(m.interests, id)
	}

	return len(toRemove)
}
