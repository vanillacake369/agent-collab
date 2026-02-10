// Package e2e provides end-to-end tests using real P2P networking.
package e2e

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	v1 "agent-collab/src/api/v1"
	"agent-collab/src/infrastructure/network/libp2p"
)

// TestP2PContextSharing tests context sharing between two P2P nodes.
func TestP2PContextSharing(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping P2P test in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Create two nodes
	cfg1 := libp2p.DefaultConfig()
	cfg1.ProjectID = "test-project"
	cfg1.ListenAddrs = []string{"/ip4/127.0.0.1/tcp/0"}

	cfg2 := libp2p.DefaultConfig()
	cfg2.ProjectID = "test-project"
	cfg2.ListenAddrs = []string{"/ip4/127.0.0.1/tcp/0"}

	node1, err := libp2p.NewNode(ctx, cfg1)
	if err != nil {
		t.Fatalf("Failed to create node1: %v", err)
	}
	defer node1.Close()

	node2, err := libp2p.NewNode(ctx, cfg2)
	if err != nil {
		t.Fatalf("Failed to create node2: %v", err)
	}
	defer node2.Close()

	t.Logf("Node1 ID: %s", node1.ID())
	t.Logf("Node2 ID: %s", node2.ID())

	// Subscribe to project topics
	if err := node1.SubscribeProjectTopics(ctx); err != nil {
		t.Fatalf("Node1 subscribe failed: %v", err)
	}
	if err := node2.SubscribeProjectTopics(ctx); err != nil {
		t.Fatalf("Node2 subscribe failed: %v", err)
	}

	// Connect nodes
	node2Addrs := node2.Addrs()
	node2Info := node2.Host().Peerstore().PeerInfo(node2.ID())
	t.Logf("Connecting node1 to node2 at %v", node2Addrs)

	if err := node1.Host().Connect(ctx, node2Info); err != nil {
		t.Fatalf("Failed to connect nodes: %v", err)
	}

	// Wait for connection to stabilize
	time.Sleep(500 * time.Millisecond)

	// Verify connection
	peers1 := node1.ConnectedPeers()
	if len(peers1) == 0 {
		t.Fatal("Node1 has no connected peers")
	}
	t.Logf("Node1 connected to %d peers", len(peers1))

	peers2 := node2.ConnectedPeers()
	if len(peers2) == 0 {
		t.Fatal("Node2 has no connected peers")
	}
	t.Logf("Node2 connected to %d peers", len(peers2))

	// Subscribe to context topic on node2
	contextTopic := "/agent-collab/test-project/context"
	sub, err := node2.Subscribe(contextTopic)
	if err != nil {
		t.Fatalf("Node2 subscribe to context topic failed: %v", err)
	}

	// Give subscription time to propagate
	time.Sleep(500 * time.Millisecond)

	// Create context message
	ctxMsg := struct {
		Name    string `json:"name"`
		Content string `json:"content"`
		From    string `json:"from"`
	}{
		Name:    "test-context",
		Content: "Hello from Node1!",
		From:    node1.ID().String(),
	}

	msgData, _ := json.Marshal(ctxMsg)

	// Publish from node1
	t.Log("Publishing context from Node1")
	if err := node1.Publish(ctx, contextTopic, msgData); err != nil {
		t.Fatalf("Publish failed: %v", err)
	}

	// Receive on node2
	t.Log("Waiting for message on Node2")
	receivedCh := make(chan bool, 1)
	go func() {
		msg, err := sub.Next(ctx)
		if err != nil {
			t.Logf("Error receiving message: %v", err)
			return
		}

		// Decompress if needed
		data, err := node2.DecompressAndDecrypt(contextTopic, msg.Data)
		if err != nil {
			data = msg.Data
		}

		var received struct {
			Name    string `json:"name"`
			Content string `json:"content"`
			From    string `json:"from"`
		}
		if err := json.Unmarshal(data, &received); err != nil {
			t.Logf("Unmarshal failed: %v", err)
			return
		}

		t.Logf("Received: %+v", received)
		if received.Content == "Hello from Node1!" {
			receivedCh <- true
		}
	}()

	select {
	case <-receivedCh:
		t.Log("Context sharing successful!")
	case <-time.After(5 * time.Second):
		t.Fatal("Timeout waiting for message")
	}
}

// TestP2PLockNegotiation tests lock negotiation between nodes.
func TestP2PLockNegotiation(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping P2P test in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Create two nodes
	cfg1 := libp2p.DefaultConfig()
	cfg1.ProjectID = "test-lock-project"
	cfg1.ListenAddrs = []string{"/ip4/127.0.0.1/tcp/0"}

	cfg2 := libp2p.DefaultConfig()
	cfg2.ProjectID = "test-lock-project"
	cfg2.ListenAddrs = []string{"/ip4/127.0.0.1/tcp/0"}

	node1, err := libp2p.NewNode(ctx, cfg1)
	if err != nil {
		t.Fatalf("Failed to create node1: %v", err)
	}
	defer node1.Close()

	node2, err := libp2p.NewNode(ctx, cfg2)
	if err != nil {
		t.Fatalf("Failed to create node2: %v", err)
	}
	defer node2.Close()

	// Subscribe to project topics
	node1.SubscribeProjectTopics(ctx)
	node2.SubscribeProjectTopics(ctx)

	// Connect nodes
	node2Info := node2.Host().Peerstore().PeerInfo(node2.ID())
	if err := node1.Host().Connect(ctx, node2Info); err != nil {
		t.Fatalf("Failed to connect nodes: %v", err)
	}

	time.Sleep(500 * time.Millisecond)

	// Subscribe to lock intent topic
	lockTopic := "/agent-collab/test-lock-project/lock/intent"
	sub2, err := node2.Subscribe(lockTopic)
	if err != nil {
		t.Fatalf("Subscribe failed: %v", err)
	}

	time.Sleep(500 * time.Millisecond)

	// Node1 announces lock intent
	lockIntent := struct {
		LockName  string        `json:"lockName"`
		HolderID  string        `json:"holderId"`
		Target    v1.LockTarget `json:"target"`
		Intention string        `json:"intention"`
		Timestamp time.Time     `json:"timestamp"`
	}{
		LockName: "test-lock",
		HolderID: node1.ID().String(),
		Target: v1.LockTarget{
			Type:     v1.LockTargetTypeFile,
			FilePath: "/path/to/file.go",
		},
		Intention: "Editing file",
		Timestamp: time.Now(),
	}

	msgData, _ := json.Marshal(lockIntent)
	if err := node1.Publish(ctx, lockTopic, msgData); err != nil {
		t.Fatalf("Publish intent failed: %v", err)
	}

	// Node2 should receive the intent
	receivedCh := make(chan bool, 1)
	go func() {
		msg, err := sub2.Next(ctx)
		if err != nil {
			return
		}

		data, err := node2.DecompressAndDecrypt(lockTopic, msg.Data)
		if err != nil {
			data = msg.Data
		}

		var received struct {
			LockName string `json:"lockName"`
			HolderID string `json:"holderId"`
		}
		if err := json.Unmarshal(data, &received); err != nil {
			return
		}

		t.Logf("Received lock intent: %+v", received)
		if received.LockName == "test-lock" {
			receivedCh <- true
		}
	}()

	select {
	case <-receivedCh:
		t.Log("Lock intent negotiation successful!")
	case <-time.After(5 * time.Second):
		t.Fatal("Timeout waiting for lock intent")
	}
}

// TestP2PAgentDiscovery tests agent discovery between nodes.
func TestP2PAgentDiscovery(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping P2P test in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Create three nodes (agents)
	nodes := make([]*libp2p.Node, 3)
	for i := range 3 {
		cfg := libp2p.DefaultConfig()
		cfg.ProjectID = "test-discovery"
		cfg.ListenAddrs = []string{"/ip4/127.0.0.1/tcp/0"}

		node, err := libp2p.NewNode(ctx, cfg)
		if err != nil {
			t.Fatalf("Failed to create node %d: %v", i, err)
		}
		defer node.Close()
		nodes[i] = node

		t.Logf("Created node %d: %s", i, node.ID())
	}

	// Connect node 0 to nodes 1 and 2
	for i := 1; i < 3; i++ {
		info := nodes[i].Host().Peerstore().PeerInfo(nodes[i].ID())
		if err := nodes[0].Host().Connect(ctx, info); err != nil {
			t.Fatalf("Failed to connect node 0 to node %d: %v", i, err)
		}
	}

	// Connect node 1 to node 2
	info := nodes[2].Host().Peerstore().PeerInfo(nodes[2].ID())
	if err := nodes[1].Host().Connect(ctx, info); err != nil {
		t.Logf("Warning: Failed to connect node 1 to node 2: %v", err)
	}

	time.Sleep(time.Second)

	// Check connectivity
	for i, node := range nodes {
		peers := node.ConnectedPeers()
		t.Logf("Node %d connected to %d peers", i, len(peers))
	}

	// Verify all nodes can see each other
	peers0 := nodes[0].ConnectedPeers()
	if len(peers0) < 2 {
		t.Errorf("Node 0 should have at least 2 peers, got %d", len(peers0))
	}
}
