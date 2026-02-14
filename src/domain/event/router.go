package event

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"agent-collab/src/domain/interest"
	"agent-collab/src/infrastructure/storage/vector"
)

// Router routes events to interested agents.
type Router struct {
	mu sync.RWMutex

	interestMgr *interest.Manager
	eventLog    *EventLog
	vectorStore vector.Store
	broadcast   func(topic string, data []byte) error
	subscribers map[string][]chan *Event

	nodeID   string
	nodeName string
}

// RouterConfig holds configuration for the event router.
type RouterConfig struct {
	NodeID      string
	NodeName    string
	MaxEvents   int
	EventTTL    time.Duration
	VectorStore vector.Store
}

// DefaultRouterConfig returns default router configuration.
func DefaultRouterConfig() *RouterConfig {
	return &RouterConfig{
		MaxEvents: 10000,
		EventTTL:  DefaultEventTTL,
	}
}

// NewRouter creates a new event router.
func NewRouter(interestMgr *interest.Manager, cfg *RouterConfig) *Router {
	if cfg == nil {
		cfg = DefaultRouterConfig()
	}

	logCfg := &EventLogConfig{
		MaxSize:         cfg.MaxEvents,
		EventTTL:        cfg.EventTTL,
		CompactInterval: 5 * time.Minute,
	}

	return &Router{
		interestMgr: interestMgr,
		eventLog:    NewEventLog(logCfg),
		vectorStore: cfg.VectorStore,
		subscribers: make(map[string][]chan *Event),
		nodeID:      cfg.NodeID,
		nodeName:    cfg.NodeName,
	}
}

// SetBroadcastFn sets the P2P broadcast function.
func (r *Router) SetBroadcastFn(fn func(topic string, data []byte) error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.broadcast = fn
}

// SetVectorStore sets the vector store for semantic search.
func (r *Router) SetVectorStore(store vector.Store) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.vectorStore = store
}

// Publish publishes an event to interested agents.
func (r *Router) Publish(ctx context.Context, event *Event) error {
	r.storeEvent(event)
	r.storeInVectorDB(event)
	r.routeToSubscribers(event)

	return r.broadcastToCluster(ctx, event)
}

// PublishLocal publishes an event only to local subscribers (no P2P broadcast).
func (r *Router) PublishLocal(ctx context.Context, event *Event) error {
	r.storeEvent(event)
	r.routeToSubscribers(event)
	return nil
}

// storeEvent stores the event in the event log.
func (r *Router) storeEvent(event *Event) {
	r.eventLog.Append(event)
}

// storeInVectorDB stores the event in vector database for semantic search.
func (r *Router) storeInVectorDB(event *Event) {
	if r.vectorStore == nil || len(event.Embedding) == 0 {
		return
	}

	doc := &vector.Document{
		Content:   string(event.Payload),
		Embedding: event.Embedding,
		FilePath:  event.FilePath,
		Metadata: map[string]any{
			"event_id":   event.ID,
			"event_type": string(event.Type),
			"source_id":  event.SourceID,
			"timestamp":  event.Timestamp.Unix(),
		},
	}

	_ = r.vectorStore.Insert(doc)
}

// routeToSubscribers sends event to matching local subscribers.
func (r *Router) routeToSubscribers(event *Event) {
	targets := r.collectNotifyTargets(event)
	r.notifySubscribers(event, targets)
}

// collectNotifyTargets determines which agents should receive the event.
func (r *Router) collectNotifyTargets(event *Event) map[string]struct{} {
	targets := make(map[string]struct{})

	if event.FilePath == "" {
		return r.getAllSubscriberIDs()
	}

	matches := r.interestMgr.Match(event.FilePath)
	for _, match := range matches {
		if r.shouldNotify(match.Interest, event) {
			targets[match.Interest.AgentID] = struct{}{}
		}
	}

	return targets
}

// getAllSubscriberIDs returns all subscriber agent IDs.
func (r *Router) getAllSubscriberIDs() map[string]struct{} {
	r.mu.RLock()
	defer r.mu.RUnlock()

	targets := make(map[string]struct{})
	for agentID := range r.subscribers {
		targets[agentID] = struct{}{}
	}
	return targets
}

// shouldNotify checks if an agent should be notified based on interest level.
func (r *Router) shouldNotify(i *interest.Interest, event *Event) bool {
	if i.Level == interest.InterestLevelNone {
		return false
	}

	if i.Level == interest.InterestLevelLocksOnly {
		return isLockEvent(event.Type)
	}

	return true
}

// isLockEvent checks if the event type is lock-related.
func isLockEvent(t EventType) bool {
	return t == EventTypeLockAcquired ||
		t == EventTypeLockReleased ||
		t == EventTypeLockConflict
}

// notifySubscribers sends event to specified subscribers.
func (r *Router) notifySubscribers(event *Event, targets map[string]struct{}) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for agentID := range targets {
		channels, ok := r.subscribers[agentID]
		if !ok {
			continue
		}

		for _, ch := range channels {
			select {
			case ch <- event:
			default:
				// Channel full, skip
			}
		}
	}
}

// broadcastToCluster broadcasts event to P2P network.
func (r *Router) broadcastToCluster(ctx context.Context, event *Event) error {
	r.mu.RLock()
	broadcast := r.broadcast
	r.mu.RUnlock()

	if broadcast == nil {
		return nil
	}

	data, err := json.Marshal(event)
	if err != nil {
		return err
	}

	return broadcast(TopicEvents, data)
}

// Subscribe creates a subscription for an agent.
func (r *Router) Subscribe(agentID string) <-chan *Event {
	r.mu.Lock()
	defer r.mu.Unlock()

	ch := make(chan *Event, 100)
	r.subscribers[agentID] = append(r.subscribers[agentID], ch)

	return ch
}

// Unsubscribe removes all subscriptions for an agent.
func (r *Router) Unsubscribe(agentID string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	channels, ok := r.subscribers[agentID]
	if !ok {
		return
	}

	for _, ch := range channels {
		close(ch)
	}
	delete(r.subscribers, agentID)
}

// HandleRemoteEvent handles an event received from P2P network.
func (r *Router) HandleRemoteEvent(ctx context.Context, data []byte) error {
	var event Event
	if err := json.Unmarshal(data, &event); err != nil {
		return err
	}

	return r.PublishLocal(ctx, &event)
}

// GetEvents retrieves events with filtering.
func (r *Router) GetEvents(agentID string, filter *EventFilter) []*Event {
	if filter == nil {
		filter = DefaultEventFilter()
	}

	events := r.eventLog.GetRecent(filter.Limit * 2)
	events = r.filterByInterestOrAll(events, agentID, filter.IncludeAll)
	events = r.applyFilters(events, filter)

	return r.applyLimit(events, filter.Limit)
}

// filterByInterestOrAll filters events by agent interests or returns all.
func (r *Router) filterByInterestOrAll(events []*Event, agentID string, includeAll bool) []*Event {
	if includeAll {
		return events
	}

	interests := r.interestMgr.GetAgentInterests(agentID)
	return r.filterByInterests(events, interests)
}

// filterByInterests filters events by agent's interests.
func (r *Router) filterByInterests(events []*Event, interests []*interest.Interest) []*Event {
	if len(interests) == 0 {
		return nil
	}

	var filtered []*Event
	for _, event := range events {
		if r.matchesAnyInterest(event, interests) {
			filtered = append(filtered, event)
		}
	}

	return filtered
}

// matchesAnyInterest checks if event matches any of the given interests.
func (r *Router) matchesAnyInterest(event *Event, interests []*interest.Interest) bool {
	// Events without file path always match
	if event.FilePath == "" {
		return true
	}

	matches := r.interestMgr.Match(event.FilePath)
	for _, match := range matches {
		for _, i := range interests {
			if match.Interest.ID == i.ID {
				return true
			}
		}
	}

	return false
}

// applyFilters applies type, time, file path, and source filters.
func (r *Router) applyFilters(events []*Event, filter *EventFilter) []*Event {
	var result []*Event

	for _, event := range events {
		if !r.passesFilters(event, filter) {
			continue
		}
		result = append(result, event)
	}

	return result
}

// passesFilters checks if an event passes all filter criteria.
func (r *Router) passesFilters(event *Event, filter *EventFilter) bool {
	if !r.passesTypeFilter(event, filter.Types) {
		return false
	}
	if !r.passesTimeFilter(event, filter.Since) {
		return false
	}
	if !r.passesFilePathFilter(event, filter.FilePath) {
		return false
	}
	if !r.passesSourceFilter(event, filter.SourceID) {
		return false
	}
	return true
}

func (r *Router) passesTypeFilter(event *Event, types []EventType) bool {
	if len(types) == 0 {
		return true
	}
	for _, t := range types {
		if event.Type == t {
			return true
		}
	}
	return false
}

func (r *Router) passesTimeFilter(event *Event, since time.Time) bool {
	if since.IsZero() {
		return true
	}
	return !event.Timestamp.Before(since)
}

func (r *Router) passesFilePathFilter(event *Event, filePath string) bool {
	if filePath == "" {
		return true
	}
	return event.FilePath == filePath
}

func (r *Router) passesSourceFilter(event *Event, sourceID string) bool {
	if sourceID == "" {
		return true
	}
	return event.SourceID == sourceID
}

// applyLimit applies the limit to the result set.
func (r *Router) applyLimit(events []*Event, limit int) []*Event {
	if limit <= 0 || limit >= len(events) {
		return events
	}
	return events[:limit]
}

// SearchSimilar searches for similar events using vector similarity.
func (r *Router) SearchSimilar(ctx context.Context, query string, limit int) ([]*Event, error) {
	if r.vectorStore == nil {
		return nil, ErrVectorStoreNotConfigured
	}

	results, err := r.vectorStore.SearchByText(query, &vector.SearchOptions{TopK: limit})
	if err != nil {
		return nil, err
	}

	return r.resolveEventsFromSearchResults(results), nil
}

// resolveEventsFromSearchResults converts vector search results to events.
func (r *Router) resolveEventsFromSearchResults(results []*vector.SearchResult) []*Event {
	var events []*Event

	for _, result := range results {
		event := r.extractEventFromResult(result)
		if event != nil {
			events = append(events, event)
		}
	}

	return events
}

// extractEventFromResult extracts an event from a search result.
func (r *Router) extractEventFromResult(result *vector.SearchResult) *Event {
	if result == nil || result.Document == nil || result.Document.Metadata == nil {
		return nil
	}

	eventID, ok := result.Document.Metadata["event_id"].(string)
	if !ok {
		return nil
	}

	event, found := r.eventLog.Get(eventID)
	if !found {
		return nil
	}

	return event
}

// InterestManager returns the interest manager.
func (r *Router) InterestManager() *interest.Manager {
	return r.interestMgr
}

// EventLog returns the event log.
func (r *Router) EventLog() *EventLog {
	return r.eventLog
}

// Topic constants for P2P communication.
const (
	TopicEvents = "/agent-collab/events"
)
