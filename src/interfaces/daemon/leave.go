package daemon

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"
)

// LeaveState represents the state of the leave process.
type LeaveState string

const (
	LeaveStateIdle       LeaveState = "idle"
	LeaveStateInitiated  LeaveState = "initiated"
	LeaveStateReleasingL LeaveState = "releasing_locks"
	LeaveStateSyncing    LeaveState = "syncing"
	LeaveStateDisconnect LeaveState = "disconnecting"
	LeaveStateCompleted  LeaveState = "completed"
	LeaveStateFailed     LeaveState = "failed"
)

// LeaveStateMachine manages the graceful cluster leave process.
type LeaveStateMachine struct {
	mu            sync.RWMutex
	state         LeaveState
	startedAt     time.Time
	completedAt   time.Time
	currentStep   string
	error         string
	locksReleased int
	contextSynced bool
}

// NewLeaveStateMachine creates a new leave state machine.
func NewLeaveStateMachine() *LeaveStateMachine {
	return &LeaveStateMachine{
		state: LeaveStateIdle,
	}
}

// State returns the current state.
func (m *LeaveStateMachine) State() LeaveState {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.state
}

// Start initiates the leave process.
func (m *LeaveStateMachine) Start() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.state != LeaveStateIdle && m.state != LeaveStateCompleted && m.state != LeaveStateFailed {
		return fmt.Errorf("leave already in progress: %s", m.state)
	}

	m.state = LeaveStateInitiated
	m.startedAt = time.Now()
	m.currentStep = "Initiating leave process"
	m.error = ""
	m.locksReleased = 0
	m.contextSynced = false
	return nil
}

// TransitionTo moves to the next state.
func (m *LeaveStateMachine) TransitionTo(state LeaveState, step string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.state = state
	m.currentStep = step
}

// SetLocksReleased sets the number of locks released.
func (m *LeaveStateMachine) SetLocksReleased(count int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.locksReleased = count
}

// SetContextSynced marks context as synced.
func (m *LeaveStateMachine) SetContextSynced() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.contextSynced = true
}

// Complete marks the leave process as completed.
func (m *LeaveStateMachine) Complete() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.state = LeaveStateCompleted
	m.completedAt = time.Now()
	m.currentStep = "Leave completed successfully"
}

// Fail marks the leave process as failed.
func (m *LeaveStateMachine) Fail(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.state = LeaveStateFailed
	m.completedAt = time.Now()
	m.error = err.Error()
	m.currentStep = "Leave failed"
}

// Status returns the current status of the leave process.
func (m *LeaveStateMachine) Status() LeaveStatusResponse {
	m.mu.RLock()
	defer m.mu.RUnlock()

	resp := LeaveStatusResponse{
		State:         string(m.state),
		CurrentStep:   m.currentStep,
		LocksReleased: m.locksReleased,
		ContextSynced: m.contextSynced,
	}

	if !m.startedAt.IsZero() {
		resp.StartedAt = m.startedAt.Format(time.RFC3339)
	}
	if !m.completedAt.IsZero() {
		resp.CompletedAt = m.completedAt.Format(time.RFC3339)
		resp.Duration = m.completedAt.Sub(m.startedAt).String()
	}
	if m.error != "" {
		resp.Error = m.error
	}

	return resp
}

// LeaveRequest is the request to leave the cluster.
type LeaveRequest struct {
	Force   bool `json:"force"`   // Force leave without graceful cleanup
	Timeout int  `json:"timeout"` // Timeout in seconds (default: 30)
}

// LeaveStatusResponse is the response for leave status.
type LeaveStatusResponse struct {
	State         string `json:"state"`
	CurrentStep   string `json:"current_step"`
	StartedAt     string `json:"started_at,omitempty"`
	CompletedAt   string `json:"completed_at,omitempty"`
	Duration      string `json:"duration,omitempty"`
	LocksReleased int    `json:"locks_released"`
	ContextSynced bool   `json:"context_synced"`
	Error         string `json:"error,omitempty"`
}

// LeaveResponse is the response for leave request.
type LeaveResponse struct {
	Success bool                `json:"success"`
	Message string              `json:"message,omitempty"`
	Error   string              `json:"error,omitempty"`
	Status  LeaveStatusResponse `json:"status"`
}

// leaveStateMachine is the server's leave state machine.
var leaveStateMachine = NewLeaveStateMachine()

// handleLeave handles the /leave endpoint.
func (s *Server) handleLeave(w http.ResponseWriter, r *http.Request) {
	var req LeaveRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		// Default values if no body
		req.Timeout = 30
	}

	// Check current state
	currentState := leaveStateMachine.State()
	if currentState != LeaveStateIdle && currentState != LeaveStateCompleted && currentState != LeaveStateFailed {
		json.NewEncoder(w).Encode(LeaveResponse{
			Success: false,
			Error:   "leave already in progress",
			Status:  leaveStateMachine.Status(),
		})
		return
	}

	// Start leave process
	if err := leaveStateMachine.Start(); err != nil {
		json.NewEncoder(w).Encode(LeaveResponse{
			Success: false,
			Error:   err.Error(),
			Status:  leaveStateMachine.Status(),
		})
		return
	}

	// Publish leave initiated event
	s.PublishEvent(NewEvent(EventType("leave_initiated"), nil))

	// Run leave process asynchronously
	go s.executeLeaveProcess(req)

	json.NewEncoder(w).Encode(LeaveResponse{
		Success: true,
		Message: "Leave process initiated",
		Status:  leaveStateMachine.Status(),
	})
}

// handleLeaveStatus handles the /leave/status endpoint.
func (s *Server) handleLeaveStatus(w http.ResponseWriter, _ *http.Request) {
	json.NewEncoder(w).Encode(leaveStateMachine.Status())
}

// executeLeaveProcess runs the leave process.
func (s *Server) executeLeaveProcess(_ LeaveRequest) {
	// Step 1: Release all locks
	leaveStateMachine.TransitionTo(LeaveStateReleasingL, "Releasing all locks")
	locksReleased := 0

	lockService := s.app.LockService()
	if lockService != nil {
		locks := lockService.ListLocks()
		for _, l := range locks {
			if err := lockService.ReleaseLock(s.ctx, l.ID); err == nil {
				locksReleased++
			}
		}
		leaveStateMachine.SetLocksReleased(locksReleased)
	}

	// Step 2: Sync any pending context
	leaveStateMachine.TransitionTo(LeaveStateSyncing, "Syncing pending context")

	syncManager := s.app.SyncManager()
	if syncManager != nil {
		// Give time for pending syncs to complete
		time.Sleep(500 * time.Millisecond)
		leaveStateMachine.SetContextSynced()
	}

	// Step 3: Disconnect from peers
	leaveStateMachine.TransitionTo(LeaveStateDisconnect, "Disconnecting from peers")

	// Publish leave event to peers
	s.PublishEvent(NewEvent(EventType("peer_leaving"), map[string]any{
		"node_id": s.app.Node().ID().String(),
		"reason":  "graceful_leave",
	}))

	// Give time for event to propagate
	time.Sleep(200 * time.Millisecond)

	// Step 4: Mark as completed
	leaveStateMachine.Complete()

	// Publish leave completed event
	s.PublishEvent(NewEvent(EventType("leave_completed"), map[string]any{
		"locks_released": locksReleased,
	}))
}
