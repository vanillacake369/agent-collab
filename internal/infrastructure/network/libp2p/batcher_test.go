package libp2p

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"
)

func TestMessageBatcher_BasicBatching(t *testing.T) {
	ctx := context.Background()
	var mu sync.Mutex
	var published [][]byte

	// Publisher receives raw data (before compression in real scenario)
	publisher := func(_ context.Context, topic string, data []byte) error {
		mu.Lock()
		published = append(published, data)
		mu.Unlock()
		return nil
	}

	config := BatchConfig{
		MaxSize:  3,
		MaxDelay: 100 * time.Millisecond,
	}

	batcher := NewMessageBatcher(config, publisher)
	batcher.Start(ctx)
	defer batcher.Stop()

	// Add messages
	for i := range 3 {
		msg, _ := json.Marshal(map[string]int{"id": i})
		if err := batcher.Add(ctx, "test-topic", msg); err != nil {
			t.Fatalf("Failed to add message: %v", err)
		}
	}

	// Should have flushed after 3 messages
	time.Sleep(50 * time.Millisecond)

	mu.Lock()
	if len(published) != 1 {
		t.Errorf("Expected 1 batch, got %d", len(published))
	}

	if len(published) > 0 {
		var batch BatchedMessage
		// Publisher receives uncompressed batch data
		if err := json.Unmarshal(published[0], &batch); err != nil {
			t.Fatalf("Failed to unmarshal batch: %v", err)
		}
		if batch.Count != 3 {
			t.Errorf("Expected batch of 3, got %d", batch.Count)
		}
	}
	mu.Unlock()
}

func TestMessageBatcher_TimeBasedFlush(t *testing.T) {
	ctx := context.Background()
	var mu sync.Mutex
	var published [][]byte

	publisher := func(_ context.Context, topic string, data []byte) error {
		mu.Lock()
		published = append(published, data)
		mu.Unlock()
		return nil
	}

	config := BatchConfig{
		MaxSize:  100, // High max size to ensure time-based flush
		MaxDelay: 50 * time.Millisecond,
	}

	batcher := NewMessageBatcher(config, publisher)
	batcher.Start(ctx)
	defer batcher.Stop()

	// Add single message
	msg, _ := json.Marshal(map[string]string{"hello": "world"})
	if err := batcher.Add(ctx, "test-topic", msg); err != nil {
		t.Fatalf("Failed to add message: %v", err)
	}

	// Should not be flushed immediately
	mu.Lock()
	if len(published) != 0 {
		t.Errorf("Should not flush immediately, got %d batches", len(published))
	}
	mu.Unlock()

	// Wait for time-based flush
	time.Sleep(100 * time.Millisecond)

	mu.Lock()
	if len(published) != 1 {
		t.Errorf("Expected 1 batch after timeout, got %d", len(published))
	}
	mu.Unlock()
}

func TestMessageBatcher_MultipleTopics(t *testing.T) {
	ctx := context.Background()
	var mu sync.Mutex
	topicBatches := make(map[string]int)

	publisher := func(_ context.Context, topic string, data []byte) error {
		mu.Lock()
		topicBatches[topic]++
		mu.Unlock()
		return nil
	}

	config := BatchConfig{
		MaxSize:  2,
		MaxDelay: 100 * time.Millisecond,
	}

	batcher := NewMessageBatcher(config, publisher)
	batcher.Start(ctx)
	defer batcher.Stop()

	// Add messages to different topics
	for i := range 4 {
		msg, _ := json.Marshal(map[string]int{"id": i})
		topic := "topic-1"
		if i >= 2 {
			topic = "topic-2"
		}
		if err := batcher.Add(ctx, topic, msg); err != nil {
			t.Fatalf("Failed to add message: %v", err)
		}
	}

	time.Sleep(50 * time.Millisecond)

	mu.Lock()
	if topicBatches["topic-1"] != 1 {
		t.Errorf("Expected 1 batch for topic-1, got %d", topicBatches["topic-1"])
	}
	if topicBatches["topic-2"] != 1 {
		t.Errorf("Expected 1 batch for topic-2, got %d", topicBatches["topic-2"])
	}
	mu.Unlock()
}

func TestUnbatchMessage(t *testing.T) {
	// Test unbatching a batch message
	batch := BatchedMessage{
		Type:  "batch",
		Count: 2,
		Messages: []json.RawMessage{
			json.RawMessage(`{"id":1}`),
			json.RawMessage(`{"id":2}`),
		},
	}
	data, _ := json.Marshal(batch)

	messages, err := UnbatchMessage(data)
	if err != nil {
		t.Fatalf("UnbatchMessage failed: %v", err)
	}
	if len(messages) != 2 {
		t.Errorf("Expected 2 messages, got %d", len(messages))
	}

	// Test non-batch message
	nonBatch := []byte(`{"type":"context","data":"test"}`)
	messages, err = UnbatchMessage(nonBatch)
	if err != nil {
		t.Fatalf("UnbatchMessage failed for non-batch: %v", err)
	}
	if len(messages) != 1 {
		t.Errorf("Expected 1 message for non-batch, got %d", len(messages))
	}
}

func TestIsBatchMessage(t *testing.T) {
	tests := []struct {
		name     string
		data     []byte
		expected bool
	}{
		{"batch message", []byte(`{"type":"batch","count":1}`), true},
		{"context message", []byte(`{"type":"context"}`), false},
		{"lock message", []byte(`{"type":"lock_intent"}`), false},
		{"invalid json", []byte(`not json`), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsBatchMessage(tt.data)
			if result != tt.expected {
				t.Errorf("IsBatchMessage(%s) = %v, want %v", tt.data, result, tt.expected)
			}
		})
	}
}

func TestMessageBatcher_Stats(t *testing.T) {
	ctx := context.Background()
	publisher := func(_ context.Context, topic string, data []byte) error {
		return nil
	}

	config := BatchConfig{
		MaxSize:  100,
		MaxDelay: 1 * time.Hour, // Long delay to prevent auto-flush
	}

	batcher := NewMessageBatcher(config, publisher)
	batcher.Start(ctx)
	defer batcher.Stop()

	// Add some messages
	for range 5 {
		msg := []byte(`{"test": "data"}`)
		_ = batcher.Add(ctx, "topic-1", msg)
	}
	for range 3 {
		msg := []byte(`{"test": "data"}`)
		_ = batcher.Add(ctx, "topic-2", msg)
	}

	stats := batcher.Stats()
	if stats.PendingTopics != 2 {
		t.Errorf("Expected 2 pending topics, got %d", stats.PendingTopics)
	}
	if stats.PendingMessages != 8 {
		t.Errorf("Expected 8 pending messages, got %d", stats.PendingMessages)
	}
}
