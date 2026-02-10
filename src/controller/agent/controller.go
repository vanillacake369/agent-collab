// Package agent provides a controller for Agent resources.
package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	v1 "agent-collab/src/api/v1"
	"agent-collab/src/controller"
	"agent-collab/src/plugin"
	"agent-collab/src/store"
)

// agentWrapper wraps v1.Agent to implement store.Object
type agentWrapper struct {
	*v1.Agent
}

func (a *agentWrapper) DeepCopy() store.Object {
	if a == nil || a.Agent == nil {
		return nil
	}
	copy := *a.Agent
	return &agentWrapper{Agent: &copy}
}

// Controller manages Agent resources.
type Controller struct {
	*controller.GenericController[*agentWrapper]
	store   store.ResourceStore[*agentWrapper]
	network plugin.NetworkPlugin

	// Local agent ID
	agentID string

	// Topics for agent messages
	heartbeatTopic string
	statusTopic    string

	// Heartbeat configuration
	heartbeatInterval time.Duration
	heartbeatTimeout  time.Duration
}

// NewController creates a new Agent controller.
func NewController(
	agentStore store.ResourceStore[*agentWrapper],
	network plugin.NetworkPlugin,
	agentID string,
	projectID string,
) *Controller {
	c := &Controller{
		store:             agentStore,
		network:           network,
		agentID:           agentID,
		heartbeatTopic:    fmt.Sprintf("/agent-collab/%s/agent/heartbeat", projectID),
		statusTopic:       fmt.Sprintf("/agent-collab/%s/agent/status", projectID),
		heartbeatInterval: 30 * time.Second,
		heartbeatTimeout:  90 * time.Second,
	}

	// Create the generic controller with the reconciler
	c.GenericController = controller.NewGenericController(
		"agent",
		controller.ReconcilerFunc(c.Reconcile),
		agentStore,
		controller.DefaultOptions("agent"),
	)

	return c
}

// Start begins the controller and sets up network subscriptions.
func (c *Controller) Start(ctx context.Context) error {
	// Subscribe to agent topics
	if c.network != nil {
		if err := c.subscribeToTopics(ctx); err != nil {
			return fmt.Errorf("failed to subscribe to topics: %w", err)
		}

		// Start heartbeat sender for local agent
		go c.heartbeatLoop(ctx)
	}

	// Start the generic controller
	return c.GenericController.Start(ctx)
}

// Reconcile handles the reconciliation of an Agent resource.
func (c *Controller) Reconcile(ctx context.Context, req controller.Request) (controller.Result, error) {
	agent, err := c.store.Get(ctx, req.Name)
	if err != nil {
		if err == store.ErrNotFound {
			return controller.Result{}, nil
		}
		return controller.Result{}, err
	}

	switch agent.Status.Phase {
	case v1.AgentPhasePending:
		return c.reconcilePending(ctx, agent)

	case v1.AgentPhaseConnecting:
		return c.reconcileConnecting(ctx, agent)

	case v1.AgentPhaseOnline, v1.AgentPhaseBusy:
		return c.reconcileOnline(ctx, agent)

	case v1.AgentPhaseOffline:
		return c.reconcileOffline(ctx, agent)

	case v1.AgentPhaseTerminating:
		return c.reconcileTerminating(ctx, agent)

	case v1.AgentPhaseError:
		// Could implement error recovery here
		return controller.Result{}, nil
	}

	return controller.Result{}, nil
}

func (c *Controller) reconcilePending(ctx context.Context, agent *agentWrapper) (controller.Result, error) {
	// Start connecting
	agent.Status.Phase = v1.AgentPhaseConnecting
	if err := c.store.Update(ctx, agent); err != nil {
		return controller.Result{}, err
	}

	return controller.Result{Requeue: true}, nil
}

func (c *Controller) reconcileConnecting(ctx context.Context, agent *agentWrapper) (controller.Result, error) {
	// For local agent, immediately go online
	if agent.Spec.PeerID == c.agentID {
		now := time.Now()
		agent.Status.Phase = v1.AgentPhaseOnline
		agent.Status.ConnectedAt = &now
		agent.Status.LastHeartbeat = &now
		agent.Status.LastSeenAt = &now
		agent.SetCondition(v1.AgentConditionConnected, v1.ConditionTrue, "Connected", "Agent connected to cluster")
		agent.SetCondition(v1.AgentConditionReady, v1.ConditionTrue, "Ready", "Agent is ready")

		if err := c.store.Update(ctx, agent); err != nil {
			return controller.Result{}, err
		}

		// Broadcast status
		if c.network != nil {
			c.broadcastStatus(ctx, agent)
		}

		return controller.Result{RequeueAfter: c.heartbeatInterval}, nil
	}

	// For remote agents, wait for heartbeat
	return controller.Result{RequeueAfter: 5 * time.Second}, nil
}

func (c *Controller) reconcileOnline(ctx context.Context, agent *agentWrapper) (controller.Result, error) {
	// Check heartbeat timeout
	if agent.Status.LastHeartbeat != nil {
		elapsed := time.Since(*agent.Status.LastHeartbeat)
		if elapsed > c.heartbeatTimeout {
			agent.Status.Phase = v1.AgentPhaseOffline
			agent.SetCondition(v1.AgentConditionHealthy, v1.ConditionFalse, "HeartbeatTimeout", "Agent heartbeat timed out")
			agent.SetCondition(v1.AgentConditionConnected, v1.ConditionFalse, "Disconnected", "Agent disconnected")

			if err := c.store.Update(ctx, agent); err != nil {
				return controller.Result{}, err
			}

			return controller.Result{}, nil
		}
	}

	// Update last seen
	now := time.Now()
	agent.Status.LastSeenAt = &now

	// Update availability condition
	if agent.Status.CurrentTask == nil {
		agent.SetCondition(v1.AgentConditionAvailable, v1.ConditionTrue, "Available", "Agent is available for tasks")
	} else {
		agent.SetCondition(v1.AgentConditionAvailable, v1.ConditionFalse, "Busy", "Agent is working on a task")
	}

	if err := c.store.Update(ctx, agent); err != nil {
		return controller.Result{}, err
	}

	// Requeue before heartbeat timeout
	return controller.Result{RequeueAfter: c.heartbeatInterval}, nil
}

func (c *Controller) reconcileOffline(ctx context.Context, agent *agentWrapper) (controller.Result, error) {
	// Could implement reconnection logic here
	// For now, just update conditions
	agent.SetCondition(v1.AgentConditionReady, v1.ConditionFalse, "Offline", "Agent is offline")

	if err := c.store.Update(ctx, agent); err != nil {
		return controller.Result{}, err
	}

	// Check periodically for reconnection
	return controller.Result{RequeueAfter: 30 * time.Second}, nil
}

func (c *Controller) reconcileTerminating(ctx context.Context, agent *agentWrapper) (controller.Result, error) {
	// Broadcast offline status
	if c.network != nil {
		agent.Status.Phase = v1.AgentPhaseOffline
		c.broadcastStatus(ctx, agent)
	}

	// Could wait for cleanup here
	// For now, just delete the agent
	if err := c.store.Delete(ctx, agent.Name); err != nil {
		return controller.Result{}, err
	}

	return controller.Result{}, nil
}

// Network message types

// HeartbeatMessage represents an agent heartbeat.
type HeartbeatMessage struct {
	AgentID     string         `json:"agentId"`
	AgentName   string         `json:"agentName,omitempty"`
	Phase       v1.AgentPhase  `json:"phase"`
	Timestamp   time.Time      `json:"timestamp"`
	CurrentTask *v1.AgentTask  `json:"currentTask,omitempty"`
	TokenUsage  *v1.TokenUsage `json:"tokenUsage,omitempty"`
}

// StatusMessage represents an agent status update.
type StatusMessage struct {
	AgentID   string        `json:"agentId"`
	AgentName string        `json:"agentName,omitempty"`
	Phase     v1.AgentPhase `json:"phase"`
	Provider  string        `json:"provider"`
	Model     string        `json:"model,omitempty"`
	Addresses []string      `json:"addresses,omitempty"`
	Timestamp time.Time     `json:"timestamp"`
}

func (c *Controller) subscribeToTopics(ctx context.Context) error {
	// Subscribe to heartbeat topic
	heartbeatCh, err := c.network.Subscribe(ctx, c.heartbeatTopic)
	if err != nil {
		return err
	}
	go c.handleHeartbeatMessages(ctx, heartbeatCh)

	// Subscribe to status topic
	statusCh, err := c.network.Subscribe(ctx, c.statusTopic)
	if err != nil {
		return err
	}
	go c.handleStatusMessages(ctx, statusCh)

	return nil
}

func (c *Controller) handleHeartbeatMessages(ctx context.Context, ch <-chan plugin.Message) {
	for {
		select {
		case <-ctx.Done():
			return
		case msg, ok := <-ch:
			if !ok {
				return
			}
			var hb HeartbeatMessage
			if err := json.Unmarshal(msg.Data, &hb); err != nil {
				continue
			}
			c.processHeartbeat(ctx, hb)
		}
	}
}

func (c *Controller) handleStatusMessages(ctx context.Context, ch <-chan plugin.Message) {
	for {
		select {
		case <-ctx.Done():
			return
		case msg, ok := <-ch:
			if !ok {
				return
			}
			var status StatusMessage
			if err := json.Unmarshal(msg.Data, &status); err != nil {
				continue
			}
			c.processStatus(ctx, status)
		}
	}
}

func (c *Controller) processHeartbeat(ctx context.Context, hb HeartbeatMessage) {
	// Skip our own heartbeats
	if hb.AgentID == c.agentID {
		return
	}

	// Get or create agent
	agent, err := c.store.Get(ctx, hb.AgentID)
	if err == store.ErrNotFound {
		// Create new remote agent
		agent = &agentWrapper{
			Agent: v1.NewAgent(hb.AgentID, v1.AgentSpec{
				PeerID:      hb.AgentID,
				DisplayName: hb.AgentName,
			}),
		}
		if err := c.store.Create(ctx, agent); err != nil {
			return
		}
	} else if err != nil {
		return
	}

	// Update heartbeat
	agent.Status.Phase = hb.Phase
	agent.Status.LastHeartbeat = &hb.Timestamp
	agent.Status.LastSeenAt = &hb.Timestamp
	agent.Status.CurrentTask = hb.CurrentTask
	agent.Status.TokenUsage = hb.TokenUsage
	agent.SetCondition(v1.AgentConditionHealthy, v1.ConditionTrue, "HeartbeatReceived", "Heartbeat received")

	c.store.Update(ctx, agent)
	c.Enqueue(hb.AgentID)
}

func (c *Controller) processStatus(ctx context.Context, status StatusMessage) {
	// Skip our own status
	if status.AgentID == c.agentID {
		return
	}

	// Get or create agent
	agent, err := c.store.Get(ctx, status.AgentID)
	if err == store.ErrNotFound {
		// Create new remote agent
		agent = &agentWrapper{
			Agent: v1.NewAgent(status.AgentID, v1.AgentSpec{
				PeerID:      status.AgentID,
				DisplayName: status.AgentName,
				Provider:    v1.AgentProvider(status.Provider),
				Model:       status.Model,
			}),
		}
		if err := c.store.Create(ctx, agent); err != nil {
			return
		}
	} else if err != nil {
		return
	}

	// Update status
	agent.Status.Phase = status.Phase
	if status.Phase == v1.AgentPhaseOnline {
		now := time.Now()
		agent.Status.ConnectedAt = &now
		agent.SetCondition(v1.AgentConditionConnected, v1.ConditionTrue, "Connected", "Agent connected")
	} else if status.Phase == v1.AgentPhaseOffline {
		agent.SetCondition(v1.AgentConditionConnected, v1.ConditionFalse, "Disconnected", "Agent disconnected")
	}

	if len(status.Addresses) > 0 {
		agent.Status.NetworkInfo = &v1.NetworkInfo{
			Addresses: status.Addresses,
		}
	}

	c.store.Update(ctx, agent)
	c.Enqueue(status.AgentID)
}

func (c *Controller) heartbeatLoop(ctx context.Context) {
	ticker := time.NewTicker(c.heartbeatInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			c.sendHeartbeat(ctx)
		}
	}
}

func (c *Controller) sendHeartbeat(ctx context.Context) {
	// Get local agent
	agent, err := c.store.Get(ctx, c.agentID)
	if err != nil {
		return
	}

	hb := HeartbeatMessage{
		AgentID:     c.agentID,
		AgentName:   agent.Spec.DisplayName,
		Phase:       agent.Status.Phase,
		Timestamp:   time.Now(),
		CurrentTask: agent.Status.CurrentTask,
		TokenUsage:  agent.Status.TokenUsage,
	}

	data, _ := json.Marshal(hb)
	c.network.Publish(ctx, c.heartbeatTopic, data)
}

func (c *Controller) broadcastStatus(ctx context.Context, agent *agentWrapper) {
	status := StatusMessage{
		AgentID:   c.agentID,
		AgentName: agent.Spec.DisplayName,
		Phase:     agent.Status.Phase,
		Provider:  string(agent.Spec.Provider),
		Model:     agent.Spec.Model,
		Addresses: c.network.Addresses(),
		Timestamp: time.Now(),
	}

	data, _ := json.Marshal(status)
	c.network.Publish(ctx, c.statusTopic, data)
}
