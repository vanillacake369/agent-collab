package libp2p

import (
	"testing"
	"time"

	"github.com/libp2p/go-libp2p/core/peer"
)

func TestDefaultLocalityConfig(t *testing.T) {
	config := DefaultLocalityConfig()

	if config.LocalRTTThreshold != 30*time.Millisecond {
		t.Errorf("LocalRTTThreshold = %v, want 30ms", config.LocalRTTThreshold)
	}

	if config.RegionalRTTThreshold != 100*time.Millisecond {
		t.Errorf("RegionalRTTThreshold = %v, want 100ms", config.RegionalRTTThreshold)
	}

	if config.LocalPeerRatio != 0.8 {
		t.Errorf("LocalPeerRatio = %f, want 0.8", config.LocalPeerRatio)
	}

	if config.MinRemotePeers != 2 {
		t.Errorf("MinRemotePeers = %d, want 2", config.MinRemotePeers)
	}
}

func TestLocalityManager_RegisterUnregisterPeer(t *testing.T) {
	config := DefaultLocalityConfig()
	config.MyRegion = "seoul"

	lm := &LocalityManager{
		nodeID:   peer.ID("local-node"),
		myRegion: config.MyRegion,
		config:   config,
		peers:    make(map[peer.ID]*PeerLocality),
		clusters: make(map[string]*LocalityCluster),
	}

	// Register a peer
	peerID := peer.ID("test-peer-1")
	lm.RegisterPeer(peerID, &PeerLocality{
		PeerID: peerID,
		Region: "seoul",
		RTT:    10 * time.Millisecond,
	})

	if loc := lm.GetLocality(peerID); loc == nil {
		t.Error("Peer should be registered")
	}

	// Unregister
	lm.UnregisterPeer(peerID)

	if loc := lm.GetLocality(peerID); loc != nil {
		t.Error("Peer should be unregistered")
	}
}

func TestLocalityManager_ClassifyRegion(t *testing.T) {
	config := DefaultLocalityConfig()
	config.MyRegion = "local"

	lm := &LocalityManager{
		nodeID:   peer.ID("local-node"),
		myRegion: config.MyRegion,
		config:   config,
		peers:    make(map[peer.ID]*PeerLocality),
		clusters: make(map[string]*LocalityCluster),
	}

	tests := []struct {
		rtt      time.Duration
		expected string
	}{
		{10 * time.Millisecond, "local"},    // < 30ms = local
		{50 * time.Millisecond, "regional"}, // 30-100ms = regional
		{200 * time.Millisecond, "remote"},  // > 100ms = remote
	}

	for _, tt := range tests {
		region := lm.classifyRegion(tt.rtt)
		if region != tt.expected {
			t.Errorf("classifyRegion(%v) = %s, want %s", tt.rtt, region, tt.expected)
		}
	}
}

func TestLocalityManager_UpdatePeerRTT(t *testing.T) {
	config := DefaultLocalityConfig()
	config.MyRegion = "local"

	lm := &LocalityManager{
		nodeID:   peer.ID("local-node"),
		myRegion: config.MyRegion,
		config:   config,
		peers:    make(map[peer.ID]*PeerLocality),
		clusters: make(map[string]*LocalityCluster),
	}

	peerID := peer.ID("test-peer")

	// First update creates the peer
	lm.UpdatePeerRTT(peerID, 20*time.Millisecond)

	loc := lm.GetLocality(peerID)
	if loc == nil {
		t.Fatal("Peer should exist after RTT update")
	}

	if loc.RTT != 20*time.Millisecond {
		t.Errorf("RTT = %v, want 20ms", loc.RTT)
	}

	if loc.RTTSamples != 1 {
		t.Errorf("RTTSamples = %d, want 1", loc.RTTSamples)
	}

	// Second update uses EMA
	lm.UpdatePeerRTT(peerID, 50*time.Millisecond)

	loc = lm.GetLocality(peerID)
	if loc.RTTSamples != 2 {
		t.Errorf("RTTSamples = %d, want 2", loc.RTTSamples)
	}

	// RTT should be between 20 and 50 (EMA)
	if loc.RTT < 20*time.Millisecond || loc.RTT > 50*time.Millisecond {
		t.Errorf("RTT = %v, should be between 20-50ms", loc.RTT)
	}
}

func TestLocalityManager_GetPeersByRegion(t *testing.T) {
	config := DefaultLocalityConfig()
	config.MyRegion = "local"

	lm := &LocalityManager{
		nodeID:   peer.ID("local-node"),
		myRegion: config.MyRegion,
		config:   config,
		peers:    make(map[peer.ID]*PeerLocality),
		clusters: make(map[string]*LocalityCluster),
	}

	// Add peers in different regions
	lm.RegisterPeer(peer.ID("local-1"), &PeerLocality{
		PeerID: peer.ID("local-1"),
		Region: "local",
	})
	lm.RegisterPeer(peer.ID("local-2"), &PeerLocality{
		PeerID: peer.ID("local-2"),
		Region: "local",
	})
	lm.RegisterPeer(peer.ID("regional-1"), &PeerLocality{
		PeerID: peer.ID("regional-1"),
		Region: "regional",
	})
	lm.RegisterPeer(peer.ID("remote-1"), &PeerLocality{
		PeerID: peer.ID("remote-1"),
		Region: "remote",
	})

	localPeers := lm.GetLocalPeers()
	if len(localPeers) != 2 {
		t.Errorf("Expected 2 local peers, got %d", len(localPeers))
	}

	regionalPeers := lm.GetRegionalPeers()
	if len(regionalPeers) != 1 {
		t.Errorf("Expected 1 regional peer, got %d", len(regionalPeers))
	}

	remotePeers := lm.GetRemotePeers()
	if len(remotePeers) != 1 {
		t.Errorf("Expected 1 remote peer, got %d", len(remotePeers))
	}
}

func TestLocalityManager_SelectPeersForMesh(t *testing.T) {
	config := DefaultLocalityConfig()
	config.MyRegion = "local"
	config.LocalPeerRatio = 0.7
	config.MinRemotePeers = 1

	lm := &LocalityManager{
		nodeID:   peer.ID("local-node"),
		myRegion: config.MyRegion,
		config:   config,
		peers:    make(map[peer.ID]*PeerLocality),
		clusters: make(map[string]*LocalityCluster),
	}

	// Add 5 local peers with varying RTT
	for i := 0; i < 5; i++ {
		id := peer.ID("local-" + string(rune('a'+i)))
		lm.RegisterPeer(id, &PeerLocality{
			PeerID: id,
			Region: "local",
			RTT:    time.Duration((i+1)*10) * time.Millisecond,
		})
	}

	// Add 3 remote peers
	for i := 0; i < 3; i++ {
		id := peer.ID("remote-" + string(rune('a'+i)))
		lm.RegisterPeer(id, &PeerLocality{
			PeerID: id,
			Region: "remote",
			RTT:    time.Duration((i+1)*100) * time.Millisecond,
		})
	}

	// Select 5 peers for mesh
	selected := lm.SelectPeersForMesh(5)

	if len(selected) != 5 {
		t.Errorf("Expected 5 selected peers, got %d", len(selected))
	}

	// Count local vs remote
	localCount := 0
	remoteCount := 0
	for _, id := range selected {
		loc := lm.GetLocality(id)
		if loc.Region == "local" {
			localCount++
		} else {
			remoteCount++
		}
	}

	// Should have at least 1 remote peer
	if remoteCount < 1 {
		t.Errorf("Expected at least 1 remote peer, got %d", remoteCount)
	}
}

func TestLocalityManager_ClusterManagement(t *testing.T) {
	config := DefaultLocalityConfig()
	config.MyRegion = "local"

	lm := &LocalityManager{
		nodeID:   peer.ID("local-node"),
		myRegion: config.MyRegion,
		config:   config,
		peers:    make(map[peer.ID]*PeerLocality),
		clusters: make(map[string]*LocalityCluster),
	}

	// Add peers to create a cluster
	lm.RegisterPeer(peer.ID("peer-1"), &PeerLocality{
		PeerID: peer.ID("peer-1"),
		Region: "tokyo",
		RTT:    50 * time.Millisecond,
	})
	lm.RegisterPeer(peer.ID("peer-2"), &PeerLocality{
		PeerID: peer.ID("peer-2"),
		Region: "tokyo",
		RTT:    30 * time.Millisecond,
	})

	cluster := lm.GetCluster("tokyo")
	if cluster == nil {
		t.Fatal("Tokyo cluster should exist")
	}

	if cluster.PeerCount != 2 {
		t.Errorf("PeerCount = %d, want 2", cluster.PeerCount)
	}

	// Gateway should be the peer with lowest RTT
	if cluster.GatewayPeer != peer.ID("peer-2") {
		t.Errorf("GatewayPeer = %s, want peer-2", cluster.GatewayPeer)
	}

	// Get all clusters
	allClusters := lm.GetAllClusters()
	if len(allClusters) != 1 {
		t.Errorf("Expected 1 cluster, got %d", len(allClusters))
	}
}

func TestLocalityManager_GetGatewayPeer(t *testing.T) {
	config := DefaultLocalityConfig()
	config.MyRegion = "local"

	lm := &LocalityManager{
		nodeID:   peer.ID("local-node"),
		myRegion: config.MyRegion,
		config:   config,
		peers:    make(map[peer.ID]*PeerLocality),
		clusters: make(map[string]*LocalityCluster),
	}

	// No cluster exists
	gateway := lm.GetGatewayPeer("nonexistent")
	if gateway != "" {
		t.Errorf("Gateway for nonexistent cluster should be empty")
	}

	// Create cluster
	lm.RegisterPeer(peer.ID("best-peer"), &PeerLocality{
		PeerID: peer.ID("best-peer"),
		Region: "osaka",
		RTT:    20 * time.Millisecond,
	})

	gateway = lm.GetGatewayPeer("osaka")
	if gateway != peer.ID("best-peer") {
		t.Errorf("Gateway = %s, want best-peer", gateway)
	}
}

func TestLocalityManager_Stats(t *testing.T) {
	config := DefaultLocalityConfig()
	config.MyRegion = "local"

	lm := &LocalityManager{
		nodeID:   peer.ID("local-node"),
		myRegion: config.MyRegion,
		config:   config,
		peers:    make(map[peer.ID]*PeerLocality),
		clusters: make(map[string]*LocalityCluster),
	}

	// Add peers
	lm.RegisterPeer(peer.ID("local-1"), &PeerLocality{Region: "local"})
	lm.RegisterPeer(peer.ID("local-2"), &PeerLocality{Region: "local"})
	lm.RegisterPeer(peer.ID("regional-1"), &PeerLocality{Region: "regional"})
	lm.RegisterPeer(peer.ID("remote-1"), &PeerLocality{Region: "remote"})
	lm.RegisterPeer(peer.ID("remote-2"), &PeerLocality{Region: "remote"})

	stats := lm.Stats()

	if stats.MyRegion != "local" {
		t.Errorf("MyRegion = %s, want local", stats.MyRegion)
	}
	if stats.TotalPeers != 5 {
		t.Errorf("TotalPeers = %d, want 5", stats.TotalPeers)
	}
	if stats.LocalPeers != 2 {
		t.Errorf("LocalPeers = %d, want 2", stats.LocalPeers)
	}
	if stats.RegionalPeers != 1 {
		t.Errorf("RegionalPeers = %d, want 1", stats.RegionalPeers)
	}
	if stats.RemotePeers != 2 {
		t.Errorf("RemotePeers = %d, want 2", stats.RemotePeers)
	}
}

func TestLocalityManager_OnClusterChangeCallback(t *testing.T) {
	config := DefaultLocalityConfig()
	config.MyRegion = "local"

	lm := &LocalityManager{
		nodeID:   peer.ID("local-node"),
		myRegion: config.MyRegion,
		config:   config,
		peers:    make(map[peer.ID]*PeerLocality),
		clusters: make(map[string]*LocalityCluster),
	}

	var callbackCalled bool
	var capturedCluster string
	var capturedEvent ClusterEvent

	lm.OnClusterChange(func(cluster string, event ClusterEvent) {
		callbackCalled = true
		capturedCluster = cluster
		capturedEvent = event
	})

	// Add first peer to create cluster
	lm.RegisterPeer(peer.ID("peer-1"), &PeerLocality{
		PeerID: peer.ID("peer-1"),
		Region: "tokyo",
		RTT:    50 * time.Millisecond,
	})

	// Add better peer - should trigger gateway change
	lm.RegisterPeer(peer.ID("better-peer"), &PeerLocality{
		PeerID: peer.ID("better-peer"),
		Region: "tokyo",
		RTT:    20 * time.Millisecond,
	})

	if !callbackCalled {
		t.Error("Cluster change callback should have been called")
	}
	if capturedCluster != "tokyo" {
		t.Errorf("Cluster = %s, want tokyo", capturedCluster)
	}
	if capturedEvent.Type != EventGatewayChanged {
		t.Errorf("Event type = %v, want EventGatewayChanged", capturedEvent.Type)
	}
	if capturedEvent.PeerID != peer.ID("better-peer") {
		t.Errorf("New gateway = %s, want better-peer", capturedEvent.PeerID)
	}
}
