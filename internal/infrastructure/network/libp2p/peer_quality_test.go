package libp2p

import (
	"testing"
	"time"

	"github.com/libp2p/go-libp2p/core/peer"
)

func TestPeerQuality_CalculateScore(t *testing.T) {
	config := DefaultPeerQualityConfig()
	config.MinSamples = 1 // Allow scoring with 1 sample for testing

	// Create a mock monitor to access calculateScore
	m := &PeerQualityMonitor{
		config: config,
	}

	tests := []struct {
		name       string
		rtt        time.Duration
		variance   time.Duration
		packetLoss float64
		samples    int
		minScore   float64
		maxScore   float64
	}{
		{
			name:       "perfect connection",
			rtt:        10 * time.Millisecond,
			variance:   1 * time.Millisecond,
			packetLoss: 0,
			samples:    5,
			minScore:   0.9,
			maxScore:   1.0,
		},
		{
			name:       "good connection",
			rtt:        50 * time.Millisecond,
			variance:   10 * time.Millisecond,
			packetLoss: 0.01,
			samples:    5,
			minScore:   0.7,
			maxScore:   1.0,
		},
		{
			name:       "medium connection",
			rtt:        200 * time.Millisecond,
			variance:   50 * time.Millisecond,
			packetLoss: 0.05,
			samples:    5,
			minScore:   0.5,
			maxScore:   0.9,
		},
		{
			name:       "poor connection",
			rtt:        400 * time.Millisecond,
			variance:   100 * time.Millisecond,
			packetLoss: 0.2,
			samples:    5,
			minScore:   0.3,
			maxScore:   0.7,
		},
		{
			name:       "terrible connection",
			rtt:        600 * time.Millisecond,
			variance:   200 * time.Millisecond,
			packetLoss: 0.5,
			samples:    5,
			minScore:   0.1,
			maxScore:   0.5,
		},
		{
			name:       "not enough samples",
			rtt:        10 * time.Millisecond,
			variance:   0,
			packetLoss: 0,
			samples:    0,
			minScore:   0.49,
			maxScore:   0.51, // Should be ~0.5 for unknown
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			q := &PeerQuality{
				RTT:         tt.rtt,
				RTTVariance: tt.variance,
				PacketLoss:  tt.packetLoss,
				SampleCount: tt.samples,
			}

			score := m.calculateScore(q)

			if score < tt.minScore || score > tt.maxScore {
				t.Errorf("Score %.2f not in expected range [%.2f, %.2f]",
					score, tt.minScore, tt.maxScore)
			}
		})
	}
}

func TestPeerQuality_GetHighLowQualityPeers(t *testing.T) {
	config := DefaultPeerQualityConfig()
	config.MinSamples = 1

	m := &PeerQualityMonitor{
		config: config,
		peers:  make(map[peer.ID]*PeerQuality),
	}

	// Add peers with different scores
	highQualityPeer := peer.ID("high-quality")
	m.peers[highQualityPeer] = &PeerQuality{
		PeerID:      highQualityPeer,
		Score:       0.9,
		SampleCount: 5,
	}

	mediumQualityPeer := peer.ID("medium-quality")
	m.peers[mediumQualityPeer] = &PeerQuality{
		PeerID:      mediumQualityPeer,
		Score:       0.5,
		SampleCount: 5,
	}

	lowQualityPeer := peer.ID("low-quality")
	m.peers[lowQualityPeer] = &PeerQuality{
		PeerID:      lowQualityPeer,
		Score:       0.1,
		SampleCount: 5,
	}

	// Test GetHighQualityPeers
	highPeers := m.GetHighQualityPeers()
	if len(highPeers) != 1 {
		t.Errorf("Expected 1 high quality peer, got %d", len(highPeers))
	}
	if len(highPeers) > 0 && highPeers[0] != highQualityPeer {
		t.Errorf("Expected high quality peer to be %s", highQualityPeer)
	}

	// Test GetLowQualityPeers
	lowPeers := m.GetLowQualityPeers()
	if len(lowPeers) != 1 {
		t.Errorf("Expected 1 low quality peer, got %d", len(lowPeers))
	}
	if len(lowPeers) > 0 && lowPeers[0] != lowQualityPeer {
		t.Errorf("Expected low quality peer to be %s", lowQualityPeer)
	}
}

func TestPeerQuality_SelectBestPeers(t *testing.T) {
	config := DefaultPeerQualityConfig()
	config.MinSamples = 1

	m := &PeerQualityMonitor{
		config: config,
		peers:  make(map[peer.ID]*PeerQuality),
	}

	// Add peers with different scores
	peers := []struct {
		id    peer.ID
		score float64
	}{
		{peer.ID("peer1"), 0.9},
		{peer.ID("peer2"), 0.7},
		{peer.ID("peer3"), 0.5},
		{peer.ID("peer4"), 0.3},
		{peer.ID("peer5"), 0.1},
	}

	for _, p := range peers {
		m.peers[p.id] = &PeerQuality{
			PeerID:      p.id,
			Score:       p.score,
			SampleCount: 5,
		}
	}

	// Select top 3
	best := m.SelectBestPeers(3)
	if len(best) != 3 {
		t.Errorf("Expected 3 best peers, got %d", len(best))
	}

	// Verify order
	expectedOrder := []peer.ID{peer.ID("peer1"), peer.ID("peer2"), peer.ID("peer3")}
	for i, id := range best {
		if id != expectedOrder[i] {
			t.Errorf("Position %d: expected %s, got %s", i, expectedOrder[i], id)
		}
	}
}

func TestPeerQuality_Stats(t *testing.T) {
	config := DefaultPeerQualityConfig()
	config.MinSamples = 1

	m := &PeerQualityMonitor{
		config: config,
		peers:  make(map[peer.ID]*PeerQuality),
	}

	// Add peers
	m.peers[peer.ID("high1")] = &PeerQuality{
		Score:       0.9,
		RTT:         20 * time.Millisecond,
		SampleCount: 5,
	}
	m.peers[peer.ID("high2")] = &PeerQuality{
		Score:       0.8,
		RTT:         40 * time.Millisecond,
		SampleCount: 5,
	}
	m.peers[peer.ID("med1")] = &PeerQuality{
		Score:       0.5,
		RTT:         100 * time.Millisecond,
		SampleCount: 5,
	}
	m.peers[peer.ID("low1")] = &PeerQuality{
		Score:       0.1,
		RTT:         300 * time.Millisecond,
		SampleCount: 5,
	}

	stats := m.Stats()

	if stats.TotalPeers != 4 {
		t.Errorf("Expected 4 total peers, got %d", stats.TotalPeers)
	}
	if stats.HighQuality != 2 {
		t.Errorf("Expected 2 high quality, got %d", stats.HighQuality)
	}
	if stats.MediumQuality != 1 {
		t.Errorf("Expected 1 medium quality, got %d", stats.MediumQuality)
	}
	if stats.LowQuality != 1 {
		t.Errorf("Expected 1 low quality, got %d", stats.LowQuality)
	}

	expectedAvgRTT := (20 + 40 + 100 + 300) / 4 * time.Millisecond
	if stats.AverageRTT != expectedAvgRTT {
		t.Errorf("Expected avg RTT %v, got %v", expectedAvgRTT, stats.AverageRTT)
	}
}

func TestPeerQuality_GetScore(t *testing.T) {
	config := DefaultPeerQualityConfig()

	m := &PeerQualityMonitor{
		config: config,
		peers:  make(map[peer.ID]*PeerQuality),
	}

	knownPeer := peer.ID("known")
	m.peers[knownPeer] = &PeerQuality{
		Score:       0.75,
		SampleCount: 5,
	}

	// Known peer should return its score
	score := m.GetScore(knownPeer)
	if score != 0.75 {
		t.Errorf("Expected score 0.75, got %f", score)
	}

	// Unknown peer should return 0.5
	unknownScore := m.GetScore(peer.ID("unknown"))
	if unknownScore != 0.5 {
		t.Errorf("Unknown peer should return 0.5, got %f", unknownScore)
	}
}

func TestPeerQuality_GetQuality(t *testing.T) {
	config := DefaultPeerQualityConfig()

	m := &PeerQualityMonitor{
		config: config,
		peers:  make(map[peer.ID]*PeerQuality),
	}

	peerID := peer.ID("test-peer")
	m.peers[peerID] = &PeerQuality{
		PeerID:      peerID,
		RTT:         50 * time.Millisecond,
		RTTVariance: 10 * time.Millisecond,
		PacketLoss:  0.01,
		Score:       0.8,
		SampleCount: 10,
	}

	q := m.GetQuality(peerID)
	if q == nil {
		t.Fatal("Expected quality to be returned")
	}

	if q.RTT != 50*time.Millisecond {
		t.Errorf("RTT mismatch")
	}
	if q.Score != 0.8 {
		t.Errorf("Score mismatch")
	}

	// Unknown peer
	unknown := m.GetQuality(peer.ID("unknown"))
	if unknown != nil {
		t.Error("Unknown peer should return nil")
	}
}
