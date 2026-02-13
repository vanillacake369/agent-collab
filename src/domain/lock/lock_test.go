package lock

import (
	"strings"
	"testing"
	"time"

	pkgerrors "agent-collab/src/pkg/errors"
)

func TestNewSemanticLockSafe_Valid(t *testing.T) {
	target := &SemanticTarget{
		Type:      "file",
		FilePath:  "/test/file.go",
		StartLine: 1,
		EndLine:   100,
	}

	lock, err := NewSemanticLockSafe(target, "holder-123", "Alice", "editing function")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if lock == nil {
		t.Fatal("expected lock to not be nil")
	}
	if lock.HolderID != "holder-123" {
		t.Errorf("expected holder ID 'holder-123', got: %s", lock.HolderID)
	}
	if lock.HolderName != "Alice" {
		t.Errorf("expected holder name 'Alice', got: %s", lock.HolderName)
	}
	if lock.Intention != "editing function" {
		t.Errorf("expected intention 'editing function', got: %s", lock.Intention)
	}
	if !strings.HasPrefix(lock.ID, "lock-") {
		t.Errorf("expected lock ID to start with 'lock-', got: %s", lock.ID)
	}
	if lock.FencingToken == 0 {
		t.Error("expected fencing token to be non-zero")
	}
	if lock.ExpiresAt.Before(time.Now()) {
		t.Error("expected lock to not be expired")
	}
}

func TestNewSemanticLockSafe_NilTarget(t *testing.T) {
	lock, err := NewSemanticLockSafe(nil, "holder-123", "Alice", "editing")
	if err == nil {
		t.Fatal("expected error for nil target")
	}
	if lock != nil {
		t.Error("expected lock to be nil on error")
	}

	var valErr *ValidationError
	if !pkgerrors.As(err, &valErr) {
		t.Errorf("expected ValidationError, got: %T", err)
	}
	if valErr.Field != "target" {
		t.Errorf("expected field 'target', got: %s", valErr.Field)
	}
}

func TestNewSemanticLockSafe_EmptyHolderID(t *testing.T) {
	target := &SemanticTarget{Type: "file", FilePath: "/test.go"}

	lock, err := NewSemanticLockSafe(target, "", "Alice", "editing")
	if err == nil {
		t.Fatal("expected error for empty holder ID")
	}
	if lock != nil {
		t.Error("expected lock to be nil on error")
	}

	var valErr *ValidationError
	if !pkgerrors.As(err, &valErr) {
		t.Errorf("expected ValidationError, got: %T", err)
	}
	if valErr.Field != "holderID" {
		t.Errorf("expected field 'holderID', got: %s", valErr.Field)
	}
}

func TestNewSemanticLockSafe_EmptyHolderName(t *testing.T) {
	target := &SemanticTarget{Type: "file", FilePath: "/test.go"}

	lock, err := NewSemanticLockSafe(target, "holder-123", "", "editing")
	if err != nil {
		t.Fatalf("expected no error for empty holder name, got: %v", err)
	}

	// Empty holder name should default to "unknown"
	if lock.HolderName != "unknown" {
		t.Errorf("expected holder name 'unknown', got: %s", lock.HolderName)
	}
}

func TestNewSemanticLock_Panic(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic for nil target")
		}
	}()

	// This should panic (deprecated behavior)
	NewSemanticLock(nil, "holder", "name", "intention")
}

func TestNewSemanticLock_Valid(t *testing.T) {
	target := &SemanticTarget{Type: "file", FilePath: "/test.go"}

	// This should not panic
	lock := NewSemanticLock(target, "holder-123", "Alice", "editing")
	if lock == nil {
		t.Fatal("expected lock to not be nil")
	}
}

func TestSemanticLock_IsExpired(t *testing.T) {
	target := &SemanticTarget{Type: "file", FilePath: "/test.go"}
	lock, _ := NewSemanticLockSafe(target, "holder", "name", "intention")

	if lock.IsExpired() {
		t.Error("new lock should not be expired")
	}

	// Manually expire the lock
	lock.ExpiresAt = time.Now().Add(-1 * time.Second)
	if !lock.IsExpired() {
		t.Error("lock should be expired")
	}
}

func TestSemanticLock_TTLRemaining(t *testing.T) {
	target := &SemanticTarget{Type: "file", FilePath: "/test.go"}
	lock, _ := NewSemanticLockSafe(target, "holder", "name", "intention")

	remaining := lock.TTLRemaining()
	if remaining <= 0 {
		t.Error("expected positive TTL remaining")
	}
	if remaining > DefaultTTL {
		t.Errorf("expected TTL remaining <= %v, got: %v", DefaultTTL, remaining)
	}

	// Expired lock
	lock.ExpiresAt = time.Now().Add(-1 * time.Second)
	if lock.TTLRemaining() != 0 {
		t.Error("expected 0 TTL for expired lock")
	}
}

func TestSemanticLock_Renew(t *testing.T) {
	target := &SemanticTarget{Type: "file", FilePath: "/test.go"}
	lock, _ := NewSemanticLockSafe(target, "holder", "name", "intention")

	originalExpiry := lock.ExpiresAt
	time.Sleep(10 * time.Millisecond)

	err := lock.Renew()
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if lock.ExpiresAt.Before(originalExpiry) {
		t.Error("expected expiry to be extended")
	}
	if lock.RenewCount != 1 {
		t.Errorf("expected renew count 1, got: %d", lock.RenewCount)
	}
}

func TestSemanticLock_Renew_MaxRenewals(t *testing.T) {
	target := &SemanticTarget{Type: "file", FilePath: "/test.go"}
	lock, _ := NewSemanticLockSafe(target, "holder", "name", "intention")

	// Set renew count to max
	lock.RenewCount = MaxRenewals

	err := lock.Renew()
	if err != ErrMaxRenewalsExceeded {
		t.Errorf("expected ErrMaxRenewalsExceeded, got: %v", err)
	}
}

func TestGenerateLockID(t *testing.T) {
	ids := make(map[string]bool)

	// Generate multiple IDs and check uniqueness
	for i := 0; i < 100; i++ {
		id := generateLockID()
		if !strings.HasPrefix(id, "lock-") {
			t.Errorf("expected lock ID to start with 'lock-', got: %s", id)
		}
		if ids[id] {
			t.Errorf("duplicate lock ID generated: %s", id)
		}
		ids[id] = true
	}
}

func TestLockError_Category(t *testing.T) {
	if ErrLockConflictCategorized.Category() != pkgerrors.CategoryRetryable {
		t.Error("expected lock conflict to be retryable")
	}
	if ErrInvalidTargetCategorized.Category() != pkgerrors.CategoryValidation {
		t.Error("expected invalid target to be validation error")
	}
	if ErrLockNotFoundCategorized.Category() != pkgerrors.CategoryPermanent {
		t.Error("expected lock not found to be permanent")
	}
}

func TestLockError_WithContext(t *testing.T) {
	err := ErrLockConflictCategorized.
		WithLockID("lock-abc123").
		WithFilePath("/test/file.go")

	if err.LockID != "lock-abc123" {
		t.Errorf("expected lock ID 'lock-abc123', got: %s", err.LockID)
	}
	if err.FilePath != "/test/file.go" {
		t.Errorf("expected file path '/test/file.go', got: %s", err.FilePath)
	}

	// Error message should include context
	errMsg := err.Error()
	if !strings.Contains(errMsg, "lock-abc123") {
		t.Errorf("expected error message to contain lock ID, got: %s", errMsg)
	}
	if !strings.Contains(errMsg, "/test/file.go") {
		t.Errorf("expected error message to contain file path, got: %s", errMsg)
	}
}

func TestValidationError_Category(t *testing.T) {
	err := NewValidationError("field", "message")
	if err.Category() != pkgerrors.CategoryValidation {
		t.Error("expected validation error to have validation category")
	}
}
