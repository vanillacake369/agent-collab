package interest_test

import (
	"encoding/json"
	"testing"

	"agent-collab/src/domain/interest"
)

// Feature: Interest Synchronization
// As the agent-collab system
// I want to synchronize interests across nodes
// So that routing works correctly across the cluster

func TestFeature_InterestSynchronization(t *testing.T) {
	t.Run("Scenario: Interest can be serialized for P2P transmission", func(t *testing.T) {
		// Given an interest registration
		int1 := interest.NewInterest("agent-1", "Claude", []string{
			"proj-a/**",
			"shared/*.go",
		})
		int1.TrackDependencies = true
		int1.Level = interest.InterestLevelDirect

		// When serializing for P2P transmission
		data, err := json.Marshal(int1)

		// Then serialization should succeed
		if err != nil {
			t.Fatalf("Failed to serialize interest: %v", err)
		}

		// And deserialization should restore the interest
		var restored interest.Interest
		if err := json.Unmarshal(data, &restored); err != nil {
			t.Fatalf("Failed to deserialize interest: %v", err)
		}

		if restored.AgentID != "agent-1" {
			t.Errorf("Expected agent-1, got %s", restored.AgentID)
		}
		if len(restored.Patterns) != 2 {
			t.Errorf("Expected 2 patterns, got %d", len(restored.Patterns))
		}
		if restored.TrackDependencies != true {
			t.Error("TrackDependencies should be true")
		}
		if restored.Level != interest.InterestLevelDirect {
			t.Errorf("Expected DirectLevel, got %s", restored.Level)
		}
	})

	t.Run("Scenario: Interest snapshot can be created for sync", func(t *testing.T) {
		// Given a manager with multiple interests
		mgr := interest.NewManager()
		mgr.Register(interest.NewInterest("agent-1", "Claude", []string{"proj-a/**"}))
		mgr.Register(interest.NewInterest("agent-2", "Gemini", []string{"proj-b/**"}))
		mgr.Register(interest.NewInterest("agent-1", "Claude", []string{"shared/**"}))

		// When creating a snapshot
		snapshot := mgr.Snapshot()

		// Then the snapshot should contain all interests
		if len(snapshot) != 3 {
			t.Errorf("Expected 3 interests in snapshot, got %d", len(snapshot))
		}

		// And snapshot should be serializable
		data, err := json.Marshal(snapshot)
		if err != nil {
			t.Fatalf("Failed to serialize snapshot: %v", err)
		}

		// And deserializable
		var restored []*interest.Interest
		if err := json.Unmarshal(data, &restored); err != nil {
			t.Fatalf("Failed to deserialize snapshot: %v", err)
		}
		if len(restored) != 3 {
			t.Errorf("Expected 3 interests after deserialization, got %d", len(restored))
		}
	})

	t.Run("Scenario: Remote interests are merged into local manager", func(t *testing.T) {
		// Given a local manager with some interests
		localMgr := interest.NewManager()
		localMgr.Register(interest.NewInterest("local-agent", "Claude", []string{"local/**"}))

		// When receiving remote interests
		remoteInterests := []*interest.Interest{
			interest.NewInterest("remote-agent-1", "Gemini", []string{"remote-a/**"}),
			interest.NewInterest("remote-agent-2", "Cursor", []string{"remote-b/**"}),
		}
		for _, ri := range remoteInterests {
			ri.Remote = true
		}
		localMgr.MergeRemote(remoteInterests)

		// Then all interests should be in the manager
		if localMgr.Count() != 3 {
			t.Errorf("Expected 3 interests, got %d", localMgr.Count())
		}

		// And remote interests should be marked as remote
		remoteOnes := localMgr.GetRemoteInterests()
		if len(remoteOnes) != 2 {
			t.Errorf("Expected 2 remote interests, got %d", len(remoteOnes))
		}
	})

	t.Run("Scenario: Duplicate remote interests are ignored", func(t *testing.T) {
		// Given a manager
		mgr := interest.NewManager()
		original := interest.NewInterest("agent-1", "Claude", []string{"proj/**"})
		original.Remote = true
		mgr.MergeRemote([]*interest.Interest{original})

		// When merging the same interest again
		duplicate := interest.NewInterest("agent-1", "Claude", []string{"proj/**"})
		duplicate.ID = original.ID // Same ID
		duplicate.Remote = true
		mgr.MergeRemote([]*interest.Interest{duplicate})

		// Then the count should remain 1
		if mgr.Count() != 1 {
			t.Errorf("Expected 1 interest (no duplicate), got %d", mgr.Count())
		}
	})

	t.Run("Scenario: Remote interests can be cleared", func(t *testing.T) {
		// Given a manager with local and remote interests
		mgr := interest.NewManager()
		mgr.Register(interest.NewInterest("local-agent", "Claude", []string{"local/**"}))

		remoteInt := interest.NewInterest("remote-agent", "Gemini", []string{"remote/**"})
		remoteInt.Remote = true
		mgr.MergeRemote([]*interest.Interest{remoteInt})

		// When clearing remote interests
		cleared := mgr.ClearRemote()

		// Then only remote interests should be removed
		if cleared != 1 {
			t.Errorf("Expected 1 cleared, got %d", cleared)
		}
		if mgr.Count() != 1 {
			t.Errorf("Expected 1 remaining (local), got %d", mgr.Count())
		}

		// And the remaining should be local
		interests := mgr.GetAgentInterests("local-agent")
		if len(interests) != 1 {
			t.Errorf("Expected 1 local interest, got %d", len(interests))
		}
	})
}

// Feature: Interest Notification
// As an agent
// I want to be notified when interests change
// So that I can update my routing behavior

func TestFeature_InterestNotification(t *testing.T) {
	t.Run("Scenario: Listener is notified on interest registration", func(t *testing.T) {
		// Given a manager with a change listener
		mgr := interest.NewManager()

		var notifiedChange *interest.InterestChange
		mgr.OnChange(func(change interest.InterestChange) {
			notifiedChange = &change
		})

		// When an interest is registered
		int1 := interest.NewInterest("agent-1", "Claude", []string{"proj/**"})
		mgr.Register(int1)

		// Then the listener should be notified
		if notifiedChange == nil {
			t.Fatal("Expected change notification, got nil")
		}
		if notifiedChange.Type != interest.ChangeTypeAdded {
			t.Errorf("Expected ChangeTypeAdded, got %s", notifiedChange.Type)
		}
		if notifiedChange.Interest.AgentID != "agent-1" {
			t.Errorf("Expected agent-1, got %s", notifiedChange.Interest.AgentID)
		}
	})

	t.Run("Scenario: Listener is notified on interest removal", func(t *testing.T) {
		// Given a manager with an interest and listener
		mgr := interest.NewManager()
		int1 := interest.NewInterest("agent-1", "Claude", []string{"proj/**"})
		mgr.Register(int1)

		var notifiedChange *interest.InterestChange
		mgr.OnChange(func(change interest.InterestChange) {
			notifiedChange = &change
		})

		// When the interest is removed
		mgr.Unregister(int1.ID)

		// Then the listener should be notified
		if notifiedChange == nil {
			t.Fatal("Expected change notification, got nil")
		}
		if notifiedChange.Type != interest.ChangeTypeRemoved {
			t.Errorf("Expected ChangeTypeRemoved, got %s", notifiedChange.Type)
		}
	})

	t.Run("Scenario: Multiple listeners can be registered", func(t *testing.T) {
		// Given a manager with multiple listeners
		mgr := interest.NewManager()

		callCount := 0
		mgr.OnChange(func(change interest.InterestChange) {
			callCount++
		})
		mgr.OnChange(func(change interest.InterestChange) {
			callCount++
		})

		// When an interest is registered
		mgr.Register(interest.NewInterest("agent-1", "Claude", []string{"proj/**"}))

		// Then all listeners should be notified
		if callCount != 2 {
			t.Errorf("Expected 2 listener calls, got %d", callCount)
		}
	})
}
