package lock

import (
	"strings"
	"testing"
	"time"

	pkgerrors "agent-collab/src/pkg/errors"
)

// BDD-style tests for lock domain
// Feature: Semantic Lock System
// As an agent in a multi-agent environment
// I want to acquire semantic locks on files and code regions
// So that I can safely modify code without conflicts

// Scenario: Creating a semantic lock with valid parameters
func TestFeature_SemanticLock_Scenario_CreateWithValidParams(t *testing.T) {
	t.Run("Given a valid target and holder information", func(t *testing.T) {
		target := &SemanticTarget{
			Type:      TargetFile,
			FilePath:  "/src/main.go",
			StartLine: 10,
			EndLine:   50,
		}
		holderID := "agent-alice"
		holderName := "Alice"
		intention := "implementing authentication logic"

		t.Run("When I create a new semantic lock", func(t *testing.T) {
			lock, err := NewSemanticLockSafe(target, holderID, holderName, intention)

			t.Run("Then the lock should be created successfully", func(t *testing.T) {
				if err != nil {
					t.Fatalf("expected no error, got: %v", err)
				}
				if lock == nil {
					t.Fatal("lock should not be nil")
				}
			})

			t.Run("And the lock ID should have the correct prefix", func(t *testing.T) {
				if !strings.HasPrefix(lock.ID, "lock-") {
					t.Errorf("expected lock ID to start with 'lock-', got: %s", lock.ID)
				}
			})

			t.Run("And the holder information should be set", func(t *testing.T) {
				if lock.HolderID != holderID {
					t.Errorf("expected holder ID '%s', got: %s", holderID, lock.HolderID)
				}
				if lock.HolderName != holderName {
					t.Errorf("expected holder name '%s', got: %s", holderName, lock.HolderName)
				}
			})

			t.Run("And the intention should be recorded", func(t *testing.T) {
				if lock.Intention != intention {
					t.Errorf("expected intention '%s', got: %s", intention, lock.Intention)
				}
			})

			t.Run("And the fencing token should be non-zero", func(t *testing.T) {
				if lock.FencingToken == 0 {
					t.Error("fencing token should be non-zero")
				}
			})

			t.Run("And the lock should not be expired", func(t *testing.T) {
				if lock.IsExpired() {
					t.Error("new lock should not be expired")
				}
			})
		})
	})
}

// Scenario: Creating a lock with nil target should fail
func TestFeature_SemanticLock_Scenario_CreateWithNilTarget(t *testing.T) {
	t.Run("Given a nil target", func(t *testing.T) {
		var target *SemanticTarget = nil
		holderID := "agent-bob"
		holderName := "Bob"

		t.Run("When I try to create a semantic lock", func(t *testing.T) {
			lock, err := NewSemanticLockSafe(target, holderID, holderName, "editing")

			t.Run("Then it should return an error", func(t *testing.T) {
				if err == nil {
					t.Fatal("expected error for nil target")
				}
			})

			t.Run("And the lock should be nil", func(t *testing.T) {
				if lock != nil {
					t.Error("lock should be nil when error occurs")
				}
			})

			t.Run("And the error should be a ValidationError", func(t *testing.T) {
				var valErr *ValidationError
				if !pkgerrors.As(err, &valErr) {
					t.Errorf("expected ValidationError, got: %T", err)
				}
			})

			t.Run("And the error should identify the 'target' field", func(t *testing.T) {
				var valErr *ValidationError
				pkgerrors.As(err, &valErr)
				if valErr.Field != "target" {
					t.Errorf("expected field 'target', got: %s", valErr.Field)
				}
			})
		})
	})
}

// Scenario: Creating a lock with empty holder ID should fail
func TestFeature_SemanticLock_Scenario_CreateWithEmptyHolderID(t *testing.T) {
	t.Run("Given an empty holder ID", func(t *testing.T) {
		target := &SemanticTarget{Type: TargetFile, FilePath: "/test.go"}
		holderID := ""
		holderName := "Alice"

		t.Run("When I try to create a semantic lock", func(t *testing.T) {
			lock, err := NewSemanticLockSafe(target, holderID, holderName, "editing")

			t.Run("Then it should return an error", func(t *testing.T) {
				if err == nil {
					t.Fatal("expected error for empty holder ID")
				}
			})

			t.Run("And the lock should be nil", func(t *testing.T) {
				if lock != nil {
					t.Error("lock should be nil when error occurs")
				}
			})

			t.Run("And the error should identify the 'holderID' field", func(t *testing.T) {
				var valErr *ValidationError
				if pkgerrors.As(err, &valErr) {
					if valErr.Field != "holderID" {
						t.Errorf("expected field 'holderID', got: %s", valErr.Field)
					}
				}
			})
		})
	})
}

// Scenario: Creating a lock with empty holder name should default to "unknown"
func TestFeature_SemanticLock_Scenario_CreateWithEmptyHolderName(t *testing.T) {
	t.Run("Given an empty holder name", func(t *testing.T) {
		target := &SemanticTarget{Type: TargetFile, FilePath: "/test.go"}
		holderID := "agent-123"
		holderName := ""

		t.Run("When I create a semantic lock", func(t *testing.T) {
			lock, err := NewSemanticLockSafe(target, holderID, holderName, "editing")

			t.Run("Then it should succeed", func(t *testing.T) {
				if err != nil {
					t.Fatalf("expected no error, got: %v", err)
				}
			})

			t.Run("And the holder name should default to 'unknown'", func(t *testing.T) {
				if lock.HolderName != "unknown" {
					t.Errorf("expected holder name 'unknown', got: %s", lock.HolderName)
				}
			})
		})
	})
}

// Scenario: Lock expiration
func TestFeature_SemanticLock_Scenario_LockExpiration(t *testing.T) {
	t.Run("Given a newly created lock", func(t *testing.T) {
		target := &SemanticTarget{Type: TargetFile, FilePath: "/test.go"}
		lock, _ := NewSemanticLockSafe(target, "holder", "name", "editing")

		t.Run("When I check if it's expired immediately", func(t *testing.T) {
			expired := lock.IsExpired()

			t.Run("Then it should NOT be expired", func(t *testing.T) {
				if expired {
					t.Error("new lock should not be expired")
				}
			})
		})

		t.Run("When I check the remaining TTL", func(t *testing.T) {
			remaining := lock.TTLRemaining()

			t.Run("Then it should be positive", func(t *testing.T) {
				if remaining <= 0 {
					t.Error("remaining TTL should be positive")
				}
			})

			t.Run("And it should be at most the default TTL", func(t *testing.T) {
				if remaining > DefaultTTL {
					t.Errorf("remaining TTL (%v) should be <= DefaultTTL (%v)", remaining, DefaultTTL)
				}
			})
		})
	})

	t.Run("Given an expired lock", func(t *testing.T) {
		target := &SemanticTarget{Type: TargetFile, FilePath: "/test.go"}
		lock, _ := NewSemanticLockSafe(target, "holder", "name", "editing")
		lock.ExpiresAt = time.Now().Add(-1 * time.Second) // Manually expire

		t.Run("When I check if it's expired", func(t *testing.T) {
			expired := lock.IsExpired()

			t.Run("Then it should be expired", func(t *testing.T) {
				if !expired {
					t.Error("lock should be expired")
				}
			})
		})

		t.Run("When I check the remaining TTL", func(t *testing.T) {
			remaining := lock.TTLRemaining()

			t.Run("Then it should be zero", func(t *testing.T) {
				if remaining != 0 {
					t.Errorf("expected 0 TTL, got: %v", remaining)
				}
			})
		})
	})
}

// Scenario: Renewing a lock
func TestFeature_SemanticLock_Scenario_RenewLock(t *testing.T) {
	t.Run("Given an active lock", func(t *testing.T) {
		target := &SemanticTarget{Type: TargetFile, FilePath: "/test.go"}
		lock, _ := NewSemanticLockSafe(target, "holder", "name", "editing")
		originalExpiry := lock.ExpiresAt

		t.Run("When I renew the lock", func(t *testing.T) {
			time.Sleep(10 * time.Millisecond) // Small delay
			err := lock.Renew()

			t.Run("Then it should succeed", func(t *testing.T) {
				if err != nil {
					t.Fatalf("expected no error, got: %v", err)
				}
			})

			t.Run("And the expiry should be extended", func(t *testing.T) {
				if !lock.ExpiresAt.After(originalExpiry) {
					t.Error("expiry should be extended after renewal")
				}
			})

			t.Run("And the renew count should increase", func(t *testing.T) {
				if lock.RenewCount != 1 {
					t.Errorf("expected renew count 1, got: %d", lock.RenewCount)
				}
			})
		})
	})

	t.Run("Given a lock at maximum renewals", func(t *testing.T) {
		target := &SemanticTarget{Type: TargetFile, FilePath: "/test.go"}
		lock, _ := NewSemanticLockSafe(target, "holder", "name", "editing")
		lock.RenewCount = MaxRenewals

		t.Run("When I try to renew the lock", func(t *testing.T) {
			err := lock.Renew()

			t.Run("Then it should fail with ErrMaxRenewalsExceeded", func(t *testing.T) {
				if err != ErrMaxRenewalsExceeded {
					t.Errorf("expected ErrMaxRenewalsExceeded, got: %v", err)
				}
			})
		})
	})
}

// Scenario: Lock ID uniqueness
func TestFeature_SemanticLock_Scenario_LockIDUniqueness(t *testing.T) {
	t.Run("Given 100 lock creations", func(t *testing.T) {
		ids := make(map[string]bool)

		t.Run("When I create locks", func(t *testing.T) {
			for i := 0; i < 100; i++ {
				target := &SemanticTarget{Type: TargetFile, FilePath: "/test.go"}
				lock, _ := NewSemanticLockSafe(target, "holder", "name", "editing")
				ids[lock.ID] = true
			}

			t.Run("Then all lock IDs should be unique", func(t *testing.T) {
				if len(ids) != 100 {
					t.Errorf("expected 100 unique IDs, got: %d", len(ids))
				}
			})
		})
	})
}

// Scenario: Lock error categories
func TestFeature_SemanticLock_Scenario_ErrorCategories(t *testing.T) {
	t.Run("Given lock-related errors", func(t *testing.T) {
		t.Run("When I check ErrLockConflictCategorized", func(t *testing.T) {
			t.Run("Then it should be categorized as retryable", func(t *testing.T) {
				if ErrLockConflictCategorized.Category() != pkgerrors.CategoryRetryable {
					t.Errorf("expected retryable, got: %s", ErrLockConflictCategorized.Category())
				}
			})
		})

		t.Run("When I check ErrInvalidTargetCategorized", func(t *testing.T) {
			t.Run("Then it should be categorized as validation", func(t *testing.T) {
				if ErrInvalidTargetCategorized.Category() != pkgerrors.CategoryValidation {
					t.Errorf("expected validation, got: %s", ErrInvalidTargetCategorized.Category())
				}
			})
		})

		t.Run("When I check ErrLockNotFoundCategorized", func(t *testing.T) {
			t.Run("Then it should be categorized as permanent", func(t *testing.T) {
				if ErrLockNotFoundCategorized.Category() != pkgerrors.CategoryPermanent {
					t.Errorf("expected permanent, got: %s", ErrLockNotFoundCategorized.Category())
				}
			})
		})
	})
}

// Scenario: Lock error with context
func TestFeature_SemanticLock_Scenario_ErrorWithContext(t *testing.T) {
	t.Run("Given a lock error", func(t *testing.T) {
		baseErr := ErrLockConflictCategorized

		t.Run("When I add lock ID context", func(t *testing.T) {
			errWithLockID := baseErr.WithLockID("lock-abc123")

			t.Run("Then the error should contain the lock ID", func(t *testing.T) {
				if errWithLockID.LockID != "lock-abc123" {
					t.Errorf("expected lock ID 'lock-abc123', got: %s", errWithLockID.LockID)
				}
			})

			t.Run("And the error message should include the lock ID", func(t *testing.T) {
				if !strings.Contains(errWithLockID.Error(), "lock-abc123") {
					t.Error("error message should contain lock ID")
				}
			})
		})

		t.Run("When I add file path context", func(t *testing.T) {
			errWithPath := baseErr.WithFilePath("/src/handler.go")

			t.Run("Then the error should contain the file path", func(t *testing.T) {
				if errWithPath.FilePath != "/src/handler.go" {
					t.Errorf("expected file path '/src/handler.go', got: %s", errWithPath.FilePath)
				}
			})

			t.Run("And the error message should include the file path", func(t *testing.T) {
				if !strings.Contains(errWithPath.Error(), "/src/handler.go") {
					t.Error("error message should contain file path")
				}
			})
		})

		t.Run("When I chain multiple context methods", func(t *testing.T) {
			errWithBoth := baseErr.
				WithLockID("lock-xyz").
				WithFilePath("/src/service.go")

			t.Run("Then the error should contain both contexts", func(t *testing.T) {
				if errWithBoth.LockID != "lock-xyz" {
					t.Error("should contain lock ID")
				}
				if errWithBoth.FilePath != "/src/service.go" {
					t.Error("should contain file path")
				}
			})
		})
	})
}

// Scenario: Lock conflict detection
func TestFeature_SemanticLock_Scenario_LockConflict(t *testing.T) {
	t.Run("Given two overlapping lock requests", func(t *testing.T) {
		// First lock covers lines 10-50
		target1 := &SemanticTarget{
			Type:      TargetFile,
			FilePath:  "/src/handler.go",
			StartLine: 10,
			EndLine:   50,
		}
		lock1, _ := NewSemanticLockSafe(target1, "agent-alice", "Alice", "refactoring")

		// Second lock covers lines 30-70 (overlapping)
		target2 := &SemanticTarget{
			Type:      TargetFile,
			FilePath:  "/src/handler.go",
			StartLine: 30,
			EndLine:   70,
		}
		lock2, _ := NewSemanticLockSafe(target2, "agent-bob", "Bob", "adding feature")

		t.Run("When I create a LockConflict", func(t *testing.T) {
			conflict := NewLockConflict(lock2, lock1)

			t.Run("Then it should identify the requested lock", func(t *testing.T) {
				if conflict.RequestedLock != lock2 {
					t.Error("requested lock should be lock2")
				}
			})

			t.Run("And it should identify the conflicting lock", func(t *testing.T) {
				if conflict.ConflictingLock != lock1 {
					t.Error("conflicting lock should be lock1")
				}
			})

			t.Run("And the overlap type should be 'partial'", func(t *testing.T) {
				if conflict.OverlapType != "partial" {
					t.Errorf("expected overlap type 'partial', got: %s", conflict.OverlapType)
				}
			})
		})
	})
}
