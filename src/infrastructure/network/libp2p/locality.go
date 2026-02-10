package libp2p

import (
	"context"
	"sort"
	"sync"
	"time"

	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
)

// PeerLocality represents the geographic/network locality of a peer
type PeerLocality struct {
	PeerID     peer.ID       `json:"peer_id"`
	Region     string        `json:"region"`      // e.g., "ap-northeast-2", "us-west-1"
	Cluster    string        `json:"cluster"`     // e.g., "seoul-az1", "oregon-az2"
	RTT        time.Duration `json:"rtt"`         // Measured RTT to this peer
	RTTSamples int           `json:"rtt_samples"` // Number of RTT measurements
	LastProbe  time.Time     `json:"last_probe"`  // When RTT was last measured
}

// LocalityCluster represents a group of peers in the same region
type LocalityCluster struct {
	Region      string        `json:"region"`
	PeerCount   int           `json:"peer_count"`
	AverageRTT  time.Duration `json:"average_rtt"`
	GatewayPeer peer.ID       `json:"gateway_peer"` // Best peer to reach this cluster
}

// LocalityManager manages locality-aware peer clustering
type LocalityManager struct {
	mu sync.RWMutex

	host      host.Host
	nodeID    peer.ID
	myRegion  string
	myCluster string
	config    LocalityConfig
	peers     map[peer.ID]*PeerLocality
	clusters  map[string]*LocalityCluster

	// Quality monitor for RTT data
	qualityMonitor *PeerQualityMonitor

	// Callbacks
	onClusterChange func(cluster string, event ClusterEvent)

	ctx    context.Context
	cancel context.CancelFunc
}

// LocalityConfig configures the locality manager
type LocalityConfig struct {
	// MyRegion is this node's region (auto-detected if empty)
	MyRegion string
	// MyCluster is this node's cluster/datacenter (auto-detected if empty)
	MyCluster string
	// LocalRTTThreshold is max RTT to consider a peer "local" (e.g., 30ms)
	LocalRTTThreshold time.Duration
	// RegionalRTTThreshold is max RTT for same region (e.g., 100ms)
	RegionalRTTThreshold time.Duration
	// ProbeInterval is how often to probe peer RTT
	ProbeInterval time.Duration
	// LocalPeerRatio is the target ratio of local peers in mesh (e.g., 0.8 = 80%)
	LocalPeerRatio float64
	// MinRemotePeers is minimum remote peers for partition tolerance
	MinRemotePeers int
}

// DefaultLocalityConfig returns sensible defaults
func DefaultLocalityConfig() LocalityConfig {
	return LocalityConfig{
		LocalRTTThreshold:    30 * time.Millisecond,
		RegionalRTTThreshold: 100 * time.Millisecond,
		ProbeInterval:        1 * time.Minute,
		LocalPeerRatio:       0.8,
		MinRemotePeers:       2,
	}
}

// ClusterEvent represents a cluster change event
type ClusterEvent struct {
	Type      ClusterEventType
	PeerID    peer.ID
	OldRegion string
	NewRegion string
	Timestamp time.Time
}

// ClusterEventType identifies the type of cluster event
type ClusterEventType int

const (
	EventPeerJoinedCluster ClusterEventType = iota
	EventPeerLeftCluster
	EventGatewayChanged
)

// NewLocalityManager creates a new locality manager
func NewLocalityManager(h host.Host, config LocalityConfig) *LocalityManager {
	ctx, cancel := context.WithCancel(context.Background())

	lm := &LocalityManager{
		host:      h,
		nodeID:    h.ID(),
		myRegion:  config.MyRegion,
		myCluster: config.MyCluster,
		config:    config,
		peers:     make(map[peer.ID]*PeerLocality),
		clusters:  make(map[string]*LocalityCluster),
		ctx:       ctx,
		cancel:    cancel,
	}

	// Auto-detect region if not specified
	if lm.myRegion == "" {
		lm.myRegion = "unknown"
	}

	return lm
}

// SetQualityMonitor sets the peer quality monitor for RTT data
func (lm *LocalityManager) SetQualityMonitor(qm *PeerQualityMonitor) {
	lm.mu.Lock()
	lm.qualityMonitor = qm
	lm.mu.Unlock()
}

// OnClusterChange registers a callback for cluster changes
func (lm *LocalityManager) OnClusterChange(fn func(cluster string, event ClusterEvent)) {
	lm.mu.Lock()
	lm.onClusterChange = fn
	lm.mu.Unlock()
}

// Start starts the locality management loops
func (lm *LocalityManager) Start() {
	go lm.probeLoop()
	go lm.clusterLoop()
}

// Stop stops the locality manager
func (lm *LocalityManager) Stop() {
	lm.cancel()
}

// GetMyRegion returns this node's region
func (lm *LocalityManager) GetMyRegion() string {
	lm.mu.RLock()
	defer lm.mu.RUnlock()
	return lm.myRegion
}

// SetMyRegion sets this node's region
func (lm *LocalityManager) SetMyRegion(region string) {
	lm.mu.Lock()
	lm.myRegion = region
	lm.mu.Unlock()
}

// GetLocality returns the locality information for a peer
func (lm *LocalityManager) GetLocality(id peer.ID) *PeerLocality {
	lm.mu.RLock()
	defer lm.mu.RUnlock()
	return lm.peers[id]
}

// RegisterPeer registers a peer with its locality information
func (lm *LocalityManager) RegisterPeer(id peer.ID, locality *PeerLocality) {
	lm.mu.Lock()
	defer lm.mu.Unlock()

	if locality == nil {
		locality = &PeerLocality{
			PeerID: id,
			Region: "unknown",
		}
	}

	lm.peers[id] = locality
	lm.updateCluster(locality.Region)
}

// UnregisterPeer removes a peer from locality tracking
func (lm *LocalityManager) UnregisterPeer(id peer.ID) {
	lm.mu.Lock()
	defer lm.mu.Unlock()

	locality, exists := lm.peers[id]
	if !exists {
		return
	}

	delete(lm.peers, id)
	lm.updateCluster(locality.Region)
}

// UpdatePeerRTT updates the RTT measurement for a peer
func (lm *LocalityManager) UpdatePeerRTT(id peer.ID, rtt time.Duration) {
	lm.mu.Lock()
	defer lm.mu.Unlock()

	locality, exists := lm.peers[id]
	if !exists {
		locality = &PeerLocality{
			PeerID: id,
			Region: lm.classifyRegion(rtt),
		}
		lm.peers[id] = locality
	}

	// Exponential moving average for RTT
	alpha := 0.3
	if locality.RTTSamples == 0 {
		locality.RTT = rtt
	} else {
		locality.RTT = time.Duration(float64(locality.RTT)*(1-alpha) + float64(rtt)*alpha)
	}

	locality.RTTSamples++
	locality.LastProbe = time.Now()

	// Re-classify region based on new RTT
	newRegion := lm.classifyRegion(locality.RTT)
	if newRegion != locality.Region {
		oldRegion := locality.Region
		locality.Region = newRegion
		lm.updateCluster(oldRegion)
		lm.updateCluster(newRegion)
	}
}

// classifyRegion classifies a peer's region based on RTT
func (lm *LocalityManager) classifyRegion(rtt time.Duration) string {
	if rtt <= lm.config.LocalRTTThreshold {
		return lm.myRegion // Same region
	} else if rtt <= lm.config.RegionalRTTThreshold {
		return "regional" // Nearby region
	}
	return "remote" // Far region
}

// GetLocalPeers returns peers in the same region
func (lm *LocalityManager) GetLocalPeers() []peer.ID {
	lm.mu.RLock()
	defer lm.mu.RUnlock()

	var result []peer.ID
	for id, loc := range lm.peers {
		if loc.Region == lm.myRegion {
			result = append(result, id)
		}
	}
	return result
}

// GetRegionalPeers returns peers in nearby regions
func (lm *LocalityManager) GetRegionalPeers() []peer.ID {
	lm.mu.RLock()
	defer lm.mu.RUnlock()

	var result []peer.ID
	for id, loc := range lm.peers {
		if loc.Region == "regional" {
			result = append(result, id)
		}
	}
	return result
}

// GetRemotePeers returns peers in far regions
func (lm *LocalityManager) GetRemotePeers() []peer.ID {
	lm.mu.RLock()
	defer lm.mu.RUnlock()

	var result []peer.ID
	for id, loc := range lm.peers {
		if loc.Region != lm.myRegion && loc.Region != "regional" {
			result = append(result, id)
		}
	}
	return result
}

// SelectPeersForMesh selects peers for mesh with locality preference
func (lm *LocalityManager) SelectPeersForMesh(meshSize int) []peer.ID {
	lm.mu.RLock()
	defer lm.mu.RUnlock()

	localTarget := int(float64(meshSize) * lm.config.LocalPeerRatio)
	remoteTarget := meshSize - localTarget
	if remoteTarget < lm.config.MinRemotePeers {
		remoteTarget = lm.config.MinRemotePeers
		localTarget = meshSize - remoteTarget
	}

	// Get local peers sorted by RTT
	local := lm.getSortedPeersByRTT(lm.myRegion)
	regional := lm.getSortedPeersByRTT("regional")
	remote := lm.getRemotePeersSorted()

	var result []peer.ID

	// Add local peers
	for i := 0; i < len(local) && len(result) < localTarget; i++ {
		result = append(result, local[i])
	}

	// Add regional peers if not enough local
	for i := 0; i < len(regional) && len(result) < localTarget; i++ {
		result = append(result, regional[i])
	}

	// Add remote peers
	for i := 0; i < len(remote) && len(result) < meshSize; i++ {
		result = append(result, remote[i])
	}

	return result
}

// getSortedPeersByRTT returns peers in a region sorted by RTT
func (lm *LocalityManager) getSortedPeersByRTT(region string) []peer.ID {
	type peerRTT struct {
		id  peer.ID
		rtt time.Duration
	}

	var peers []peerRTT
	for id, loc := range lm.peers {
		if loc.Region == region {
			peers = append(peers, peerRTT{id: id, rtt: loc.RTT})
		}
	}

	sort.Slice(peers, func(i, j int) bool {
		return peers[i].rtt < peers[j].rtt
	})

	result := make([]peer.ID, len(peers))
	for i, p := range peers {
		result[i] = p.id
	}
	return result
}

// getRemotePeersSorted returns remote peers sorted by RTT
func (lm *LocalityManager) getRemotePeersSorted() []peer.ID {
	type peerRTT struct {
		id  peer.ID
		rtt time.Duration
	}

	var peers []peerRTT
	for id, loc := range lm.peers {
		if loc.Region != lm.myRegion && loc.Region != "regional" {
			peers = append(peers, peerRTT{id: id, rtt: loc.RTT})
		}
	}

	sort.Slice(peers, func(i, j int) bool {
		return peers[i].rtt < peers[j].rtt
	})

	result := make([]peer.ID, len(peers))
	for i, p := range peers {
		result[i] = p.id
	}
	return result
}

// GetCluster returns information about a cluster
func (lm *LocalityManager) GetCluster(region string) *LocalityCluster {
	lm.mu.RLock()
	defer lm.mu.RUnlock()
	return lm.clusters[region]
}

// GetAllClusters returns all known clusters
func (lm *LocalityManager) GetAllClusters() map[string]*LocalityCluster {
	lm.mu.RLock()
	defer lm.mu.RUnlock()

	result := make(map[string]*LocalityCluster, len(lm.clusters))
	for k, v := range lm.clusters {
		clusterCopy := *v
		result[k] = &clusterCopy
	}
	return result
}

// GetGatewayPeer returns the best peer to reach a remote region
func (lm *LocalityManager) GetGatewayPeer(region string) peer.ID {
	lm.mu.RLock()
	defer lm.mu.RUnlock()

	cluster, exists := lm.clusters[region]
	if !exists {
		return ""
	}
	return cluster.GatewayPeer
}

// updateCluster updates cluster information for a region
func (lm *LocalityManager) updateCluster(region string) {
	if region == "" {
		return
	}

	var totalRTT time.Duration
	var count int
	var bestPeer peer.ID
	bestRTT := time.Hour // Start with a large value

	for id, loc := range lm.peers {
		if loc.Region == region {
			totalRTT += loc.RTT
			count++

			if loc.RTT < bestRTT {
				bestRTT = loc.RTT
				bestPeer = id
			}
		}
	}

	if count == 0 {
		delete(lm.clusters, region)
		return
	}

	cluster, exists := lm.clusters[region]
	if !exists {
		cluster = &LocalityCluster{Region: region}
		lm.clusters[region] = cluster
	}

	oldGateway := cluster.GatewayPeer
	cluster.PeerCount = count
	cluster.AverageRTT = totalRTT / time.Duration(count)
	cluster.GatewayPeer = bestPeer

	// Notify if gateway changed
	if oldGateway != bestPeer && lm.onClusterChange != nil {
		lm.onClusterChange(region, ClusterEvent{
			Type:      EventGatewayChanged,
			PeerID:    bestPeer,
			NewRegion: region,
			Timestamp: time.Now(),
		})
	}
}

// probeLoop periodically probes peer RTT
func (lm *LocalityManager) probeLoop() {
	ticker := time.NewTicker(lm.config.ProbeInterval)
	defer ticker.Stop()

	for {
		select {
		case <-lm.ctx.Done():
			return
		case <-ticker.C:
			lm.probePeers()
		}
	}
}

// probePeers probes RTT for all connected peers
func (lm *LocalityManager) probePeers() {
	lm.mu.RLock()
	peers := make([]peer.ID, 0, len(lm.peers))
	for id := range lm.peers {
		peers = append(peers, id)
	}
	lm.mu.RUnlock()

	// Get RTT from quality monitor if available
	lm.mu.RLock()
	qm := lm.qualityMonitor
	lm.mu.RUnlock()

	if qm == nil {
		return
	}

	for _, id := range peers {
		quality := qm.GetQuality(id)
		if quality != nil && quality.RTT > 0 {
			lm.UpdatePeerRTT(id, quality.RTT)
		}
	}
}

// clusterLoop periodically updates cluster information
func (lm *LocalityManager) clusterLoop() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-lm.ctx.Done():
			return
		case <-ticker.C:
			lm.refreshClusters()
		}
	}
}

// refreshClusters refreshes all cluster information
func (lm *LocalityManager) refreshClusters() {
	lm.mu.Lock()
	defer lm.mu.Unlock()

	// Get all unique regions
	regions := make(map[string]bool)
	for _, loc := range lm.peers {
		regions[loc.Region] = true
	}

	// Update each cluster
	for region := range regions {
		lm.updateCluster(region)
	}

	// Remove empty clusters
	for region, cluster := range lm.clusters {
		if cluster.PeerCount == 0 {
			delete(lm.clusters, region)
		}
	}
}

// Stats returns locality statistics
func (lm *LocalityManager) Stats() LocalityStats {
	lm.mu.RLock()
	defer lm.mu.RUnlock()

	stats := LocalityStats{
		MyRegion:     lm.myRegion,
		TotalPeers:   len(lm.peers),
		ClusterCount: len(lm.clusters),
		Clusters:     make(map[string]int),
	}

	for _, loc := range lm.peers {
		stats.Clusters[loc.Region]++
		if loc.Region == lm.myRegion {
			stats.LocalPeers++
		} else if loc.Region == "regional" {
			stats.RegionalPeers++
		} else {
			stats.RemotePeers++
		}
	}

	return stats
}

// LocalityStats holds locality statistics
type LocalityStats struct {
	MyRegion      string         `json:"my_region"`
	TotalPeers    int            `json:"total_peers"`
	LocalPeers    int            `json:"local_peers"`
	RegionalPeers int            `json:"regional_peers"`
	RemotePeers   int            `json:"remote_peers"`
	ClusterCount  int            `json:"cluster_count"`
	Clusters      map[string]int `json:"clusters"`
}
