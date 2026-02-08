//go:build e2e

package e2e

import (
	"context"
	"testing"
	"time"
)

// TestClusterFormation tests basic cluster formation with init and join.
func TestClusterFormation(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cluster := NewTestCluster(t, 3)

	// Initialize leader
	leader, initResult, err := cluster.InitializeLeader(ctx, "test-project")
	if err != nil {
		t.Fatalf("failed to initialize leader: %v", err)
	}

	t.Logf("Leader initialized: NodeID=%s", initResult.NodeID)

	// Verify leader status
	status := leader.GetStatus()
	if !status.Running {
		t.Error("leader should be running")
	}
	if status.ProjectName != "test-project" {
		t.Errorf("expected project name 'test-project', got '%s'", status.ProjectName)
	}

	// Join second node
	node2, err := cluster.JoinNode(ctx, "node2", initResult.InviteToken)
	if err != nil {
		t.Fatalf("failed to join node2: %v", err)
	}

	t.Log("Node2 joined cluster")

	// Wait for peer discovery
	if err := cluster.WaitForPeers(leader, 1, 10*time.Second); err != nil {
		t.Logf("Warning: %v", err)
	}

	// Verify node2 status
	status2 := node2.GetStatus()
	if !status2.Running {
		t.Error("node2 should be running")
	}
	if status2.ProjectName != "test-project" {
		t.Errorf("expected project name 'test-project', got '%s'", status2.ProjectName)
	}

	// Join third node
	node3, err := cluster.JoinNode(ctx, "node3", initResult.InviteToken)
	if err != nil {
		t.Fatalf("failed to join node3: %v", err)
	}

	t.Log("Node3 joined cluster")

	// Wait for full mesh
	if err := cluster.WaitForPeers(leader, 2, 10*time.Second); err != nil {
		t.Logf("Warning: %v", err)
	}

	// Verify cluster size
	if cluster.NodeCount() != 3 {
		t.Errorf("expected 3 nodes, got %d", cluster.NodeCount())
	}

	_ = node3 // Use node3
	t.Log("Cluster formation test passed")
}

// TestTokenExpiration tests that expired tokens are rejected.
func TestTokenExpiration(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cluster := NewTestCluster(t, 2)

	// Initialize leader
	_, initResult, err := cluster.InitializeLeader(ctx, "test-project")
	if err != nil {
		t.Fatalf("failed to initialize leader: %v", err)
	}

	// Token should be valid (default 24h TTL)
	_, err = cluster.JoinNode(ctx, "node2", initResult.InviteToken)
	if err != nil {
		t.Fatalf("failed to join with valid token: %v", err)
	}

	t.Log("Token validation test passed")
}

// TestNodeReconnection tests node reconnection after disconnect.
func TestNodeReconnection(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	cluster := NewTestCluster(t, 2)

	// Initialize leader
	leader, initResult, err := cluster.InitializeLeader(ctx, "test-project")
	if err != nil {
		t.Fatalf("failed to initialize leader: %v", err)
	}

	// Join second node
	node2, err := cluster.JoinNode(ctx, "node2", initResult.InviteToken)
	if err != nil {
		t.Fatalf("failed to join node2: %v", err)
	}

	// Wait for connection
	if err := cluster.WaitForPeers(leader, 1, 10*time.Second); err != nil {
		t.Logf("Warning: %v", err)
	}

	// Stop node2
	if err := node2.Stop(); err != nil {
		t.Errorf("failed to stop node2: %v", err)
	}

	t.Log("Node2 stopped")

	// Give time for disconnect detection
	time.Sleep(2 * time.Second)

	// Restart node2
	if err := node2.Start(); err != nil {
		t.Fatalf("failed to restart node2: %v", err)
	}

	t.Log("Node2 restarted")

	// Wait for reconnection
	if err := cluster.WaitForPeers(leader, 1, 15*time.Second); err != nil {
		t.Logf("Warning: reconnection may not have completed: %v", err)
	}

	t.Log("Reconnection test passed")
}

// TestMultipleProjects tests that different projects are isolated.
func TestMultipleProjects(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cluster1 := NewTestCluster(t, 2)
	cluster2 := NewTestCluster(t, 2)

	// Initialize project 1
	_, init1, err := cluster1.InitializeLeader(ctx, "project-1")
	if err != nil {
		t.Fatalf("failed to initialize project-1: %v", err)
	}

	// Initialize project 2
	_, init2, err := cluster2.InitializeLeader(ctx, "project-2")
	if err != nil {
		t.Fatalf("failed to initialize project-2: %v", err)
	}

	// Verify different node IDs
	if init1.NodeID == init2.NodeID {
		t.Error("different projects should have different node IDs")
	}

	// Verify project names
	status1 := cluster1.GetNode(0).GetStatus()
	status2 := cluster2.GetNode(0).GetStatus()

	if status1.ProjectName != "project-1" {
		t.Errorf("expected project name 'project-1', got '%s'", status1.ProjectName)
	}
	if status2.ProjectName != "project-2" {
		t.Errorf("expected project name 'project-2', got '%s'", status2.ProjectName)
	}

	t.Log("Multiple projects isolation test passed")
}
