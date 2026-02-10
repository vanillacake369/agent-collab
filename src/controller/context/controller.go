// Package context provides a controller for Context resources.
package context

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	v1 "agent-collab/src/api/v1"
	"agent-collab/src/controller"
	"agent-collab/src/plugin"
	"agent-collab/src/store"
)

// contextWrapper wraps v1.Context to implement store.Object
type contextWrapper struct {
	*v1.Context
}

func (c *contextWrapper) DeepCopy() store.Object {
	if c == nil || c.Context == nil {
		return nil
	}
	copy := *c.Context
	return &contextWrapper{Context: &copy}
}

// Controller manages Context resources.
type Controller struct {
	*controller.GenericController[*contextWrapper]
	store     store.ResourceStore[*contextWrapper]
	network   plugin.NetworkPlugin
	embedding plugin.EmbeddingPlugin
	vector    plugin.VectorPlugin

	// Local agent ID
	agentID string

	// Topics for context messages
	syncTopic string
	ackTopic  string
}

// NewController creates a new Context controller.
func NewController(
	contextStore store.ResourceStore[*contextWrapper],
	network plugin.NetworkPlugin,
	embedding plugin.EmbeddingPlugin,
	vector plugin.VectorPlugin,
	agentID string,
	projectID string,
) *Controller {
	c := &Controller{
		store:     contextStore,
		network:   network,
		embedding: embedding,
		vector:    vector,
		agentID:   agentID,
		syncTopic: fmt.Sprintf("/agent-collab/%s/context/sync", projectID),
		ackTopic:  fmt.Sprintf("/agent-collab/%s/context/ack", projectID),
	}

	// Create the generic controller with the reconciler
	c.GenericController = controller.NewGenericController(
		"context",
		controller.ReconcilerFunc(c.Reconcile),
		contextStore,
		controller.DefaultOptions("context"),
	)

	return c
}

// Start begins the controller and sets up network subscriptions.
func (c *Controller) Start(ctx context.Context) error {
	// Subscribe to context topics
	if c.network != nil {
		if err := c.subscribeToTopics(ctx); err != nil {
			return fmt.Errorf("failed to subscribe to topics: %w", err)
		}
	}

	// Start the generic controller
	return c.GenericController.Start(ctx)
}

// Reconcile handles the reconciliation of a Context resource.
func (c *Controller) Reconcile(ctx context.Context, req controller.Request) (controller.Result, error) {
	ctxRes, err := c.store.Get(ctx, req.Name)
	if err != nil {
		if err == store.ErrNotFound {
			return controller.Result{}, nil
		}
		return controller.Result{}, err
	}

	switch ctxRes.Status.Phase {
	case v1.ContextPhasePending:
		return c.reconcilePending(ctx, ctxRes)

	case v1.ContextPhaseSyncing:
		return c.reconcileSyncing(ctx, ctxRes)

	case v1.ContextPhaseSynced:
		return c.reconcileSynced(ctx, ctxRes)

	case v1.ContextPhaseEmbedding:
		return c.reconcileEmbedding(ctx, ctxRes)

	case v1.ContextPhaseReady, v1.ContextPhaseFailed:
		// Terminal states
		return controller.Result{}, nil
	}

	return controller.Result{}, nil
}

func (c *Controller) reconcilePending(ctx context.Context, ctxRes *contextWrapper) (controller.Result, error) {
	// Calculate content hash if not set
	if ctxRes.Spec.ContentHash == "" {
		hash := sha256.Sum256([]byte(ctxRes.Spec.Content))
		ctxRes.Spec.ContentHash = hex.EncodeToString(hash[:])
	}

	// Start syncing - broadcast to peers
	if c.network != nil && ctxRes.Spec.SourceAgentID == c.agentID {
		if err := c.broadcastContext(ctx, ctxRes); err != nil {
			return controller.Result{Requeue: true}, err
		}
	}

	// Move to syncing phase
	ctxRes.Status.Phase = v1.ContextPhaseSyncing
	ctxRes.SetCondition(v1.ContextConditionSynced, v1.ConditionFalse, "Syncing", "Context is being synced to peers")

	if err := c.store.Update(ctx, ctxRes); err != nil {
		return controller.Result{}, err
	}

	// Requeue to check sync progress
	return controller.Result{RequeueAfter: time.Second}, nil
}

func (c *Controller) reconcileSyncing(ctx context.Context, ctxRes *contextWrapper) (controller.Result, error) {
	// For now, immediately move to synced
	// In a real implementation, we would wait for acknowledgments
	now := time.Now()
	ctxRes.Status.LastSyncTime = &now
	ctxRes.Status.Phase = v1.ContextPhaseSynced
	ctxRes.SetCondition(v1.ContextConditionSynced, v1.ConditionTrue, "SyncComplete", "Context synced to peers")

	if err := c.store.Update(ctx, ctxRes); err != nil {
		return controller.Result{}, err
	}

	// Continue to embedding
	return controller.Result{Requeue: true}, nil
}

func (c *Controller) reconcileSynced(ctx context.Context, ctxRes *contextWrapper) (controller.Result, error) {
	// Start embedding if we have an embedding plugin
	if c.embedding == nil {
		// No embedding, move to ready
		ctxRes.Status.Phase = v1.ContextPhaseReady
		ctxRes.SetCondition(v1.ContextConditionReady, v1.ConditionTrue, "Ready", "Context is ready (no embedding)")
		return controller.Result{}, c.store.Update(ctx, ctxRes)
	}

	ctxRes.Status.Phase = v1.ContextPhaseEmbedding
	if err := c.store.Update(ctx, ctxRes); err != nil {
		return controller.Result{}, err
	}

	return controller.Result{Requeue: true}, nil
}

func (c *Controller) reconcileEmbedding(ctx context.Context, ctxRes *contextWrapper) (controller.Result, error) {
	if c.embedding == nil {
		ctxRes.Status.Phase = v1.ContextPhaseReady
		return controller.Result{}, c.store.Update(ctx, ctxRes)
	}

	// Generate embedding
	embedding, err := c.embedding.Embed(ctx, ctxRes.Spec.Content)
	if err != nil {
		ctxRes.Status.Phase = v1.ContextPhaseFailed
		ctxRes.Status.Message = fmt.Sprintf("Embedding failed: %v", err)
		ctxRes.SetCondition(v1.ContextConditionEmbedded, v1.ConditionFalse, "EmbeddingFailed", err.Error())
		return controller.Result{}, c.store.Update(ctx, ctxRes)
	}

	// Store in vector DB if available
	if c.vector != nil {
		doc := plugin.VectorDocument{
			ID:        ctxRes.Name,
			Content:   ctxRes.Spec.Content,
			Embedding: embedding,
			Metadata: map[string]any{
				"sourceAgentId": ctxRes.Spec.SourceAgentID,
				"filePath":      ctxRes.Spec.FilePath,
				"type":          string(ctxRes.Spec.Type),
				"contentHash":   ctxRes.Spec.ContentHash,
			},
			Collection: "contexts",
		}
		if err := c.vector.Store(ctx, doc); err != nil {
			// Non-fatal, just log
			fmt.Printf("warning: failed to store in vector DB: %v\n", err)
		}
	}

	// Update status
	now := time.Now()
	ctxRes.Status.Embedding = &v1.EmbeddingInfo{
		Provider:   c.embedding.Provider(),
		Model:      c.embedding.Model(),
		Dimensions: int32(c.embedding.Dimension()),
		EmbeddedAt: now,
		DocumentID: ctxRes.Name,
	}
	ctxRes.Status.Phase = v1.ContextPhaseReady
	ctxRes.SetCondition(v1.ContextConditionEmbedded, v1.ConditionTrue, "EmbeddingComplete", "Embedding generated")
	ctxRes.SetCondition(v1.ContextConditionReady, v1.ConditionTrue, "Ready", "Context is ready")

	return controller.Result{}, c.store.Update(ctx, ctxRes)
}

// Network message types

// ContextSyncMessage represents a context sync message.
type ContextSyncMessage struct {
	ContextName   string            `json:"contextName"`
	SourceAgentID string            `json:"sourceAgentId"`
	Type          v1.ContextType    `json:"type"`
	FilePath      string            `json:"filePath,omitempty"`
	Content       string            `json:"content"`
	ContentHash   string            `json:"contentHash"`
	VectorClock   map[string]uint64 `json:"vectorClock,omitempty"`
	Delta         *v1.ContextDelta  `json:"delta,omitempty"`
	Timestamp     time.Time         `json:"timestamp"`
}

// ContextAckMessage represents a context acknowledgment.
type ContextAckMessage struct {
	ContextName string    `json:"contextName"`
	AgentID     string    `json:"agentId"`
	Timestamp   time.Time `json:"timestamp"`
}

func (c *Controller) subscribeToTopics(ctx context.Context) error {
	// Subscribe to sync topic
	syncCh, err := c.network.Subscribe(ctx, c.syncTopic)
	if err != nil {
		return err
	}
	go c.handleSyncMessages(ctx, syncCh)

	// Subscribe to ack topic
	ackCh, err := c.network.Subscribe(ctx, c.ackTopic)
	if err != nil {
		return err
	}
	go c.handleAckMessages(ctx, ackCh)

	return nil
}

func (c *Controller) handleSyncMessages(ctx context.Context, ch <-chan plugin.Message) {
	for {
		select {
		case <-ctx.Done():
			return
		case msg, ok := <-ch:
			if !ok {
				return
			}
			var syncMsg ContextSyncMessage
			if err := json.Unmarshal(msg.Data, &syncMsg); err != nil {
				continue
			}
			c.processSync(ctx, syncMsg)
		}
	}
}

func (c *Controller) handleAckMessages(ctx context.Context, ch <-chan plugin.Message) {
	for {
		select {
		case <-ctx.Done():
			return
		case msg, ok := <-ch:
			if !ok {
				return
			}
			var ackMsg ContextAckMessage
			if err := json.Unmarshal(msg.Data, &ackMsg); err != nil {
				continue
			}
			c.processAck(ctx, ackMsg)
		}
	}
}

func (c *Controller) processSync(ctx context.Context, syncMsg ContextSyncMessage) {
	// Skip our own messages
	if syncMsg.SourceAgentID == c.agentID {
		return
	}

	// Check if we already have this context
	existing, err := c.store.Get(ctx, syncMsg.ContextName)
	if err == nil {
		// Already have it, check if newer
		if existing.Spec.ContentHash == syncMsg.ContentHash {
			// Same content, send ack
			c.sendAck(ctx, syncMsg.ContextName)
			return
		}
		// Different content, could implement conflict resolution here
	}

	// Create new context from sync message
	newCtx := &contextWrapper{
		Context: v1.NewContext(syncMsg.ContextName, v1.ContextSpec{
			Type:          syncMsg.Type,
			SourceAgentID: syncMsg.SourceAgentID,
			FilePath:      syncMsg.FilePath,
			Content:       syncMsg.Content,
			ContentHash:   syncMsg.ContentHash,
			VectorClock:   syncMsg.VectorClock,
			Delta:         syncMsg.Delta,
		}),
	}

	if err := c.store.Create(ctx, newCtx); err != nil {
		if err == store.ErrAlreadyExists {
			// Update instead
			c.store.Update(ctx, newCtx)
		}
	}

	// Send acknowledgment
	c.sendAck(ctx, syncMsg.ContextName)
}

func (c *Controller) processAck(ctx context.Context, ackMsg ContextAckMessage) {
	// Update the context with acknowledgment
	ctxRes, err := c.store.Get(ctx, ackMsg.ContextName)
	if err != nil {
		return
	}

	// Add to synced list
	ctxRes.Status.SyncedTo = append(ctxRes.Status.SyncedTo, v1.SyncTarget{
		AgentID:      ackMsg.AgentID,
		SyncedAt:     ackMsg.Timestamp,
		Acknowledged: true,
	})

	c.store.Update(ctx, ctxRes)
}

func (c *Controller) broadcastContext(ctx context.Context, ctxRes *contextWrapper) error {
	msg := ContextSyncMessage{
		ContextName:   ctxRes.Name,
		SourceAgentID: ctxRes.Spec.SourceAgentID,
		Type:          ctxRes.Spec.Type,
		FilePath:      ctxRes.Spec.FilePath,
		Content:       ctxRes.Spec.Content,
		ContentHash:   ctxRes.Spec.ContentHash,
		VectorClock:   ctxRes.Spec.VectorClock,
		Delta:         ctxRes.Spec.Delta,
		Timestamp:     time.Now(),
	}
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	return c.network.Publish(ctx, c.syncTopic, data)
}

func (c *Controller) sendAck(ctx context.Context, contextName string) {
	ack := ContextAckMessage{
		ContextName: contextName,
		AgentID:     c.agentID,
		Timestamp:   time.Now(),
	}
	data, _ := json.Marshal(ack)
	c.network.Publish(ctx, c.ackTopic, data)
}
