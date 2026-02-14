package event_test

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"agent-collab/src/domain/event"
	"agent-collab/src/domain/interest"
)

// Feature: P2P Event Broadcasting
// As the event router
// I want to broadcast events to the P2P network
// So that agents on different nodes receive relevant events

func TestFeature_P2PEventBroadcasting(t *testing.T) {
	t.Run("Scenario: Event is broadcast to P2P network when published", func(t *testing.T) {
		// Given an event router with P2P broadcast function configured
		mgr := interest.NewManager()
		router := event.NewRouter(mgr, &event.RouterConfig{
			NodeID:   "node-1",
			NodeName: "TestNode",
		})

		var broadcastedTopic string
		var broadcastedData []byte
		router.SetBroadcastFn(func(topic string, data []byte) error {
			broadcastedTopic = topic
			broadcastedData = data
			return nil
		})

		// When an event is published
		ctx := context.Background()
		evt := event.NewFileChangeEvent("agent-1", "Claude", "/project/file.go", &event.FileChangePayload{
			ChangeType: "modify",
			Summary:    "Added new function",
		})
		err := router.Publish(ctx, evt)

		// Then the event should be broadcast to the P2P network
		if err != nil {
			t.Fatalf("Publish failed: %v", err)
		}
		if broadcastedTopic != event.TopicEvents {
			t.Errorf("Expected topic %s, got %s", event.TopicEvents, broadcastedTopic)
		}
		if broadcastedData == nil {
			t.Fatal("Expected broadcast data, got nil")
		}

		// And the broadcast data should be valid JSON
		var receivedEvent event.Event
		if err := json.Unmarshal(broadcastedData, &receivedEvent); err != nil {
			t.Fatalf("Failed to unmarshal broadcast data: %v", err)
		}
		if receivedEvent.ID != evt.ID {
			t.Errorf("Expected event ID %s, got %s", evt.ID, receivedEvent.ID)
		}
	})

	t.Run("Scenario: Local publish does NOT broadcast to P2P", func(t *testing.T) {
		// Given an event router with P2P broadcast function configured
		mgr := interest.NewManager()
		router := event.NewRouter(mgr, nil)

		broadcastCalled := false
		router.SetBroadcastFn(func(topic string, data []byte) error {
			broadcastCalled = true
			return nil
		})

		// When a local event is published
		ctx := context.Background()
		evt := event.NewFileChangeEvent("agent-1", "Claude", "/project/file.go", nil)
		err := router.PublishLocal(ctx, evt)

		// Then the event should NOT be broadcast
		if err != nil {
			t.Fatalf("PublishLocal failed: %v", err)
		}
		if broadcastCalled {
			t.Error("PublishLocal should not trigger P2P broadcast")
		}
	})

	t.Run("Scenario: Event is stored even without P2P broadcast", func(t *testing.T) {
		// Given an event router without P2P broadcast configured
		mgr := interest.NewManager()
		router := event.NewRouter(mgr, nil)
		// No broadcast function set

		// When an event is published
		ctx := context.Background()
		evt := event.NewFileChangeEvent("agent-1", "Claude", "/project/file.go", nil)
		err := router.Publish(ctx, evt)

		// Then the event should still be stored
		if err != nil {
			t.Fatalf("Publish failed: %v", err)
		}
		if router.EventLog().Size() != 1 {
			t.Errorf("Expected 1 event in log, got %d", router.EventLog().Size())
		}
	})
}

// Feature: Remote Event Handling
// As the event router
// I want to handle events received from P2P network
// So that local agents are notified of remote changes

func TestFeature_RemoteEventHandling(t *testing.T) {
	t.Run("Scenario: Remote event is stored and routed locally", func(t *testing.T) {
		// Given an event router with a subscribed agent
		mgr := interest.NewManager()
		int1 := interest.NewInterest("agent-1", "Claude", []string{"project/**"})
		mgr.Register(int1)

		router := event.NewRouter(mgr, nil)
		ch := router.Subscribe("agent-1")

		// When a remote event is received
		remoteEvent := event.NewFileChangeEvent("remote-agent", "Gemini", "project/file.go", nil)
		data, _ := json.Marshal(remoteEvent)
		err := router.HandleRemoteEvent(context.Background(), data)

		// Then the event should be processed successfully
		if err != nil {
			t.Fatalf("HandleRemoteEvent failed: %v", err)
		}

		// And the event should be stored
		if router.EventLog().Size() != 1 {
			t.Errorf("Expected 1 event in log, got %d", router.EventLog().Size())
		}

		// And local subscribers should receive the event
		select {
		case received := <-ch:
			if received.ID != remoteEvent.ID {
				t.Errorf("Expected event ID %s, got %s", remoteEvent.ID, received.ID)
			}
		case <-time.After(time.Second):
			t.Fatal("Timeout waiting for remote event")
		}
	})

	t.Run("Scenario: Remote event does NOT rebroadcast", func(t *testing.T) {
		// Given an event router with P2P broadcast configured
		mgr := interest.NewManager()
		router := event.NewRouter(mgr, nil)

		broadcastCalled := false
		router.SetBroadcastFn(func(topic string, data []byte) error {
			broadcastCalled = true
			return nil
		})

		// When a remote event is received
		remoteEvent := event.NewFileChangeEvent("remote-agent", "Gemini", "project/file.go", nil)
		data, _ := json.Marshal(remoteEvent)
		_ = router.HandleRemoteEvent(context.Background(), data)

		// Then it should NOT be rebroadcast
		if broadcastCalled {
			t.Error("Remote events should not be rebroadcast")
		}
	})

	t.Run("Scenario: Invalid remote event data is rejected", func(t *testing.T) {
		// Given an event router
		mgr := interest.NewManager()
		router := event.NewRouter(mgr, nil)

		// When invalid JSON is received
		err := router.HandleRemoteEvent(context.Background(), []byte("invalid json"))

		// Then an error should be returned
		if err == nil {
			t.Error("Expected error for invalid JSON, got nil")
		}
	})
}

// Feature: Multi-Node Event Synchronization
// As the agent-collab system
// I want events to synchronize across multiple nodes
// So that all agents have consistent view of changes

func TestFeature_MultiNodeEventSync(t *testing.T) {
	t.Run("Scenario: Two nodes sync events through simulated P2P", func(t *testing.T) {
		// Given two nodes with their own routers
		mgr1 := interest.NewManager()
		mgr2 := interest.NewManager()

		router1 := event.NewRouter(mgr1, &event.RouterConfig{
			NodeID:   "node-1",
			NodeName: "Node1",
		})
		router2 := event.NewRouter(mgr2, &event.RouterConfig{
			NodeID:   "node-2",
			NodeName: "Node2",
		})

		// Setup P2P simulation: router1 broadcasts to router2
		router1.SetBroadcastFn(func(topic string, data []byte) error {
			return router2.HandleRemoteEvent(context.Background(), data)
		})

		// And an agent on node2 subscribes to all events
		mgr2.Register(interest.NewInterest("agent-2", "Gemini", []string{"**"}))
		ch2 := router2.Subscribe("agent-2")

		// When node1 publishes an event
		ctx := context.Background()
		evt := event.NewFileChangeEvent("agent-1", "Claude", "shared/file.go", nil)
		err := router1.Publish(ctx, evt)

		// Then the event should be received on node2
		if err != nil {
			t.Fatalf("Publish failed: %v", err)
		}

		select {
		case received := <-ch2:
			if received.ID != evt.ID {
				t.Errorf("Expected event ID %s, got %s", evt.ID, received.ID)
			}
			if received.SourceName != "Claude" {
				t.Errorf("Expected source name Claude, got %s", received.SourceName)
			}
		case <-time.After(time.Second):
			t.Fatal("Timeout waiting for synced event on node2")
		}

		// And both nodes should have the event in their logs
		if router1.EventLog().Size() != 1 {
			t.Errorf("Node1 should have 1 event, got %d", router1.EventLog().Size())
		}
		if router2.EventLog().Size() != 1 {
			t.Errorf("Node2 should have 1 event, got %d", router2.EventLog().Size())
		}
	})

	t.Run("Scenario: Bidirectional event sync between nodes", func(t *testing.T) {
		// Given two nodes configured for bidirectional sync
		mgr1 := interest.NewManager()
		mgr2 := interest.NewManager()

		router1 := event.NewRouter(mgr1, &event.RouterConfig{
			NodeID:   "node-1",
			NodeName: "Node1",
		})
		router2 := event.NewRouter(mgr2, &event.RouterConfig{
			NodeID:   "node-2",
			NodeName: "Node2",
		})

		// Bidirectional P2P simulation
		router1.SetBroadcastFn(func(topic string, data []byte) error {
			return router2.HandleRemoteEvent(context.Background(), data)
		})
		router2.SetBroadcastFn(func(topic string, data []byte) error {
			return router1.HandleRemoteEvent(context.Background(), data)
		})

		// Both nodes have subscribers
		mgr1.Register(interest.NewInterest("agent-1", "Claude", []string{"**"}))
		mgr2.Register(interest.NewInterest("agent-2", "Gemini", []string{"**"}))
		ch1 := router1.Subscribe("agent-1")
		ch2 := router2.Subscribe("agent-2")

		ctx := context.Background()

		// When node1 publishes
		evt1 := event.NewFileChangeEvent("agent-1", "Claude", "proj/a.go", nil)
		router1.Publish(ctx, evt1)

		// And node2 publishes
		evt2 := event.NewFileChangeEvent("agent-2", "Gemini", "proj/b.go", nil)
		router2.Publish(ctx, evt2)

		// Then agent on node1 should receive an event from node2
		// (might also receive local event first due to broadcast order)
		receivedFromGemini := false
		receivedFromClaude := false

		// Drain events from ch1 (expect event from Gemini)
		for i := 0; i < 2; i++ {
			select {
			case received := <-ch1:
				if received.SourceName == "Gemini" {
					receivedFromGemini = true
				}
			case <-time.After(time.Second):
				break
			}
		}

		// Drain events from ch2 (expect event from Claude)
		for i := 0; i < 2; i++ {
			select {
			case received := <-ch2:
				if received.SourceName == "Claude" {
					receivedFromClaude = true
				}
			case <-time.After(time.Second):
				break
			}
		}

		if !receivedFromGemini {
			t.Error("Node1 agent should have received event from Gemini")
		}
		if !receivedFromClaude {
			t.Error("Node2 agent should have received event from Claude")
		}
	})
}

// Feature: Concurrent Event Processing
// As the event router
// I want to handle concurrent event publications safely
// So that the system remains stable under load

func TestFeature_ConcurrentEventProcessing(t *testing.T) {
	t.Run("Scenario: Multiple agents publish events concurrently", func(t *testing.T) {
		// Given an event router
		mgr := interest.NewManager()
		router := event.NewRouter(mgr, nil)

		// Register interests for all agents
		for i := 0; i < 5; i++ {
			agentID := agentIDForIndex(i)
			mgr.Register(interest.NewInterest(agentID, agentID, []string{"**"}))
		}

		var broadcastCount int
		var mu sync.Mutex
		router.SetBroadcastFn(func(topic string, data []byte) error {
			mu.Lock()
			broadcastCount++
			mu.Unlock()
			return nil
		})

		ctx := context.Background()

		// When 100 events are published concurrently
		var wg sync.WaitGroup
		for i := 0; i < 100; i++ {
			wg.Add(1)
			go func(idx int) {
				defer wg.Done()
				agentID := agentIDForIndex(idx % 5)
				evt := event.NewFileChangeEvent(agentID, agentID, "file.go", nil)
				router.Publish(ctx, evt)
			}(i)
		}
		wg.Wait()

		// Then all events should be stored
		if router.EventLog().Size() != 100 {
			t.Errorf("Expected 100 events, got %d", router.EventLog().Size())
		}

		// And all events should be broadcast
		mu.Lock()
		if broadcastCount != 100 {
			t.Errorf("Expected 100 broadcasts, got %d", broadcastCount)
		}
		mu.Unlock()
	})

	t.Run("Scenario: Concurrent subscribe and unsubscribe", func(t *testing.T) {
		// Given an event router
		mgr := interest.NewManager()
		router := event.NewRouter(mgr, nil)

		// When multiple goroutines subscribe and unsubscribe
		var wg sync.WaitGroup
		for i := 0; i < 50; i++ {
			wg.Add(1)
			go func(idx int) {
				defer wg.Done()
				agentID := agentIDForIndex(idx)
				ch := router.Subscribe(agentID)
				time.Sleep(10 * time.Millisecond)
				router.Unsubscribe(agentID)
				// Channel should be closed
				select {
				case _, ok := <-ch:
					if ok {
						// Might receive events before close, that's OK
					}
				default:
				}
			}(i)
		}
		wg.Wait()

		// Then no panics should occur (test passes if no panic)
	})
}

// Helper function to generate agent IDs
func agentIDForIndex(i int) string {
	return "agent-" + string(rune('A'+i%26))
}
