package event_test

import (
	"context"
	"testing"
	"time"

	"agent-collab/src/domain/event"
	"agent-collab/src/domain/interest"
)

// Feature: Event Publishing
// As an AI agent
// I want to publish events about my actions
// So that other agents can be aware of changes

func TestFeature_EventPublishing(t *testing.T) {
	t.Run("Scenario: Agent publishes file change event", func(t *testing.T) {
		// Given an event router
		mgr := interest.NewManager()
		router := event.NewRouter(mgr, &event.RouterConfig{
			NodeID:   "node-1",
			NodeName: "TestNode",
		})

		// When an agent publishes a file change event
		ctx := context.Background()
		evt := event.NewFileChangeEvent("agent-1", "Claude", "/proj/file.go", &event.FileChangePayload{
			ChangeType: "modify",
			Summary:    "Added new function",
			LinesDiff:  25,
		})
		err := router.Publish(ctx, evt)

		// Then the event should be stored successfully
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}
		if router.EventLog().Size() != 1 {
			t.Errorf("Expected 1 event in log, got %d", router.EventLog().Size())
		}
	})

	t.Run("Scenario: Agent publishes lock acquired event", func(t *testing.T) {
		// Given an event router
		mgr := interest.NewManager()
		router := event.NewRouter(mgr, nil)

		// When an agent publishes a lock acquired event
		ctx := context.Background()
		evt := event.NewLockAcquiredEvent("agent-1", "Claude", "/proj/file.go", 10, 50, &event.LockPayload{
			LockID:     "lock-123",
			HolderID:   "agent-1",
			HolderName: "Claude",
			Purpose:    "Refactoring authentication",
		})
		err := router.Publish(ctx, evt)

		// Then the event should be stored
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		// And retrievable by type
		events := router.EventLog().GetByType(event.EventTypeLockAcquired)
		if len(events) != 1 {
			t.Errorf("Expected 1 lock event, got %d", len(events))
		}
	})
}

// Feature: Interest-based Event Routing
// As the event router
// I want to route events based on agent interests
// So that agents only receive relevant events

func TestFeature_InterestBasedRouting(t *testing.T) {
	t.Run("Scenario: Event routed to interested agent", func(t *testing.T) {
		// Given an agent interested in proj-a
		mgr := interest.NewManager()
		int1 := interest.NewInterest("agent-1", "Claude", []string{"proj-a/**"})
		mgr.Register(int1)

		router := event.NewRouter(mgr, nil)
		ch := router.Subscribe("agent-1")

		// When a file change event in proj-a is published
		ctx := context.Background()
		evt := event.NewFileChangeEvent("agent-2", "Gemini", "proj-a/main.go", nil)
		router.PublishLocal(ctx, evt)

		// Then the interested agent should receive the event
		select {
		case received := <-ch:
			if received.ID != evt.ID {
				t.Errorf("Expected event %s, got %s", evt.ID, received.ID)
			}
		case <-time.After(time.Second):
			t.Fatal("Timeout waiting for event")
		}
	})

	t.Run("Scenario: Event NOT routed to uninterested agent", func(t *testing.T) {
		// Given an agent interested in proj-a only
		mgr := interest.NewManager()
		int1 := interest.NewInterest("agent-1", "Claude", []string{"proj-a/**"})
		mgr.Register(int1)

		router := event.NewRouter(mgr, nil)
		ch := router.Subscribe("agent-1")

		// When a file change event in proj-b is published
		ctx := context.Background()
		evt := event.NewFileChangeEvent("agent-2", "Gemini", "proj-b/file.go", nil)
		router.PublishLocal(ctx, evt)

		// Then the agent should NOT receive the event
		select {
		case <-ch:
			t.Fatal("Should not have received event for unrelated project")
		case <-time.After(100 * time.Millisecond):
			// Expected - no event received
		}
	})

	t.Run("Scenario: Multiple agents receive same event", func(t *testing.T) {
		// Given two agents interested in the same area
		mgr := interest.NewManager()
		mgr.Register(interest.NewInterest("agent-1", "Claude", []string{"shared/**"}))
		mgr.Register(interest.NewInterest("agent-2", "Gemini", []string{"shared/**"}))

		router := event.NewRouter(mgr, nil)
		ch1 := router.Subscribe("agent-1")
		ch2 := router.Subscribe("agent-2")

		// When an event in the shared area is published
		ctx := context.Background()
		evt := event.NewFileChangeEvent("agent-3", "Cursor", "shared/utils.go", nil)
		router.PublishLocal(ctx, evt)

		// Then both agents should receive the event
		received := 0
		timeout := time.After(time.Second)

		for received < 2 {
			select {
			case <-ch1:
				received++
			case <-ch2:
				received++
			case <-timeout:
				t.Fatalf("Expected 2 receives, got %d", received)
			}
		}
	})

	t.Run("Scenario: Broadcast events go to all subscribers", func(t *testing.T) {
		// Given two agents with different interests
		mgr := interest.NewManager()
		mgr.Register(interest.NewInterest("agent-1", "Claude", []string{"proj-a/**"}))
		mgr.Register(interest.NewInterest("agent-2", "Gemini", []string{"proj-b/**"}))

		router := event.NewRouter(mgr, nil)
		ch1 := router.Subscribe("agent-1")
		ch2 := router.Subscribe("agent-2")

		// When an event without file path (broadcast) is published
		ctx := context.Background()
		evt := event.NewWarningEvent("system", "System", &event.WarningPayload{
			Level:   "warning",
			Message: "Cluster maintenance in 5 minutes",
		})
		router.PublishLocal(ctx, evt)

		// Then both agents should receive the broadcast
		received := 0
		timeout := time.After(time.Second)

		for received < 2 {
			select {
			case <-ch1:
				received++
			case <-ch2:
				received++
			case <-timeout:
				t.Fatalf("Expected 2 receives for broadcast, got %d", received)
			}
		}
	})
}

// Feature: Event Filtering
// As an AI agent
// I want to filter events by type and time
// So that I can query relevant history

func TestFeature_EventFiltering(t *testing.T) {
	t.Run("Scenario: Filter events by type", func(t *testing.T) {
		// Given a router with mixed event types
		mgr := interest.NewManager()
		router := event.NewRouter(mgr, nil)
		ctx := context.Background()

		router.PublishLocal(ctx, event.NewFileChangeEvent("s", "S", "f.go", nil))
		router.PublishLocal(ctx, event.NewLockAcquiredEvent("s", "S", "f.go", 1, 10, nil))
		router.PublishLocal(ctx, event.NewFileChangeEvent("s", "S", "g.go", nil))
		router.PublishLocal(ctx, event.NewWarningEvent("s", "S", &event.WarningPayload{}))

		// When filtering by file change type
		events := router.GetEvents("any", &event.EventFilter{
			Types:      []event.EventType{event.EventTypeFileChange},
			IncludeAll: true,
		})

		// Then only file change events should be returned
		if len(events) != 2 {
			t.Errorf("Expected 2 file change events, got %d", len(events))
		}
	})

	t.Run("Scenario: Filter events by time", func(t *testing.T) {
		// Given events published at different times
		mgr := interest.NewManager()
		router := event.NewRouter(mgr, nil)
		ctx := context.Background()

		// Publish old event
		oldEvt := event.NewFileChangeEvent("s", "S", "old.go", nil)
		oldEvt.Timestamp = time.Now().Add(-time.Hour)
		router.EventLog().Append(oldEvt)

		// Publish new event
		router.PublishLocal(ctx, event.NewFileChangeEvent("s", "S", "new.go", nil))

		// When filtering by time (last 30 minutes)
		since := time.Now().Add(-30 * time.Minute)
		events := router.GetEvents("any", &event.EventFilter{
			Since:      since,
			IncludeAll: true,
		})

		// Then only recent events should be returned
		if len(events) != 1 {
			t.Errorf("Expected 1 recent event, got %d", len(events))
		}
	})

	t.Run("Scenario: Limit number of returned events", func(t *testing.T) {
		// Given many events
		mgr := interest.NewManager()
		router := event.NewRouter(mgr, nil)
		ctx := context.Background()

		for i := 0; i < 20; i++ {
			router.PublishLocal(ctx, event.NewFileChangeEvent("s", "S", "file.go", nil))
		}

		// When requesting with limit
		events := router.GetEvents("any", &event.EventFilter{
			Limit:      5,
			IncludeAll: true,
		})

		// Then only the requested number should be returned
		if len(events) != 5 {
			t.Errorf("Expected 5 events, got %d", len(events))
		}
	})

	t.Run("Scenario: Filter respects agent's interests", func(t *testing.T) {
		// Given an agent interested in proj-a only
		mgr := interest.NewManager()
		mgr.Register(interest.NewInterest("agent-1", "Claude", []string{"proj-a/**"}))

		router := event.NewRouter(mgr, nil)
		ctx := context.Background()

		router.PublishLocal(ctx, event.NewFileChangeEvent("s", "S", "proj-a/file.go", nil))
		router.PublishLocal(ctx, event.NewFileChangeEvent("s", "S", "proj-b/file.go", nil))
		router.PublishLocal(ctx, event.NewFileChangeEvent("s", "S", "proj-a/other.go", nil))

		// When getting events without IncludeAll flag
		events := router.GetEvents("agent-1", &event.EventFilter{
			IncludeAll: false,
		})

		// Then only events matching interests should be returned
		if len(events) != 2 {
			t.Errorf("Expected 2 events matching interests, got %d", len(events))
		}
	})
}

// Feature: Event Subscription
// As an AI agent
// I want to subscribe to real-time events
// So that I can react immediately to changes

func TestFeature_EventSubscription(t *testing.T) {
	t.Run("Scenario: Agent subscribes and receives events", func(t *testing.T) {
		// Given an event router
		mgr := interest.NewManager()
		router := event.NewRouter(mgr, nil)

		// When an agent subscribes
		ch := router.Subscribe("agent-1")

		// Then they should have a valid channel
		if ch == nil {
			t.Fatal("Subscribe should return non-nil channel")
		}
	})

	t.Run("Scenario: Agent unsubscribes and channel closes", func(t *testing.T) {
		// Given a subscribed agent
		mgr := interest.NewManager()
		router := event.NewRouter(mgr, nil)
		ch := router.Subscribe("agent-1")

		// When the agent unsubscribes
		router.Unsubscribe("agent-1")

		// Then the channel should be closed
		time.Sleep(10 * time.Millisecond) // Give time for close to propagate
		select {
		case _, ok := <-ch:
			if ok {
				t.Error("Expected channel to be closed")
			}
		default:
			// Channel might not have data but should eventually close
		}
	})
}
