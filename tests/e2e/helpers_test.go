//go:build e2e

package e2e

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"agent-collab/internal/application"
)

// TestCluster manages multiple nodes for E2E testing.
type TestCluster struct {
	t       *testing.T
	nodes   []*application.App
	dataDir string
}

// NewTestCluster creates a new test cluster.
func NewTestCluster(t *testing.T, nodeCount int) *TestCluster {
	t.Helper()

	dataDir, err := os.MkdirTemp("", "agent-collab-e2e-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}

	cluster := &TestCluster{
		t:       t,
		nodes:   make([]*application.App, 0, nodeCount),
		dataDir: dataDir,
	}

	t.Cleanup(func() {
		cluster.Shutdown()
		os.RemoveAll(dataDir)
	})

	return cluster
}

// CreateNode creates a new node in the cluster.
func (tc *TestCluster) CreateNode(name string) (*application.App, error) {
	nodeDir := filepath.Join(tc.dataDir, name)
	if err := os.MkdirAll(nodeDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create node dir: %w", err)
	}

	cfg := &application.Config{
		DataDir:    nodeDir,
		ListenPort: 0, // Auto-assign
	}

	app, err := application.New(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create app: %w", err)
	}

	tc.nodes = append(tc.nodes, app)
	return app, nil
}

// InitializeLeader initializes the first node as cluster leader.
func (tc *TestCluster) InitializeLeader(ctx context.Context, projectName string) (*application.App, *application.InitResult, error) {
	if len(tc.nodes) == 0 {
		app, err := tc.CreateNode("leader")
		if err != nil {
			return nil, nil, err
		}
		tc.nodes = append(tc.nodes, app)
	}

	leader := tc.nodes[0]
	result, err := leader.Initialize(ctx, projectName)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to initialize leader: %w", err)
	}

	if err := leader.Start(); err != nil {
		return nil, nil, fmt.Errorf("failed to start leader: %w", err)
	}

	return leader, result, nil
}

// JoinNode creates and joins a new node to the cluster.
func (tc *TestCluster) JoinNode(ctx context.Context, name, token string) (*application.App, error) {
	app, err := tc.CreateNode(name)
	if err != nil {
		return nil, err
	}

	_, err = app.Join(ctx, token)
	if err != nil {
		return nil, fmt.Errorf("failed to join: %w", err)
	}

	if err := app.Start(); err != nil {
		return nil, fmt.Errorf("failed to start: %w", err)
	}

	return app, nil
}

// Shutdown stops all nodes in the cluster.
func (tc *TestCluster) Shutdown() {
	for _, node := range tc.nodes {
		if node != nil {
			node.Stop()
		}
	}
}

// WaitForPeers waits until a node has the expected number of peers.
func (tc *TestCluster) WaitForPeers(app *application.App, expectedPeers int, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		status := app.GetStatus()
		if status.PeerCount >= expectedPeers {
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}

	status := app.GetStatus()
	return fmt.Errorf("timeout waiting for peers: expected %d, got %d", expectedPeers, status.PeerCount)
}

// WaitForSync waits until all nodes have synced deltas.
func (tc *TestCluster) WaitForSync(expectedDeltas int, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		allSynced := true
		for _, node := range tc.nodes {
			status := node.GetStatus()
			if status.DeltaCount < expectedDeltas {
				allSynced = false
				break
			}
		}
		if allSynced {
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}

	return fmt.Errorf("timeout waiting for sync")
}

// GetNode returns a node by index.
func (tc *TestCluster) GetNode(index int) *application.App {
	if index < 0 || index >= len(tc.nodes) {
		return nil
	}
	return tc.nodes[index]
}

// NodeCount returns the number of nodes.
func (tc *TestCluster) NodeCount() int {
	return len(tc.nodes)
}
