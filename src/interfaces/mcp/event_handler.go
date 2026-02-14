package mcp

import (
	"context"
	"encoding/json"
	"sync"

	"agent-collab/src/interfaces/daemon"
)

// EventHandler manages daemon events for MCP clients.
type EventHandler struct {
	mu sync.RWMutex

	client       *daemon.Client
	recentEvents []daemon.Event
	maxEvents    int
	warnings     []string

	ctx    context.Context
	cancel context.CancelFunc
}

// NewEventHandler creates a new event handler.
func NewEventHandler(client *daemon.Client) *EventHandler {
	return &EventHandler{
		client:       client,
		recentEvents: make([]daemon.Event, 0),
		maxEvents:    50,
		warnings:     make([]string, 0),
	}
}

// Start starts listening for events.
func (h *EventHandler) Start(ctx context.Context) {
	h.ctx, h.cancel = context.WithCancel(ctx)

	eventCh := h.client.SubscribeEventsWithRetry(h.ctx)

	go func() {
		for {
			select {
			case <-h.ctx.Done():
				return
			case event, ok := <-eventCh:
				if !ok {
					return
				}
				h.handleEvent(event)
			}
		}
	}()
}

// Stop stops listening for events.
func (h *EventHandler) Stop() {
	if h.cancel != nil {
		h.cancel()
	}
	h.client.CloseEvents()
}

func (h *EventHandler) handleEvent(event daemon.Event) {
	h.mu.Lock()
	defer h.mu.Unlock()

	// Store recent event
	h.recentEvents = append(h.recentEvents, event)
	if len(h.recentEvents) > h.maxEvents {
		h.recentEvents = h.recentEvents[1:]
	}

	// Generate warnings for important events
	switch event.Type {
	case daemon.EventLockConflict:
		var data daemon.LockConflictData
		if err := json.Unmarshal(event.Data, &data); err == nil {
			h.warnings = append(h.warnings,
				"⚠️ Lock conflict detected on "+data.FilePath+": held by "+data.HolderID)
		}
	case daemon.EventLockAcquired:
		var data daemon.LockEventData
		if err := json.Unmarshal(event.Data, &data); err == nil {
			h.warnings = append(h.warnings,
				"🔒 Lock acquired on "+data.FilePath+" by "+data.AgentID+": "+data.Intention)
		}
	case daemon.EventAgentJoined:
		var data daemon.AgentEventData
		if err := json.Unmarshal(event.Data, &data); err == nil {
			h.warnings = append(h.warnings,
				"👋 New agent joined: "+data.Name+" ("+data.Provider+")")
		}
	case daemon.EventContextUpdated:
		var data daemon.ContextEventData
		if err := json.Unmarshal(event.Data, &data); err == nil {
			msg := "📄 Context shared"
			if data.FilePath != "" {
				msg += ": " + data.FilePath
			}
			if data.Content != "" {
				// Truncate content for display
				preview := data.Content
				if len(preview) > 100 {
					preview = preview[:100] + "..."
				}
				msg += " - " + preview
			}
			h.warnings = append(h.warnings, msg)
		}
	case daemon.EventPeerConnected:
		var data daemon.PeerEventData
		if err := json.Unmarshal(event.Data, &data); err == nil {
			h.warnings = append(h.warnings,
				"🔗 Peer connected: "+data.PeerID)
		}
	case daemon.EventDaemonShutdown:
		h.warnings = append(h.warnings, "⛔ Daemon is shutting down")
	}
}

// GetRecentEvents returns recent events.
func (h *EventHandler) GetRecentEvents() []daemon.Event {
	h.mu.RLock()
	defer h.mu.RUnlock()

	result := make([]daemon.Event, len(h.recentEvents))
	copy(result, h.recentEvents)
	return result
}

// GetEventsSince returns events since a given timestamp.
func (h *EventHandler) GetEventsSince(eventType daemon.EventType) []daemon.Event {
	h.mu.RLock()
	defer h.mu.RUnlock()

	result := make([]daemon.Event, 0)
	for _, e := range h.recentEvents {
		if e.Type == eventType {
			result = append(result, e)
		}
	}
	return result
}

// PopWarnings returns and clears pending warnings.
func (h *EventHandler) PopWarnings() []string {
	h.mu.Lock()
	defer h.mu.Unlock()

	result := h.warnings
	h.warnings = make([]string, 0)
	return result
}

// HasWarnings returns true if there are pending warnings.
func (h *EventHandler) HasWarnings() bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.warnings) > 0
}

// RegisterEventTools registers event-related MCP tools.
func RegisterEventTools(server *Server, handler *EventHandler) {
	server.RegisterTool(Tool{
		Name:        "get_events",
		Description: "Get recent cluster events (lock acquisitions, context shares, agent joins, etc.)",
		InputSchema: InputSchema{
			Type: "object",
			Properties: map[string]Property{
				"type": {
					Type:        "string",
					Description: "Filter by event type (optional). Values: lock.acquired, lock.released, lock.conflict, context.updated, context.synced, agent.joined, agent.left, peer.connected, peer.disconnected",
				},
				"limit": {
					Type:        "integer",
					Description: "Maximum number of events to return (default 10)",
				},
			},
		},
	}, func(ctx context.Context, args map[string]any) (*ToolCallResult, error) {
		return handleGetEvents(handler, args)
	})

	server.RegisterTool(Tool{
		Name:        "get_warnings",
		Description: "Get pending warnings about cluster events that may affect your work",
		InputSchema: InputSchema{
			Type:       "object",
			Properties: map[string]Property{},
		},
	}, func(ctx context.Context, args map[string]any) (*ToolCallResult, error) {
		return handleGetWarnings(handler)
	})
}

func handleGetEvents(handler *EventHandler, args map[string]any) (*ToolCallResult, error) {
	events := handler.GetRecentEvents()

	// Filter by type if specified
	if eventType, ok := args["type"].(string); ok && eventType != "" {
		filtered := make([]daemon.Event, 0)
		for _, e := range events {
			if string(e.Type) == eventType {
				filtered = append(filtered, e)
			}
		}
		events = filtered
	}

	// Apply limit
	limit := 10
	if l, ok := args["limit"].(float64); ok && l > 0 {
		limit = int(l)
	}
	if len(events) > limit {
		events = events[len(events)-limit:]
	}

	if len(events) == 0 {
		return textResult("No recent events"), nil
	}

	data, _ := json.MarshalIndent(events, "", "  ")
	return textResult(string(data)), nil
}

func handleGetWarnings(handler *EventHandler) (*ToolCallResult, error) {
	warnings := handler.PopWarnings()

	if len(warnings) == 0 {
		return textResult("No pending warnings"), nil
	}

	result := "Warnings:\n"
	for _, w := range warnings {
		result += "- " + w + "\n"
	}
	return textResult(result), nil
}
