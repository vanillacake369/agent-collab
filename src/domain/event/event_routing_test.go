package event_test

import (
	"context"
	"testing"
	"time"

	"agent-collab/src/domain/event"
	"agent-collab/src/domain/interest"
)

// BDD Test: Event Routing Based on Interest Patterns
// Given: 여러 에이전트가 각자의 Interest 패턴을 등록한 경우
// When: 특정 파일에 대한 이벤트가 발행되면
// Then: 해당 패턴과 일치하는 에이전트에게만 이벤트가 라우팅되어야 함

func TestEventRouter_PublishToMatchingAgents(t *testing.T) {
	t.Run("Given agents with different interest patterns", func(t *testing.T) {
		interestMgr := interest.NewManager()

		// Alice: auth-lib/** (Direct matching only - no proximity)
		aliceInterest := interest.NewInterest("alice", "Alice", []string{"auth-lib/**"})
		aliceInterest.Level = interest.InterestLevelDirect
		_ = interestMgr.Register(aliceInterest)

		// Bob: user-service/**, auth-lib/token.go (Direct matching only)
		bobInterest := interest.NewInterest("bob", "Bob", []string{
			"user-service/**",
			"auth-lib/token.go",
		})
		bobInterest.Level = interest.InterestLevelDirect
		_ = interestMgr.Register(bobInterest)

		// Charlie: api-gateway/**, auth-lib/jwt.go, user-service/api/* (Direct matching only)
		charlieInterest := interest.NewInterest("charlie", "Charlie", []string{
			"api-gateway/**",
			"auth-lib/jwt.go",
			"user-service/api/*",
		})
		charlieInterest.Level = interest.InterestLevelDirect
		_ = interestMgr.Register(charlieInterest)

		router := event.NewRouter(interestMgr, &event.RouterConfig{
			NodeID:   "test-node",
			NodeName: "TestNode",
		})

		// Subscribe agents
		aliceChan := router.Subscribe("alice")
		bobChan := router.Subscribe("bob")
		charlieChan := router.Subscribe("charlie")

		ctx := context.Background()

		t.Run("When auth-lib/jwt.go event is published", func(t *testing.T) {
			evt := event.NewContextSharedEvent(
				"alice", "Alice",
				"auth-lib/jwt.go",
				&event.ContextSharedPayload{Content: "JWT validation updated"},
			)

			err := router.PublishLocal(ctx, evt)
			if err != nil {
				t.Fatalf("failed to publish: %v", err)
			}

			t.Run("Then Alice receives (auth-lib/**)", func(t *testing.T) {
				assertReceivesEvent(t, aliceChan, evt.ID)
			})

			t.Run("Then Charlie receives (auth-lib/jwt.go)", func(t *testing.T) {
				assertReceivesEvent(t, charlieChan, evt.ID)
			})

			t.Run("Then Bob does NOT receive (no jwt.go interest)", func(t *testing.T) {
				assertNoEvent(t, bobChan)
			})
		})

		// Clear channels
		drainChannel(aliceChan)
		drainChannel(bobChan)
		drainChannel(charlieChan)

		t.Run("When auth-lib/token.go event is published", func(t *testing.T) {
			evt := event.NewContextSharedEvent(
				"alice", "Alice",
				"auth-lib/token.go",
				&event.ContextSharedPayload{Content: "TokenClaims updated"},
			)

			_ = router.PublishLocal(ctx, evt)

			t.Run("Then Alice receives (auth-lib/**)", func(t *testing.T) {
				assertReceivesEvent(t, aliceChan, evt.ID)
			})

			t.Run("Then Bob receives (auth-lib/token.go)", func(t *testing.T) {
				assertReceivesEvent(t, bobChan, evt.ID)
			})

			t.Run("Then Charlie does NOT receive (no token.go interest)", func(t *testing.T) {
				assertNoEvent(t, charlieChan)
			})
		})

		// Clear channels
		drainChannel(aliceChan)
		drainChannel(bobChan)
		drainChannel(charlieChan)

		t.Run("When user-service/api/handler.go event is published", func(t *testing.T) {
			evt := event.NewContextSharedEvent(
				"bob", "Bob",
				"user-service/api/handler.go",
				&event.ContextSharedPayload{Content: "New endpoint added"},
			)

			_ = router.PublishLocal(ctx, evt)

			t.Run("Then Bob receives (user-service/**)", func(t *testing.T) {
				assertReceivesEvent(t, bobChan, evt.ID)
			})

			t.Run("Then Charlie receives (user-service/api/*)", func(t *testing.T) {
				assertReceivesEvent(t, charlieChan, evt.ID)
			})

			t.Run("Then Alice does NOT receive (no user-service interest)", func(t *testing.T) {
				assertNoEvent(t, aliceChan)
			})
		})
	})
}

func TestEventRouter_GetEventsFilteredByInterest(t *testing.T) {
	t.Run("Given router with events from multiple sources", func(t *testing.T) {
		interestMgr := interest.NewManager()

		// Bob: user-service/**, auth-lib/token.go (Direct matching only)
		bobInterest := interest.NewInterest("bob", "Bob", []string{
			"user-service/**",
			"auth-lib/token.go",
		})
		bobInterest.Level = interest.InterestLevelDirect
		_ = interestMgr.Register(bobInterest)

		router := event.NewRouter(interestMgr, nil)
		ctx := context.Background()

		// Publish various events
		events := []*event.Event{
			event.NewContextSharedEvent("alice", "Alice", "auth-lib/jwt.go", &event.ContextSharedPayload{Content: "jwt"}),
			event.NewContextSharedEvent("alice", "Alice", "auth-lib/token.go", &event.ContextSharedPayload{Content: "token"}),
			event.NewContextSharedEvent("bob", "Bob", "user-service/main.go", &event.ContextSharedPayload{Content: "main"}),
			event.NewContextSharedEvent("charlie", "Charlie", "api-gateway/router.go", &event.ContextSharedPayload{Content: "router"}),
		}

		for _, evt := range events {
			_ = router.PublishLocal(ctx, evt)
		}

		t.Run("When Bob queries events", func(t *testing.T) {
			bobEvents := router.GetEvents("bob", &event.EventFilter{Limit: 100})

			t.Run("Then only matching events are returned", func(t *testing.T) {
				// Bob should see: auth-lib/token.go, user-service/main.go
				// Bob should NOT see: auth-lib/jwt.go, api-gateway/router.go

				if len(bobEvents) != 2 {
					t.Fatalf("expected 2 events for Bob, got %d", len(bobEvents))
				}

				paths := make(map[string]bool)
				for _, e := range bobEvents {
					paths[e.FilePath] = true
				}

				if !paths["auth-lib/token.go"] {
					t.Error("Bob should see auth-lib/token.go")
				}
				if !paths["user-service/main.go"] {
					t.Error("Bob should see user-service/main.go")
				}
				if paths["auth-lib/jwt.go"] {
					t.Error("Bob should NOT see auth-lib/jwt.go")
				}
				if paths["api-gateway/router.go"] {
					t.Error("Bob should NOT see api-gateway/router.go")
				}
			})
		})

		t.Run("When querying with IncludeAll filter", func(t *testing.T) {
			allEvents := router.GetEvents("bob", &event.EventFilter{
				Limit:      100,
				IncludeAll: true,
			})

			t.Run("Then all events are returned", func(t *testing.T) {
				if len(allEvents) != 4 {
					t.Errorf("expected 4 events with IncludeAll, got %d", len(allEvents))
				}
			})
		})
	})
}

func TestEventRouter_HandleRemoteEvent(t *testing.T) {
	t.Run("Given router with subscribed agents", func(t *testing.T) {
		interestMgr := interest.NewManager()

		aliceInterest := interest.NewInterest("alice", "Alice", []string{"auth-lib/**"})
		_ = interestMgr.Register(aliceInterest)

		router := event.NewRouter(interestMgr, nil)
		aliceChan := router.Subscribe("alice")
		ctx := context.Background()

		t.Run("When remote event is received via P2P", func(t *testing.T) {
			// Simulate remote event JSON
			remoteEventJSON := []byte(`{
				"id": "remote-evt-1",
				"type": "context_shared",
				"timestamp": "2026-01-01T00:00:00Z",
				"source_id": "bob",
				"source_name": "Bob",
				"file_path": "auth-lib/jwt.go",
				"payload": {"content": "Remote update"}
			}`)

			err := router.HandleRemoteEvent(ctx, remoteEventJSON)
			if err != nil {
				t.Fatalf("failed to handle remote event: %v", err)
			}

			t.Run("Then event is routed to matching local agents", func(t *testing.T) {
				assertReceivesEvent(t, aliceChan, "remote-evt-1")
			})
		})
	})
}

func TestEventRouter_EventsWithoutFilePath(t *testing.T) {
	t.Run("Given events without file path (system events)", func(t *testing.T) {
		interestMgr := interest.NewManager()

		aliceInterest := interest.NewInterest("alice", "Alice", []string{"auth-lib/**"})
		_ = interestMgr.Register(aliceInterest)

		router := event.NewRouter(interestMgr, nil)
		aliceChan := router.Subscribe("alice")
		ctx := context.Background()

		t.Run("When agent joined event is published", func(t *testing.T) {
			evt := event.NewAgentJoinedEvent("bob", "Bob", &event.AgentPayload{
				AgentID:   "bob",
				AgentName: "Bob",
			})

			_ = router.PublishLocal(ctx, evt)

			t.Run("Then all subscribers receive it (no file path filter)", func(t *testing.T) {
				assertReceivesEvent(t, aliceChan, evt.ID)
			})
		})
	})
}

func TestEventRouter_InterestLevelFiltering(t *testing.T) {
	t.Run("Given agent with InterestLevelLocksOnly", func(t *testing.T) {
		interestMgr := interest.NewManager()

		aliceInterest := interest.NewInterest("alice", "Alice", []string{"auth-lib/**"})
		aliceInterest.Level = interest.InterestLevelLocksOnly
		_ = interestMgr.Register(aliceInterest)

		router := event.NewRouter(interestMgr, nil)
		aliceChan := router.Subscribe("alice")
		ctx := context.Background()

		t.Run("When context_shared event is published", func(t *testing.T) {
			evt := event.NewContextSharedEvent(
				"bob", "Bob",
				"auth-lib/jwt.go",
				&event.ContextSharedPayload{Content: "update"},
			)

			_ = router.PublishLocal(ctx, evt)

			t.Run("Then agent does NOT receive (locks only)", func(t *testing.T) {
				assertNoEvent(t, aliceChan)
			})
		})

		t.Run("When lock_acquired event is published", func(t *testing.T) {
			evt := event.NewLockAcquiredEvent(
				"bob", "Bob",
				"auth-lib/jwt.go", 10, 50,
				&event.LockPayload{LockID: "lock-1", HolderID: "bob"},
			)

			_ = router.PublishLocal(ctx, evt)

			t.Run("Then agent receives lock event", func(t *testing.T) {
				assertReceivesEvent(t, aliceChan, evt.ID)
			})
		})
	})
}

// Helper functions

func assertReceivesEvent(t *testing.T, ch <-chan *event.Event, expectedID string) {
	t.Helper()
	select {
	case evt := <-ch:
		if evt.ID != expectedID {
			t.Errorf("expected event %s, got %s", expectedID, evt.ID)
		}
	case <-time.After(100 * time.Millisecond):
		t.Errorf("expected to receive event %s but got nothing", expectedID)
	}
}

func assertNoEvent(t *testing.T, ch <-chan *event.Event) {
	t.Helper()
	select {
	case evt := <-ch:
		t.Errorf("expected no event but received %s (file: %s)", evt.ID, evt.FilePath)
	case <-time.After(50 * time.Millisecond):
		// Good, no event
	}
}

func drainChannel(ch <-chan *event.Event) {
	for {
		select {
		case <-ch:
		case <-time.After(10 * time.Millisecond):
			return
		}
	}
}
