package libp2p

import (
	"context"
	"encoding/json"
	"sync"
	"time"
)

// BatchConfig contains configuration for message batching
type BatchConfig struct {
	MaxSize      int           // Maximum messages per batch (default: 100)
	MaxDelay     time.Duration // Maximum wait before flush (default: 50ms)
	MaxBatchSize int           // Maximum batch size in bytes (default: 64KB)
}

// DefaultBatchConfig returns the default batching configuration
func DefaultBatchConfig() BatchConfig {
	return BatchConfig{
		MaxSize:      100,
		MaxDelay:     50 * time.Millisecond,
		MaxBatchSize: 64 * 1024, // 64KB
	}
}

// MessageBatcher batches messages for efficient transmission
type MessageBatcher struct {
	config    BatchConfig
	publisher func(ctx context.Context, topic string, data []byte) error

	mu       sync.Mutex
	batches  map[string]*topicBatch
	flushCh  chan string
	shutdown chan struct{}
	wg       sync.WaitGroup
}

// topicBatch holds pending messages for a single topic
type topicBatch struct {
	messages  []json.RawMessage
	size      int
	timer     *time.Timer
	lastFlush time.Time
}

// BatchedMessage represents a batch of messages
type BatchedMessage struct {
	Type     string            `json:"type"`
	Count    int               `json:"count"`
	Messages []json.RawMessage `json:"messages"`
}

// NewMessageBatcher creates a new message batcher
func NewMessageBatcher(config BatchConfig, publisher func(ctx context.Context, topic string, data []byte) error) *MessageBatcher {
	if config.MaxSize == 0 {
		config.MaxSize = DefaultBatchConfig().MaxSize
	}
	if config.MaxDelay == 0 {
		config.MaxDelay = DefaultBatchConfig().MaxDelay
	}
	if config.MaxBatchSize == 0 {
		config.MaxBatchSize = DefaultBatchConfig().MaxBatchSize
	}

	mb := &MessageBatcher{
		config:    config,
		publisher: publisher,
		batches:   make(map[string]*topicBatch),
		flushCh:   make(chan string, 100),
		shutdown:  make(chan struct{}),
	}

	return mb
}

// Start starts the batcher background processing
func (mb *MessageBatcher) Start(ctx context.Context) {
	mb.wg.Add(1)
	go mb.processFlushRequests(ctx)
}

// Stop stops the batcher and flushes remaining messages
func (mb *MessageBatcher) Stop() {
	close(mb.shutdown)
	mb.wg.Wait()

	// Flush remaining messages
	mb.mu.Lock()
	topics := make([]string, 0, len(mb.batches))
	for topic := range mb.batches {
		topics = append(topics, topic)
	}
	mb.mu.Unlock()

	ctx := context.Background()
	for _, topic := range topics {
		mb.flush(ctx, topic)
	}
}

// Add adds a message to the batch for a topic
func (mb *MessageBatcher) Add(ctx context.Context, topic string, data []byte) error {
	mb.mu.Lock()
	defer mb.mu.Unlock()

	batch, exists := mb.batches[topic]
	if !exists {
		batch = &topicBatch{
			messages:  make([]json.RawMessage, 0, mb.config.MaxSize),
			lastFlush: time.Now(),
		}
		mb.batches[topic] = batch
	}

	// Add message to batch
	batch.messages = append(batch.messages, json.RawMessage(data))
	batch.size += len(data)

	// Start timer for first message in batch
	if len(batch.messages) == 1 {
		batch.timer = time.AfterFunc(mb.config.MaxDelay, func() {
			select {
			case mb.flushCh <- topic:
			default:
			}
		})
	}

	// Flush if batch is full or too large
	if len(batch.messages) >= mb.config.MaxSize || batch.size >= mb.config.MaxBatchSize {
		return mb.flushLocked(ctx, topic)
	}

	return nil
}

// Flush immediately flushes all pending messages for a topic
func (mb *MessageBatcher) Flush(ctx context.Context, topic string) error {
	mb.mu.Lock()
	defer mb.mu.Unlock()
	return mb.flushLocked(ctx, topic)
}

// flushLocked flushes the batch (caller must hold lock)
func (mb *MessageBatcher) flushLocked(ctx context.Context, topic string) error {
	batch, exists := mb.batches[topic]
	if !exists || len(batch.messages) == 0 {
		return nil
	}

	// Cancel pending timer
	if batch.timer != nil {
		batch.timer.Stop()
		batch.timer = nil
	}

	// Create batched message
	batchedMsg := BatchedMessage{
		Type:     "batch",
		Count:    len(batch.messages),
		Messages: batch.messages,
	}

	data, err := json.Marshal(batchedMsg)
	if err != nil {
		return err
	}

	// Clear batch
	batch.messages = batch.messages[:0]
	batch.size = 0
	batch.lastFlush = time.Now()

	// Publish (outside lock to avoid deadlock)
	mb.mu.Unlock()
	err = mb.publisher(ctx, topic, data)
	mb.mu.Lock()

	return err
}

// flush is the unlocked version for external calls
func (mb *MessageBatcher) flush(ctx context.Context, topic string) error {
	mb.mu.Lock()
	defer mb.mu.Unlock()
	return mb.flushLocked(ctx, topic)
}

// processFlushRequests handles timer-based flush requests
func (mb *MessageBatcher) processFlushRequests(ctx context.Context) {
	defer mb.wg.Done()

	for {
		select {
		case topic := <-mb.flushCh:
			_ = mb.flush(ctx, topic)
		case <-mb.shutdown:
			return
		case <-ctx.Done():
			return
		}
	}
}

// FlushAll flushes all topics
func (mb *MessageBatcher) FlushAll(ctx context.Context) {
	mb.mu.Lock()
	topics := make([]string, 0, len(mb.batches))
	for topic := range mb.batches {
		topics = append(topics, topic)
	}
	mb.mu.Unlock()

	for _, topic := range topics {
		_ = mb.flush(ctx, topic)
	}
}

// Stats returns batching statistics
type BatcherStats struct {
	PendingTopics   int
	PendingMessages int
	PendingBytes    int
}

func (mb *MessageBatcher) Stats() BatcherStats {
	mb.mu.Lock()
	defer mb.mu.Unlock()

	stats := BatcherStats{
		PendingTopics: len(mb.batches),
	}
	for _, batch := range mb.batches {
		stats.PendingMessages += len(batch.messages)
		stats.PendingBytes += batch.size
	}
	return stats
}

// UnbatchMessage extracts individual messages from a batched message
func UnbatchMessage(data []byte) ([]json.RawMessage, error) {
	var batch BatchedMessage
	if err := json.Unmarshal(data, &batch); err != nil {
		return nil, err
	}

	if batch.Type != "batch" {
		// Not a batch message, return as single item
		return []json.RawMessage{json.RawMessage(data)}, nil
	}

	return batch.Messages, nil
}

// IsBatchMessage checks if the data is a batched message
func IsBatchMessage(data []byte) bool {
	// Quick check for batch message type
	var header struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(data, &header); err != nil {
		return false
	}
	return header.Type == "batch"
}
