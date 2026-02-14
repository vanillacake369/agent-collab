package event

import (
	"context"
	"testing"
	"time"

	"agent-collab/src/domain/interest"
)

func TestNewRouter(t *testing.T) {
	mgr := interest.NewManager()
	router := NewRouter(mgr, nil)

	if router == nil {
		t.Fatal("NewRouter returned nil")
	}
	if router.InterestManager() != mgr {
		t.Error("InterestManager mismatch")
	}
}

func TestRouter_Publish(t *testing.T) {
	mgr := interest.NewManager()
	router := NewRouter(mgr, &RouterConfig{
		NodeID:   "node-1",
		NodeName: "TestNode",
	})

	ctx := context.Background()
	event := NewFileChangeEvent("source-1", "Agent1", "/path/to/file.go", &FileChangePayload{
		ChangeType: "modify",
		Summary:    "Updated function",
	})

	if err := router.Publish(ctx, event); err != nil {
		t.Fatalf("Publish failed: %v", err)
	}

	// Verify event is stored
	if router.EventLog().Size() != 1 {
		t.Errorf("expected 1 event in log, got %d", router.EventLog().Size())
	}
}

func TestRouter_Subscribe(t *testing.T) {
	mgr := interest.NewManager()
	router := NewRouter(mgr, nil)

	// Subscribe to events
	ch := router.Subscribe("agent-1")
	if ch == nil {
		t.Fatal("Subscribe returned nil channel")
	}

	// Unsubscribe
	router.Unsubscribe("agent-1")

	// Channel should be closed
	select {
	case _, ok := <-ch:
		if ok {
			t.Error("expected channel to be closed")
		}
	default:
		// Channel might not have been drained yet
	}
}

func TestRouter_EventRouting(t *testing.T) {
	mgr := interest.NewManager()
	router := NewRouter(mgr, nil)

	// Register interest
	int1 := interest.NewInterest("agent-1", "Claude", []string{"proj-a/**"})
	mgr.Register(int1)

	// Subscribe
	ch := router.Subscribe("agent-1")

	ctx := context.Background()

	// Publish matching event
	event := NewFileChangeEvent("source", "Source", "proj-a/file.go", nil)
	router.PublishLocal(ctx, event)

	// Should receive the event
	select {
	case received := <-ch:
		if received.ID != event.ID {
			t.Errorf("received wrong event: %s != %s", received.ID, event.ID)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for event")
	}
}

func TestRouter_EventFiltering(t *testing.T) {
	mgr := interest.NewManager()
	router := NewRouter(mgr, nil)

	// Register interest for proj-a only
	int1 := interest.NewInterest("agent-1", "Claude", []string{"proj-a/**"})
	mgr.Register(int1)

	ch := router.Subscribe("agent-1")

	ctx := context.Background()

	// Publish non-matching event
	event := NewFileChangeEvent("source", "Source", "proj-b/file.go", nil)
	router.PublishLocal(ctx, event)

	// Should NOT receive (different project)
	select {
	case <-ch:
		t.Fatal("should not have received event for non-matching path")
	case <-time.After(100 * time.Millisecond):
		// Expected
	}
}

func TestRouter_GetEvents(t *testing.T) {
	mgr := interest.NewManager()
	router := NewRouter(mgr, nil)

	ctx := context.Background()

	// Publish some events
	for i := 0; i < 5; i++ {
		event := NewFileChangeEvent("source", "Source", "file.go", nil)
		router.PublishLocal(ctx, event)
	}

	// Get events
	events := router.GetEvents("agent-1", &EventFilter{
		Limit:      3,
		IncludeAll: true,
	})

	if len(events) != 3 {
		t.Errorf("expected 3 events, got %d", len(events))
	}
}

func TestRouter_GetEvents_TypeFilter(t *testing.T) {
	mgr := interest.NewManager()
	router := NewRouter(mgr, nil)

	ctx := context.Background()

	// Publish different event types
	router.PublishLocal(ctx, NewFileChangeEvent("s", "S", "f.go", nil))
	router.PublishLocal(ctx, NewLockAcquiredEvent("s", "S", "f.go", 1, 10, nil))
	router.PublishLocal(ctx, NewWarningEvent("s", "S", &WarningPayload{Message: "test"}))

	// Filter by type
	events := router.GetEvents("agent-1", &EventFilter{
		Types:      []EventType{EventTypeFileChange},
		IncludeAll: true,
	})

	if len(events) != 1 {
		t.Errorf("expected 1 file change event, got %d", len(events))
	}
}

func TestEventLog_Append(t *testing.T) {
	log := NewEventLog(&EventLogConfig{MaxSize: 100})

	event := NewEvent(EventTypeFileChange, "source", "Source")
	log.Append(event)

	if log.Size() != 1 {
		t.Errorf("expected 1 event, got %d", log.Size())
	}

	// Test duplicate prevention
	log.Append(event)
	if log.Size() != 1 {
		t.Errorf("expected 1 event (no duplicates), got %d", log.Size())
	}
}

func TestEventLog_SizeLimit(t *testing.T) {
	log := NewEventLog(&EventLogConfig{MaxSize: 5})

	for i := 0; i < 10; i++ {
		event := NewEvent(EventTypeFileChange, "source", "Source")
		log.Append(event)
	}

	if log.TotalSize() != 5 {
		t.Errorf("expected 5 events (max size), got %d", log.TotalSize())
	}
}

func TestEventLog_GetRecent(t *testing.T) {
	log := NewEventLog(&EventLogConfig{MaxSize: 100})

	for i := 0; i < 10; i++ {
		log.Append(NewEvent(EventTypeFileChange, "source", "Source"))
	}

	recent := log.GetRecent(3)
	if len(recent) != 3 {
		t.Errorf("expected 3 recent events, got %d", len(recent))
	}
}

func TestEventLog_GetByType(t *testing.T) {
	log := NewEventLog(&EventLogConfig{MaxSize: 100})

	log.Append(NewEvent(EventTypeFileChange, "s", "S"))
	log.Append(NewEvent(EventTypeLockAcquired, "s", "S"))
	log.Append(NewEvent(EventTypeFileChange, "s", "S"))

	fileChanges := log.GetByType(EventTypeFileChange)
	if len(fileChanges) != 2 {
		t.Errorf("expected 2 file changes, got %d", len(fileChanges))
	}
}

func TestEvent_SetGetPayload(t *testing.T) {
	event := NewEvent(EventTypeFileChange, "source", "Source")

	payload := &FileChangePayload{
		ChangeType: "modify",
		Summary:    "Test change",
		LinesDiff:  10,
	}

	if err := event.SetPayload(payload); err != nil {
		t.Fatalf("SetPayload failed: %v", err)
	}

	var retrieved FileChangePayload
	if err := event.GetPayload(&retrieved); err != nil {
		t.Fatalf("GetPayload failed: %v", err)
	}

	if retrieved.ChangeType != "modify" {
		t.Errorf("expected modify, got %s", retrieved.ChangeType)
	}
	if retrieved.LinesDiff != 10 {
		t.Errorf("expected 10, got %d", retrieved.LinesDiff)
	}
}
