package interest_test

import (
	"testing"
	"time"

	"agent-collab/src/domain/interest"
)

// Feature: Interest Registration
// As an AI agent
// I want to register my areas of interest
// So that I only receive relevant events

func TestFeature_InterestRegistration(t *testing.T) {
	t.Run("Scenario: Agent registers interest with glob patterns", func(t *testing.T) {
		// Given a new interest manager
		mgr := interest.NewManager()

		// When an agent registers interest for specific patterns
		int1 := interest.NewInterest("claude-agent-1", "Claude", []string{
			"proj-a/**",
			"proj-b/src/*.go",
		})
		err := mgr.Register(int1)

		// Then the registration should succeed
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		// And the interest should be retrievable
		retrieved, err := mgr.Get(int1.ID)
		if err != nil {
			t.Fatalf("Expected to retrieve interest, got error: %v", err)
		}
		if retrieved.AgentID != "claude-agent-1" {
			t.Errorf("Expected agent ID 'claude-agent-1', got '%s'", retrieved.AgentID)
		}
	})

	t.Run("Scenario: Agent registers multiple interests", func(t *testing.T) {
		// Given a new interest manager
		mgr := interest.NewManager()

		// When an agent registers multiple interests
		int1 := interest.NewInterest("agent-1", "Claude", []string{"proj-a/**"})
		int2 := interest.NewInterest("agent-1", "Claude", []string{"proj-b/**"})
		mgr.Register(int1)
		mgr.Register(int2)

		// Then both interests should be registered
		interests := mgr.GetAgentInterests("agent-1")
		if len(interests) != 2 {
			t.Errorf("Expected 2 interests, got %d", len(interests))
		}
	})

	t.Run("Scenario: Registration fails with empty patterns", func(t *testing.T) {
		// Given a new interest manager
		mgr := interest.NewManager()

		// When an agent tries to register with no patterns
		int1 := &interest.Interest{
			AgentID:   "agent-1",
			AgentName: "Claude",
			Patterns:  []string{},
		}
		err := mgr.Register(int1)

		// Then registration should fail with ErrEmptyPatterns
		if err != interest.ErrEmptyPatterns {
			t.Errorf("Expected ErrEmptyPatterns, got %v", err)
		}
	})
}

// Feature: Interest Matching
// As the event router
// I want to match events against registered interests
// So that I can route events to the right agents

func TestFeature_InterestMatching(t *testing.T) {
	t.Run("Scenario: Direct pattern match", func(t *testing.T) {
		// Given an interest manager with a registered pattern
		mgr := interest.NewManager()
		int1 := interest.NewInterest("agent-1", "Claude", []string{"proj-a/*.go"})
		mgr.Register(int1)

		// When a file path matches the pattern
		matches := mgr.Match("proj-a/main.go")

		// Then the interest should be matched
		if len(matches) != 1 {
			t.Fatalf("Expected 1 match, got %d", len(matches))
		}
		if matches[0].MatchType != interest.MatchTypeDirect {
			t.Errorf("Expected direct match, got %s", matches[0].MatchType)
		}
		if matches[0].Interest.AgentID != "agent-1" {
			t.Errorf("Expected agent-1, got %s", matches[0].Interest.AgentID)
		}
	})

	t.Run("Scenario: Recursive glob pattern (**)", func(t *testing.T) {
		// Given an interest manager with recursive pattern
		mgr := interest.NewManager()
		int1 := interest.NewInterest("agent-1", "Claude", []string{"proj-a/**"})
		mgr.Register(int1)

		// When a deeply nested file path is checked
		matches := mgr.Match("proj-a/src/internal/handler.go")

		// Then the interest should be matched
		if len(matches) != 1 {
			t.Fatalf("Expected 1 match for recursive pattern, got %d", len(matches))
		}
	})

	t.Run("Scenario: No match for unrelated path", func(t *testing.T) {
		// Given an interest manager with a specific pattern
		mgr := interest.NewManager()
		int1 := interest.NewInterest("agent-1", "Claude", []string{"proj-a/**"})
		mgr.Register(int1)

		// When a non-matching file path is checked
		matches := mgr.Match("proj-b/file.go")

		// Then no interests should be matched
		if len(matches) != 0 {
			t.Errorf("Expected 0 matches, got %d", len(matches))
		}
	})

	t.Run("Scenario: Multiple agents interested in same file", func(t *testing.T) {
		// Given multiple agents with overlapping interests
		mgr := interest.NewManager()
		int1 := interest.NewInterest("agent-1", "Claude", []string{"shared/**"})
		int2 := interest.NewInterest("agent-2", "Gemini", []string{"shared/utils/*"})
		mgr.Register(int1)
		mgr.Register(int2)

		// When a file in the shared area is changed
		matches := mgr.Match("shared/utils/helper.go")

		// Then both agents should be matched
		if len(matches) != 2 {
			t.Errorf("Expected 2 matches, got %d", len(matches))
		}
	})

	t.Run("Scenario: Expired interests are not matched", func(t *testing.T) {
		// Given an interest that has expired
		mgr := interest.NewManager()
		int1 := interest.NewInterest("agent-1", "Claude", []string{"proj-a/**"})
		int1.ExpiresAt = time.Now().Add(-time.Hour) // Already expired
		mgr.Register(int1)

		// When checking for matches
		matches := mgr.Match("proj-a/file.go")

		// Then expired interest should not match
		if len(matches) != 0 {
			t.Errorf("Expected 0 matches for expired interest, got %d", len(matches))
		}
	})
}

// Feature: Interest Unregistration
// As an AI agent
// I want to unregister my interests
// So that I stop receiving events when I'm done working

func TestFeature_InterestUnregistration(t *testing.T) {
	t.Run("Scenario: Agent unregisters specific interest", func(t *testing.T) {
		// Given an agent with registered interests
		mgr := interest.NewManager()
		int1 := interest.NewInterest("agent-1", "Claude", []string{"proj-a/**"})
		int2 := interest.NewInterest("agent-1", "Claude", []string{"proj-b/**"})
		mgr.Register(int1)
		mgr.Register(int2)

		// When the agent unregisters one interest
		err := mgr.Unregister(int1.ID)

		// Then the unregistration should succeed
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		// And only the other interest should remain
		interests := mgr.GetAgentInterests("agent-1")
		if len(interests) != 1 {
			t.Errorf("Expected 1 interest remaining, got %d", len(interests))
		}
	})

	t.Run("Scenario: Agent leaves and all interests are removed", func(t *testing.T) {
		// Given an agent with multiple registered interests
		mgr := interest.NewManager()
		mgr.Register(interest.NewInterest("agent-1", "Claude", []string{"proj-a/**"}))
		mgr.Register(interest.NewInterest("agent-1", "Claude", []string{"proj-b/**"}))
		mgr.Register(interest.NewInterest("agent-2", "Gemini", []string{"proj-c/**"}))

		// When the agent leaves (unregisters all)
		err := mgr.UnregisterAgent("agent-1")

		// Then all agent's interests should be removed
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}
		interests := mgr.GetAgentInterests("agent-1")
		if len(interests) != 0 {
			t.Errorf("Expected 0 interests for agent-1, got %d", len(interests))
		}

		// And other agents' interests should remain
		interests = mgr.GetAgentInterests("agent-2")
		if len(interests) != 1 {
			t.Errorf("Expected 1 interest for agent-2, got %d", len(interests))
		}
	})

	t.Run("Scenario: Cleanup removes expired interests", func(t *testing.T) {
		// Given a mix of expired and valid interests
		mgr := interest.NewManager()

		expired := interest.NewInterest("agent-1", "Claude", []string{"old/**"})
		expired.ExpiresAt = time.Now().Add(-time.Hour)
		mgr.Register(expired)

		valid := interest.NewInterest("agent-2", "Gemini", []string{"new/**"})
		mgr.Register(valid)

		// When cleanup is triggered
		removed := mgr.CleanupExpired()

		// Then only expired interests should be removed
		if removed != 1 {
			t.Errorf("Expected 1 removed, got %d", removed)
		}
		if mgr.Count() != 1 {
			t.Errorf("Expected 1 remaining, got %d", mgr.Count())
		}
	})
}
