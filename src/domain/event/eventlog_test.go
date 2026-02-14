package event

import (
	"testing"
	"time"
)

func TestEventLog_Compaction(t *testing.T) {
	cfg := &EventLogConfig{
		MaxSize:          100,
		MaxEventsPerFile: 3,
		CompactInterval:  0, // Disable auto-compaction for test
	}
	log := NewEventLog(cfg)
	defer log.Stop()

	t.Run("When multiple events for same file", func(t *testing.T) {
		// Add 5 events for the same file
		for i := 0; i < 5; i++ {
			event := NewContextSharedEvent("source", "Source", "auth/jwt.go", &ContextSharedPayload{
				Content: "update " + string(rune('0'+i)),
			})
			log.Append(event)
		}

		// Only MaxEventsPerFile (3) should be active
		fileEvents := log.GetByFile("auth/jwt.go")
		if len(fileEvents) != 3 {
			t.Errorf("expected 3 active events per file, got %d", len(fileEvents))
		}

		// Verify older events are archived
		latest := log.GetLatestByFile("auth/jwt.go")
		if latest == nil {
			t.Fatal("expected to get latest event")
		}
	})
}

func TestEventLog_TTLExpiration(t *testing.T) {
	cfg := &EventLogConfig{
		MaxSize:         100,
		EventTTL:        50 * time.Millisecond,
		CompactInterval: 0,
	}
	log := NewEventLog(cfg)
	defer log.Stop()

	// Add event with short TTL
	event := NewEvent(EventTypeContextShared, "source", "Source")
	event.ExpiresAt = time.Now().Add(50 * time.Millisecond)
	log.Append(event)

	if log.Size() != 1 {
		t.Errorf("expected 1 event, got %d", log.Size())
	}

	// Wait for expiration
	time.Sleep(60 * time.Millisecond)

	// Trigger compaction
	log.Compact()

	if log.Size() != 0 {
		t.Errorf("expected 0 events after TTL expiration, got %d", log.Size())
	}
}

func TestEventLog_MarkFileEventsCompleted(t *testing.T) {
	log := NewEventLog(&EventLogConfig{MaxSize: 100})
	defer log.Stop()

	// Add events for a file
	for i := 0; i < 3; i++ {
		event := NewContextSharedEvent("source", "Source", "api/handler.go", nil)
		log.Append(event)
	}

	// Mark all as completed
	log.MarkFileEventsCompleted("api/handler.go")

	// Check status
	events := log.GetByFile("api/handler.go")
	for _, event := range events {
		if event.Status != EventStatusCompleted {
			t.Errorf("expected completed status, got %s", event.Status)
		}
	}
}

func TestEventLog_GetSummary(t *testing.T) {
	log := NewEventLog(&EventLogConfig{MaxSize: 100})
	defer log.Stop()

	// Add various events
	log.Append(NewEvent(EventTypeFileChange, "s1", "Source1"))
	log.Append(NewEvent(EventTypeFileChange, "s2", "Source2"))
	log.Append(NewEvent(EventTypeLockAcquired, "s1", "Source1"))

	summary := log.GetSummary()

	if summary.TotalEvents != 3 {
		t.Errorf("expected 3 total events, got %d", summary.TotalEvents)
	}
	if summary.UniqueSources != 2 {
		t.Errorf("expected 2 unique sources, got %d", summary.UniqueSources)
	}
	if summary.EventsByType[EventTypeFileChange] != 2 {
		t.Errorf("expected 2 file change events, got %d", summary.EventsByType[EventTypeFileChange])
	}
}

func TestEventLog_GetActiveFiles(t *testing.T) {
	log := NewEventLog(&EventLogConfig{MaxSize: 100})
	defer log.Stop()

	// Add events for different files
	e1 := NewContextSharedEvent("s", "S", "file1.go", nil)
	e2 := NewContextSharedEvent("s", "S", "file2.go", nil)
	log.Append(e1)
	log.Append(e2)

	// Archive one
	e1.MarkArchived()

	files := log.GetActiveFiles()
	if len(files) != 1 {
		t.Errorf("expected 1 active file, got %d", len(files))
	}
	if len(files) > 0 && files[0] != "file2.go" {
		t.Errorf("expected file2.go, got %s", files[0])
	}
}

func TestEvent_Lifecycle(t *testing.T) {
	event := NewEvent(EventTypeContextShared, "source", "Source")

	t.Run("Initial state", func(t *testing.T) {
		if event.Status != EventStatusActive {
			t.Errorf("expected active status, got %s", event.Status)
		}
		if event.IsExpired() {
			t.Error("new event should not be expired")
		}
		if event.IsSuperseded() {
			t.Error("new event should not be superseded")
		}
	})

	t.Run("Mark completed", func(t *testing.T) {
		event.MarkCompleted()
		if event.Status != EventStatusCompleted {
			t.Errorf("expected completed status, got %s", event.Status)
		}
	})

	t.Run("Mark archived", func(t *testing.T) {
		event.MarkArchived()
		if event.Status != EventStatusArchived {
			t.Errorf("expected archived status, got %s", event.Status)
		}
	})

	t.Run("Superseded", func(t *testing.T) {
		event.SupersededBy = "new-event-id"
		if !event.IsSuperseded() {
			t.Error("event should be superseded")
		}
	})
}

func TestEventLog_FilterActive(t *testing.T) {
	log := NewEventLog(&EventLogConfig{MaxSize: 100})
	defer log.Stop()

	// Add mix of events
	active := NewEvent(EventTypeFileChange, "s", "S")
	archived := NewEvent(EventTypeFileChange, "s", "S")
	archived.MarkArchived()
	expired := NewEvent(EventTypeFileChange, "s", "S")
	expired.ExpiresAt = time.Now().Add(-1 * time.Hour)

	log.Append(active)
	log.Append(archived)
	log.Append(expired)

	// GetRecent should only return active
	recent := log.GetRecent(10)
	if len(recent) != 1 {
		t.Errorf("expected 1 active event, got %d", len(recent))
	}

	// TotalSize includes all
	if log.TotalSize() != 3 {
		t.Errorf("expected 3 total events, got %d", log.TotalSize())
	}
}
