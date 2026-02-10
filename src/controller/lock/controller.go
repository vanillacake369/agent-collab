// Package lock provides a controller for Lock resources.
package lock

import (
	"context"
	"encoding/json"
	"fmt"
	"sync/atomic"
	"time"

	v1 "agent-collab/src/api/v1"
	"agent-collab/src/controller"
	"agent-collab/src/plugin"
	"agent-collab/src/store"
)

// lockWrapper wraps v1.Lock to implement store.Object
type lockWrapper struct {
	*v1.Lock
}

func (l *lockWrapper) DeepCopy() store.Object {
	if l == nil || l.Lock == nil {
		return nil
	}
	copy := *l.Lock
	return &lockWrapper{Lock: &copy}
}

// Controller manages Lock resources.
type Controller struct {
	*controller.GenericController[*lockWrapper]
	store   store.ResourceStore[*lockWrapper]
	network plugin.NetworkPlugin

	// Fencing token counter
	fencingToken atomic.Uint64

	// Local agent ID
	agentID string

	// Topics for lock messages
	intentTopic  string
	lockTopic    string
	releaseTopic string
}

// NewController creates a new Lock controller.
func NewController(
	lockStore store.ResourceStore[*lockWrapper],
	network plugin.NetworkPlugin,
	agentID string,
	projectID string,
) *Controller {
	c := &Controller{
		store:        lockStore,
		network:      network,
		agentID:      agentID,
		intentTopic:  fmt.Sprintf("/agent-collab/%s/lock/intent", projectID),
		lockTopic:    fmt.Sprintf("/agent-collab/%s/lock/acquire", projectID),
		releaseTopic: fmt.Sprintf("/agent-collab/%s/lock/release", projectID),
	}

	// Create the generic controller with the reconciler
	c.GenericController = controller.NewGenericController(
		"lock",
		controller.ReconcilerFunc(c.Reconcile),
		lockStore,
		controller.DefaultOptions("lock"),
	)

	return c
}

// Start begins the controller and sets up network subscriptions.
func (c *Controller) Start(ctx context.Context) error {
	// Subscribe to lock topics
	if c.network != nil {
		if err := c.subscribeToTopics(ctx); err != nil {
			return fmt.Errorf("failed to subscribe to topics: %w", err)
		}
	}

	// Start the generic controller
	return c.GenericController.Start(ctx)
}

// Reconcile handles the reconciliation of a Lock resource.
func (c *Controller) Reconcile(ctx context.Context, req controller.Request) (controller.Result, error) {
	lock, err := c.store.Get(ctx, req.Name)
	if err != nil {
		if err == store.ErrNotFound {
			// Lock was deleted, nothing to do
			return controller.Result{}, nil
		}
		return controller.Result{}, err
	}

	switch lock.Status.Phase {
	case v1.LockPhasePending:
		return c.reconcilePending(ctx, lock)

	case v1.LockPhaseNegotiating:
		return c.reconcileNegotiating(ctx, lock)

	case v1.LockPhaseActive:
		return c.reconcileActive(ctx, lock)

	case v1.LockPhaseReleasing:
		return c.reconcileReleasing(ctx, lock)

	case v1.LockPhaseExpired, v1.LockPhaseFailed, v1.LockPhaseReleased:
		// Terminal states, no action needed
		return controller.Result{}, nil
	}

	return controller.Result{}, nil
}

func (c *Controller) reconcilePending(ctx context.Context, lock *lockWrapper) (controller.Result, error) {
	// Check if we own this lock request
	if lock.Spec.HolderID != c.agentID {
		// Not our lock, skip
		return controller.Result{}, nil
	}

	// Start negotiation - broadcast intent
	intent := LockIntent{
		LockName:  lock.Name,
		HolderID:  lock.Spec.HolderID,
		Target:    lock.Spec.Target,
		Intention: lock.Spec.Intention,
		Priority:  lock.Spec.Priority,
		Timestamp: time.Now(),
	}

	if err := c.broadcastIntent(ctx, intent); err != nil {
		return controller.Result{Requeue: true}, err
	}

	// Move to negotiating phase
	lock.Status.Phase = v1.LockPhaseNegotiating
	lock.SetCondition(v1.LockConditionNegotiating, v1.ConditionTrue, "IntentBroadcast", "Lock intent broadcasted")

	if err := c.store.Update(ctx, lock); err != nil {
		return controller.Result{}, err
	}

	// Requeue after negotiation timeout
	return controller.Result{RequeueAfter: 500 * time.Millisecond}, nil
}

func (c *Controller) reconcileNegotiating(ctx context.Context, lock *lockWrapper) (controller.Result, error) {
	// Check for conflicts
	if lock.HasConflict() {
		lock.Status.Phase = v1.LockPhaseFailed
		lock.SetCondition(v1.LockConditionConflict, v1.ConditionTrue, "ConflictDetected", "Lock conflict detected")
		lock.Status.Message = "Lock acquisition failed due to conflicts"

		if err := c.store.Update(ctx, lock); err != nil {
			return controller.Result{}, err
		}
		return controller.Result{}, nil
	}

	// No conflicts, acquire the lock
	token := c.fencingToken.Add(1)
	now := time.Now()
	expires := now.Add(lock.Spec.TTL.Duration)

	lock.Status.Phase = v1.LockPhaseActive
	lock.Status.FencingToken = token
	lock.Status.AcquiredAt = &now
	lock.Status.ExpiresAt = &expires
	lock.SetCondition(v1.LockConditionReady, v1.ConditionTrue, "LockAcquired", "Lock acquired successfully")

	if err := c.store.Update(ctx, lock); err != nil {
		return controller.Result{}, err
	}

	// Broadcast lock acquisition
	if c.network != nil {
		c.broadcastAcquisition(ctx, lock)
	}

	// Schedule check before expiration
	remaining := time.Until(expires)
	return controller.Result{RequeueAfter: remaining - (remaining / 10)}, nil
}

func (c *Controller) reconcileActive(ctx context.Context, lock *lockWrapper) (controller.Result, error) {
	// Check if lock has expired
	if lock.IsExpired() {
		lock.Status.Phase = v1.LockPhaseExpired
		lock.SetCondition(v1.LockConditionReady, v1.ConditionFalse, "LockExpired", "Lock has expired")

		if err := c.store.Update(ctx, lock); err != nil {
			return controller.Result{}, err
		}

		// Broadcast release
		if c.network != nil {
			c.broadcastRelease(ctx, lock)
		}

		return controller.Result{}, nil
	}

	// Check if expiring soon (within 10% of TTL)
	if lock.Status.ExpiresAt != nil {
		remaining := time.Until(*lock.Status.ExpiresAt)
		threshold := lock.Spec.TTL.Duration / 10

		if remaining < threshold {
			lock.SetCondition(v1.LockConditionExpiring, v1.ConditionTrue, "ExpiringSoon", "Lock is about to expire")
			if err := c.store.Update(ctx, lock); err != nil {
				return controller.Result{}, err
			}
		}

		// Requeue before expiration
		return controller.Result{RequeueAfter: remaining}, nil
	}

	return controller.Result{}, nil
}

func (c *Controller) reconcileReleasing(ctx context.Context, lock *lockWrapper) (controller.Result, error) {
	// Broadcast release
	if c.network != nil {
		c.broadcastRelease(ctx, lock)
	}

	// Move to released
	lock.Status.Phase = v1.LockPhaseReleased
	lock.SetCondition(v1.LockConditionReady, v1.ConditionFalse, "LockReleased", "Lock has been released")

	if err := c.store.Update(ctx, lock); err != nil {
		return controller.Result{}, err
	}

	return controller.Result{}, nil
}

// Network message types

// LockIntent represents a lock intent message.
type LockIntent struct {
	LockName  string        `json:"lockName"`
	HolderID  string        `json:"holderId"`
	Target    v1.LockTarget `json:"target"`
	Intention string        `json:"intention"`
	Priority  int32         `json:"priority"`
	Timestamp time.Time     `json:"timestamp"`
}

// LockAcquisition represents a lock acquisition message.
type LockAcquisition struct {
	LockName     string        `json:"lockName"`
	HolderID     string        `json:"holderId"`
	Target       v1.LockTarget `json:"target"`
	FencingToken uint64        `json:"fencingToken"`
	AcquiredAt   time.Time     `json:"acquiredAt"`
	ExpiresAt    time.Time     `json:"expiresAt"`
}

// LockRelease represents a lock release message.
type LockRelease struct {
	LockName   string    `json:"lockName"`
	HolderID   string    `json:"holderId"`
	ReleasedAt time.Time `json:"releasedAt"`
}

func (c *Controller) subscribeToTopics(ctx context.Context) error {
	// Subscribe to intent topic
	intentCh, err := c.network.Subscribe(ctx, c.intentTopic)
	if err != nil {
		return err
	}
	go c.handleIntentMessages(ctx, intentCh)

	// Subscribe to lock topic
	lockCh, err := c.network.Subscribe(ctx, c.lockTopic)
	if err != nil {
		return err
	}
	go c.handleLockMessages(ctx, lockCh)

	// Subscribe to release topic
	releaseCh, err := c.network.Subscribe(ctx, c.releaseTopic)
	if err != nil {
		return err
	}
	go c.handleReleaseMessages(ctx, releaseCh)

	return nil
}

func (c *Controller) handleIntentMessages(ctx context.Context, ch <-chan plugin.Message) {
	for {
		select {
		case <-ctx.Done():
			return
		case msg, ok := <-ch:
			if !ok {
				return
			}
			var intent LockIntent
			if err := json.Unmarshal(msg.Data, &intent); err != nil {
				continue
			}
			c.processIntent(ctx, intent)
		}
	}
}

func (c *Controller) handleLockMessages(ctx context.Context, ch <-chan plugin.Message) {
	for {
		select {
		case <-ctx.Done():
			return
		case msg, ok := <-ch:
			if !ok {
				return
			}
			var acq LockAcquisition
			if err := json.Unmarshal(msg.Data, &acq); err != nil {
				continue
			}
			c.processAcquisition(ctx, acq)
		}
	}
}

func (c *Controller) handleReleaseMessages(ctx context.Context, ch <-chan plugin.Message) {
	for {
		select {
		case <-ctx.Done():
			return
		case msg, ok := <-ch:
			if !ok {
				return
			}
			var release LockRelease
			if err := json.Unmarshal(msg.Data, &release); err != nil {
				continue
			}
			c.processRelease(ctx, release)
		}
	}
}

func (c *Controller) processIntent(ctx context.Context, intent LockIntent) {
	// Check if this intent conflicts with any of our pending locks
	locks, _ := c.store.List(ctx, store.ListOptions{})
	for _, lock := range locks {
		if lock.Spec.HolderID != c.agentID {
			continue
		}
		if lock.Status.Phase != v1.LockPhaseNegotiating && lock.Status.Phase != v1.LockPhasePending {
			continue
		}
		if c.targetsOverlap(lock.Spec.Target, intent.Target) {
			// Conflict detected
			lock.Status.ConflictingLocks = append(lock.Status.ConflictingLocks, v1.LockConflict{
				LockName: intent.LockName,
				HolderID: intent.HolderID,
				Target:   intent.Target,
			})
			c.store.Update(ctx, lock)
			c.Enqueue(lock.Name)
		}
	}
}

func (c *Controller) processAcquisition(ctx context.Context, acq LockAcquisition) {
	// Record remote lock acquisition
	// This could be used to track active locks from other agents
}

func (c *Controller) processRelease(ctx context.Context, release LockRelease) {
	// Clear conflicts when a remote lock is released
	locks, _ := c.store.List(ctx, store.ListOptions{})
	for _, lock := range locks {
		for i, conflict := range lock.Status.ConflictingLocks {
			if conflict.LockName == release.LockName {
				lock.Status.ConflictingLocks = append(
					lock.Status.ConflictingLocks[:i],
					lock.Status.ConflictingLocks[i+1:]...,
				)
				c.store.Update(ctx, lock)
				c.Enqueue(lock.Name)
				break
			}
		}
	}
}

func (c *Controller) targetsOverlap(a, b v1.LockTarget) bool {
	if a.FilePath != b.FilePath {
		return false
	}
	if a.Type == v1.LockTargetTypeFile || b.Type == v1.LockTargetTypeFile {
		return true
	}
	// Check line range overlap
	if a.StartLine != 0 && b.StartLine != 0 {
		return a.StartLine <= b.EndLine && b.StartLine <= a.EndLine
	}
	// Check symbol overlap
	if a.Symbol != "" && b.Symbol != "" {
		return a.Symbol == b.Symbol
	}
	return true
}

func (c *Controller) broadcastIntent(ctx context.Context, intent LockIntent) error {
	data, err := json.Marshal(intent)
	if err != nil {
		return err
	}
	return c.network.Publish(ctx, c.intentTopic, data)
}

func (c *Controller) broadcastAcquisition(ctx context.Context, lock *lockWrapper) {
	acq := LockAcquisition{
		LockName:     lock.Name,
		HolderID:     lock.Spec.HolderID,
		Target:       lock.Spec.Target,
		FencingToken: lock.Status.FencingToken,
		AcquiredAt:   *lock.Status.AcquiredAt,
		ExpiresAt:    *lock.Status.ExpiresAt,
	}
	data, _ := json.Marshal(acq)
	c.network.Publish(ctx, c.lockTopic, data)
}

func (c *Controller) broadcastRelease(ctx context.Context, lock *lockWrapper) {
	release := LockRelease{
		LockName:   lock.Name,
		HolderID:   lock.Spec.HolderID,
		ReleasedAt: time.Now(),
	}
	data, _ := json.Marshal(release)
	c.network.Publish(ctx, c.releaseTopic, data)
}
