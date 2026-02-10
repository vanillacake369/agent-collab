package libp2p

import (
	"context"
	"sort"
	"sync"
	"time"

	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
)

// PeerRole represents the role of a peer in the hierarchical topology
type PeerRole int

const (
	// RoleLeaf is a regular peer that connects to 1-3 super peers
	RoleLeaf PeerRole = iota
	// RoleSuper is a super peer that connects to many leaves and other super peers
	RoleSuper
)

func (r PeerRole) String() string {
	switch r {
	case RoleLeaf:
		return "leaf"
	case RoleSuper:
		return "super"
	default:
		return "unknown"
	}
}

// SuperPeerCriteria defines the criteria for super peer election
type SuperPeerCriteria struct {
	// MinUptime is the minimum uptime required to become a super peer
	MinUptime time.Duration
	// MinBandwidthMbps is the minimum bandwidth in Mbps
	MinBandwidthMbps float64
	// MaxLatency99p is the maximum 99th percentile latency
	MaxLatency99p time.Duration
	// MinConnections is the minimum number of connections
	MinConnections int
	// MinScore is the minimum peer quality score (0-1)
	MinScore float64
}

// DefaultSuperPeerCriteria returns sensible defaults for super peer election
func DefaultSuperPeerCriteria() SuperPeerCriteria {
	return SuperPeerCriteria{
		MinUptime:        30 * time.Minute,
		MinBandwidthMbps: 10.0,
		MaxLatency99p:    100 * time.Millisecond,
		MinConnections:   10,
		MinScore:         0.7,
	}
}

// PeerInfo holds information about a peer for topology management
type PeerInfo struct {
	ID            peer.ID
	Role          PeerRole
	Uptime        time.Duration
	BandwidthMbps float64
	Latency99p    time.Duration
	Connections   int
	Score         float64
	JoinedAt      time.Time
	LastSeen      time.Time
	SuperPeers    []peer.ID // For leaf peers: their super peers
	LeafPeers     []peer.ID // For super peers: their leaf peers
}

// TopologyManager manages the hierarchical network topology
type TopologyManager struct {
	mu sync.RWMutex

	host         host.Host
	nodeID       peer.ID
	myRole       PeerRole
	criteria     SuperPeerCriteria
	peers        map[peer.ID]*PeerInfo
	superPeers   map[peer.ID]struct{} // Set of known super peers
	mySuperPeers []peer.ID            // For leaf: connected super peers
	myLeafPeers  []peer.ID            // For super: connected leaf peers

	// Configuration
	config TopologyConfig

	// Quality monitor integration
	qualityMonitor *PeerQualityMonitor

	// Callbacks
	onRoleChange     func(oldRole, newRole PeerRole)
	onTopologyChange func(event TopologyEvent)

	ctx    context.Context
	cancel context.CancelFunc
}

// TopologyConfig configures the topology manager
type TopologyConfig struct {
	// MaxSuperPeersPerLeaf is how many super peers a leaf should connect to
	MaxSuperPeersPerLeaf int
	// MaxLeafPeersPerSuper is how many leaf peers a super peer can handle
	MaxLeafPeersPerSuper int
	// SuperPeerRatio is the target ratio of super peers (e.g., 0.1 = 10%)
	SuperPeerRatio float64
	// ElectionInterval is how often to re-evaluate super peer election
	ElectionInterval time.Duration
	// HeartbeatInterval is how often to send heartbeats
	HeartbeatInterval time.Duration
	// PeerTimeout is how long before a peer is considered disconnected
	PeerTimeout time.Duration
}

// DefaultTopologyConfig returns sensible defaults
func DefaultTopologyConfig() TopologyConfig {
	return TopologyConfig{
		MaxSuperPeersPerLeaf: 3,
		MaxLeafPeersPerSuper: 50,
		SuperPeerRatio:       0.1,
		ElectionInterval:     5 * time.Minute,
		HeartbeatInterval:    30 * time.Second,
		PeerTimeout:          2 * time.Minute,
	}
}

// TopologyEvent represents a topology change event
type TopologyEvent struct {
	Type      TopologyEventType
	PeerID    peer.ID
	OldRole   PeerRole
	NewRole   PeerRole
	Timestamp time.Time
}

// TopologyEventType identifies the type of topology event
type TopologyEventType int

const (
	EventPeerJoined TopologyEventType = iota
	EventPeerLeft
	EventRoleChanged
	EventSuperPeerElected
	EventSuperPeerDemoted
)

// NewTopologyManager creates a new topology manager
func NewTopologyManager(h host.Host, config TopologyConfig, criteria SuperPeerCriteria) *TopologyManager {
	ctx, cancel := context.WithCancel(context.Background())

	tm := &TopologyManager{
		host:         h,
		nodeID:       h.ID(),
		myRole:       RoleLeaf, // Start as leaf
		criteria:     criteria,
		config:       config,
		peers:        make(map[peer.ID]*PeerInfo),
		superPeers:   make(map[peer.ID]struct{}),
		mySuperPeers: make([]peer.ID, 0),
		myLeafPeers:  make([]peer.ID, 0),
		ctx:          ctx,
		cancel:       cancel,
	}

	return tm
}

// SetQualityMonitor sets the peer quality monitor for score-based decisions
func (tm *TopologyManager) SetQualityMonitor(qm *PeerQualityMonitor) {
	tm.mu.Lock()
	tm.qualityMonitor = qm
	tm.mu.Unlock()
}

// OnRoleChange registers a callback for role changes
func (tm *TopologyManager) OnRoleChange(fn func(oldRole, newRole PeerRole)) {
	tm.mu.Lock()
	tm.onRoleChange = fn
	tm.mu.Unlock()
}

// OnTopologyChange registers a callback for topology changes
func (tm *TopologyManager) OnTopologyChange(fn func(event TopologyEvent)) {
	tm.mu.Lock()
	tm.onTopologyChange = fn
	tm.mu.Unlock()
}

// Start starts the topology management loops
func (tm *TopologyManager) Start() {
	go tm.electionLoop()
	go tm.maintenanceLoop()
}

// Stop stops the topology manager
func (tm *TopologyManager) Stop() {
	tm.cancel()
}

// GetRole returns the current role of this node
func (tm *TopologyManager) GetRole() PeerRole {
	tm.mu.RLock()
	defer tm.mu.RUnlock()
	return tm.myRole
}

// GetSuperPeers returns the list of known super peers
func (tm *TopologyManager) GetSuperPeers() []peer.ID {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	result := make([]peer.ID, 0, len(tm.superPeers))
	for id := range tm.superPeers {
		result = append(result, id)
	}
	return result
}

// GetMySuperPeers returns the super peers this leaf is connected to
func (tm *TopologyManager) GetMySuperPeers() []peer.ID {
	tm.mu.RLock()
	defer tm.mu.RUnlock()
	return append([]peer.ID{}, tm.mySuperPeers...)
}

// GetMyLeafPeers returns the leaf peers this super peer is serving
func (tm *TopologyManager) GetMyLeafPeers() []peer.ID {
	tm.mu.RLock()
	defer tm.mu.RUnlock()
	return append([]peer.ID{}, tm.myLeafPeers...)
}

// RegisterPeer registers a peer with the topology manager
func (tm *TopologyManager) RegisterPeer(id peer.ID, info *PeerInfo) {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	if info == nil {
		info = &PeerInfo{
			ID:       id,
			Role:     RoleLeaf,
			JoinedAt: time.Now(),
			LastSeen: time.Now(),
		}
	}

	tm.peers[id] = info

	if info.Role == RoleSuper {
		tm.superPeers[id] = struct{}{}
	}

	// Emit event
	if tm.onTopologyChange != nil {
		tm.onTopologyChange(TopologyEvent{
			Type:      EventPeerJoined,
			PeerID:    id,
			NewRole:   info.Role,
			Timestamp: time.Now(),
		})
	}
}

// UnregisterPeer removes a peer from the topology
func (tm *TopologyManager) UnregisterPeer(id peer.ID) {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	info, exists := tm.peers[id]
	if !exists {
		return
	}

	delete(tm.peers, id)
	delete(tm.superPeers, id)

	// Remove from our super peers list if present
	tm.mySuperPeers = removePeer(tm.mySuperPeers, id)
	tm.myLeafPeers = removePeer(tm.myLeafPeers, id)

	// Emit event
	if tm.onTopologyChange != nil {
		tm.onTopologyChange(TopologyEvent{
			Type:      EventPeerLeft,
			PeerID:    id,
			OldRole:   info.Role,
			Timestamp: time.Now(),
		})
	}
}

// UpdatePeerInfo updates information about a peer
func (tm *TopologyManager) UpdatePeerInfo(id peer.ID, update func(*PeerInfo)) {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	info, exists := tm.peers[id]
	if !exists {
		return
	}

	oldRole := info.Role
	update(info)
	info.LastSeen = time.Now()

	// Handle role change
	if info.Role != oldRole {
		if info.Role == RoleSuper {
			tm.superPeers[id] = struct{}{}
		} else {
			delete(tm.superPeers, id)
		}

		if tm.onTopologyChange != nil {
			tm.onTopologyChange(TopologyEvent{
				Type:      EventRoleChanged,
				PeerID:    id,
				OldRole:   oldRole,
				NewRole:   info.Role,
				Timestamp: time.Now(),
			})
		}
	}
}

// electionLoop periodically evaluates if this node should become a super peer
func (tm *TopologyManager) electionLoop() {
	ticker := time.NewTicker(tm.config.ElectionInterval)
	defer ticker.Stop()

	for {
		select {
		case <-tm.ctx.Done():
			return
		case <-ticker.C:
			tm.evaluateElection()
		}
	}
}

// evaluateElection checks if this node qualifies to be a super peer
func (tm *TopologyManager) evaluateElection() {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	// Get our own metrics
	uptime := time.Since(tm.getNodeStartTime())
	connections := len(tm.host.Network().Peers())

	score := 0.5 // Default
	if tm.qualityMonitor != nil {
		// Use average score of our connections as a proxy for our quality
		stats := tm.qualityMonitor.Stats()
		score = stats.AverageScore
	}

	// Check if we meet super peer criteria
	qualifies := uptime >= tm.criteria.MinUptime &&
		connections >= tm.criteria.MinConnections &&
		score >= tm.criteria.MinScore

	// Calculate target super peer count
	totalPeers := len(tm.peers) + 1 // Include ourselves
	targetSupers := int(float64(totalPeers) * tm.config.SuperPeerRatio)
	if targetSupers < 1 {
		targetSupers = 1
	}

	currentSupers := len(tm.superPeers)
	if tm.myRole == RoleSuper {
		currentSupers++
	}

	// Decision logic
	oldRole := tm.myRole

	if qualifies && currentSupers < targetSupers {
		// Need more super peers and we qualify
		tm.myRole = RoleSuper
	} else if !qualifies && tm.myRole == RoleSuper && currentSupers > targetSupers {
		// We don't qualify and there are enough super peers
		tm.myRole = RoleLeaf
	}

	// Notify if role changed
	if tm.myRole != oldRole {
		if tm.onRoleChange != nil {
			tm.onRoleChange(oldRole, tm.myRole)
		}

		eventType := EventSuperPeerElected
		if tm.myRole == RoleLeaf {
			eventType = EventSuperPeerDemoted
		}

		if tm.onTopologyChange != nil {
			tm.onTopologyChange(TopologyEvent{
				Type:      eventType,
				PeerID:    tm.nodeID,
				OldRole:   oldRole,
				NewRole:   tm.myRole,
				Timestamp: time.Now(),
			})
		}
	}
}

// maintenanceLoop handles ongoing topology maintenance
func (tm *TopologyManager) maintenanceLoop() {
	ticker := time.NewTicker(tm.config.HeartbeatInterval)
	defer ticker.Stop()

	for {
		select {
		case <-tm.ctx.Done():
			return
		case <-ticker.C:
			tm.maintainConnections()
			tm.cleanupStalePeers()
		}
	}
}

// maintainConnections ensures proper connectivity based on role
func (tm *TopologyManager) maintainConnections() {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	if tm.myRole == RoleLeaf {
		tm.maintainLeafConnections()
	} else {
		tm.maintainSuperConnections()
	}
}

// maintainLeafConnections ensures leaf is connected to enough super peers
func (tm *TopologyManager) maintainLeafConnections() {
	// Remove disconnected super peers
	var connected []peer.ID
	for _, sp := range tm.mySuperPeers {
		if tm.host.Network().Connectedness(sp) == 1 { // Connected
			connected = append(connected, sp)
		}
	}
	tm.mySuperPeers = connected

	// Need more super peers?
	if len(tm.mySuperPeers) < tm.config.MaxSuperPeersPerLeaf {
		needed := tm.config.MaxSuperPeersPerLeaf - len(tm.mySuperPeers)
		candidates := tm.selectBestSuperPeers(needed)
		tm.mySuperPeers = append(tm.mySuperPeers, candidates...)
	}
}

// maintainSuperConnections ensures super peer has proper connectivity
func (tm *TopologyManager) maintainSuperConnections() {
	// Remove disconnected leaf peers
	var connected []peer.ID
	for _, lp := range tm.myLeafPeers {
		if tm.host.Network().Connectedness(lp) == 1 {
			connected = append(connected, lp)
		}
	}
	tm.myLeafPeers = connected

	// Limit leaf peers if needed
	if len(tm.myLeafPeers) > tm.config.MaxLeafPeersPerSuper {
		// Keep the best ones based on quality
		tm.myLeafPeers = tm.selectBestLeafPeers(tm.config.MaxLeafPeersPerSuper)
	}
}

// selectBestSuperPeers selects the best N super peers based on quality
func (tm *TopologyManager) selectBestSuperPeers(n int) []peer.ID {
	type scoredPeer struct {
		id    peer.ID
		score float64
	}

	var candidates []scoredPeer

	for id := range tm.superPeers {
		// Skip if already connected
		if containsPeer(tm.mySuperPeers, id) {
			continue
		}

		score := 0.5
		if tm.qualityMonitor != nil {
			score = tm.qualityMonitor.GetScore(id)
		}

		candidates = append(candidates, scoredPeer{id: id, score: score})
	}

	// Sort by score descending
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].score > candidates[j].score
	})

	result := make([]peer.ID, 0, n)
	for i := 0; i < len(candidates) && i < n; i++ {
		result = append(result, candidates[i].id)
	}

	return result
}

// selectBestLeafPeers selects the best N leaf peers based on quality
func (tm *TopologyManager) selectBestLeafPeers(n int) []peer.ID {
	type scoredPeer struct {
		id    peer.ID
		score float64
	}

	var scored []scoredPeer
	for _, id := range tm.myLeafPeers {
		score := 0.5
		if tm.qualityMonitor != nil {
			score = tm.qualityMonitor.GetScore(id)
		}
		scored = append(scored, scoredPeer{id: id, score: score})
	}

	// Sort by score descending
	sort.Slice(scored, func(i, j int) bool {
		return scored[i].score > scored[j].score
	})

	result := make([]peer.ID, 0, n)
	for i := 0; i < len(scored) && i < n; i++ {
		result = append(result, scored[i].id)
	}

	return result
}

// cleanupStalePeers removes peers that haven't been seen recently
func (tm *TopologyManager) cleanupStalePeers() {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	now := time.Now()
	var toRemove []peer.ID

	for id, info := range tm.peers {
		if now.Sub(info.LastSeen) > tm.config.PeerTimeout {
			toRemove = append(toRemove, id)
		}
	}

	for _, id := range toRemove {
		delete(tm.peers, id)
		delete(tm.superPeers, id)
		tm.mySuperPeers = removePeer(tm.mySuperPeers, id)
		tm.myLeafPeers = removePeer(tm.myLeafPeers, id)
	}
}

// AddLeafPeer adds a leaf peer (called when a leaf connects to us as super)
func (tm *TopologyManager) AddLeafPeer(id peer.ID) bool {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	if tm.myRole != RoleSuper {
		return false
	}

	// Already exists - return true
	if containsPeer(tm.myLeafPeers, id) {
		return true
	}

	// Check capacity
	if len(tm.myLeafPeers) >= tm.config.MaxLeafPeersPerSuper {
		return false
	}

	tm.myLeafPeers = append(tm.myLeafPeers, id)
	return true
}

// Stats returns topology statistics
func (tm *TopologyManager) Stats() TopologyStats {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	return TopologyStats{
		MyRole:           tm.myRole,
		TotalPeers:       len(tm.peers),
		SuperPeerCount:   len(tm.superPeers),
		MySuperPeerCount: len(tm.mySuperPeers),
		MyLeafPeerCount:  len(tm.myLeafPeers),
	}
}

// TopologyStats holds topology statistics
type TopologyStats struct {
	MyRole           PeerRole `json:"my_role"`
	TotalPeers       int      `json:"total_peers"`
	SuperPeerCount   int      `json:"super_peer_count"`
	MySuperPeerCount int      `json:"my_super_peer_count"`
	MyLeafPeerCount  int      `json:"my_leaf_peer_count"`
}

// getNodeStartTime returns when this node started (placeholder - should be set at startup)
func (tm *TopologyManager) getNodeStartTime() time.Time {
	// In production, this would be set at node startup
	return time.Now().Add(-time.Hour) // Default: 1 hour ago
}

// Helper functions

func removePeer(peers []peer.ID, id peer.ID) []peer.ID {
	result := make([]peer.ID, 0, len(peers))
	for _, p := range peers {
		if p != id {
			result = append(result, p)
		}
	}
	return result
}

func containsPeer(peers []peer.ID, id peer.ID) bool {
	for _, p := range peers {
		if p == id {
			return true
		}
	}
	return false
}
