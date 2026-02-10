package libp2p

import (
	"context"
	"sync"
	"time"

	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
)

// PeerQuality represents the quality metrics of a peer
type PeerQuality struct {
	PeerID      peer.ID       `json:"peer_id"`
	RTT         time.Duration `json:"rtt"`
	RTTVariance time.Duration `json:"rtt_variance"`
	PacketLoss  float64       `json:"packet_loss"`
	Bandwidth   float64       `json:"bandwidth_mbps"`
	Score       float64       `json:"score"`
	LastUpdate  time.Time     `json:"last_update"`
	SampleCount int           `json:"sample_count"`
}

// PeerQualityMonitor monitors peer connection quality
type PeerQualityMonitor struct {
	mu       sync.RWMutex
	host     host.Host
	peers    map[peer.ID]*PeerQuality
	config   PeerQualityConfig
	ctx      context.Context
	cancel   context.CancelFunc
	handlers []QualityChangeHandler
}

// PeerQualityConfig configures the quality monitor
type PeerQualityConfig struct {
	// PingInterval is how often to ping peers
	PingInterval time.Duration
	// PingTimeout is the timeout for ping
	PingTimeout time.Duration
	// MinSamples is the minimum samples before scoring
	MinSamples int
	// LowScoreThreshold below which peers are considered low quality
	LowScoreThreshold float64
	// HighScoreThreshold above which peers are considered high quality
	HighScoreThreshold float64
	// MaxRTT is the maximum acceptable RTT (above this, score is 0)
	MaxRTT time.Duration
	// TargetRTT is the ideal RTT (below this, RTT score is 1)
	TargetRTT time.Duration
}

// QualityChangeHandler is called when peer quality changes significantly
type QualityChangeHandler func(peerID peer.ID, oldScore, newScore float64)

// DefaultPeerQualityConfig returns the default configuration
func DefaultPeerQualityConfig() PeerQualityConfig {
	return PeerQualityConfig{
		PingInterval:       30 * time.Second,
		PingTimeout:        5 * time.Second,
		MinSamples:         3,
		LowScoreThreshold:  0.3,
		HighScoreThreshold: 0.7,
		MaxRTT:             500 * time.Millisecond,
		TargetRTT:          50 * time.Millisecond,
	}
}

// NewPeerQualityMonitor creates a new peer quality monitor
func NewPeerQualityMonitor(h host.Host, config PeerQualityConfig) *PeerQualityMonitor {
	ctx, cancel := context.WithCancel(context.Background())

	pqm := &PeerQualityMonitor{
		host:   h,
		peers:  make(map[peer.ID]*PeerQuality),
		config: config,
		ctx:    ctx,
		cancel: cancel,
	}

	return pqm
}

// Start starts the quality monitoring
func (m *PeerQualityMonitor) Start() {
	go m.monitorLoop()
}

// Stop stops the quality monitoring
func (m *PeerQualityMonitor) Stop() {
	m.cancel()
}

// OnQualityChange registers a handler for quality changes
func (m *PeerQualityMonitor) OnQualityChange(handler QualityChangeHandler) {
	m.mu.Lock()
	m.handlers = append(m.handlers, handler)
	m.mu.Unlock()
}

// GetQuality returns the quality metrics for a peer
func (m *PeerQualityMonitor) GetQuality(id peer.ID) *PeerQuality {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.peers[id]
}

// GetAllQualities returns quality metrics for all peers
func (m *PeerQualityMonitor) GetAllQualities() map[peer.ID]*PeerQuality {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make(map[peer.ID]*PeerQuality, len(m.peers))
	for id, q := range m.peers {
		qCopy := *q
		result[id] = &qCopy
	}
	return result
}

// GetHighQualityPeers returns peers with score above threshold
func (m *PeerQualityMonitor) GetHighQualityPeers() []peer.ID {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []peer.ID
	for id, q := range m.peers {
		if q.Score >= m.config.HighScoreThreshold {
			result = append(result, id)
		}
	}
	return result
}

// GetLowQualityPeers returns peers with score below threshold
func (m *PeerQualityMonitor) GetLowQualityPeers() []peer.ID {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []peer.ID
	for id, q := range m.peers {
		if q.Score < m.config.LowScoreThreshold && q.SampleCount >= m.config.MinSamples {
			result = append(result, id)
		}
	}
	return result
}

// GetScore returns the quality score for a peer (0-1)
func (m *PeerQualityMonitor) GetScore(id peer.ID) float64 {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if q, ok := m.peers[id]; ok {
		return q.Score
	}
	return 0.5 // Unknown peers get neutral score
}

// monitorLoop continuously monitors peer quality
func (m *PeerQualityMonitor) monitorLoop() {
	ticker := time.NewTicker(m.config.PingInterval)
	defer ticker.Stop()

	for {
		select {
		case <-m.ctx.Done():
			return
		case <-ticker.C:
			m.measureAllPeers()
		}
	}
}

// measureAllPeers measures quality for all connected peers
func (m *PeerQualityMonitor) measureAllPeers() {
	peers := m.host.Network().Peers()

	for _, id := range peers {
		go m.measurePeer(id)
	}

	// Clean up disconnected peers
	m.cleanupDisconnected(peers)
}

// measurePeer measures a single peer's quality
func (m *PeerQualityMonitor) measurePeer(id peer.ID) {
	ctx, cancel := context.WithTimeout(m.ctx, m.config.PingTimeout)
	defer cancel()

	start := time.Now()

	// Use host's Peerstore latency as a baseline
	baseLatency := m.host.Peerstore().LatencyEWMA(id)

	// Try to ping the peer
	rtt := baseLatency
	packetLoss := 0.0

	// Simple connectivity check
	conns := m.host.Network().ConnsToPeer(id)
	if len(conns) == 0 {
		packetLoss = 1.0
	} else {
		// Use the latency from peerstore or estimate
		if rtt == 0 {
			rtt = time.Since(start)
		}
	}

	select {
	case <-ctx.Done():
		packetLoss = 1.0
	default:
	}

	m.updateQuality(id, rtt, packetLoss)
}

// updateQuality updates a peer's quality metrics
func (m *PeerQualityMonitor) updateQuality(id peer.ID, rtt time.Duration, packetLoss float64) {
	m.mu.Lock()

	q, exists := m.peers[id]
	if !exists {
		q = &PeerQuality{
			PeerID: id,
		}
		m.peers[id] = q
	}

	// Exponential moving average for RTT
	alpha := 0.3
	if q.SampleCount == 0 {
		q.RTT = rtt
		q.RTTVariance = 0
	} else {
		diff := rtt - q.RTT
		if diff < 0 {
			diff = -diff
		}
		q.RTTVariance = time.Duration(float64(q.RTTVariance)*(1-alpha) + float64(diff)*alpha)
		q.RTT = time.Duration(float64(q.RTT)*(1-alpha) + float64(rtt)*alpha)
	}

	// EMA for packet loss
	if q.SampleCount == 0 {
		q.PacketLoss = packetLoss
	} else {
		q.PacketLoss = q.PacketLoss*(1-alpha) + packetLoss*alpha
	}

	q.SampleCount++
	q.LastUpdate = time.Now()

	// Calculate score
	oldScore := q.Score
	q.Score = m.calculateScore(q)

	handlers := m.handlers
	m.mu.Unlock()

	// Notify handlers if score changed significantly
	if q.SampleCount >= m.config.MinSamples {
		scoreDiff := q.Score - oldScore
		if scoreDiff < 0 {
			scoreDiff = -scoreDiff
		}
		if scoreDiff > 0.1 {
			for _, h := range handlers {
				h(id, oldScore, q.Score)
			}
		}
	}
}

// calculateScore calculates a quality score (0-1) from metrics
func (m *PeerQualityMonitor) calculateScore(q *PeerQuality) float64 {
	if q.SampleCount < m.config.MinSamples {
		return 0.5 // Not enough data
	}

	// RTT score (1 = good, 0 = bad)
	rttScore := 1.0
	if q.RTT > m.config.TargetRTT {
		rttRange := m.config.MaxRTT - m.config.TargetRTT
		rttExcess := q.RTT - m.config.TargetRTT
		rttScore = 1.0 - float64(rttExcess)/float64(rttRange)
		if rttScore < 0 {
			rttScore = 0
		}
	}

	// Jitter score (variance)
	jitterScore := 1.0
	if q.RTTVariance > 0 {
		jitterRatio := float64(q.RTTVariance) / float64(q.RTT+1)
		jitterScore = 1.0 - jitterRatio
		if jitterScore < 0 {
			jitterScore = 0
		}
	}

	// Packet loss score
	lossScore := 1.0 - q.PacketLoss

	// Weighted average
	// RTT: 40%, Jitter: 20%, Loss: 40%
	score := rttScore*0.4 + jitterScore*0.2 + lossScore*0.4

	// Clamp to [0, 1]
	if score < 0 {
		score = 0
	}
	if score > 1 {
		score = 1
	}

	return score
}

// cleanupDisconnected removes metrics for disconnected peers
func (m *PeerQualityMonitor) cleanupDisconnected(connected []peer.ID) {
	connectedSet := make(map[peer.ID]bool, len(connected))
	for _, id := range connected {
		connectedSet[id] = true
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	for id := range m.peers {
		if !connectedSet[id] {
			delete(m.peers, id)
		}
	}
}

// PeerQualityStats provides aggregate statistics
type PeerQualityStats struct {
	TotalPeers    int           `json:"total_peers"`
	HighQuality   int           `json:"high_quality"`
	MediumQuality int           `json:"medium_quality"`
	LowQuality    int           `json:"low_quality"`
	AverageRTT    time.Duration `json:"average_rtt"`
	AverageScore  float64       `json:"average_score"`
}

// Stats returns aggregate quality statistics
func (m *PeerQualityMonitor) Stats() PeerQualityStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := PeerQualityStats{}
	var totalRTT time.Duration
	var totalScore float64

	for _, q := range m.peers {
		if q.SampleCount < m.config.MinSamples {
			continue
		}

		stats.TotalPeers++
		totalRTT += q.RTT
		totalScore += q.Score

		if q.Score >= m.config.HighScoreThreshold {
			stats.HighQuality++
		} else if q.Score >= m.config.LowScoreThreshold {
			stats.MediumQuality++
		} else {
			stats.LowQuality++
		}
	}

	if stats.TotalPeers > 0 {
		stats.AverageRTT = totalRTT / time.Duration(stats.TotalPeers)
		stats.AverageScore = totalScore / float64(stats.TotalPeers)
	}

	return stats
}

// SelectBestPeers returns the top N peers by quality
func (m *PeerQualityMonitor) SelectBestPeers(n int) []peer.ID {
	m.mu.RLock()
	defer m.mu.RUnlock()

	type peerScore struct {
		id    peer.ID
		score float64
	}

	var peers []peerScore
	for id, q := range m.peers {
		if q.SampleCount >= m.config.MinSamples {
			peers = append(peers, peerScore{id: id, score: q.Score})
		}
	}

	// Sort by score (descending)
	for i := 0; i < len(peers)-1; i++ {
		for j := i + 1; j < len(peers); j++ {
			if peers[i].score < peers[j].score {
				peers[i], peers[j] = peers[j], peers[i]
			}
		}
	}

	// Take top N
	result := make([]peer.ID, 0, n)
	for i := 0; i < len(peers) && i < n; i++ {
		result = append(result, peers[i].id)
	}

	return result
}
