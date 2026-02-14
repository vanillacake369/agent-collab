package application

import (
	"context"
	"encoding/json"

	"agent-collab/src/infrastructure/network/libp2p"
)

// MessageHandler processes a single message payload.
type MessageHandler func(ctx context.Context, data []byte)

// MessageProcessor handles the common message processing loop for P2P topics.
type MessageProcessor struct {
	node      *libp2p.Node
	topicName string
	handler   MessageHandler
	logger    Logger
}

// Logger is the minimal interface needed for message processing.
type Logger interface {
	Warn(msg string, keysAndValues ...any)
	Error(msg string, keysAndValues ...any)
}

// UnmarshalResult indicates whether unmarshal succeeded and processing should continue.
type UnmarshalResult int

const (
	// UnmarshalOK means unmarshal succeeded.
	UnmarshalOK UnmarshalResult = iota
	// UnmarshalError means unmarshal failed with error.
	UnmarshalError
	// UnmarshalNilValue means unmarshal succeeded but result was nil.
	UnmarshalNilValue
)

// UnmarshalMessage unmarshals JSON data and logs errors consistently.
// Returns UnmarshalOK if successful, UnmarshalError if unmarshal failed,
// UnmarshalNilValue if the result is nil (for pointer types).
func UnmarshalMessage[T any](data []byte, target *T, msgType string, logger Logger) UnmarshalResult {
	if err := json.Unmarshal(data, target); err != nil {
		logger.Error("failed to unmarshal "+msgType, "error", err)
		return UnmarshalError
	}
	return UnmarshalOK
}

// UnmarshalMessagePtr unmarshals JSON data and checks for nil pointer fields.
// Use this when the target has a pointer field that must not be nil.
func UnmarshalMessagePtr[T any, P any](data []byte, target *T, getPtr func(*T) *P, msgType string, logger Logger) UnmarshalResult {
	if err := json.Unmarshal(data, target); err != nil {
		logger.Error("failed to unmarshal "+msgType, "error", err)
		return UnmarshalError
	}
	if getPtr(target) == nil {
		logger.Warn("received " + msgType + " with nil value")
		return UnmarshalNilValue
	}
	return UnmarshalOK
}

// NewMessageProcessor creates a new message processor.
func NewMessageProcessor(node *libp2p.Node, topicName string, handler MessageHandler, logger Logger) *MessageProcessor {
	return &MessageProcessor{
		node:      node,
		topicName: topicName,
		handler:   handler,
		logger:    logger,
	}
}

// Run starts the message processing loop. Blocks until context is cancelled.
func (p *MessageProcessor) Run(ctx context.Context) {
	sub := p.node.GetSubscription(p.topicName)
	if sub == nil {
		p.logger.Warn("no subscription for topic", "topic", p.topicName)
		return
	}

	for {
		msg, err := sub.Next(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return // Context cancelled, graceful shutdown
			}
			p.logger.Error("failed to receive message", "error", err, "topic", p.topicName)
			continue
		}

		// Skip messages from ourselves
		if msg.ReceivedFrom == p.node.ID() {
			continue
		}

		// Decompress message if needed
		data, err := libp2p.DecompressMessage(msg.Data)
		if err != nil {
			// Try raw data for backward compatibility
			data = msg.Data
		}

		// Handle batch or single message
		messages, err := libp2p.UnbatchMessage(data)
		if err != nil {
			p.logger.Error("failed to unbatch message", "error", err, "topic", p.topicName)
			continue
		}

		for _, msgData := range messages {
			p.handler(ctx, msgData)
		}
	}
}
