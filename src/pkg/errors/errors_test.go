package errors

import (
	"testing"
)

func TestValidationError(t *testing.T) {
	err := NewValidationError("email", "is required")

	if err.Field != "email" {
		t.Errorf("expected field 'email', got: %s", err.Field)
	}
	if err.Message != "is required" {
		t.Errorf("expected message 'is required', got: %s", err.Message)
	}
	if err.Category() != CategoryValidation {
		t.Errorf("expected category validation, got: %s", err.Category())
	}

	// Error message format
	expected := "validation error: email is required"
	if err.Error() != expected {
		t.Errorf("expected error '%s', got: %s", expected, err.Error())
	}
}

func TestInternalError(t *testing.T) {
	cause := New("database connection failed")
	err := NewInternalError("queryUsers", cause)

	if err.Operation != "queryUsers" {
		t.Errorf("expected operation 'queryUsers', got: %s", err.Operation)
	}
	if err.Category() != CategoryInternal {
		t.Errorf("expected category internal, got: %s", err.Category())
	}
	if err.Unwrap() != cause {
		t.Error("expected Unwrap to return cause")
	}
}

func TestIsRetryable(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "validation error is not retryable",
			err:      NewValidationError("field", "error"),
			expected: false,
		},
		{
			name:     "internal error is not retryable",
			err:      NewInternalError("op", nil),
			expected: false,
		},
		{
			name:     "plain error is not retryable",
			err:      New("plain error"),
			expected: false,
		},
		{
			name:     "nil is not retryable",
			err:      nil,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsRetryable(tt.err)
			if got != tt.expected {
				t.Errorf("IsRetryable() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestIsValidation(t *testing.T) {
	valErr := NewValidationError("field", "error")
	if !IsValidation(valErr) {
		t.Error("expected validation error to be identified")
	}

	intErr := NewInternalError("op", nil)
	if IsValidation(intErr) {
		t.Error("expected internal error to not be validation")
	}

	plainErr := New("plain error")
	if IsValidation(plainErr) {
		t.Error("expected plain error to not be validation")
	}
}

func TestWrap(t *testing.T) {
	cause := New("original error")
	wrapped := Wrap(cause, "context")

	if wrapped == nil {
		t.Fatal("expected wrapped error to not be nil")
	}
	if !Is(wrapped, cause) {
		t.Error("expected wrapped error to contain cause")
	}
	expectedMsg := "context: original error"
	if wrapped.Error() != expectedMsg {
		t.Errorf("expected '%s', got '%s'", expectedMsg, wrapped.Error())
	}

	// Wrap nil should return nil
	if Wrap(nil, "context") != nil {
		t.Error("expected Wrap(nil) to return nil")
	}
}

func TestWrapf(t *testing.T) {
	cause := New("original error")
	wrapped := Wrapf(cause, "failed to process %s", "request")

	expectedMsg := "failed to process request: original error"
	if wrapped.Error() != expectedMsg {
		t.Errorf("expected '%s', got '%s'", expectedMsg, wrapped.Error())
	}

	// Wrapf nil should return nil
	if Wrapf(nil, "context %s", "value") != nil {
		t.Error("expected Wrapf(nil) to return nil")
	}
}

func TestJoin(t *testing.T) {
	err1 := New("error 1")
	err2 := New("error 2")
	joined := Join(err1, err2)

	if joined == nil {
		t.Fatal("expected joined error to not be nil")
	}
	if !Is(joined, err1) {
		t.Error("expected joined error to contain err1")
	}
	if !Is(joined, err2) {
		t.Error("expected joined error to contain err2")
	}
}

func TestAs(t *testing.T) {
	valErr := NewValidationError("field", "error")
	wrapped := Wrap(valErr, "wrapped")

	var target *ValidationError
	if !As(wrapped, &target) {
		t.Error("expected As to find ValidationError in wrapped error")
	}
	if target.Field != "field" {
		t.Errorf("expected field 'field', got: %s", target.Field)
	}
}
