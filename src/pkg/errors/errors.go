// Package errors provides error types and utilities for the application.
package errors

import (
	"errors"
	"fmt"
)

// Category represents error classification for handling decisions.
type Category string

const (
	// CategoryValidation indicates input validation failures.
	CategoryValidation Category = "validation"
	// CategoryNetwork indicates network-related failures.
	CategoryNetwork Category = "network"
	// CategoryRetryable indicates errors that may succeed on retry.
	CategoryRetryable Category = "retryable"
	// CategoryPermanent indicates errors that will not succeed on retry.
	CategoryPermanent Category = "permanent"
	// CategoryInternal indicates internal/unexpected errors.
	CategoryInternal Category = "internal"
)

// Categorized is an error that has a category.
type Categorized interface {
	error
	Category() Category
}

// IsRetryable checks if an error should trigger a retry.
func IsRetryable(err error) bool {
	var cat Categorized
	if errors.As(err, &cat) {
		return cat.Category() == CategoryRetryable
	}
	return false
}

// IsValidation checks if an error is a validation error.
func IsValidation(err error) bool {
	var cat Categorized
	if errors.As(err, &cat) {
		return cat.Category() == CategoryValidation
	}
	return false
}

// IsNetwork checks if an error is a network error.
func IsNetwork(err error) bool {
	var cat Categorized
	if errors.As(err, &cat) {
		return cat.Category() == CategoryNetwork
	}
	return false
}

// Wrap adds context to an error.
func Wrap(err error, msg string) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%s: %w", msg, err)
}

// Wrapf adds formatted context to an error.
func Wrapf(err error, format string, args ...any) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%s: %w", fmt.Sprintf(format, args...), err)
}

// New creates a simple error.
func New(msg string) error {
	return errors.New(msg)
}

// Newf creates a formatted error.
func Newf(format string, args ...any) error {
	return fmt.Errorf(format, args...)
}

// Is is errors.Is.
func Is(err, target error) bool {
	return errors.Is(err, target)
}

// As is errors.As.
func As(err error, target any) bool {
	return errors.As(err, target)
}

// Join combines multiple errors into one.
func Join(errs ...error) error {
	return errors.Join(errs...)
}

// ValidationError represents input validation failures.
type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("validation error: %s %s", e.Field, e.Message)
}

func (e *ValidationError) Category() Category {
	return CategoryValidation
}

// NewValidationError creates a new validation error.
func NewValidationError(field, message string) *ValidationError {
	return &ValidationError{
		Field:   field,
		Message: message,
	}
}

// InternalError represents unexpected internal failures.
type InternalError struct {
	Operation string
	Cause     error
}

func (e *InternalError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("internal error in %s: %v", e.Operation, e.Cause)
	}
	return fmt.Sprintf("internal error in %s", e.Operation)
}

func (e *InternalError) Category() Category {
	return CategoryInternal
}

func (e *InternalError) Unwrap() error {
	return e.Cause
}

// NewInternalError creates a new internal error.
func NewInternalError(operation string, cause error) *InternalError {
	return &InternalError{
		Operation: operation,
		Cause:     cause,
	}
}
