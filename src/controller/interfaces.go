// Package controller provides interfaces and utilities for building controllers
// that reconcile the desired state of resources with their actual state.
// This follows the Kubernetes controller pattern.
package controller

import (
	"context"
	"time"
)

// Request contains the information needed to reconcile a resource.
type Request struct {
	// Name is the name of the resource to reconcile.
	Name string
}

// Result contains the result of a Reconcile operation.
type Result struct {
	// Requeue indicates if the request should be requeued for reconciliation.
	Requeue bool

	// RequeueAfter indicates when to requeue the request.
	// If this is set, Requeue is ignored.
	RequeueAfter time.Duration
}

// Reconciler reconciles a resource to its desired state.
type Reconciler interface {
	// Reconcile performs the reconciliation for a single resource.
	// It returns a Result indicating if/when to requeue.
	Reconcile(ctx context.Context, req Request) (Result, error)
}

// ReconcilerFunc is a function that implements Reconciler.
type ReconcilerFunc func(ctx context.Context, req Request) (Result, error)

// Reconcile implements Reconciler.
func (f ReconcilerFunc) Reconcile(ctx context.Context, req Request) (Result, error) {
	return f(ctx, req)
}

// Controller runs a reconciliation loop for a resource type.
type Controller interface {
	// Start begins the reconciliation loop.
	Start(ctx context.Context) error

	// Enqueue adds a resource to the work queue for reconciliation.
	Enqueue(name string)

	// EnqueueAfter adds a resource to the work queue after a delay.
	EnqueueAfter(name string, delay time.Duration)
}

// Options configures a controller.
type Options struct {
	// Name is the name of the controller.
	Name string

	// MaxConcurrentReconciles is the maximum number of concurrent reconciles.
	MaxConcurrentReconciles int

	// RateLimiter controls how often items can be requeued.
	RateLimiter RateLimiter

	// RecoverPanic indicates if the controller should recover from panics.
	RecoverPanic bool
}

// DefaultOptions returns the default controller options.
func DefaultOptions(name string) Options {
	return Options{
		Name:                    name,
		MaxConcurrentReconciles: 1,
		RateLimiter:             NewDefaultRateLimiter(),
		RecoverPanic:            true,
	}
}

// RateLimiter controls how often items can be requeued.
type RateLimiter interface {
	// When returns the delay before an item should be requeued.
	When(item string) time.Duration

	// Forget indicates that an item is finished being retried.
	Forget(item string)

	// NumRequeues returns the number of times an item has been requeued.
	NumRequeues(item string) int
}

// Source provides events to a controller.
type Source interface {
	// Start begins producing events.
	Start(ctx context.Context, handler EventHandler) error
}

// EventHandler handles events from a Source.
type EventHandler interface {
	// Create is called when a resource is created.
	Create(ctx context.Context, name string)

	// Update is called when a resource is updated.
	Update(ctx context.Context, name string)

	// Delete is called when a resource is deleted.
	Delete(ctx context.Context, name string)
}

// EventHandlerFuncs is an adapter to use functions as EventHandler.
type EventHandlerFuncs struct {
	CreateFunc func(ctx context.Context, name string)
	UpdateFunc func(ctx context.Context, name string)
	DeleteFunc func(ctx context.Context, name string)
}

// Create calls CreateFunc if set.
func (e EventHandlerFuncs) Create(ctx context.Context, name string) {
	if e.CreateFunc != nil {
		e.CreateFunc(ctx, name)
	}
}

// Update calls UpdateFunc if set.
func (e EventHandlerFuncs) Update(ctx context.Context, name string) {
	if e.UpdateFunc != nil {
		e.UpdateFunc(ctx, name)
	}
}

// Delete calls DeleteFunc if set.
func (e EventHandlerFuncs) Delete(ctx context.Context, name string) {
	if e.DeleteFunc != nil {
		e.DeleteFunc(ctx, name)
	}
}
