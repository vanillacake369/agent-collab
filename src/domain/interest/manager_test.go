package interest

import (
	"testing"
	"time"
)

func TestNewManager(t *testing.T) {
	mgr := NewManager()
	if mgr == nil {
		t.Fatal("NewManager returned nil")
	}
	if mgr.Count() != 0 {
		t.Errorf("expected 0 interests, got %d", mgr.Count())
	}
}

func TestManager_Register(t *testing.T) {
	mgr := NewManager()

	interest := NewInterest("agent-1", "Claude", []string{"proj-a/**", "proj-b/src/*.go"})
	if err := mgr.Register(interest); err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	if mgr.Count() != 1 {
		t.Errorf("expected 1 interest, got %d", mgr.Count())
	}

	// Verify retrieval
	got, err := mgr.Get(interest.ID)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if got.AgentID != "agent-1" {
		t.Errorf("expected agent-1, got %s", got.AgentID)
	}
}

func TestManager_Register_Validation(t *testing.T) {
	mgr := NewManager()

	// Test nil interest
	if err := mgr.Register(nil); err != ErrNilInterest {
		t.Errorf("expected ErrNilInterest, got %v", err)
	}

	// Test empty agent ID
	interest := &Interest{Patterns: []string{"**"}}
	if err := mgr.Register(interest); err != ErrEmptyAgentID {
		t.Errorf("expected ErrEmptyAgentID, got %v", err)
	}

	// Test empty patterns
	interest = &Interest{AgentID: "agent-1"}
	if err := mgr.Register(interest); err != ErrEmptyPatterns {
		t.Errorf("expected ErrEmptyPatterns, got %v", err)
	}
}

func TestManager_Unregister(t *testing.T) {
	mgr := NewManager()

	interest := NewInterest("agent-1", "Claude", []string{"**"})
	mgr.Register(interest)

	if err := mgr.Unregister(interest.ID); err != nil {
		t.Fatalf("Unregister failed: %v", err)
	}

	if mgr.Count() != 0 {
		t.Errorf("expected 0 interests after unregister, got %d", mgr.Count())
	}
}

func TestManager_UnregisterAgent(t *testing.T) {
	mgr := NewManager()

	// Register multiple interests for same agent
	mgr.Register(NewInterest("agent-1", "Claude", []string{"proj-a/**"}))
	mgr.Register(NewInterest("agent-1", "Claude", []string{"proj-b/**"}))
	mgr.Register(NewInterest("agent-2", "Gemini", []string{"proj-c/**"}))

	if mgr.Count() != 3 {
		t.Fatalf("expected 3 interests, got %d", mgr.Count())
	}

	// Unregister agent-1
	if err := mgr.UnregisterAgent("agent-1"); err != nil {
		t.Fatalf("UnregisterAgent failed: %v", err)
	}

	if mgr.Count() != 1 {
		t.Errorf("expected 1 interest after agent unregister, got %d", mgr.Count())
	}

	// Verify agent-2's interest remains
	interests := mgr.GetAgentInterests("agent-2")
	if len(interests) != 1 {
		t.Errorf("expected 1 interest for agent-2, got %d", len(interests))
	}
}

func TestManager_Match_Direct(t *testing.T) {
	mgr := NewManager()

	mgr.Register(NewInterest("agent-1", "Claude", []string{"proj-a/*.go"}))
	mgr.Register(NewInterest("agent-2", "Gemini", []string{"proj-b/**"}))

	// Test direct match
	matches := mgr.Match("proj-a/main.go")
	if len(matches) != 1 {
		t.Fatalf("expected 1 match, got %d", len(matches))
	}
	if matches[0].Interest.AgentID != "agent-1" {
		t.Errorf("expected agent-1, got %s", matches[0].Interest.AgentID)
	}
	if matches[0].MatchType != MatchTypeDirect {
		t.Errorf("expected direct match, got %s", matches[0].MatchType)
	}
}

func TestManager_Match_NoMatch(t *testing.T) {
	mgr := NewManager()

	mgr.Register(NewInterest("agent-1", "Claude", []string{"proj-a/**"}))

	matches := mgr.Match("proj-c/file.go")
	if len(matches) != 0 {
		t.Errorf("expected 0 matches, got %d", len(matches))
	}
}

func TestManager_CleanupExpired(t *testing.T) {
	mgr := NewManager()

	// Create expired interest
	expired := NewInterest("agent-1", "Claude", []string{"**"})
	expired.ExpiresAt = time.Now().Add(-time.Hour)
	mgr.Register(expired)

	// Create valid interest
	valid := NewInterest("agent-2", "Gemini", []string{"**"})
	mgr.Register(valid)

	if mgr.Count() != 2 {
		t.Fatalf("expected 2 interests, got %d", mgr.Count())
	}

	count := mgr.CleanupExpired()
	if count != 1 {
		t.Errorf("expected 1 cleaned up, got %d", count)
	}

	if mgr.Count() != 1 {
		t.Errorf("expected 1 interest after cleanup, got %d", mgr.Count())
	}
}

func TestInterest_IsExpired(t *testing.T) {
	interest := NewInterest("agent-1", "Claude", []string{"**"})

	if interest.IsExpired() {
		t.Error("new interest should not be expired")
	}

	interest.ExpiresAt = time.Now().Add(-time.Hour)
	if !interest.IsExpired() {
		t.Error("interest with past expiry should be expired")
	}
}

func TestInterest_SetTTL(t *testing.T) {
	interest := NewInterest("agent-1", "Claude", []string{"**"})

	interest.SetTTL(time.Minute)

	if interest.IsExpired() {
		t.Error("interest should not be expired after SetTTL")
	}

	// Should expire in roughly a minute
	if time.Until(interest.ExpiresAt) < 50*time.Second {
		t.Error("TTL not set correctly")
	}
}

func TestInterestMatch_Relevance(t *testing.T) {
	interest := NewInterest("agent-1", "Claude", []string{"**"})

	direct := NewInterestMatch(interest, MatchTypeDirect, "file.go")
	if direct.Relevance != 1.0 {
		t.Errorf("direct match should have relevance 1.0, got %f", direct.Relevance)
	}

	dep := NewInterestMatch(interest, MatchTypeDependency, "file.go")
	if dep.Relevance != 0.8 {
		t.Errorf("dependency match should have relevance 0.8, got %f", dep.Relevance)
	}

	prox := NewInterestMatch(interest, MatchTypeProximity, "file.go")
	if prox.Relevance != 0.5 {
		t.Errorf("proximity match should have relevance 0.5, got %f", prox.Relevance)
	}
}
