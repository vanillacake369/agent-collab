// Package store provides interfaces and implementations for resource storage.
// It follows the Kubernetes-style informer/watch pattern for reactive programming.
package store

import (
	"context"
	"errors"

	v1 "agent-collab/src/api/v1"
)

// Common errors returned by store operations.
var (
	// ErrNotFound indicates the requested resource does not exist.
	ErrNotFound = errors.New("resource not found")

	// ErrAlreadyExists indicates a resource with the same name already exists.
	ErrAlreadyExists = errors.New("resource already exists")

	// ErrConflict indicates a concurrent modification conflict.
	ErrConflict = errors.New("resource version conflict")

	// ErrStoreClosed indicates the store has been closed.
	ErrStoreClosed = errors.New("store is closed")
)

// Object represents a storable resource with metadata.
type Object interface {
	v1.Object
	DeepCopy() Object
}

// ResourceStore provides CRUD operations for resources with watch support.
// It follows the Kubernetes API server storage semantics.
type ResourceStore[T Object] interface {
	// Create stores a new resource. Returns ErrAlreadyExists if a resource
	// with the same name already exists.
	Create(ctx context.Context, obj T) error

	// Update modifies an existing resource. Returns ErrNotFound if the resource
	// doesn't exist, or ErrConflict if the resource version doesn't match.
	Update(ctx context.Context, obj T) error

	// Delete removes a resource by name. Returns ErrNotFound if the resource
	// doesn't exist.
	Delete(ctx context.Context, name string) error

	// Get retrieves a resource by name. Returns ErrNotFound if not found.
	Get(ctx context.Context, name string) (T, error)

	// List returns all resources matching the optional selector.
	List(ctx context.Context, opts ListOptions) ([]T, error)

	// Watch returns a watcher that delivers events for matching resources.
	// The watcher should be closed when no longer needed.
	Watch(ctx context.Context, opts WatchOptions) (Watcher[T], error)

	// Close shuts down the store and releases resources.
	Close() error
}

// ListOptions configures a List operation.
type ListOptions struct {
	// LabelSelector filters resources by labels.
	// Empty selector matches all resources.
	LabelSelector map[string]string

	// Limit is the maximum number of resources to return.
	// Zero means no limit.
	Limit int

	// Continue is a token for pagination.
	Continue string
}

// WatchOptions configures a Watch operation.
type WatchOptions struct {
	// LabelSelector filters resources by labels.
	LabelSelector map[string]string

	// ResourceVersion is the starting point for the watch.
	// Empty string means watch from the current version.
	ResourceVersion string

	// SendInitialEvents sends ADDED events for existing resources.
	SendInitialEvents bool
}

// Watcher delivers resource events.
type Watcher[T Object] interface {
	// ResultChan returns a channel that receives watch events.
	// The channel is closed when the watcher is stopped.
	ResultChan() <-chan WatchEvent[T]

	// Stop closes the watcher. After Stop, ResultChan will be closed.
	// It's safe to call Stop multiple times.
	Stop()
}

// WatchEvent represents a change to a watched resource.
type WatchEvent[T Object] struct {
	// Type is the type of event (Added, Modified, Deleted).
	Type v1.EventType

	// Object is the resource after the change.
	// For Deleted events, this is the last known state.
	Object T
}

// Indexer provides fast lookups by indexed fields.
type Indexer[T Object] interface {
	// AddIndexer adds a new index with the given name and key function.
	AddIndexer(name string, keyFunc IndexKeyFunc[T]) error

	// ByIndex returns all objects that match the given index value.
	ByIndex(indexName, indexValue string) ([]T, error)

	// IndexKeys returns all index keys for the given index name.
	IndexKeys(indexName string) ([]string, error)
}

// IndexKeyFunc returns the index keys for an object.
// An object can have multiple keys for the same index.
type IndexKeyFunc[T Object] func(obj T) []string

// TransactionalStore supports atomic batch operations.
type TransactionalStore[T Object] interface {
	ResourceStore[T]

	// Transaction executes multiple operations atomically.
	// If the function returns an error, all operations are rolled back.
	Transaction(ctx context.Context, fn func(tx Transaction[T]) error) error
}

// Transaction represents an atomic batch of operations.
type Transaction[T Object] interface {
	// Create adds a new resource within the transaction.
	Create(obj T) error

	// Update modifies an existing resource within the transaction.
	Update(obj T) error

	// Delete removes a resource within the transaction.
	Delete(name string) error

	// Get retrieves a resource within the transaction.
	Get(name string) (T, error)
}

// EventHandler processes watch events.
// Used by informers and controllers.
type EventHandler[T Object] interface {
	// OnAdd is called when a resource is created.
	OnAdd(obj T)

	// OnUpdate is called when a resource is modified.
	OnUpdate(oldObj, newObj T)

	// OnDelete is called when a resource is deleted.
	OnDelete(obj T)
}

// EventHandlerFuncs is an adapter to use functions as EventHandler.
type EventHandlerFuncs[T Object] struct {
	AddFunc    func(obj T)
	UpdateFunc func(oldObj, newObj T)
	DeleteFunc func(obj T)
}

// OnAdd calls AddFunc if set.
func (f EventHandlerFuncs[T]) OnAdd(obj T) {
	if f.AddFunc != nil {
		f.AddFunc(obj)
	}
}

// OnUpdate calls UpdateFunc if set.
func (f EventHandlerFuncs[T]) OnUpdate(oldObj, newObj T) {
	if f.UpdateFunc != nil {
		f.UpdateFunc(oldObj, newObj)
	}
}

// OnDelete calls DeleteFunc if set.
func (f EventHandlerFuncs[T]) OnDelete(obj T) {
	if f.DeleteFunc != nil {
		f.DeleteFunc(obj)
	}
}
