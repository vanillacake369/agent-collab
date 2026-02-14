package application

import (
	"context"
	"encoding/json"
	"fmt"

	"agent-collab/src/domain/ctxsync"
	"agent-collab/src/domain/lock"
	"agent-collab/src/infrastructure/storage/vector"
)

// setupMessageHandlers는 메시지 핸들러를 설정합니다.
func (a *App) setupMessageHandlers() {
	// 락 서비스 브로드캐스트 설정
	a.lockService.SetBroadcastFn(func(msg any) error {
		data, err := json.Marshal(msg)
		if err != nil {
			return err
		}
		topicName := "/agent-collab/" + a.config.ProjectName + "/lock"
		return a.node.Publish(a.ctx, topicName, data)
	})

	// 동기화 관리자 브로드캐스트 설정
	a.syncManager.SetBroadcastFn(func(delta *ctxsync.Delta) error {
		data, err := json.Marshal(delta)
		if err != nil {
			return err
		}
		topicName := "/agent-collab/" + a.config.ProjectName + "/context"
		return a.node.Publish(a.ctx, topicName, data)
	})

	// 충돌 핸들러 설정
	conflictLog := a.logger.Component("conflict")
	a.lockService.SetConflictHandler(func(conflict *lock.LockConflict) error {
		conflictLog.Warn("lock conflict detected",
			"requested_by", conflict.RequestedLock.HolderName,
			"conflicting_with", conflict.ConflictingLock.HolderName,
			"overlap_type", conflict.OverlapType)
		return nil
	})

	a.syncManager.SetConflictHandler(func(conflict *ctxsync.Conflict) error {
		conflictLog.Warn("concurrent modification conflict", "file_path", conflict.FilePath)
		return nil
	})
}

// LockMessageBase is a base type for determining message type.
type LockMessageBase struct {
	Type string `json:"type"`
}

// IntentMessageWrapper matches the format from lock.IntentMessage.
type IntentMessageWrapper struct {
	Type   string           `json:"type"`
	Intent *lock.LockIntent `json:"intent"`
}

// AcquireMessageWrapper matches the format from lock.AcquireMessage.
type AcquireMessageWrapper struct {
	Type string             `json:"type"`
	Lock *lock.SemanticLock `json:"lock"`
}

// ReleaseMessageWrapper matches the format from lock.ReleaseMessage.
type ReleaseMessageWrapper struct {
	Type   string `json:"type"`
	LockID string `json:"lock_id"`
}

// processLockMessages processes incoming lock messages from P2P network.
func (a *App) processLockMessages(ctx context.Context) {
	topicName := "/agent-collab/" + a.config.ProjectName + "/lock"
	processor := NewMessageProcessor(
		a.node,
		topicName,
		func(_ context.Context, data []byte) {
			a.handleSingleLockMessage(data)
		},
		a.logger.Component("lock-processor"),
	)
	processor.Run(ctx)
}

// handleSingleLockMessage processes a single lock message
func (a *App) handleSingleLockMessage(data []byte) {
	log := a.logger.Component("lock-handler")

	var baseMsg LockMessageBase
	if UnmarshalMessage(data, &baseMsg, "lock message type", log) != UnmarshalOK {
		return
	}

	switch baseMsg.Type {
	case "lock_intent":
		var msg IntentMessageWrapper
		if UnmarshalMessagePtr(data, &msg, func(m *IntentMessageWrapper) *lock.LockIntent { return m.Intent }, "lock intent", log) != UnmarshalOK {
			return
		}
		if err := a.lockService.HandleRemoteLockIntent(msg.Intent); err != nil {
			log.Error("failed to handle lock intent", "error", err)
		}

	case "lock_acquired":
		var msg AcquireMessageWrapper
		if UnmarshalMessagePtr(data, &msg, func(m *AcquireMessageWrapper) *lock.SemanticLock { return m.Lock }, "acquired lock", log) != UnmarshalOK {
			return
		}
		if err := a.lockService.HandleRemoteLockAcquired(msg.Lock); err != nil {
			log.Error("failed to handle lock acquired", "error", err)
		}

	case "lock_released":
		var msg ReleaseMessageWrapper
		if UnmarshalMessage(data, &msg, "lock release", log) != UnmarshalOK {
			return
		}
		if err := a.lockService.HandleRemoteLockReleased(msg.LockID); err != nil {
			log.Error("failed to handle lock released", "error", err)
		}

	default:
		log.Warn("unknown lock message type", "type", baseMsg.Type)
	}
}

// ContextMessageBase is used to determine the message type.
type ContextMessageBase struct {
	Type string `json:"type"`
}

// processContextMessages processes incoming context sync messages from P2P network.
func (a *App) processContextMessages(ctx context.Context) {
	topicName := "/agent-collab/" + a.config.ProjectName + "/context"
	processor := NewMessageProcessor(
		a.node,
		topicName,
		a.handleSingleContextMessage,
		a.logger.Component("context-processor"),
	)
	processor.Run(ctx)
}

// handleSingleContextMessage processes a single context message
func (a *App) handleSingleContextMessage(ctx context.Context, data []byte) {
	log := a.logger.Component("context-handler")

	var baseMsg ContextMessageBase
	if UnmarshalMessage(data, &baseMsg, "context message type", log) != UnmarshalOK {
		return
	}

	switch baseMsg.Type {
	case "shared_context":
		var ctxMsg ContextMessage
		if UnmarshalMessage(data, &ctxMsg, "shared context", log) != UnmarshalOK {
			return
		}
		a.handleSharedContext(ctx, &ctxMsg)

	default:
		// Assume it's a Delta message (for backward compatibility)
		var delta ctxsync.Delta
		if UnmarshalMessage(data, &delta, "delta", log) != UnmarshalOK {
			return
		}

		if err := a.syncManager.ReceiveDelta(&delta); err != nil {
			log.Error("failed to handle delta", "error", err)
		}

		// Also store in VectorDB if it's a file change with content
		a.storeDeltaInVectorDB(ctx, &delta)
	}
}

// handleSharedContext processes shared context from a peer and stores it in VectorDB.
func (a *App) handleSharedContext(ctx context.Context, msg *ContextMessage) {
	log := a.logger.Component("context-handler")

	if a.vectorStore == nil {
		return
	}

	// Use provided embedding or generate new one
	embedding := msg.Embedding
	if len(embedding) == 0 && a.embedService != nil && msg.Content != "" {
		var err error
		embedding, err = a.embedService.Embed(ctx, msg.Content)
		if err != nil {
			log.Error("failed to generate embedding for shared context", "error", err)
			return
		}
	}

	// Create and store document
	doc := &vector.Document{
		Content:   msg.Content,
		Embedding: embedding,
		FilePath:  msg.FilePath,
		Metadata:  msg.Metadata,
	}
	if doc.Metadata == nil {
		doc.Metadata = make(map[string]any)
	}
	doc.Metadata["source_id"] = msg.SourceID
	doc.Metadata["type"] = "shared_context"

	if err := a.vectorStore.Insert(doc); err != nil {
		log.Error("failed to store shared context in VectorDB", "error", err)
		return
	}

	// Async flush
	go func() {
		if err := a.vectorStore.Flush(); err != nil {
			log.Error("failed to flush VectorDB", "error", err)
		}
	}()

	log.Info("received shared context", "source_id", msg.SourceID, "file_path", msg.FilePath)
}

// storeDeltaInVectorDB stores delta content in VectorDB for search.
func (a *App) storeDeltaInVectorDB(ctx context.Context, delta *ctxsync.Delta) {
	log := a.logger.Component("vector-store")

	if a.vectorStore == nil || a.embedService == nil {
		return
	}

	// Only process file changes
	if delta.Type != ctxsync.DeltaFileChange || delta.Payload.FilePath == "" {
		return
	}

	// Build content description from delta info
	content := fmt.Sprintf("File change: %s from %s",
		delta.Payload.FilePath, delta.SourceName)

	// Add symbol info if available
	if delta.Payload.FileDiff != nil {
		for _, d := range delta.Payload.FileDiff.Diffs {
			if d.Symbol != nil {
				content += fmt.Sprintf("\n%s %s: %s",
					d.Type, d.Symbol.Type, d.Symbol.Name)
			}
		}
	}

	// Generate embedding
	embedding, err := a.embedService.Embed(ctx, content)
	if err != nil {
		log.Error("failed to generate embedding for delta", "error", err, "file_path", delta.Payload.FilePath)
		return
	}

	// Create and store document
	doc := &vector.Document{
		Content:   content,
		Embedding: embedding,
		FilePath:  delta.Payload.FilePath,
		Metadata: map[string]any{
			"source_id":   delta.SourceID,
			"source_name": delta.SourceName,
			"delta_id":    delta.ID,
			"timestamp":   delta.Timestamp,
			"type":        "delta_sync",
		},
	}

	if err := a.vectorStore.Insert(doc); err != nil {
		log.Error("failed to store delta in VectorDB", "error", err, "file_path", delta.Payload.FilePath)
		return
	}

	// Async flush to avoid blocking
	go func() {
		if err := a.vectorStore.Flush(); err != nil {
			log.Error("failed to flush VectorDB", "error", err)
		}
	}()
}

// ContextMessage is a message for sharing context via P2P.
type ContextMessage struct {
	Type      string         `json:"type"`
	FilePath  string         `json:"file_path"`
	Content   string         `json:"content"`
	Embedding []float32      `json:"embedding,omitempty"`
	Metadata  map[string]any `json:"metadata,omitempty"`
	SourceID  string         `json:"source_id"`
}

// BroadcastContext broadcasts shared context to all peers.
func (a *App) BroadcastContext(filePath, content string, embedding []float32, metadata map[string]any) error {
	if a.node == nil {
		return fmt.Errorf("node not initialized")
	}

	msg := ContextMessage{
		Type:      "shared_context",
		FilePath:  filePath,
		Content:   content,
		Embedding: embedding,
		Metadata:  metadata,
		SourceID:  a.node.ID().String(),
	}

	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	topicName := "/agent-collab/" + a.config.ProjectName + "/context"
	return a.node.Publish(a.ctx, topicName, data)
}
