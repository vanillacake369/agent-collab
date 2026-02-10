package libp2p

import (
	"testing"
	"time"
)

func TestNetworkMetrics_RecordMessageSent(t *testing.T) {
	m := NewNetworkMetrics()

	m.RecordMessageSent("topic1", "context", 100)
	m.RecordMessageSent("topic1", "context", 200)
	m.RecordMessageSent("topic2", "lock", 50)

	snap := m.Snapshot()

	if snap.TotalMessagesSent != 3 {
		t.Errorf("Expected 3 messages sent, got %d", snap.TotalMessagesSent)
	}
	if snap.BytesSent != 350 {
		t.Errorf("Expected 350 bytes sent, got %d", snap.BytesSent)
	}
	if snap.MessagesByTopic["topic1"] != 2 {
		t.Errorf("Expected 2 messages for topic1, got %d", snap.MessagesByTopic["topic1"])
	}
}

func TestNetworkMetrics_RecordMessageReceived(t *testing.T) {
	m := NewNetworkMetrics()

	m.RecordMessageReceived("topic1", "context", 100)
	m.RecordMessageReceived("topic1", "delta", 200)

	snap := m.Snapshot()

	if snap.TotalMessagesReceived != 2 {
		t.Errorf("Expected 2 messages received, got %d", snap.TotalMessagesReceived)
	}
	if snap.BytesReceived != 300 {
		t.Errorf("Expected 300 bytes received, got %d", snap.BytesReceived)
	}
}

func TestNetworkMetrics_CompressionRatio(t *testing.T) {
	m := NewNetworkMetrics()

	// 1000 bytes original, 400 bytes compressed = 0.4 ratio
	m.RecordCompression(1000, 400)

	snap := m.Snapshot()

	if snap.CompressionRatio < 0.39 || snap.CompressionRatio > 0.41 {
		t.Errorf("Expected compression ratio ~0.4, got %f", snap.CompressionRatio)
	}
}

func TestNetworkMetrics_BatchStats(t *testing.T) {
	m := NewNetworkMetrics()

	m.RecordBatch(10)
	m.RecordBatch(20)
	m.RecordBatch(30)

	snap := m.Snapshot()

	if snap.BatchesSent != 3 {
		t.Errorf("Expected 3 batches sent, got %d", snap.BatchesSent)
	}
	if snap.AvgMessagesPerBatch != 20 {
		t.Errorf("Expected avg 20 messages per batch, got %f", snap.AvgMessagesPerBatch)
	}
}

func TestNetworkMetrics_Latency(t *testing.T) {
	m := NewNetworkMetrics()

	// Add some latencies
	latencies := []time.Duration{
		10 * time.Millisecond,
		20 * time.Millisecond,
		30 * time.Millisecond,
		40 * time.Millisecond,
		50 * time.Millisecond,
		60 * time.Millisecond,
		70 * time.Millisecond,
		80 * time.Millisecond,
		90 * time.Millisecond,
		100 * time.Millisecond,
	}

	for _, l := range latencies {
		m.RecordLatency(l)
	}

	snap := m.Snapshot()

	// P50 should be around 50ms (5th element in sorted 10 elements)
	if snap.LatencyP50 < 40*time.Millisecond || snap.LatencyP50 > 60*time.Millisecond {
		t.Errorf("P50 latency should be ~50ms, got %v", snap.LatencyP50)
	}

	// P99 should be around 100ms (last element)
	if snap.LatencyP99 < 90*time.Millisecond {
		t.Errorf("P99 latency should be ~100ms, got %v", snap.LatencyP99)
	}
}

func TestNetworkMetrics_PeerStats(t *testing.T) {
	m := NewNetworkMetrics()

	m.RecordPeerConnected()
	m.RecordPeerConnected()
	m.RecordPeerConnected()
	m.RecordPeerDisconnected()

	snap := m.Snapshot()

	if snap.PeersConnected != 2 {
		t.Errorf("Expected 2 peers connected, got %d", snap.PeersConnected)
	}
	if snap.TotalDisconnects != 1 {
		t.Errorf("Expected 1 disconnect, got %d", snap.TotalDisconnects)
	}
}

func TestNetworkMetrics_Errors(t *testing.T) {
	m := NewNetworkMetrics()

	m.RecordError("publish_failed")
	m.RecordError("publish_failed")
	m.RecordError("decompress_failed")

	snap := m.Snapshot()

	if snap.Errors["publish_failed"] != 2 {
		t.Errorf("Expected 2 publish_failed errors, got %d", snap.Errors["publish_failed"])
	}
	if snap.Errors["decompress_failed"] != 1 {
		t.Errorf("Expected 1 decompress_failed error, got %d", snap.Errors["decompress_failed"])
	}
}

func TestNetworkMetrics_Reset(t *testing.T) {
	m := NewNetworkMetrics()

	m.RecordMessageSent("topic1", "context", 100)
	m.RecordError("test_error")

	m.Reset()

	snap := m.Snapshot()

	if snap.TotalMessagesSent != 0 {
		t.Errorf("Expected 0 messages after reset, got %d", snap.TotalMessagesSent)
	}
	if len(snap.Errors) != 0 {
		t.Errorf("Expected 0 errors after reset, got %d", len(snap.Errors))
	}
}

func TestNetworkMetrics_Uptime(t *testing.T) {
	m := NewNetworkMetrics()

	time.Sleep(50 * time.Millisecond)

	snap := m.Snapshot()

	if snap.Uptime < 50*time.Millisecond {
		t.Errorf("Uptime should be at least 50ms, got %v", snap.Uptime)
	}
}
