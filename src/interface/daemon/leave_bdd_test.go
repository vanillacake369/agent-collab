package daemon

import (
	"errors"
	"testing"
	"time"
)

// BDD-style tests for leave endpoint
// Feature: Graceful Cluster Leave
// As an agent in a collaboration cluster
// I want to leave the cluster gracefully
// So that my locks are released and context is synced before disconnecting

// Scenario: Initiating a leave process
func TestFeature_GracefulLeave_Scenario_InitiateLeave(t *testing.T) {
	t.Run("Given an idle leave state machine", func(t *testing.T) {
		sm := NewLeaveStateMachine()

		t.Run("When I start the leave process", func(t *testing.T) {
			err := sm.Start()

			t.Run("Then it should succeed", func(t *testing.T) {
				if err != nil {
					t.Fatalf("expected no error, got: %v", err)
				}
			})

			t.Run("And the state should be 'initiated'", func(t *testing.T) {
				if sm.State() != LeaveStateInitiated {
					t.Errorf("expected state 'initiated', got: %s", sm.State())
				}
			})

			t.Run("And the start time should be recorded", func(t *testing.T) {
				status := sm.Status()
				if status.StartedAt == "" {
					t.Error("start time should be recorded")
				}
			})
		})
	})
}

// Scenario: Cannot start leave when already in progress
func TestFeature_GracefulLeave_Scenario_PreventDoubleStart(t *testing.T) {
	t.Run("Given a leave process already in progress", func(t *testing.T) {
		sm := NewLeaveStateMachine()
		sm.Start()

		t.Run("When I try to start again", func(t *testing.T) {
			err := sm.Start()

			t.Run("Then it should fail", func(t *testing.T) {
				if err == nil {
					t.Error("expected error when starting again")
				}
			})

			t.Run("And the error message should indicate it's in progress", func(t *testing.T) {
				if err != nil && !contains(err.Error(), "in progress") {
					t.Errorf("error should mention 'in progress', got: %s", err.Error())
				}
			})
		})
	})
}

// Scenario: Complete leave process workflow
func TestFeature_GracefulLeave_Scenario_CompleteWorkflow(t *testing.T) {
	t.Run("Given a started leave process", func(t *testing.T) {
		sm := NewLeaveStateMachine()
		sm.Start()

		t.Run("When the process transitions through all states", func(t *testing.T) {
			// Step 1: Release locks
			sm.TransitionTo(LeaveStateReleasingL, "Releasing all locks")
			if sm.State() != LeaveStateReleasingL {
				t.Errorf("expected releasing_locks state")
			}

			sm.SetLocksReleased(5)

			// Step 2: Sync context
			sm.TransitionTo(LeaveStateSyncing, "Syncing pending context")
			if sm.State() != LeaveStateSyncing {
				t.Errorf("expected syncing state")
			}

			sm.SetContextSynced()

			// Step 3: Disconnect
			sm.TransitionTo(LeaveStateDisconnect, "Disconnecting from peers")
			if sm.State() != LeaveStateDisconnect {
				t.Errorf("expected disconnecting state")
			}

			// Step 4: Complete
			sm.Complete()

			t.Run("Then the final state should be 'completed'", func(t *testing.T) {
				if sm.State() != LeaveStateCompleted {
					t.Errorf("expected completed state, got: %s", sm.State())
				}
			})

			t.Run("And the status should reflect all operations", func(t *testing.T) {
				status := sm.Status()

				if status.LocksReleased != 5 {
					t.Errorf("expected 5 locks released, got: %d", status.LocksReleased)
				}

				if !status.ContextSynced {
					t.Error("context should be marked as synced")
				}

				if status.CompletedAt == "" {
					t.Error("completion time should be recorded")
				}

				if status.Duration == "" {
					t.Error("duration should be calculated")
				}
			})
		})
	})
}

// Scenario: Handle leave process failure
func TestFeature_GracefulLeave_Scenario_HandleFailure(t *testing.T) {
	t.Run("Given a leave process in progress", func(t *testing.T) {
		sm := NewLeaveStateMachine()
		sm.Start()
		sm.TransitionTo(LeaveStateReleasingL, "Releasing locks")

		t.Run("When an error occurs", func(t *testing.T) {
			failureErr := errors.New("failed to release lock: permission denied")
			sm.Fail(failureErr)

			t.Run("Then the state should be 'failed'", func(t *testing.T) {
				if sm.State() != LeaveStateFailed {
					t.Errorf("expected failed state, got: %s", sm.State())
				}
			})

			t.Run("And the error should be recorded in status", func(t *testing.T) {
				status := sm.Status()
				if status.Error == "" {
					t.Error("error should be recorded")
				}
				if status.Error != failureErr.Error() {
					t.Errorf("expected error '%s', got: '%s'", failureErr.Error(), status.Error)
				}
			})

			t.Run("And the completion time should be set", func(t *testing.T) {
				status := sm.Status()
				if status.CompletedAt == "" {
					t.Error("completion time should be set even on failure")
				}
			})
		})
	})
}

// Scenario: Restart after completion
func TestFeature_GracefulLeave_Scenario_RestartAfterComplete(t *testing.T) {
	t.Run("Given a completed leave process", func(t *testing.T) {
		sm := NewLeaveStateMachine()
		sm.Start()
		sm.Complete()

		t.Run("When I start a new leave process", func(t *testing.T) {
			err := sm.Start()

			t.Run("Then it should succeed", func(t *testing.T) {
				if err != nil {
					t.Fatalf("expected no error, got: %v", err)
				}
			})

			t.Run("And the state should reset to 'initiated'", func(t *testing.T) {
				if sm.State() != LeaveStateInitiated {
					t.Errorf("expected initiated state, got: %s", sm.State())
				}
			})

			t.Run("And the counters should be reset", func(t *testing.T) {
				status := sm.Status()
				if status.LocksReleased != 0 {
					t.Error("locks released should be reset")
				}
				if status.ContextSynced {
					t.Error("context synced should be reset")
				}
			})
		})
	})
}

// Scenario: Restart after failure
func TestFeature_GracefulLeave_Scenario_RestartAfterFailure(t *testing.T) {
	t.Run("Given a failed leave process", func(t *testing.T) {
		sm := NewLeaveStateMachine()
		sm.Start()
		sm.Fail(errors.New("network error"))

		t.Run("When I retry the leave process", func(t *testing.T) {
			err := sm.Start()

			t.Run("Then it should succeed", func(t *testing.T) {
				if err != nil {
					t.Fatalf("expected no error, got: %v", err)
				}
			})

			t.Run("And the error should be cleared", func(t *testing.T) {
				status := sm.Status()
				if status.Error != "" {
					t.Error("error should be cleared on restart")
				}
			})
		})
	})
}

// Scenario: Status during each phase
func TestFeature_GracefulLeave_Scenario_StatusDuringPhases(t *testing.T) {
	t.Run("Given a leave state machine", func(t *testing.T) {
		sm := NewLeaveStateMachine()

		t.Run("When I check status at idle", func(t *testing.T) {
			status := sm.Status()

			t.Run("Then state should be 'idle'", func(t *testing.T) {
				if status.State != string(LeaveStateIdle) {
					t.Errorf("expected idle state")
				}
			})

			t.Run("And timestamps should be empty", func(t *testing.T) {
				if status.StartedAt != "" {
					t.Error("started_at should be empty")
				}
				if status.CompletedAt != "" {
					t.Error("completed_at should be empty")
				}
			})
		})

		sm.Start()

		t.Run("When I check status at initiated", func(t *testing.T) {
			status := sm.Status()

			t.Run("Then state should be 'initiated'", func(t *testing.T) {
				if status.State != string(LeaveStateInitiated) {
					t.Errorf("expected initiated state")
				}
			})

			t.Run("And started_at should be set", func(t *testing.T) {
				if status.StartedAt == "" {
					t.Error("started_at should be set")
				}
			})
		})
	})
}

// Scenario: Leave process timing
func TestFeature_GracefulLeave_Scenario_Timing(t *testing.T) {
	t.Run("Given a leave process", func(t *testing.T) {
		sm := NewLeaveStateMachine()

		beforeStart := time.Now()
		sm.Start()

		// Simulate some work
		time.Sleep(10 * time.Millisecond)

		sm.Complete()
		afterComplete := time.Now()

		t.Run("When the process completes", func(t *testing.T) {
			status := sm.Status()

			t.Run("Then the duration should be positive", func(t *testing.T) {
				if status.Duration == "" {
					t.Fatal("duration should be set")
				}
			})

			t.Run("And the duration should be reasonable", func(t *testing.T) {
				duration, _ := time.ParseDuration(status.Duration)
				totalTime := afterComplete.Sub(beforeStart)

				if duration <= 0 {
					t.Error("duration should be positive")
				}
				if duration > totalTime {
					t.Errorf("duration (%v) should not exceed total time (%v)", duration, totalTime)
				}
			})
		})
	})
}

// Scenario: Leave states are valid strings
func TestFeature_GracefulLeave_Scenario_StateStrings(t *testing.T) {
	t.Run("Given all leave states", func(t *testing.T) {
		states := []LeaveState{
			LeaveStateIdle,
			LeaveStateInitiated,
			LeaveStateReleasingL,
			LeaveStateSyncing,
			LeaveStateDisconnect,
			LeaveStateCompleted,
			LeaveStateFailed,
		}

		t.Run("When I check their string values", func(t *testing.T) {
			for _, state := range states {
				t.Run("Then "+string(state)+" should be non-empty", func(t *testing.T) {
					if string(state) == "" {
						t.Error("state string should not be empty")
					}
				})
			}
		})
	})
}

// Helper function
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
