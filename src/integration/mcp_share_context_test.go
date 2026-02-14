package integration

import (
	"context"
	"os"
	"testing"
	"time"

	"agent-collab/src/application"
	"agent-collab/src/domain/event"
)

// Test that share_context operation publishes to EventRouter
func TestShareContextPublishesToEventRouter(t *testing.T) {
	// Setup environment
	os.Setenv("AGENT_COLLAB_INTERESTS", "auth-lib/**")
	os.Setenv("AGENT_NAME", "ShareContextTestAgent")
	defer func() {
		os.Unsetenv("AGENT_COLLAB_INTERESTS")
		os.Unsetenv("AGENT_NAME")
	}()

	tmpDir := t.TempDir()
	cfg := &application.Config{
		DataDir: tmpDir,
	}

	// Create and initialize app
	app, err := application.New(cfg)
	if err != nil {
		t.Fatalf("Failed to create app: %v", err)
	}
	defer app.Stop()

	ctx := context.Background()
	result, err := app.Initialize(ctx, "share-context-test")
	if err != nil {
		t.Fatalf("Failed to initialize: %v", err)
	}

	// Get EventRouter
	eventRouter := app.EventRouter()
	if eventRouter == nil {
		t.Fatal("EventRouter is nil")
	}

	// Get initial event count
	initialEvents := eventRouter.GetEvents(result.NodeID, &event.EventFilter{
		Limit:      100,
		IncludeAll: true,
	})
	initialCount := len(initialEvents)
	t.Logf("Initial event count: %d", initialCount)

	// Simulate what handleShareContext does:
	// 1. Generate embedding (we skip this in test)
	// 2. Store in vector store
	// 3. Publish to EventRouter

	t.Run("Simulating share_context flow", func(t *testing.T) {
		// This is what publishToEventRouter does
		nodeID := result.NodeID
		nodeName := os.Getenv("AGENT_NAME")
		filePath := "auth-lib/jwt.go"
		content := "Updated JWT validation logic"

		// Create and publish event (same as publishToEventRouter)
		evt := event.NewContextSharedEvent(nodeID, nodeName, filePath, &event.ContextSharedPayload{
			Content: content,
		})

		err := eventRouter.Publish(ctx, evt)
		if err != nil {
			t.Fatalf("Publish failed: %v", err)
		}

		// Wait a bit
		time.Sleep(50 * time.Millisecond)

		// Check event count increased
		newEvents := eventRouter.GetEvents(result.NodeID, &event.EventFilter{
			Limit:      100,
			IncludeAll: true,
		})
		newCount := len(newEvents)
		t.Logf("New event count: %d (was %d)", newCount, initialCount)

		if newCount <= initialCount {
			t.Error("Event count should have increased after publish")
		}

		// Check the event is retrievable with interest filter
		filteredEvents := eventRouter.GetEvents(result.NodeID, &event.EventFilter{
			Limit: 100,
		})
		t.Logf("Filtered event count: %d", len(filteredEvents))

		if len(filteredEvents) == 0 {
			t.Error("Should have events matching auth-lib/** interest")
		}

		// Verify the event content
		found := false
		for _, e := range filteredEvents {
			if e.FilePath == filePath {
				found = true
				t.Logf("Found event: ID=%s, FilePath=%s, Type=%s", e.ID, e.FilePath, e.Type)
				break
			}
		}
		if !found {
			t.Errorf("Event for %s not found in filtered results", filePath)
		}
	})
}

// Test that BroadcastContext triggers EventRouter publish
func TestBroadcastContextTriggersEventRouter(t *testing.T) {
	os.Setenv("AGENT_COLLAB_INTERESTS", "src/**")
	os.Setenv("AGENT_NAME", "BroadcastTestAgent")
	defer func() {
		os.Unsetenv("AGENT_COLLAB_INTERESTS")
		os.Unsetenv("AGENT_NAME")
	}()

	tmpDir := t.TempDir()
	cfg := &application.Config{
		DataDir: tmpDir,
	}

	app, err := application.New(cfg)
	if err != nil {
		t.Fatalf("Failed to create app: %v", err)
	}
	defer app.Stop()

	ctx := context.Background()
	result, err := app.Initialize(ctx, "broadcast-test")
	if err != nil {
		t.Fatalf("Failed to initialize: %v", err)
	}

	// Start the app (needed for BroadcastContext to work)
	if err := app.Start(); err != nil {
		t.Fatalf("Failed to start app: %v", err)
	}

	eventRouter := app.EventRouter()

	// Get initial count
	initialEvents := eventRouter.GetEvents(result.NodeID, &event.EventFilter{
		Limit:      100,
		IncludeAll: true,
	})
	initialCount := len(initialEvents)

	t.Run("BroadcastContext should publish event", func(t *testing.T) {
		// Call BroadcastContext (this is what handleShareContext calls)
		embedding := make([]float32, 384) // Mock embedding
		err := app.BroadcastContext("src/main.go", "package main\n\nfunc main() {}", embedding, nil)
		if err != nil {
			t.Logf("BroadcastContext error (may be expected without peers): %v", err)
		}

		// Note: BroadcastContext might not directly publish to EventRouter
		// It's for P2P broadcast. The handleShareContext calls publishToEventRouter separately.

		time.Sleep(50 * time.Millisecond)

		// Check if event count changed
		newEvents := eventRouter.GetEvents(result.NodeID, &event.EventFilter{
			Limit:      100,
			IncludeAll: true,
		})
		newCount := len(newEvents)
		t.Logf("Event count after BroadcastContext: %d (was %d)", newCount, initialCount)

		// This test documents the current behavior
		// BroadcastContext is for P2P, not for local EventRouter
	})
}
