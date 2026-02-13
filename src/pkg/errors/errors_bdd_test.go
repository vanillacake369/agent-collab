package errors

import (
	"testing"
)

// BDD-style tests for errors package
// Feature: Categorized Error System
// As a developer
// I want a categorized error system
// So that I can handle different error types appropriately

// Scenario: Creating a validation error
func TestFeature_CategorizedErrors_Scenario_ValidationError(t *testing.T) {
	t.Run("Given a validation failure for a field", func(t *testing.T) {
		fieldName := "email"
		message := "must be a valid email address"

		t.Run("When I create a ValidationError", func(t *testing.T) {
			err := NewValidationError(fieldName, message)

			t.Run("Then the error should have the field name", func(t *testing.T) {
				if err.Field != fieldName {
					t.Errorf("expected field '%s', got '%s'", fieldName, err.Field)
				}
			})

			t.Run("And the error should have the message", func(t *testing.T) {
				if err.Message != message {
					t.Errorf("expected message '%s', got '%s'", message, err.Message)
				}
			})

			t.Run("And the error should be categorized as validation", func(t *testing.T) {
				if err.Category() != CategoryValidation {
					t.Errorf("expected CategoryValidation, got %s", err.Category())
				}
			})

			t.Run("And the error string should be human-readable", func(t *testing.T) {
				expected := "validation error: email must be a valid email address"
				if err.Error() != expected {
					t.Errorf("expected '%s', got '%s'", expected, err.Error())
				}
			})
		})
	})
}

// Scenario: Creating an internal error with cause
func TestFeature_CategorizedErrors_Scenario_InternalError(t *testing.T) {
	t.Run("Given an operation that failed with a cause", func(t *testing.T) {
		operation := "database.query"
		cause := New("connection timeout")

		t.Run("When I create an InternalError", func(t *testing.T) {
			err := NewInternalError(operation, cause)

			t.Run("Then the error should have the operation name", func(t *testing.T) {
				if err.Operation != operation {
					t.Errorf("expected operation '%s', got '%s'", operation, err.Operation)
				}
			})

			t.Run("And the error should be categorized as internal", func(t *testing.T) {
				if err.Category() != CategoryInternal {
					t.Errorf("expected CategoryInternal, got %s", err.Category())
				}
			})

			t.Run("And I should be able to unwrap to get the cause", func(t *testing.T) {
				if err.Unwrap() != cause {
					t.Error("Unwrap should return the cause")
				}
			})
		})
	})
}

// Scenario: Checking if an error is retryable
func TestFeature_CategorizedErrors_Scenario_RetryableCheck(t *testing.T) {
	t.Run("Given different types of errors", func(t *testing.T) {
		validationErr := NewValidationError("field", "invalid")
		internalErr := NewInternalError("op", nil)
		plainErr := New("plain error")

		t.Run("When I check if ValidationError is retryable", func(t *testing.T) {
			result := IsRetryable(validationErr)

			t.Run("Then it should return false", func(t *testing.T) {
				if result {
					t.Error("validation errors should not be retryable")
				}
			})
		})

		t.Run("When I check if InternalError is retryable", func(t *testing.T) {
			result := IsRetryable(internalErr)

			t.Run("Then it should return false", func(t *testing.T) {
				if result {
					t.Error("internal errors should not be retryable by default")
				}
			})
		})

		t.Run("When I check if plain error is retryable", func(t *testing.T) {
			result := IsRetryable(plainErr)

			t.Run("Then it should return false", func(t *testing.T) {
				if result {
					t.Error("plain errors should not be retryable")
				}
			})
		})

		t.Run("When I check if nil is retryable", func(t *testing.T) {
			result := IsRetryable(nil)

			t.Run("Then it should return false", func(t *testing.T) {
				if result {
					t.Error("nil should not be retryable")
				}
			})
		})
	})
}

// Scenario: Checking if an error is a validation error
func TestFeature_CategorizedErrors_Scenario_ValidationCheck(t *testing.T) {
	t.Run("Given a ValidationError", func(t *testing.T) {
		err := NewValidationError("name", "required")

		t.Run("When I check IsValidation", func(t *testing.T) {
			result := IsValidation(err)

			t.Run("Then it should return true", func(t *testing.T) {
				if !result {
					t.Error("ValidationError should be identified as validation error")
				}
			})
		})
	})

	t.Run("Given a wrapped ValidationError", func(t *testing.T) {
		innerErr := NewValidationError("name", "required")
		wrappedErr := Wrap(innerErr, "user creation failed")

		t.Run("When I check IsValidation on the wrapped error", func(t *testing.T) {
			result := IsValidation(wrappedErr)

			t.Run("Then it should return true", func(t *testing.T) {
				if !result {
					t.Error("wrapped ValidationError should still be identified")
				}
			})
		})
	})

	t.Run("Given an InternalError", func(t *testing.T) {
		err := NewInternalError("op", nil)

		t.Run("When I check IsValidation", func(t *testing.T) {
			result := IsValidation(err)

			t.Run("Then it should return false", func(t *testing.T) {
				if result {
					t.Error("InternalError should not be identified as validation error")
				}
			})
		})
	})
}

// Scenario: Wrapping errors with context
func TestFeature_CategorizedErrors_Scenario_WrappingErrors(t *testing.T) {
	t.Run("Given an original error", func(t *testing.T) {
		original := New("database connection failed")

		t.Run("When I wrap it with context", func(t *testing.T) {
			wrapped := Wrap(original, "failed to fetch user")

			t.Run("Then the wrapped error should include both messages", func(t *testing.T) {
				expected := "failed to fetch user: database connection failed"
				if wrapped.Error() != expected {
					t.Errorf("expected '%s', got '%s'", expected, wrapped.Error())
				}
			})

			t.Run("And I should be able to check if it contains the original", func(t *testing.T) {
				if !Is(wrapped, original) {
					t.Error("wrapped error should contain original")
				}
			})
		})
	})

	t.Run("Given nil", func(t *testing.T) {
		t.Run("When I try to wrap it", func(t *testing.T) {
			result := Wrap(nil, "context")

			t.Run("Then it should return nil", func(t *testing.T) {
				if result != nil {
					t.Error("wrapping nil should return nil")
				}
			})
		})
	})
}

// Scenario: Wrapping errors with formatted context
func TestFeature_CategorizedErrors_Scenario_FormattedWrapping(t *testing.T) {
	t.Run("Given an original error", func(t *testing.T) {
		original := New("file not found")

		t.Run("When I wrap it with formatted context", func(t *testing.T) {
			userID := "user-123"
			fileName := "config.yaml"
			wrapped := Wrapf(original, "failed to load config for user %s, file %s", userID, fileName)

			t.Run("Then the message should include formatted values", func(t *testing.T) {
				expected := "failed to load config for user user-123, file config.yaml: file not found"
				if wrapped.Error() != expected {
					t.Errorf("expected '%s', got '%s'", expected, wrapped.Error())
				}
			})
		})
	})
}

// Scenario: Joining multiple errors
func TestFeature_CategorizedErrors_Scenario_JoiningErrors(t *testing.T) {
	t.Run("Given multiple errors", func(t *testing.T) {
		err1 := New("validation failed for field A")
		err2 := New("validation failed for field B")
		err3 := New("validation failed for field C")

		t.Run("When I join them", func(t *testing.T) {
			joined := Join(err1, err2, err3)

			t.Run("Then the joined error should not be nil", func(t *testing.T) {
				if joined == nil {
					t.Fatal("joined error should not be nil")
				}
			})

			t.Run("And it should contain all original errors", func(t *testing.T) {
				if !Is(joined, err1) {
					t.Error("should contain err1")
				}
				if !Is(joined, err2) {
					t.Error("should contain err2")
				}
				if !Is(joined, err3) {
					t.Error("should contain err3")
				}
			})
		})
	})
}

// Scenario: Extracting typed errors with As
func TestFeature_CategorizedErrors_Scenario_ErrorExtraction(t *testing.T) {
	t.Run("Given a wrapped ValidationError", func(t *testing.T) {
		innerErr := NewValidationError("email", "invalid format")
		wrappedErr := Wrap(innerErr, "user registration failed")

		t.Run("When I extract the ValidationError using As", func(t *testing.T) {
			var target *ValidationError
			found := As(wrappedErr, &target)

			t.Run("Then the extraction should succeed", func(t *testing.T) {
				if !found {
					t.Fatal("As should find ValidationError")
				}
			})

			t.Run("And the target should have the original values", func(t *testing.T) {
				if target.Field != "email" {
					t.Errorf("expected field 'email', got '%s'", target.Field)
				}
				if target.Message != "invalid format" {
					t.Errorf("expected message 'invalid format', got '%s'", target.Message)
				}
			})
		})
	})

	t.Run("Given a plain error", func(t *testing.T) {
		plainErr := New("something went wrong")

		t.Run("When I try to extract ValidationError", func(t *testing.T) {
			var target *ValidationError
			found := As(plainErr, &target)

			t.Run("Then the extraction should fail", func(t *testing.T) {
				if found {
					t.Error("should not find ValidationError in plain error")
				}
			})
		})
	})
}

// Scenario: Error categories for routing
func TestFeature_CategorizedErrors_Scenario_CategoryRouting(t *testing.T) {
	t.Run("Given errors of different categories", func(t *testing.T) {
		cases := []struct {
			name         string
			err          Categorized
			expectedCat  Category
		}{
			{
				name:        "ValidationError",
				err:         NewValidationError("field", "msg"),
				expectedCat: CategoryValidation,
			},
			{
				name:        "InternalError",
				err:         NewInternalError("op", nil),
				expectedCat: CategoryInternal,
			},
		}

		for _, tc := range cases {
			t.Run("When I check the category of "+tc.name, func(t *testing.T) {
				t.Run("Then it should return "+string(tc.expectedCat), func(t *testing.T) {
					if tc.err.Category() != tc.expectedCat {
						t.Errorf("expected %s, got %s", tc.expectedCat, tc.err.Category())
					}
				})
			})
		}
	})
}
