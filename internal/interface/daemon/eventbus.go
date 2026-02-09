package daemon

import (
	"sync"
)

// EventBus is a simple publish-subscribe event bus.
type EventBus struct {
	mu          sync.RWMutex
	subscribers map[string]chan Event
	bufferSize  int
}

// NewEventBus creates a new event bus.
func NewEventBus() *EventBus {
	return &EventBus{
		subscribers: make(map[string]chan Event),
		bufferSize:  64, // Buffer size per subscriber
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
	eb.mu.RLock()
	defer eb.mu.RUnlock()

	for _, ch := range eb.subscribers {
		select {
		case ch <- event:
		default:
			// Channel full, drop event for this subscriber
			// This prevents slow subscribers from blocking others
		}
	}
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
