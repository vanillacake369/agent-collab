package v1

import (
	"encoding/json"
	"testing"
	"time"
)

func TestDurationMarshalJSON(t *testing.T) {
	tests := []struct {
		name     string
		duration Duration
		want     string
	}{
		{"zero", Duration{0}, `"0s"`},
		{"one second", Duration{time.Second}, `"1s"`},
		{"one minute", Duration{time.Minute}, `"1m0s"`},
		{"complex", Duration{time.Hour + 30*time.Minute + 15*time.Second}, `"1h30m15s"`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := json.Marshal(tt.duration)
			if err != nil {
				t.Errorf("Marshal() error = %v", err)
				return
			}
			if string(got) != tt.want {
				t.Errorf("Marshal() = %s, want %s", got, tt.want)
			}
		})
	}
}

func TestDurationUnmarshalJSON(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    time.Duration
		wantErr bool
	}{
		{"zero", `"0s"`, 0, false},
		{"one second", `"1s"`, time.Second, false},
		{"one minute", `"1m"`, time.Minute, false},
		{"complex", `"1h30m15s"`, time.Hour + 30*time.Minute + 15*time.Second, false},
		{"invalid", `"invalid"`, 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var d Duration
			err := json.Unmarshal([]byte(tt.input), &d)
			if (err != nil) != tt.wantErr {
				t.Errorf("Unmarshal() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && d.Duration != tt.want {
				t.Errorf("Unmarshal() = %v, want %v", d.Duration, tt.want)
			}
		})
	}
}

func TestLockSerialization(t *testing.T) {
	lock := NewLock("test-lock", LockSpec{
		Target: LockTarget{
			Type:     LockTargetTypeFile,
			FilePath: "/path/to/file.go",
		},
		HolderID:  "agent-123",
		Intention: "Modifying authentication logic",
		TTL:       Duration{5 * time.Minute},
		Exclusive: true,
	})
	lock.UID = "uid-123"
	lock.Status.Phase = LockPhaseActive
	now := time.Now()
	lock.Status.AcquiredAt = &now
	expires := now.Add(5 * time.Minute)
	lock.Status.ExpiresAt = &expires
	lock.Status.FencingToken = 42

	data, err := json.MarshalIndent(lock, "", "  ")
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}

	t.Logf("Serialized Lock:\n%s", data)

	var decoded Lock
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}

	if decoded.Kind != LockKind {
		t.Errorf("Kind = %s, want %s", decoded.Kind, LockKind)
	}
	if decoded.APIVersion != GroupVersion {
		t.Errorf("APIVersion = %s, want %s", decoded.APIVersion, GroupVersion)
	}
	if decoded.Name != "test-lock" {
		t.Errorf("Name = %s, want test-lock", decoded.Name)
	}
	if decoded.Spec.HolderID != "agent-123" {
		t.Errorf("HolderID = %s, want agent-123", decoded.Spec.HolderID)
	}
	if decoded.Spec.TTL.Duration != 5*time.Minute {
		t.Errorf("TTL = %v, want 5m", decoded.Spec.TTL.Duration)
	}
	if decoded.Status.Phase != LockPhaseActive {
		t.Errorf("Phase = %s, want %s", decoded.Status.Phase, LockPhaseActive)
	}
	if decoded.Status.FencingToken != 42 {
		t.Errorf("FencingToken = %d, want 42", decoded.Status.FencingToken)
	}
}

func TestContextSerialization(t *testing.T) {
	ctx := NewContext("test-context", ContextSpec{
		Type:          ContextTypeDelta,
		SourceAgentID: "agent-456",
		FilePath:      "/path/to/file.go",
		Content:       "func main() { /* changed */ }",
		VectorClock: map[string]uint64{
			"agent-123": 5,
			"agent-456": 3,
		},
		Delta: &ContextDelta{
			Operation:  DeltaOperationModify,
			OldContent: "func main() {}",
			NewContent: "func main() { /* changed */ }",
			StartLine:  10,
			EndLine:    15,
		},
	})
	ctx.UID = "uid-456"

	data, err := json.MarshalIndent(ctx, "", "  ")
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}

	t.Logf("Serialized Context:\n%s", data)

	var decoded Context
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}

	if decoded.Kind != ContextKind {
		t.Errorf("Kind = %s, want %s", decoded.Kind, ContextKind)
	}
	if decoded.Spec.Type != ContextTypeDelta {
		t.Errorf("Type = %s, want %s", decoded.Spec.Type, ContextTypeDelta)
	}
	if decoded.Spec.Delta == nil {
		t.Fatal("Delta is nil")
	}
	if decoded.Spec.Delta.Operation != DeltaOperationModify {
		t.Errorf("Operation = %s, want %s", decoded.Spec.Delta.Operation, DeltaOperationModify)
	}
}

func TestAgentSerialization(t *testing.T) {
	agent := NewAgent("claude-agent", AgentSpec{
		Provider: AgentProviderAnthropic,
		Model:    "claude-3-opus",
		Capabilities: []AgentCapability{
			AgentCapabilityCodeEdit,
			AgentCapabilityCodeReview,
			AgentCapabilityToolUse,
		},
		PeerID:             "12D3KooW...",
		DisplayName:        "Claude Code Agent",
		HeartbeatInterval:  Duration{30 * time.Second},
		MaxConcurrentTasks: 3,
	})
	agent.UID = "uid-789"
	agent.Status.Phase = AgentPhaseOnline
	now := time.Now()
	agent.Status.LastHeartbeat = &now
	agent.Status.ConnectedAt = &now

	data, err := json.MarshalIndent(agent, "", "  ")
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}

	t.Logf("Serialized Agent:\n%s", data)

	var decoded Agent
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}

	if decoded.Kind != AgentKind {
		t.Errorf("Kind = %s, want %s", decoded.Kind, AgentKind)
	}
	if decoded.Spec.Provider != AgentProviderAnthropic {
		t.Errorf("Provider = %s, want %s", decoded.Spec.Provider, AgentProviderAnthropic)
	}
	if len(decoded.Spec.Capabilities) != 3 {
		t.Errorf("Capabilities count = %d, want 3", len(decoded.Spec.Capabilities))
	}
	if decoded.Status.Phase != AgentPhaseOnline {
		t.Errorf("Phase = %s, want %s", decoded.Status.Phase, AgentPhaseOnline)
	}
}

func TestLockHelpers(t *testing.T) {
	lock := NewLock("test", LockSpec{
		HolderID: "agent-1",
		TTL:      Duration{time.Minute},
	})

	// Test initial state
	if lock.IsActive() {
		t.Error("New lock should not be active")
	}
	if lock.IsExpired() {
		t.Error("New lock should not be expired")
	}

	// Test active state
	lock.Status.Phase = LockPhaseActive
	now := time.Now()
	lock.Status.AcquiredAt = &now
	expires := now.Add(time.Minute)
	lock.Status.ExpiresAt = &expires

	if !lock.IsActive() {
		t.Error("Lock should be active")
	}
	if lock.IsExpired() {
		t.Error("Lock should not be expired yet")
	}

	// Test expired state
	expired := now.Add(-time.Minute)
	lock.Status.ExpiresAt = &expired
	if !lock.IsExpired() {
		t.Error("Lock should be expired")
	}

	// Test conditions
	lock.SetCondition(LockConditionReady, ConditionTrue, "LockAcquired", "Lock was acquired successfully")
	cond := lock.GetCondition(LockConditionReady)
	if cond == nil {
		t.Fatal("Condition should exist")
	}
	if cond.Status != ConditionTrue {
		t.Errorf("Condition status = %s, want True", cond.Status)
	}

	// Test condition update
	lock.SetCondition(LockConditionReady, ConditionFalse, "LockReleased", "Lock was released")
	cond = lock.GetCondition(LockConditionReady)
	if cond.Status != ConditionFalse {
		t.Errorf("Condition status = %s, want False", cond.Status)
	}
}

func TestAgentHelpers(t *testing.T) {
	agent := NewAgent("test", AgentSpec{
		Capabilities: []AgentCapability{
			AgentCapabilityCodeEdit,
			AgentCapabilityToolUse,
		},
	})

	// Test capability check
	if !agent.HasCapability(AgentCapabilityCodeEdit) {
		t.Error("Agent should have CodeEdit capability")
	}
	if agent.HasCapability(AgentCapabilityVision) {
		t.Error("Agent should not have Vision capability")
	}

	// Test state helpers
	if agent.IsOnline() {
		t.Error("New agent should not be online")
	}
	if agent.IsAvailable() {
		t.Error("New agent should not be available")
	}

	agent.Status.Phase = AgentPhaseOnline
	if !agent.IsOnline() {
		t.Error("Agent should be online")
	}
	if !agent.IsAvailable() {
		t.Error("Agent should be available")
	}

	// Busy agent
	agent.Status.CurrentTask = &AgentTask{TaskID: "task-1"}
	if agent.IsAvailable() {
		t.Error("Agent with task should not be available")
	}
}
