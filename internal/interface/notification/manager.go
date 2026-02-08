package notification

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sync"
	"time"
)

// Manager manages notifications and human-in-the-loop interactions.
type Manager struct {
	mu sync.RWMutex

	// Pending notifications awaiting response
	pending map[string]*Notification

	// Notification history
	history     []*Notification
	maxHistory  int

	// Handlers
	notifiers []Notifier
	onResponse ResponseHandler

	// Configuration
	defaultTimeout time.Duration

	// Background processing
	ctx    context.Context
	cancel context.CancelFunc
}

// Notifier sends notifications through a specific channel.
type Notifier interface {
	Name() string
	Send(ctx context.Context, n *Notification) error
	SupportsResponse() bool
}

// NewManager creates a new notification manager.
func NewManager() *Manager {
	ctx, cancel := context.WithCancel(context.Background())

	m := &Manager{
		pending:        make(map[string]*Notification),
		history:        make([]*Notification, 0, 100),
		maxHistory:     100,
		notifiers:      make([]Notifier, 0),
		defaultTimeout: 5 * time.Minute,
		ctx:            ctx,
		cancel:         cancel,
	}

	go m.cleanupLoop()

	return m
}

// Close stops the manager.
func (m *Manager) Close() error {
	m.cancel()
	return nil
}

// AddNotifier adds a notification channel.
func (m *Manager) AddNotifier(n Notifier) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.notifiers = append(m.notifiers, n)
}

// SetResponseHandler sets the handler for user responses.
func (m *Manager) SetResponseHandler(handler ResponseHandler) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.onResponse = handler
}

// SetDefaultTimeout sets the default timeout for notifications.
func (m *Manager) SetDefaultTimeout(d time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.defaultTimeout = d
}

// Notify sends a notification through all channels.
func (m *Manager) Notify(ctx context.Context, n *Notification) error {
	m.mu.Lock()

	// Generate ID if not set
	if n.ID == "" {
		n.ID = generateNotificationID()
	}

	// Set timestamps
	if n.CreatedAt.IsZero() {
		n.CreatedAt = time.Now()
	}
	if n.ExpiresAt.IsZero() {
		n.ExpiresAt = n.CreatedAt.Add(m.defaultTimeout)
	}

	// Store if actions are available (awaiting response)
	if len(n.Actions) > 0 {
		m.pending[n.ID] = n
	}

	// Add to history
	m.history = append(m.history, n)
	if len(m.history) > m.maxHistory {
		m.history = m.history[1:]
	}

	notifiers := make([]Notifier, len(m.notifiers))
	copy(notifiers, m.notifiers)
	m.mu.Unlock()

	// Send through all notifiers
	var lastErr error
	for _, notifier := range notifiers {
		if err := notifier.Send(ctx, n); err != nil {
			lastErr = err
		}
	}

	return lastErr
}

// NotifyLockConflict creates and sends a lock conflict notification.
func (m *Manager) NotifyLockConflict(ctx context.Context, details *LockConflictDetails) (*Notification, error) {
	n := &Notification{
		Category: CategoryLockConflict,
		Priority: PriorityHigh,
		Title:    "Lock Conflict Detected",
		Message: fmt.Sprintf(
			"Agent '%s' wants to modify %s, but '%s' is currently editing overlapping code.",
			details.RequestedBy,
			details.RequestedTarget,
			details.HeldBy,
		),
		Details: map[string]any{
			"file_path":          details.FilePath,
			"requested_lock_id":  details.RequestedLockID,
			"conflicting_lock_id": details.ConflictingLockID,
			"overlap_type":       details.OverlapType,
		},
		Actions: []Action{
			{ID: "wait", Label: "Wait", Description: "Wait for the other agent to finish", IsDefault: true},
			{ID: "force", Label: "Force Acquire", Description: "Force acquire the lock (may cause conflicts)", IsDangerous: true},
			{ID: "cancel", Label: "Cancel", Description: "Cancel the lock request"},
		},
	}

	if err := m.Notify(ctx, n); err != nil {
		return nil, err
	}

	return n, nil
}

// NotifySyncConflict creates and sends a sync conflict notification.
func (m *Manager) NotifySyncConflict(ctx context.Context, details *SyncConflictDetails) (*Notification, error) {
	n := &Notification{
		Category: CategorySyncConflict,
		Priority: PriorityHigh,
		Title:    "Concurrent Modification Conflict",
		Message: fmt.Sprintf(
			"File '%s' was modified by both '%s' (local) and '%s' (remote) simultaneously.",
			details.FilePath,
			details.LocalAgent,
			details.RemoteAgent,
		),
		Details: map[string]any{
			"file_path":     details.FilePath,
			"local_change":  details.LocalChange,
			"remote_change": details.RemoteChange,
		},
		Actions: []Action{
			{ID: "keep_local", Label: "Keep Local", Description: "Keep local changes"},
			{ID: "keep_remote", Label: "Keep Remote", Description: "Accept remote changes"},
			{ID: "merge", Label: "Merge", Description: "Attempt to merge both changes", IsDefault: true},
			{ID: "review", Label: "Review", Description: "Open in editor for manual resolution"},
		},
	}

	if err := m.Notify(ctx, n); err != nil {
		return nil, err
	}

	return n, nil
}

// NotifyNegotiationTimeout creates and sends a negotiation timeout notification.
func (m *Manager) NotifyNegotiationTimeout(ctx context.Context, details *NegotiationDetails) (*Notification, error) {
	n := &Notification{
		Category: CategoryNegotiation,
		Priority: PriorityCritical,
		Title:    "Negotiation Requires Human Decision",
		Message: fmt.Sprintf(
			"Agent negotiation failed to resolve: %s. Human intervention required.",
			details.Reason,
		),
		Details: map[string]any{
			"session_id":   details.SessionID,
			"participants": details.Participants,
		},
		Actions: []Action{
			{ID: "approve", Label: "Approve", Description: "Approve the requested action"},
			{ID: "reject", Label: "Reject", Description: "Reject the requested action"},
			{ID: "defer", Label: "Defer", Description: "Defer decision for later"},
		},
	}

	if err := m.Notify(ctx, n); err != nil {
		return nil, err
	}

	return n, nil
}

// Respond records a user response to a notification.
func (m *Manager) Respond(notificationID string, response *Response) error {
	m.mu.Lock()

	n, exists := m.pending[notificationID]
	if !exists {
		m.mu.Unlock()
		return fmt.Errorf("notification not found or expired: %s", notificationID)
	}

	// Validate action
	validAction := false
	for _, action := range n.Actions {
		if action.ID == response.ActionID {
			validAction = true
			break
		}
	}
	if !validAction {
		m.mu.Unlock()
		return fmt.Errorf("invalid action: %s", response.ActionID)
	}

	// Record response
	response.RespondedAt = time.Now()
	n.Response = response
	n.Acknowledged = true

	delete(m.pending, notificationID)

	handler := m.onResponse
	m.mu.Unlock()

	// Call response handler
	if handler != nil {
		return handler(n, response)
	}

	return nil
}

// GetPending returns all pending notifications.
func (m *Manager) GetPending() []*Notification {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*Notification, 0, len(m.pending))
	for _, n := range m.pending {
		result = append(result, n)
	}
	return result
}

// GetHistory returns recent notification history.
func (m *Manager) GetHistory(count int) []*Notification {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if count <= 0 || count > len(m.history) {
		count = len(m.history)
	}

	result := make([]*Notification, count)
	copy(result, m.history[len(m.history)-count:])
	return result
}

// Acknowledge marks a notification as acknowledged without a specific action.
func (m *Manager) Acknowledge(notificationID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	n, exists := m.pending[notificationID]
	if !exists {
		return fmt.Errorf("notification not found: %s", notificationID)
	}

	n.Acknowledged = true
	delete(m.pending, notificationID)
	return nil
}

// cleanupLoop removes expired notifications.
func (m *Manager) cleanupLoop() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-m.ctx.Done():
			return
		case <-ticker.C:
			m.cleanupExpired()
		}
	}
}

// cleanupExpired removes expired pending notifications.
func (m *Manager) cleanupExpired() {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	for id, n := range m.pending {
		if !n.ExpiresAt.IsZero() && now.After(n.ExpiresAt) {
			delete(m.pending, id)
		}
	}
}

// generateNotificationID generates a unique notification ID.
func generateNotificationID() string {
	bytes := make([]byte, 8)
	rand.Read(bytes)
	return "notif-" + hex.EncodeToString(bytes)
}
