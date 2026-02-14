package integration

import (
	"context"
	"os"
	"testing"

	"agent-collab/src/application"
	"agent-collab/src/domain/event"
)

// Debug test to trace the exact event flow
func TestEventFlowDebug(t *testing.T) {
	// Setup environment
	os.Setenv("AGENT_COLLAB_INTERESTS", "auth-lib/**")
	os.Setenv("AGENT_NAME", "DebugAgent")
	defer func() {
		os.Unsetenv("AGENT_COLLAB_INTERESTS")
		os.Unsetenv("AGENT_NAME")
	}()

	tmpDir := t.TempDir()
	cfg := &application.Config{
		DataDir: tmpDir,
	}

	// Step 1: Create Application
	t.Log("Step 1: Creating Application...")
	app, err := application.New(cfg)
	if err != nil {
		t.Fatalf("Failed to create app: %v", err)
	}
	defer app.Stop()

	// Step 2: Initialize
	t.Log("Step 2: Initializing...")
	ctx := context.Background()
	result, err := app.Initialize(ctx, "debug-project")
	if err != nil {
		t.Fatalf("Failed to initialize: %v", err)
	}
	t.Logf("  NodeID: %s", result.NodeID)

	// Step 3: Check InterestManager
	t.Log("Step 3: Checking InterestManager...")
	interestMgr := app.InterestManager()
	if interestMgr == nil {
		t.Fatal("InterestManager is nil!")
	}
	t.Log("  InterestManager: OK")

	// Step 4: Check registered interests
	t.Log("Step 4: Checking registered interests...")
	interests := interestMgr.GetAgentInterests(result.NodeID)
	t.Logf("  Interests for nodeID %s: %d", result.NodeID, len(interests))
	for i, in := range interests {
		t.Logf("    [%d] AgentID=%s, Patterns=%v, Level=%s", i, in.AgentID, in.Patterns, in.Level.String())
	}

	if len(interests) == 0 {
		t.Error("PROBLEM: No interests registered!")
	}

	// Step 5: Check EventRouter
	t.Log("Step 5: Checking EventRouter...")
	eventRouter := app.EventRouter()
	if eventRouter == nil {
		t.Fatal("EventRouter is nil!")
	}
	t.Log("  EventRouter: OK")

	// Step 6: Check EventLog is empty
	t.Log("Step 6: Checking EventLog...")
	eventLog := eventRouter.EventLog()
	t.Logf("  EventLog size before publish: %d", eventLog.Size())

	// Step 7: Publish an event
	t.Log("Step 7: Publishing test event...")
	evt := event.NewContextSharedEvent(
		result.NodeID, "DebugAgent",
		"auth-lib/jwt.go",
		&event.ContextSharedPayload{Content: "test content"},
	)
	t.Logf("  Event ID: %s, FilePath: %s", evt.ID, evt.FilePath)

	err = eventRouter.Publish(ctx, evt)
	if err != nil {
		t.Fatalf("Publish failed: %v", err)
	}
	t.Log("  Publish: OK")

	// Step 8: Check EventLog after publish
	t.Log("Step 8: Checking EventLog after publish...")
	t.Logf("  EventLog size after publish: %d", eventLog.Size())

	storedEvt, found := eventLog.Get(evt.ID)
	if !found {
		t.Error("PROBLEM: Event not found in EventLog!")
	} else {
		t.Logf("  Event found: ID=%s, FilePath=%s", storedEvt.ID, storedEvt.FilePath)
	}

	// Step 9: Test GetEvents with IncludeAll
	t.Log("Step 9: Testing GetEvents with IncludeAll=true...")
	allEvents := eventRouter.GetEvents(result.NodeID, &event.EventFilter{
		Limit:      10,
		IncludeAll: true,
	})
	t.Logf("  Events with IncludeAll: %d", len(allEvents))

	// Step 10: Test GetEvents with Interest filtering
	t.Log("Step 10: Testing GetEvents with Interest filtering...")
	filteredEvents := eventRouter.GetEvents(result.NodeID, &event.EventFilter{
		Limit: 10,
	})
	t.Logf("  Events with Interest filter: %d", len(filteredEvents))

	if len(filteredEvents) == 0 && len(allEvents) > 0 {
		t.Error("PROBLEM: Interest filtering returned 0 events!")

		// Debug: Check what interests are being used
		t.Log("  Debug: Checking interests used for filtering...")
		debugInterests := interestMgr.GetAgentInterests(result.NodeID)
		t.Logf("  Interests for agentID=%s: %d", result.NodeID, len(debugInterests))

		// Debug: Check if the file path matches
		t.Log("  Debug: Testing Match for auth-lib/jwt.go...")
		matches := interestMgr.Match("auth-lib/jwt.go")
		t.Logf("  Matches: %d", len(matches))
		for _, m := range matches {
			t.Logf("    Match: AgentID=%s, Patterns=%v", m.Interest.AgentID, m.Interest.Patterns)
		}
	}

	// Step 11: Verify all is well
	if len(interests) > 0 && len(filteredEvents) > 0 {
		t.Log("SUCCESS: Event flow is working correctly!")
	} else {
		t.Log("FAILURE: Event flow has issues.")
	}
}

