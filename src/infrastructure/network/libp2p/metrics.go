package libp2p

import (
	"sync"
	"time"
)

// NetworkMetrics collects P2P network metrics
type NetworkMetrics struct {
	mu sync.RWMutex

	// Message counts by topic and type
	messagesSent     map[string]map[string]int64
	messagesReceived map[string]map[string]int64

	// Bytes transferred
	bytesSent     int64
	bytesReceived int64

	// Compression stats
	bytesBeforeCompression int64
	bytesAfterCompression  int64

	// Batch stats
	batchesSent      int64
	messagesPerBatch int64
	totalBatchedMsgs int64

	// Latency tracking (for propagation time)
	latencies    []time.Duration
	maxLatencies int

	// Peer stats
	peersConnected    int
	peersDisconnected int64

	// Error counts
	errors map[string]int64

	// Start time for uptime calculation
	startTime time.Time
}

// NewNetworkMetrics creates a new metrics collector
func NewNetworkMetrics() *NetworkMetrics {
	return &NetworkMetrics{
		messagesSent:     make(map[string]map[string]int64),
		messagesReceived: make(map[string]map[string]int64),
		latencies:        make([]time.Duration, 0, 1000),
		maxLatencies:     1000,
		errors:           make(map[string]int64),
		startTime:        time.Now(),
	}
}

// RecordMessageSent records a sent message
func (m *NetworkMetrics) RecordMessageSent(topic, msgType string, size int) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.messagesSent[topic] == nil {
		m.messagesSent[topic] = make(map[string]int64)
	}
	m.messagesSent[topic][msgType]++
	m.bytesSent += int64(size)
}

// RecordMessageReceived records a received message
func (m *NetworkMetrics) RecordMessageReceived(topic, msgType string, size int) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.messagesReceived[topic] == nil {
		m.messagesReceived[topic] = make(map[string]int64)
	}
	m.messagesReceived[topic][msgType]++
	m.bytesReceived += int64(size)
}

// RecordCompression records compression statistics
func (m *NetworkMetrics) RecordCompression(originalSize, compressedSize int) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.bytesBeforeCompression += int64(originalSize)
	m.bytesAfterCompression += int64(compressedSize)
}

// RecordBatch records batch statistics
func (m *NetworkMetrics) RecordBatch(messageCount int) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.batchesSent++
	m.totalBatchedMsgs += int64(messageCount)
}

// RecordLatency records a message propagation latency
func (m *NetworkMetrics) RecordLatency(d time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if len(m.latencies) >= m.maxLatencies {
		// Remove oldest entry
		m.latencies = m.latencies[1:]
	}
	m.latencies = append(m.latencies, d)
}

// RecordPeerConnected records a new peer connection
func (m *NetworkMetrics) RecordPeerConnected() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.peersConnected++
}

// RecordPeerDisconnected records a peer disconnection
func (m *NetworkMetrics) RecordPeerDisconnected() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.peersConnected--
	m.peersDisconnected++
}

// RecordError records an error
func (m *NetworkMetrics) RecordError(errorType string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.errors[errorType]++
}

// MetricsSnapshot represents a point-in-time snapshot of metrics
type MetricsSnapshot struct {
	// Uptime
	Uptime time.Duration `json:"uptime"`

	// Message counts
	TotalMessagesSent     int64            `json:"total_messages_sent"`
	TotalMessagesReceived int64            `json:"total_messages_received"`
	MessagesByTopic       map[string]int64 `json:"messages_by_topic"`

	// Bytes transferred
	BytesSent     int64 `json:"bytes_sent"`
	BytesReceived int64 `json:"bytes_received"`

	// Compression efficiency
	CompressionRatio float64 `json:"compression_ratio"`

	// Batch stats
	BatchesSent         int64   `json:"batches_sent"`
	AvgMessagesPerBatch float64 `json:"avg_messages_per_batch"`

	// Latency percentiles
	LatencyP50 time.Duration `json:"latency_p50"`
	LatencyP95 time.Duration `json:"latency_p95"`
	LatencyP99 time.Duration `json:"latency_p99"`

	// Peer stats
	PeersConnected   int   `json:"peers_connected"`
	TotalDisconnects int64 `json:"total_disconnects"`

	// Error counts
	Errors map[string]int64 `json:"errors"`
}

// Snapshot returns a point-in-time snapshot of metrics
func (m *NetworkMetrics) Snapshot() MetricsSnapshot {
	m.mu.RLock()
	defer m.mu.RUnlock()

	snap := MetricsSnapshot{
		Uptime:           time.Since(m.startTime),
		BytesSent:        m.bytesSent,
		BytesReceived:    m.bytesReceived,
		BatchesSent:      m.batchesSent,
		PeersConnected:   m.peersConnected,
		TotalDisconnects: m.peersDisconnected,
		MessagesByTopic:  make(map[string]int64),
		Errors:           make(map[string]int64),
	}

	// Calculate totals
	for topic, types := range m.messagesSent {
		for _, count := range types {
			snap.TotalMessagesSent += count
			snap.MessagesByTopic[topic] += count
		}
	}
	for topic, types := range m.messagesReceived {
		for _, count := range types {
			snap.TotalMessagesReceived += count
			snap.MessagesByTopic[topic] += count
		}
	}

	// Calculate compression ratio
	if m.bytesBeforeCompression > 0 {
		snap.CompressionRatio = float64(m.bytesAfterCompression) / float64(m.bytesBeforeCompression)
	}

	// Calculate average messages per batch
	if m.batchesSent > 0 {
		snap.AvgMessagesPerBatch = float64(m.totalBatchedMsgs) / float64(m.batchesSent)
	}

	// Calculate latency percentiles
	if len(m.latencies) > 0 {
		sorted := make([]time.Duration, len(m.latencies))
		copy(sorted, m.latencies)
		sortDurations(sorted)

		snap.LatencyP50 = percentile(sorted, 50)
		snap.LatencyP95 = percentile(sorted, 95)
		snap.LatencyP99 = percentile(sorted, 99)
	}

	// Copy errors
	for k, v := range m.errors {
		snap.Errors[k] = v
	}

	return snap
}

// Reset resets all metrics
func (m *NetworkMetrics) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.messagesSent = make(map[string]map[string]int64)
	m.messagesReceived = make(map[string]map[string]int64)
	m.bytesSent = 0
	m.bytesReceived = 0
	m.bytesBeforeCompression = 0
	m.bytesAfterCompression = 0
	m.batchesSent = 0
	m.messagesPerBatch = 0
	m.totalBatchedMsgs = 0
	m.latencies = m.latencies[:0]
	m.peersConnected = 0
	m.peersDisconnected = 0
	m.errors = make(map[string]int64)
	m.startTime = time.Now()
}

// Helper functions

func sortDurations(d []time.Duration) {
	// Simple insertion sort for small arrays
	for i := 1; i < len(d); i++ {
		key := d[i]
		j := i - 1
		for j >= 0 && d[j] > key {
			d[j+1] = d[j]
			j--
		}
		d[j+1] = key
	}
}

func percentile(sorted []time.Duration, p int) time.Duration {
	if len(sorted) == 0 {
		return 0
	}
	idx := (len(sorted) - 1) * p / 100
	return sorted[idx]
}
