package libp2p

import (
	"context"
	"encoding/json"
	"sync"

	pubsub "github.com/libp2p/go-libp2p-pubsub"

	"agent-collab/src/domain/event"
	"agent-collab/src/domain/interest"
)

// EventBridge connects the domain event router to the P2P network.
// It handles:
// - Publishing domain events to P2P topics
// - Receiving P2P messages and routing to domain
// - Interest synchronization across nodes
type EventBridge struct {
	node   *Node
	router *event.Router

	// Optional interest manager for sync
	interestMgr *interest.Manager

	// Subscription for events topic
	eventSub *pubsub.Subscription

	// Lifecycle management
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	mu        sync.RWMutex
	running   bool
	connected bool
}

// NewEventBridge creates a new event bridge.
func NewEventBridge(node *Node, router *event.Router) *EventBridge {
	bridge := &EventBridge{
		node:      node,
		router:    router,
		connected: true,
	}

	// Set broadcast function on router
	router.SetBroadcastFn(bridge.broadcast)

	return bridge
}

// SetInterestManager sets the interest manager for sync.
func (b *EventBridge) SetInterestManager(mgr *interest.Manager) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.interestMgr = mgr

	// Register change listener for interest sync
	if mgr != nil {
		mgr.OnChange(b.onInterestChange)
	}
}

// Start starts the event bridge.
func (b *EventBridge) Start(ctx context.Context) error {
	b.mu.Lock()
	if b.running {
		b.mu.Unlock()
		return nil
	}

	b.ctx, b.cancel = context.WithCancel(ctx)
	b.running = true
	b.mu.Unlock()

	// Subscribe to events topic
	sub, err := b.node.Subscribe(TopicEvents)
	if err != nil {
		b.mu.Lock()
		b.running = false
		b.mu.Unlock()
		return err
	}
	b.eventSub = sub

	// Start message handler
	b.wg.Add(1)
	go b.handleMessages()

	return nil
}

// Stop stops the event bridge.
func (b *EventBridge) Stop() {
	b.mu.Lock()
	if !b.running {
		b.mu.Unlock()
		return
	}
	b.running = false
	b.mu.Unlock()

	if b.cancel != nil {
		b.cancel()
	}

	if b.eventSub != nil {
		b.eventSub.Cancel()
	}

	b.wg.Wait()
}

// IsConnected returns whether the bridge is connected.
func (b *EventBridge) IsConnected() bool {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.connected
}

// IsRunning returns whether the bridge is running.
func (b *EventBridge) IsRunning() bool {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.running
}

// broadcast publishes an event to the P2P network.
func (b *EventBridge) broadcast(topic string, data []byte) error {
	b.mu.RLock()
	running := b.running
	b.mu.RUnlock()

	if !running {
		// Not running, silently skip
		return nil
	}

	return b.node.Publish(b.ctx, topic, data)
}

// handleMessages handles incoming messages from the P2P network.
func (b *EventBridge) handleMessages() {
	defer b.wg.Done()

	for {
		msg, err := b.eventSub.Next(b.ctx)
		if err != nil {
			// Context cancelled or subscription closed
			return
		}

		// Skip messages from self
		if msg.ReceivedFrom == b.node.ID() {
			continue
		}

		// Process message
		b.HandleIncomingMessage(b.ctx, msg.Data)
	}
}

// HandleIncomingMessage processes an incoming P2P message.
func (b *EventBridge) HandleIncomingMessage(ctx context.Context, data []byte) {
	// Decompress message
	decompressed, err := DecompressMessage(data)
	if err != nil {
		// Try as uncompressed
		decompressed = data
	}

	// Decrypt if needed
	decompressed, err = b.node.DecryptMessage(TopicEvents, decompressed)
	if err != nil {
		return
	}

	// Route to domain
	if err := b.router.HandleRemoteEvent(ctx, decompressed); err != nil {
		// Log error but continue
		_ = err
	}
}

// onInterestChange handles interest changes for sync.
func (b *EventBridge) onInterestChange(change interest.InterestChange) {
	b.mu.RLock()
	running := b.running
	b.mu.RUnlock()

	if !running {
		return
	}

	// Broadcast interest change to cluster
	b.broadcastInterestChange(change)
}

// broadcastInterestChange broadcasts an interest change to the P2P network.
func (b *EventBridge) broadcastInterestChange(change interest.InterestChange) {
	// Create an event for interest change
	evt := event.NewEvent(event.EventTypeAgentJoined, b.node.ID().String(), "system")
	if change.Type == interest.ChangeTypeRemoved {
		evt = event.NewEvent(event.EventTypeAgentLeft, b.node.ID().String(), "system")
	}

	// Encode interest in payload
	payload := map[string]interface{}{
		"change_type": change.Type,
		"interest":    change.Interest,
	}
	payloadData, err := json.Marshal(payload)
	if err != nil {
		return
	}
	evt.Payload = payloadData

	// Publish
	data, err := json.Marshal(evt)
	if err != nil {
		return
	}

	b.broadcast(TopicEvents, data)
}

// SyncInterests synchronizes interests with connected peers.
func (b *EventBridge) SyncInterests(ctx context.Context) error {
	b.mu.RLock()
	mgr := b.interestMgr
	b.mu.RUnlock()

	if mgr == nil {
		return nil
	}

	// Get local interests snapshot
	snapshot := mgr.Snapshot()

	// Create sync event
	evt := event.NewEvent(event.EventTypeContextShared, b.node.ID().String(), "system")
	payload := map[string]interface{}{
		"sync_type":  "interests",
		"interests":  snapshot,
		"node_id":    b.node.ID().String(),
	}
	payloadData, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	evt.Payload = payloadData

	// Broadcast
	data, err := json.Marshal(evt)
	if err != nil {
		return err
	}

	return b.broadcast(TopicEvents, data)
}
