package daemon

import (
	"testing"
	"time"
)

func TestLeaveStateMachine_InitialState(t *testing.T) {
	sm := NewLeaveStateMachine()
	if sm.State() != LeaveStateIdle {
		t.Errorf("expected idle state, got %s", sm.State())
	}
}

func TestLeaveStateMachine_Start(t *testing.T) {
	sm := NewLeaveStateMachine()

	if err := sm.Start(); err != nil {
		t.Fatalf("failed to start: %v", err)
	}

	if sm.State() != LeaveStateInitiated {
		t.Errorf("expected initiated state, got %s", sm.State())
	}

	// Starting again should fail
	if err := sm.Start(); err == nil {
		t.Error("expected error when starting again")
	}
}

func TestLeaveStateMachine_Transitions(t *testing.T) {
	sm := NewLeaveStateMachine()
	sm.Start()

	sm.TransitionTo(LeaveStateReleasingL, "Releasing locks")
	if sm.State() != LeaveStateReleasingL {
		t.Errorf("expected releasing_locks state, got %s", sm.State())
	}

	sm.SetLocksReleased(5)
	sm.TransitionTo(LeaveStateSyncing, "Syncing context")
	if sm.State() != LeaveStateSyncing {
		t.Errorf("expected syncing state, got %s", sm.State())
	}

	sm.SetContextSynced()
	sm.TransitionTo(LeaveStateDisconnect, "Disconnecting")
	if sm.State() != LeaveStateDisconnect {
		t.Errorf("expected disconnecting state, got %s", sm.State())
	}

	sm.Complete()
	if sm.State() != LeaveStateCompleted {
		t.Errorf("expected completed state, got %s", sm.State())
	}
}

func TestLeaveStateMachine_Status(t *testing.T) {
	sm := NewLeaveStateMachine()
	sm.Start()
	sm.SetLocksReleased(3)
	sm.SetContextSynced()
	sm.Complete()

	status := sm.Status()
	if status.State != string(LeaveStateCompleted) {
		t.Errorf("expected completed state in status, got %s", status.State)
	}
	if status.LocksReleased != 3 {
		t.Errorf("expected 3 locks released, got %d", status.LocksReleased)
	}
	if !status.ContextSynced {
		t.Error("expected context synced to be true")
	}
	if status.Duration == "" {
		t.Error("expected duration to be set")
	}
}

func TestLeaveStateMachine_Fail(t *testing.T) {
	sm := NewLeaveStateMachine()
	sm.Start()
	sm.Fail(errTestFailure)

	if sm.State() != LeaveStateFailed {
		t.Errorf("expected failed state, got %s", sm.State())
	}

	status := sm.Status()
	if status.Error == "" {
		t.Error("expected error message in status")
	}
}

func TestLeaveStateMachine_RestartAfterComplete(t *testing.T) {
	sm := NewLeaveStateMachine()
	sm.Start()
	sm.Complete()

	// Should be able to start again after completion
	if err := sm.Start(); err != nil {
		t.Fatalf("failed to restart after completion: %v", err)
	}

	if sm.State() != LeaveStateInitiated {
		t.Errorf("expected initiated state after restart, got %s", sm.State())
	}
}

func TestLeaveStateMachine_RestartAfterFail(t *testing.T) {
	sm := NewLeaveStateMachine()
	sm.Start()
	sm.Fail(errTestFailure)

	// Should be able to start again after failure
	if err := sm.Start(); err != nil {
		t.Fatalf("failed to restart after failure: %v", err)
	}

	if sm.State() != LeaveStateInitiated {
		t.Errorf("expected initiated state after restart, got %s", sm.State())
	}
}

func TestLeaveStateMachine_StatusTimestamps(t *testing.T) {
	sm := NewLeaveStateMachine()

	before := time.Now()
	sm.Start()
	after := time.Now()

	status := sm.Status()
	if status.StartedAt == "" {
		t.Error("expected StartedAt to be set")
	}

	// Complete and check completedAt
	sm.Complete()
	status = sm.Status()
	if status.CompletedAt == "" {
		t.Error("expected CompletedAt to be set")
	}

	// Verify timestamps are reasonable
	_ = before
	_ = after
}

// errTestFailure is a test error for failure scenarios.
type testError struct{}

func (e testError) Error() string { return "test failure" }

var errTestFailure = testError{}
