package daemon

import (
	"sync"
)

// EventBus is a simple publish-subscribe event bus.
type EventBus struct {
	mu          sync.RWMutex
	subscribers map[string]chan Event
	bufferSize  int

	// Event history for late-joining clients
	history    []Event
	maxHistory int
}

// NewEventBus creates a new event bus.
func NewEventBus() *EventBus {
	return &EventBus{
		subscribers: make(map[string]chan Event),
		bufferSize:  64, // Buffer size per subscriber
		history:     make([]Event, 0, 100),
		maxHistory:  100,
	}
}

// Subscribe creates a new subscription and returns an event channel.
func (eb *EventBus) Subscribe(clientID string) <-chan Event {
	eb.mu.Lock()
	defer eb.mu.Unlock()

	// Remove existing subscription if any
	if existing, ok := eb.subscribers[clientID]; ok {
		close(existing)
	}

	ch := make(chan Event, eb.bufferSize)
	eb.subscribers[clientID] = ch
	return ch
}

// Unsubscribe removes a subscription.
func (eb *EventBus) Unsubscribe(clientID string) {
	eb.mu.Lock()
	defer eb.mu.Unlock()

	if ch, ok := eb.subscribers[clientID]; ok {
		close(ch)
		delete(eb.subscribers, clientID)
	}
}

// Publish sends an event to all subscribers.
// Non-blocking: if a subscriber's channel is full, the event is dropped for that subscriber.
func (eb *EventBus) Publish(event Event) {
	eb.mu.Lock()
	defer eb.mu.Unlock()

	// Store in history
	eb.history = append(eb.history, event)
	if len(eb.history) > eb.maxHistory {
		eb.history = eb.history[1:]
	}

	// Notify subscribers
	for _, ch := range eb.subscribers {
		select {
		case ch <- event:
		default:
			// Channel full, drop event for this subscriber
			// This prevents slow subscribers from blocking others
		}
	}
}

// GetRecentEvents returns recent events from history.
func (eb *EventBus) GetRecentEvents(limit int) []Event {
	eb.mu.RLock()
	defer eb.mu.RUnlock()

	if limit <= 0 || limit > len(eb.history) {
		limit = len(eb.history)
	}

	// Return most recent events
	start := len(eb.history) - limit
	if start < 0 {
		start = 0
	}

	result := make([]Event, len(eb.history)-start)
	copy(result, eb.history[start:])
	return result
}

// GetEventsByType returns events of a specific type.
func (eb *EventBus) GetEventsByType(eventType EventType, limit int) []Event {
	eb.mu.RLock()
	defer eb.mu.RUnlock()

	result := make([]Event, 0)
	for i := len(eb.history) - 1; i >= 0 && len(result) < limit; i-- {
		if eb.history[i].Type == eventType {
			result = append(result, eb.history[i])
		}
	}

	// Reverse to get chronological order
	for i, j := 0, len(result)-1; i < j; i, j = i+1, j-1 {
		result[i], result[j] = result[j], result[i]
	}

	return result
}

// SubscriberCount returns the number of active subscribers.
func (eb *EventBus) SubscriberCount() int {
	eb.mu.RLock()
	defer eb.mu.RUnlock()
	return len(eb.subscribers)
}

// Close closes the event bus and all subscriber channels.
func (eb *EventBus) Close() {
	eb.mu.Lock()
	defer eb.mu.Unlock()

	for id, ch := range eb.subscribers {
		close(ch)
		delete(eb.subscribers, id)
	}
}
