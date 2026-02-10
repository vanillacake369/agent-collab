package libp2p

import (
	"testing"
	"time"

	"github.com/libp2p/go-libp2p/core/peer"
)

func TestPeerRole_String(t *testing.T) {
	tests := []struct {
		role     PeerRole
		expected string
	}{
		{RoleLeaf, "leaf"},
		{RoleSuper, "super"},
		{PeerRole(99), "unknown"},
	}

	for _, tt := range tests {
		if got := tt.role.String(); got != tt.expected {
			t.Errorf("PeerRole(%d).String() = %s, want %s", tt.role, got, tt.expected)
		}
	}
}

func TestDefaultSuperPeerCriteria(t *testing.T) {
	criteria := DefaultSuperPeerCriteria()

	if criteria.MinUptime != 30*time.Minute {
		t.Errorf("MinUptime = %v, want 30m", criteria.MinUptime)
	}

	if criteria.MinScore != 0.7 {
		t.Errorf("MinScore = %v, want 0.7", criteria.MinScore)
	}

	if criteria.MinConnections != 10 {
		t.Errorf("MinConnections = %v, want 10", criteria.MinConnections)
	}
}

func TestDefaultTopologyConfig(t *testing.T) {
	config := DefaultTopologyConfig()

	if config.MaxSuperPeersPerLeaf != 3 {
		t.Errorf("MaxSuperPeersPerLeaf = %d, want 3", config.MaxSuperPeersPerLeaf)
	}

	if config.MaxLeafPeersPerSuper != 50 {
		t.Errorf("MaxLeafPeersPerSuper = %d, want 50", config.MaxLeafPeersPerSuper)
	}

	if config.SuperPeerRatio != 0.1 {
		t.Errorf("SuperPeerRatio = %f, want 0.1", config.SuperPeerRatio)
	}
}

func TestTopologyManager_RegisterUnregisterPeer(t *testing.T) {
	config := DefaultTopologyConfig()
	criteria := DefaultSuperPeerCriteria()

	// Create a mock host (nil for testing without network)
	tm := &TopologyManager{
		nodeID:       peer.ID("local-node"),
		myRole:       RoleLeaf,
		criteria:     criteria,
		config:       config,
		peers:        make(map[peer.ID]*PeerInfo),
		superPeers:   make(map[peer.ID]struct{}),
		mySuperPeers: make([]peer.ID, 0),
		myLeafPeers:  make([]peer.ID, 0),
	}

	// Register a regular peer
	peerID := peer.ID("test-peer-1")
	tm.RegisterPeer(peerID, nil)

	tm.mu.RLock()
	if _, exists := tm.peers[peerID]; !exists {
		t.Error("Peer should be registered")
	}
	tm.mu.RUnlock()

	// Register a super peer
	superPeerID := peer.ID("super-peer-1")
	tm.RegisterPeer(superPeerID, &PeerInfo{
		ID:   superPeerID,
		Role: RoleSuper,
	})

	tm.mu.RLock()
	if _, exists := tm.superPeers[superPeerID]; !exists {
		t.Error("Super peer should be in superPeers set")
	}
	tm.mu.RUnlock()

	// Unregister
	tm.UnregisterPeer(peerID)

	tm.mu.RLock()
	if _, exists := tm.peers[peerID]; exists {
		t.Error("Peer should be unregistered")
	}
	tm.mu.RUnlock()
}

func TestTopologyManager_UpdatePeerInfo(t *testing.T) {
	config := DefaultTopologyConfig()
	criteria := DefaultSuperPeerCriteria()

	tm := &TopologyManager{
		nodeID:       peer.ID("local-node"),
		myRole:       RoleLeaf,
		criteria:     criteria,
		config:       config,
		peers:        make(map[peer.ID]*PeerInfo),
		superPeers:   make(map[peer.ID]struct{}),
		mySuperPeers: make([]peer.ID, 0),
		myLeafPeers:  make([]peer.ID, 0),
	}

	peerID := peer.ID("test-peer")
	tm.RegisterPeer(peerID, &PeerInfo{
		ID:    peerID,
		Role:  RoleLeaf,
		Score: 0.5,
	})

	// Update peer info
	tm.UpdatePeerInfo(peerID, func(info *PeerInfo) {
		info.Score = 0.9
		info.Role = RoleSuper
	})

	tm.mu.RLock()
	info := tm.peers[peerID]
	if info.Score != 0.9 {
		t.Errorf("Score = %f, want 0.9", info.Score)
	}
	if info.Role != RoleSuper {
		t.Errorf("Role = %v, want RoleSuper", info.Role)
	}
	if _, exists := tm.superPeers[peerID]; !exists {
		t.Error("Should be in superPeers after role change")
	}
	tm.mu.RUnlock()
}

func TestTopologyManager_GetSuperPeers(t *testing.T) {
	config := DefaultTopologyConfig()
	criteria := DefaultSuperPeerCriteria()

	tm := &TopologyManager{
		nodeID:       peer.ID("local-node"),
		myRole:       RoleLeaf,
		criteria:     criteria,
		config:       config,
		peers:        make(map[peer.ID]*PeerInfo),
		superPeers:   make(map[peer.ID]struct{}),
		mySuperPeers: make([]peer.ID, 0),
		myLeafPeers:  make([]peer.ID, 0),
	}

	// Add super peers
	for i := 0; i < 3; i++ {
		id := peer.ID("super-" + string(rune('a'+i)))
		tm.RegisterPeer(id, &PeerInfo{
			ID:   id,
			Role: RoleSuper,
		})
	}

	superPeers := tm.GetSuperPeers()
	if len(superPeers) != 3 {
		t.Errorf("Expected 3 super peers, got %d", len(superPeers))
	}
}

func TestTopologyManager_Stats(t *testing.T) {
	config := DefaultTopologyConfig()
	criteria := DefaultSuperPeerCriteria()

	tm := &TopologyManager{
		nodeID:       peer.ID("local-node"),
		myRole:       RoleSuper,
		criteria:     criteria,
		config:       config,
		peers:        make(map[peer.ID]*PeerInfo),
		superPeers:   make(map[peer.ID]struct{}),
		mySuperPeers: make([]peer.ID, 0),
		myLeafPeers:  []peer.ID{peer.ID("leaf-1"), peer.ID("leaf-2")},
	}

	// Add some peers
	tm.RegisterPeer(peer.ID("peer-1"), nil)
	tm.RegisterPeer(peer.ID("super-1"), &PeerInfo{ID: peer.ID("super-1"), Role: RoleSuper})

	stats := tm.Stats()

	if stats.MyRole != RoleSuper {
		t.Errorf("MyRole = %v, want RoleSuper", stats.MyRole)
	}
	if stats.TotalPeers != 2 {
		t.Errorf("TotalPeers = %d, want 2", stats.TotalPeers)
	}
	if stats.SuperPeerCount != 1 {
		t.Errorf("SuperPeerCount = %d, want 1", stats.SuperPeerCount)
	}
	if stats.MyLeafPeerCount != 2 {
		t.Errorf("MyLeafPeerCount = %d, want 2", stats.MyLeafPeerCount)
	}
}

func TestTopologyManager_AddLeafPeer(t *testing.T) {
	config := DefaultTopologyConfig()
	config.MaxLeafPeersPerSuper = 2
	criteria := DefaultSuperPeerCriteria()

	tm := &TopologyManager{
		nodeID:       peer.ID("local-node"),
		myRole:       RoleSuper,
		criteria:     criteria,
		config:       config,
		peers:        make(map[peer.ID]*PeerInfo),
		superPeers:   make(map[peer.ID]struct{}),
		mySuperPeers: make([]peer.ID, 0),
		myLeafPeers:  make([]peer.ID, 0),
	}

	// Add first leaf
	if !tm.AddLeafPeer(peer.ID("leaf-1")) {
		t.Error("Should accept first leaf peer")
	}

	// Add second leaf
	if !tm.AddLeafPeer(peer.ID("leaf-2")) {
		t.Error("Should accept second leaf peer")
	}

	// Third should be rejected (max is 2)
	if tm.AddLeafPeer(peer.ID("leaf-3")) {
		t.Error("Should reject third leaf peer (over limit)")
	}

	// Duplicate should be accepted (already exists)
	if !tm.AddLeafPeer(peer.ID("leaf-1")) {
		t.Error("Should accept duplicate (already exists)")
	}

	if len(tm.myLeafPeers) != 2 {
		t.Errorf("Expected 2 leaf peers, got %d", len(tm.myLeafPeers))
	}
}

func TestTopologyManager_LeafCannotAddLeafPeers(t *testing.T) {
	config := DefaultTopologyConfig()
	criteria := DefaultSuperPeerCriteria()

	tm := &TopologyManager{
		nodeID:       peer.ID("local-node"),
		myRole:       RoleLeaf, // We are a leaf
		criteria:     criteria,
		config:       config,
		peers:        make(map[peer.ID]*PeerInfo),
		superPeers:   make(map[peer.ID]struct{}),
		mySuperPeers: make([]peer.ID, 0),
		myLeafPeers:  make([]peer.ID, 0),
	}

	// Leaf nodes cannot accept leaf peers
	if tm.AddLeafPeer(peer.ID("leaf-1")) {
		t.Error("Leaf node should not accept leaf peers")
	}
}

func TestTopologyManager_OnRoleChangeCallback(t *testing.T) {
	config := DefaultTopologyConfig()
	criteria := DefaultSuperPeerCriteria()

	tm := &TopologyManager{
		nodeID:       peer.ID("local-node"),
		myRole:       RoleLeaf,
		criteria:     criteria,
		config:       config,
		peers:        make(map[peer.ID]*PeerInfo),
		superPeers:   make(map[peer.ID]struct{}),
		mySuperPeers: make([]peer.ID, 0),
		myLeafPeers:  make([]peer.ID, 0),
	}

	var callbackCalled bool
	var capturedOld, capturedNew PeerRole

	tm.OnRoleChange(func(oldRole, newRole PeerRole) {
		callbackCalled = true
		capturedOld = oldRole
		capturedNew = newRole
	})

	// Manually trigger role change for testing
	tm.mu.Lock()
	oldRole := tm.myRole
	tm.myRole = RoleSuper
	if tm.onRoleChange != nil {
		tm.onRoleChange(oldRole, tm.myRole)
	}
	tm.mu.Unlock()

	if !callbackCalled {
		t.Error("Role change callback should have been called")
	}
	if capturedOld != RoleLeaf || capturedNew != RoleSuper {
		t.Errorf("Callback got (%v, %v), want (RoleLeaf, RoleSuper)", capturedOld, capturedNew)
	}
}

func TestHelperFunctions(t *testing.T) {
	t.Run("removePeer", func(t *testing.T) {
		peers := []peer.ID{"a", "b", "c"}
		result := removePeer(peers, "b")

		if len(result) != 2 {
			t.Errorf("Expected 2 peers, got %d", len(result))
		}
		if containsPeer(result, "b") {
			t.Error("Should not contain removed peer")
		}
	})

	t.Run("containsPeer", func(t *testing.T) {
		peers := []peer.ID{"a", "b", "c"}

		if !containsPeer(peers, "b") {
			t.Error("Should contain 'b'")
		}
		if containsPeer(peers, "d") {
			t.Error("Should not contain 'd'")
		}
	})
}

func TestTopologyManager_SelectBestSuperPeers(t *testing.T) {
	config := DefaultTopologyConfig()
	criteria := DefaultSuperPeerCriteria()

	tm := &TopologyManager{
		nodeID:       peer.ID("local-node"),
		myRole:       RoleLeaf,
		criteria:     criteria,
		config:       config,
		peers:        make(map[peer.ID]*PeerInfo),
		superPeers:   make(map[peer.ID]struct{}),
		mySuperPeers: make([]peer.ID, 0),
		myLeafPeers:  make([]peer.ID, 0),
	}

	// Add super peers with different quality (simulated by adding them)
	superPeers := []peer.ID{"super-a", "super-b", "super-c", "super-d"}
	for _, sp := range superPeers {
		tm.superPeers[sp] = struct{}{}
	}

	// Select best 2
	best := tm.selectBestSuperPeers(2)

	if len(best) != 2 {
		t.Errorf("Expected 2 best super peers, got %d", len(best))
	}
}
