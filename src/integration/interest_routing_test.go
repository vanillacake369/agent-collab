package integration

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"agent-collab/src/application"
	"agent-collab/src/domain/event"
	"agent-collab/src/domain/interest"
)

// Integration Test: Interest-based Event Routing
// 실제 daemon server를 시작하고 HTTP API를 통해 Interest 등록 및 이벤트 라우팅을 검증합니다.

func TestInterestRegistrationFromEnvironment(t *testing.T) {
	// Given: AGENT_COLLAB_INTERESTS 환경변수가 설정된 상태
	originalEnv := os.Getenv("AGENT_COLLAB_INTERESTS")
	originalName := os.Getenv("AGENT_NAME")
	defer func() {
		os.Setenv("AGENT_COLLAB_INTERESTS", originalEnv)
		os.Setenv("AGENT_NAME", originalName)
	}()

	os.Setenv("AGENT_COLLAB_INTERESTS", "auth-lib/**,user-service/api/*")
	os.Setenv("AGENT_NAME", "TestAgent")

	// When: Application이 초기화되면
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
	_, err = app.Initialize(ctx, "test-project")
	if err != nil {
		t.Fatalf("Failed to initialize: %v", err)
	}

	// Then: InterestManager에 Interest가 등록되어야 함
	t.Run("Then InterestManager should have registered interests", func(t *testing.T) {
		interestMgr := app.InterestManager()
		if interestMgr == nil {
			t.Fatal("InterestManager should not be nil")
		}

		// Get node ID to check interests
		node := app.Node()
		if node == nil {
			t.Fatal("Node should not be nil")
		}
		nodeID := node.ID().String()

		interests := interestMgr.GetAgentInterests(nodeID)
		if len(interests) == 0 {
			t.Fatal("Should have registered interests from environment")
		}

		// Verify patterns
		found := interests[0]
		if len(found.Patterns) != 2 {
			t.Errorf("Expected 2 patterns, got %d", len(found.Patterns))
		}

		expectedPatterns := map[string]bool{
			"auth-lib/**":        true,
			"user-service/api/*": true,
		}
		for _, p := range found.Patterns {
			if !expectedPatterns[p] {
				t.Errorf("Unexpected pattern: %s", p)
			}
		}
	})

	// Then: 패턴 매칭이 동작해야 함
	t.Run("Then pattern matching should work", func(t *testing.T) {
		interestMgr := app.InterestManager()

		// auth-lib/** 패턴 테스트
		matches := interestMgr.Match("auth-lib/jwt.go")
		if len(matches) == 0 {
			t.Error("Should match auth-lib/jwt.go")
		}

		// user-service/api/* 패턴 테스트
		matches = interestMgr.Match("user-service/api/handler.go")
		if len(matches) == 0 {
			t.Error("Should match user-service/api/handler.go")
		}

		// 매칭되지 않아야 하는 패턴
		matches = interestMgr.Match("user-service/db/repository.go")
		if len(matches) > 0 {
			t.Error("Should NOT match user-service/db/repository.go")
		}
	})
}

func TestEventRouterIntegration(t *testing.T) {
	// Given: 여러 에이전트가 각자의 Interest 패턴을 등록한 상태
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
	result, err := app.Initialize(ctx, "test-project")
	if err != nil {
		t.Fatalf("Failed to initialize: %v", err)
	}

	// Interest Manager와 Event Router 가져오기
	interestMgr := app.InterestManager()
	eventRouter := app.EventRouter()

	if interestMgr == nil || eventRouter == nil {
		t.Fatal("InterestManager or EventRouter is nil")
	}

	// Alice: auth-lib/** (Direct matching only)
	aliceInterest := interest.NewInterest("alice", "Alice", []string{"auth-lib/**"})
	aliceInterest.Level = interest.InterestLevelDirect
	_ = interestMgr.Register(aliceInterest)

	// Bob: user-service/**, auth-lib/token.go
	bobInterest := interest.NewInterest("bob", "Bob", []string{
		"user-service/**",
		"auth-lib/token.go",
	})
	bobInterest.Level = interest.InterestLevelDirect
	_ = interestMgr.Register(bobInterest)

	// Subscribe agents
	aliceChan := eventRouter.Subscribe("alice")
	bobChan := eventRouter.Subscribe("bob")

	t.Run("When context_shared event is published for auth-lib/jwt.go", func(t *testing.T) {
		evt := event.NewContextSharedEvent(
			result.NodeID, "TestNode",
			"auth-lib/jwt.go",
			&event.ContextSharedPayload{Content: "JWT validation updated"},
		)

		err := eventRouter.PublishLocal(ctx, evt)
		if err != nil {
			t.Fatalf("Failed to publish: %v", err)
		}

		t.Run("Then Alice receives (auth-lib/**)", func(t *testing.T) {
			select {
			case received := <-aliceChan:
				if received.ID != evt.ID {
					t.Errorf("Expected event %s, got %s", evt.ID, received.ID)
				}
			case <-time.After(100 * time.Millisecond):
				t.Error("Alice should have received the event")
			}
		})

		t.Run("Then Bob does NOT receive (no jwt.go interest)", func(t *testing.T) {
			select {
			case received := <-bobChan:
				t.Errorf("Bob should NOT receive event, but got %s", received.ID)
			case <-time.After(50 * time.Millisecond):
				// Good, no event
			}
		})
	})

	// Drain channels
	drainEventChannel(aliceChan)
	drainEventChannel(bobChan)

	t.Run("When context_shared event is published for auth-lib/token.go", func(t *testing.T) {
		evt := event.NewContextSharedEvent(
			result.NodeID, "TestNode",
			"auth-lib/token.go",
			&event.ContextSharedPayload{Content: "TokenClaims updated"},
		)

		_ = eventRouter.PublishLocal(ctx, evt)

		t.Run("Then Alice receives (auth-lib/**)", func(t *testing.T) {
			select {
			case received := <-aliceChan:
				if received.ID != evt.ID {
					t.Errorf("Expected event %s, got %s", evt.ID, received.ID)
				}
			case <-time.After(100 * time.Millisecond):
				t.Error("Alice should have received the event")
			}
		})

		t.Run("Then Bob receives (auth-lib/token.go)", func(t *testing.T) {
			select {
			case received := <-bobChan:
				if received.ID != evt.ID {
					t.Errorf("Expected event %s, got %s", evt.ID, received.ID)
				}
			case <-time.After(100 * time.Millisecond):
				t.Error("Bob should have received the event")
			}
		})
	})
}

func TestEventRouterPublishAndGetEvents(t *testing.T) {
	// Given: Application이 초기화되고 Interest가 등록된 상태
	originalEnv := os.Getenv("AGENT_COLLAB_INTERESTS")
	originalName := os.Getenv("AGENT_NAME")
	defer func() {
		os.Setenv("AGENT_COLLAB_INTERESTS", originalEnv)
		os.Setenv("AGENT_NAME", originalName)
	}()

	os.Setenv("AGENT_COLLAB_INTERESTS", "auth-lib/**,user-service/**")
	os.Setenv("AGENT_NAME", "TestDaemonAgent")

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
	result, err := app.Initialize(ctx, "daemon-test-project")
	if err != nil {
		t.Fatalf("Failed to initialize: %v", err)
	}

	eventRouter := app.EventRouter()
	if eventRouter == nil {
		t.Fatal("EventRouter should not be nil")
	}

	nodeID := result.NodeID

	t.Run("When context_shared event is published", func(t *testing.T) {
		// Publish event for auth-lib/jwt.go
		evt := event.NewContextSharedEvent(
			nodeID, "TestNode",
			"auth-lib/jwt.go",
			&event.ContextSharedPayload{Content: "JWT validation logic updated"},
		)
		err := eventRouter.Publish(ctx, evt)
		if err != nil {
			t.Fatalf("Publish failed: %v", err)
		}

		t.Run("Then GetEvents with IncludeAll returns the event", func(t *testing.T) {
			events := eventRouter.GetEvents(nodeID, &event.EventFilter{
				Limit:      10,
				IncludeAll: true,
			})

			if len(events) == 0 {
				t.Error("Should have at least 1 event after publish")
			}

			found := false
			for _, e := range events {
				if e.ID == evt.ID {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("Published event %s should be in GetEvents result", evt.ID)
			}
		})

		t.Run("Then GetEvents with interest filtering returns matching events", func(t *testing.T) {
			// The agent registered interest for auth-lib/**, so auth-lib/jwt.go should be visible
			events := eventRouter.GetEvents(nodeID, &event.EventFilter{
				Limit: 10,
			})

			if len(events) == 0 {
				t.Error("Agent with auth-lib/** interest should see auth-lib/jwt.go event")
			}

			// Verify the event is for auth-lib
			for _, e := range events {
				if e.FilePath != "" && !strings.HasPrefix(e.FilePath, "auth-lib/") && !strings.HasPrefix(e.FilePath, "user-service/") {
					t.Errorf("Event with file_path %s should not be visible to agent", e.FilePath)
				}
			}
		})
	})

	t.Run("When event is published for non-matching path", func(t *testing.T) {
		// Publish event for api-gateway (not in interest)
		evt := event.NewContextSharedEvent(
			nodeID, "TestNode",
			"api-gateway/router.go",
			&event.ContextSharedPayload{Content: "Router updated"},
		)
		_ = eventRouter.Publish(ctx, evt)

		t.Run("Then GetEvents without IncludeAll does NOT return it", func(t *testing.T) {
			events := eventRouter.GetEvents(nodeID, &event.EventFilter{
				Limit: 100,
			})

			for _, e := range events {
				if e.FilePath == "api-gateway/router.go" {
					t.Error("Agent without api-gateway interest should NOT see api-gateway/router.go")
				}
			}
		})

		t.Run("Then GetEvents with IncludeAll DOES return it", func(t *testing.T) {
			events := eventRouter.GetEvents(nodeID, &event.EventFilter{
				Limit:      100,
				IncludeAll: true,
			})

			found := false
			for _, e := range events {
				if e.FilePath == "api-gateway/router.go" {
					found = true
					break
				}
			}
			if !found {
				t.Error("With IncludeAll, api-gateway/router.go should be visible")
			}
		})
	})
}

func TestInterestLevelFiltering(t *testing.T) {
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
	result, err := app.Initialize(ctx, "level-test")
	if err != nil {
		t.Fatalf("Failed to initialize: %v", err)
	}

	interestMgr := app.InterestManager()
	eventRouter := app.EventRouter()

	// Agent with InterestLevelLocksOnly
	locksOnlyInterest := interest.NewInterest("locks-only-agent", "LocksOnlyAgent", []string{"auth-lib/**"})
	locksOnlyInterest.Level = interest.InterestLevelLocksOnly
	_ = interestMgr.Register(locksOnlyInterest)

	locksChan := eventRouter.Subscribe("locks-only-agent")

	t.Run("When context_shared event is published", func(t *testing.T) {
		evt := event.NewContextSharedEvent(
			result.NodeID, "TestNode",
			"auth-lib/jwt.go",
			&event.ContextSharedPayload{Content: "update"},
		)
		_ = eventRouter.PublishLocal(ctx, evt)

		t.Run("Then agent with InterestLevelLocksOnly does NOT receive", func(t *testing.T) {
			select {
			case <-locksChan:
				t.Error("Agent with InterestLevelLocksOnly should NOT receive context_shared")
			case <-time.After(50 * time.Millisecond):
				// Good
			}
		})
	})

	t.Run("When lock_acquired event is published", func(t *testing.T) {
		evt := event.NewLockAcquiredEvent(
			result.NodeID, "TestNode",
			"auth-lib/jwt.go", 10, 50,
			&event.LockPayload{LockID: "lock-1", HolderID: "test"},
		)
		_ = eventRouter.PublishLocal(ctx, evt)

		t.Run("Then agent with InterestLevelLocksOnly DOES receive", func(t *testing.T) {
			select {
			case received := <-locksChan:
				if received.ID != evt.ID {
					t.Errorf("Expected event %s, got %s", evt.ID, received.ID)
				}
			case <-time.After(100 * time.Millisecond):
				t.Error("Agent should receive lock_acquired event")
			}
		})
	})
}

func TestGetEventsWithNoInterests(t *testing.T) {
	// Given: Interest가 등록되지 않은 에이전트
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
	result, err := app.Initialize(ctx, "no-interest-test")
	if err != nil {
		t.Fatalf("Failed to initialize: %v", err)
	}

	eventRouter := app.EventRouter()

	// Publish some events
	for i := 0; i < 3; i++ {
		evt := event.NewContextSharedEvent(
			result.NodeID, "TestNode",
			"some-file.go",
			&event.ContextSharedPayload{Content: "content"},
		)
		_ = eventRouter.PublishLocal(ctx, evt)
	}

	t.Run("When GetEvents is called for agent without interests", func(t *testing.T) {
		events := eventRouter.GetEvents("non-existent-agent", &event.EventFilter{
			Limit: 10,
		})

		t.Run("Then returns empty (no interests = no events)", func(t *testing.T) {
			// filterByInterests returns nil when no interests
			if len(events) != 0 {
				t.Errorf("Expected 0 events for agent without interests, got %d", len(events))
			}
		})
	})

	t.Run("When GetEvents is called with IncludeAll=true", func(t *testing.T) {
		events := eventRouter.GetEvents("non-existent-agent", &event.EventFilter{
			Limit:      10,
			IncludeAll: true,
		})

		t.Run("Then returns all events", func(t *testing.T) {
			if len(events) != 3 {
				t.Errorf("Expected 3 events with IncludeAll, got %d", len(events))
			}
		})
	})
}

// Helper function
func drainEventChannel(ch <-chan *event.Event) {
	for {
		select {
		case <-ch:
		case <-time.After(10 * time.Millisecond):
			return
		}
	}
}
