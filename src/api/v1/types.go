// Package v1 contains Kubernetes-style API types for the agent-collab system.
// These types follow the Resource/Controller pattern with clear Spec/Status separation.
package v1

import (
	"encoding/json"
	"time"
)

// GroupVersion is the identifier for the API version.
const GroupVersion = "agent-collab.io/v1"

// TypeMeta describes an individual resource in the system with metadata about the type.
type TypeMeta struct {
	// Kind is a string value representing the resource type.
	// Examples: "Lock", "Context", "Agent"
	Kind string `json:"kind"`

	// APIVersion defines the versioned schema of this resource.
	// Example: "agent-collab.io/v1"
	APIVersion string `json:"apiVersion"`
}

// ObjectMeta contains metadata that all persisted resources must have.
type ObjectMeta struct {
	// Name is the unique identifier within a namespace.
	// It must be unique among resources of the same kind.
	Name string `json:"name"`

	// UID is a unique identifier for this resource across time and space.
	// It is typically a UUID.
	UID string `json:"uid"`

	// ResourceVersion is an opaque value that changes whenever the resource is modified.
	// Used for optimistic concurrency control.
	ResourceVersion string `json:"resourceVersion"`

	// Labels are key-value pairs that can be used to organize and select resources.
	Labels map[string]string `json:"labels,omitempty"`

	// Annotations are key-value pairs for storing arbitrary metadata.
	Annotations map[string]string `json:"annotations,omitempty"`

	// CreationTimestamp is the timestamp when this resource was created.
	CreationTimestamp time.Time `json:"creationTimestamp"`

	// DeletionTimestamp is set when a graceful deletion is requested.
	// The resource will be deleted after finalizers complete.
	DeletionTimestamp *time.Time `json:"deletionTimestamp,omitempty"`

	// Finalizers are identifiers of controllers that need to perform
	// cleanup before the resource can be deleted.
	Finalizers []string `json:"finalizers,omitempty"`

	// OwnerReferences lists resources that "own" this resource.
	// When the owner is deleted, this resource may be garbage collected.
	OwnerReferences []OwnerReference `json:"ownerReferences,omitempty"`
}

// OwnerReference contains information about the owning resource.
type OwnerReference struct {
	// Kind of the referent.
	Kind string `json:"kind"`

	// Name of the referent.
	Name string `json:"name"`

	// UID of the referent.
	UID string `json:"uid"`

	// Controller indicates if this reference is the managing controller.
	Controller *bool `json:"controller,omitempty"`

	// BlockOwnerDeletion prevents deletion of the owner until this resource is removed.
	BlockOwnerDeletion *bool `json:"blockOwnerDeletion,omitempty"`
}

// ConditionStatus represents the status of a condition.
type ConditionStatus string

const (
	// ConditionTrue means the condition is satisfied.
	ConditionTrue ConditionStatus = "True"

	// ConditionFalse means the condition is not satisfied.
	ConditionFalse ConditionStatus = "False"

	// ConditionUnknown means the condition status cannot be determined.
	ConditionUnknown ConditionStatus = "Unknown"
)

// Condition represents an observation of a resource's state at a point in time.
type Condition struct {
	// Type is the type of the condition.
	// Examples: "Ready", "Synced", "Available"
	Type string `json:"type"`

	// Status is the status of the condition.
	// One of: "True", "False", "Unknown".
	Status ConditionStatus `json:"status"`

	// LastTransitionTime is the last time the condition transitioned from one status to another.
	LastTransitionTime time.Time `json:"lastTransitionTime"`

	// Reason is a brief machine-readable explanation for the condition's last transition.
	Reason string `json:"reason,omitempty"`

	// Message is a human-readable message indicating details about the transition.
	Message string `json:"message,omitempty"`

	// ObservedGeneration represents the generation of the resource that was observed.
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
}

// Duration is a wrapper around time.Duration for JSON serialization.
type Duration struct {
	time.Duration
}

// MarshalJSON implements json.Marshaler.
func (d Duration) MarshalJSON() ([]byte, error) {
	return json.Marshal(d.String())
}

// UnmarshalJSON implements json.Unmarshaler.
func (d *Duration) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return err
	}
	parsed, err := time.ParseDuration(s)
	if err != nil {
		return err
	}
	d.Duration = parsed
	return nil
}

// ListMeta describes metadata for a list of resources.
type ListMeta struct {
	// ResourceVersion is the version of the list.
	ResourceVersion string `json:"resourceVersion,omitempty"`

	// Continue is the token for retrieving the next page of results.
	Continue string `json:"continue,omitempty"`

	// RemainingItemCount is the number of items remaining in the list (if known).
	RemainingItemCount *int64 `json:"remainingItemCount,omitempty"`
}

// EventType defines the type of watch event.
type EventType string

const (
	// EventAdded indicates a resource was added.
	EventAdded EventType = "ADDED"

	// EventModified indicates a resource was modified.
	EventModified EventType = "MODIFIED"

	// EventDeleted indicates a resource was deleted.
	EventDeleted EventType = "DELETED"

	// EventBookmark is a periodic update indicating the current resourceVersion.
	EventBookmark EventType = "BOOKMARK"

	// EventError indicates an error occurred during watch.
	EventError EventType = "ERROR"
)

// WatchEvent represents a change to a watched resource.
type WatchEvent struct {
	// Type is the type of event.
	Type EventType `json:"type"`

	// Object is the resource that changed.
	// The concrete type depends on the watched resource type.
	Object json.RawMessage `json:"object"`
}

// Status is a common return type for API operations that don't return a resource.
type Status struct {
	TypeMeta `json:",inline"`

	// Status indicates success or failure.
	// One of: "Success", "Failure".
	Status string `json:"status,omitempty"`

	// Message is a human-readable description of the status.
	Message string `json:"message,omitempty"`

	// Reason is a machine-readable explanation for the status.
	Reason string `json:"reason,omitempty"`

	// Code is the HTTP status code.
	Code int32 `json:"code,omitempty"`
}

// Object is the interface all API resources must implement.
type Object interface {
	GetObjectMeta() *ObjectMeta
	GetTypeMeta() *TypeMeta
}

// GetObjectMeta returns the ObjectMeta for TypeMeta-embedded structs.
// This is a helper for types that embed TypeMeta and ObjectMeta.
func (t *TypeMeta) GetTypeMeta() *TypeMeta {
	return t
}
