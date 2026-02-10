// Package integration provides integration tests for the agent-collab system.
package integration

import (
	"context"
	"testing"
	"time"

	v1 "agent-collab/src/api/v1"
	"agent-collab/src/store"
	"agent-collab/src/store/memory"
)

// lockObject implements store.Object for Lock
type lockObject struct {
	*v1.Lock
}

func (l *lockObject) DeepCopy() store.Object {
	if l == nil || l.Lock == nil {
		return nil
	}
	copy := *l.Lock
	return &lockObject{Lock: &copy}
}

// contextObject implements store.Object for Context
type contextObject struct {
	*v1.Context
}

func (c *contextObject) DeepCopy() store.Object {
	if c == nil || c.Context == nil {
		return nil
	}
	copy := *c.Context
	return &contextObject{Context: &copy}
}

// agentObject implements store.Object for Agent
type agentObject struct {
	*v1.Agent
}

func (a *agentObject) DeepCopy() store.Object {
	if a == nil || a.Agent == nil {
		return nil
	}
	copy := *a.Agent
	return &agentObject{Agent: &copy}
}

func TestStoreIntegration(t *testing.T) {
	ctx := context.Background()

	// Create stores
	lockStore := memory.New[*lockObject]()
	defer lockStore.Close()

	contextStore := memory.New[*contextObject]()
	defer contextStore.Close()

	agentStore := memory.New[*agentObject]()
	defer agentStore.Close()

	t.Run("LockLifecycle", func(t *testing.T) {
		// Create a lock
		lock := &lockObject{
			Lock: v1.NewLock("test-lock", v1.LockSpec{
				Target: v1.LockTarget{
					Type:     v1.LockTargetTypeFile,
					FilePath: "/path/to/file.go",
				},
				HolderID:  "agent-1",
				Intention: "Editing authentication logic",
				TTL:       v1.Duration{Duration: 5 * time.Minute},
				Exclusive: true,
			}),
		}

		// Create
		if err := lockStore.Create(ctx, lock); err != nil {
			t.Fatalf("Create failed: %v", err)
		}

		// Verify creation
		got, err := lockStore.Get(ctx, "test-lock")
		if err != nil {
			t.Fatalf("Get failed: %v", err)
		}
		if got.Spec.HolderID != "agent-1" {
			t.Errorf("HolderID = %s, want agent-1", got.Spec.HolderID)
		}

		// Update status to Active
		got.Status.Phase = v1.LockPhaseActive
		now := time.Now()
		got.Status.AcquiredAt = &now
		expires := now.Add(5 * time.Minute)
		got.Status.ExpiresAt = &expires
		got.Status.FencingToken = 1

		if err := lockStore.Update(ctx, got); err != nil {
			t.Fatalf("Update failed: %v", err)
		}

		// Verify update
		updated, _ := lockStore.Get(ctx, "test-lock")
		if updated.Status.Phase != v1.LockPhaseActive {
			t.Errorf("Phase = %s, want Active", updated.Status.Phase)
		}
		if updated.Status.FencingToken != 1 {
			t.Errorf("FencingToken = %d, want 1", updated.Status.FencingToken)
		}

		// Delete
		if err := lockStore.Delete(ctx, "test-lock"); err != nil {
			t.Fatalf("Delete failed: %v", err)
		}

		// Verify deletion
		_, err = lockStore.Get(ctx, "test-lock")
		if err != store.ErrNotFound {
			t.Errorf("Get after delete error = %v, want ErrNotFound", err)
		}
	})

	t.Run("ContextSharing", func(t *testing.T) {
		// Agent 1 shares context
		ctx1 := &contextObject{
			Context: v1.NewContext("ctx-1", v1.ContextSpec{
				Type:          v1.ContextTypeFile,
				SourceAgentID: "agent-1",
				FilePath:      "/path/to/file.go",
				Content:       "package main\n\nfunc main() {}",
				VectorClock: map[string]uint64{
					"agent-1": 1,
				},
			}),
		}

		if err := contextStore.Create(ctx, ctx1); err != nil {
			t.Fatalf("Create context failed: %v", err)
		}

		// Agent 2 shares related context
		ctx2 := &contextObject{
			Context: v1.NewContext("ctx-2", v1.ContextSpec{
				Type:          v1.ContextTypeDelta,
				SourceAgentID: "agent-2",
				FilePath:      "/path/to/file.go",
				Content:       "package main\n\nfunc main() {\n\tfmt.Println(\"Hello\")\n}",
				VectorClock: map[string]uint64{
					"agent-1": 1,
					"agent-2": 1,
				},
				Delta: &v1.ContextDelta{
					Operation:  v1.DeltaOperationModify,
					OldContent: "func main() {}",
					NewContent: "func main() {\n\tfmt.Println(\"Hello\")\n}",
					StartLine:  3,
					EndLine:    5,
				},
			}),
		}

		if err := contextStore.Create(ctx, ctx2); err != nil {
			t.Fatalf("Create context 2 failed: %v", err)
		}

		// List all contexts
		contexts, err := contextStore.List(ctx, store.ListOptions{})
		if err != nil {
			t.Fatalf("List contexts failed: %v", err)
		}
		if len(contexts) != 2 {
			t.Errorf("Context count = %d, want 2", len(contexts))
		}
	})

	t.Run("AgentRegistry", func(t *testing.T) {
		// Register agents
		agent1 := &agentObject{
			Agent: v1.NewAgent("agent-1", v1.AgentSpec{
				Provider: v1.AgentProviderAnthropic,
				Model:    "claude-3-opus",
				Capabilities: []v1.AgentCapability{
					v1.AgentCapabilityCodeEdit,
					v1.AgentCapabilityToolUse,
				},
				PeerID:      "12D3KooW...",
				DisplayName: "Claude Agent 1",
			}),
		}
		agent1.Status.Phase = v1.AgentPhaseOnline

		agent2 := &agentObject{
			Agent: v1.NewAgent("agent-2", v1.AgentSpec{
				Provider: v1.AgentProviderOpenAI,
				Model:    "gpt-4",
				Capabilities: []v1.AgentCapability{
					v1.AgentCapabilityCodeEdit,
				},
				PeerID:      "12D3KooX...",
				DisplayName: "GPT-4 Agent",
			}),
		}
		agent2.Status.Phase = v1.AgentPhaseOnline

		if err := agentStore.Create(ctx, agent1); err != nil {
			t.Fatalf("Create agent 1 failed: %v", err)
		}
		if err := agentStore.Create(ctx, agent2); err != nil {
			t.Fatalf("Create agent 2 failed: %v", err)
		}

		// List agents
		agents, err := agentStore.List(ctx, store.ListOptions{})
		if err != nil {
			t.Fatalf("List agents failed: %v", err)
		}
		if len(agents) != 2 {
			t.Errorf("Agent count = %d, want 2", len(agents))
		}

		// Check capabilities
		got, _ := agentStore.Get(ctx, "agent-1")
		if !got.HasCapability(v1.AgentCapabilityCodeEdit) {
			t.Error("Agent 1 should have CodeEdit capability")
		}
	})
}

func TestWatchIntegration(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	lockStore := memory.New[*lockObject]()
	defer lockStore.Close()

	// Start watching
	watcher, err := lockStore.Watch(ctx, store.WatchOptions{
		SendInitialEvents: true,
	})
	if err != nil {
		t.Fatalf("Watch failed: %v", err)
	}
	defer watcher.Stop()

	// Create locks and verify watch events
	events := make(chan store.WatchEvent[*lockObject], 10)
	go func() {
		for event := range watcher.ResultChan() {
			events <- event
		}
	}()

	// Create a lock
	lock := &lockObject{
		Lock: v1.NewLock("watch-lock", v1.LockSpec{
			HolderID: "agent-1",
			TTL:      v1.Duration{Duration: time.Minute},
		}),
	}
	if err := lockStore.Create(ctx, lock); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	// Wait for ADDED event
	select {
	case event := <-events:
		if event.Type != v1.EventAdded {
			t.Errorf("Event type = %s, want ADDED", event.Type)
		}
		if event.Object.Name != "watch-lock" {
			t.Errorf("Event object name = %s, want watch-lock", event.Object.Name)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Timeout waiting for ADDED event")
	}

	// Update the lock
	got, _ := lockStore.Get(ctx, "watch-lock")
	got.Status.Phase = v1.LockPhaseActive
	if err := lockStore.Update(ctx, got); err != nil {
		t.Fatalf("Update failed: %v", err)
	}

	// Wait for MODIFIED event
	select {
	case event := <-events:
		if event.Type != v1.EventModified {
			t.Errorf("Event type = %s, want MODIFIED", event.Type)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Timeout waiting for MODIFIED event")
	}

	// Delete the lock
	if err := lockStore.Delete(ctx, "watch-lock"); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	// Wait for DELETED event
	select {
	case event := <-events:
		if event.Type != v1.EventDeleted {
			t.Errorf("Event type = %s, want DELETED", event.Type)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Timeout waiting for DELETED event")
	}
}

func TestConcurrentAccess(t *testing.T) {
	ctx := context.Background()

	lockStore := memory.New[*lockObject]()
	defer lockStore.Close()

	// Concurrent creates from multiple "agents"
	const numAgents = 10
	const locksPerAgent = 10

	errCh := make(chan error, numAgents*locksPerAgent)

	for i := range numAgents {
		go func(agentID int) {
			for j := range locksPerAgent {
				lock := &lockObject{
					Lock: v1.NewLock(
						"lock-"+string(rune('A'+agentID))+"-"+string(rune('0'+j)),
						v1.LockSpec{
							HolderID: "agent-" + string(rune('0'+agentID)),
							TTL:      v1.Duration{Duration: time.Minute},
						},
					),
				}
				if err := lockStore.Create(ctx, lock); err != nil {
					errCh <- err
					return
				}
			}
		}(i)
	}

	// Wait for all creates
	time.Sleep(500 * time.Millisecond)

	// Check for errors
	select {
	case err := <-errCh:
		t.Fatalf("Concurrent create failed: %v", err)
	default:
	}

	// Verify count
	locks, _ := lockStore.List(ctx, store.ListOptions{})
	if len(locks) != numAgents*locksPerAgent {
		t.Errorf("Lock count = %d, want %d", len(locks), numAgents*locksPerAgent)
	}
}
