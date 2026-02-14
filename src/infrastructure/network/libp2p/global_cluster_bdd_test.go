package libp2p_test

import (
	"context"
	"testing"
	"time"

	"agent-collab/src/infrastructure/network/libp2p"
)

// Feature: Global Cluster Topology
// As the agent-collab system
// I want to use a single global cluster without project scoping
// So that agents can collaborate across multiple repositories

func TestFeature_GlobalClusterTopology(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping P2P test in short mode")
	}

	t.Run("Scenario: Nodes join global cluster without projectID", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		// Given a node configuration without projectID
		cfg := libp2p.DefaultConfig()
		cfg.ListenAddrs = []string{"/ip4/127.0.0.1/tcp/0"}

		// When creating a node
		node, err := libp2p.NewNode(ctx, cfg)

		// Then the node should be created successfully
		if err != nil {
			t.Fatalf("Failed to create node: %v", err)
		}
		defer node.Close()

		// And it should have a valid ID
		if node.ID().String() == "" {
			t.Error("Node should have a valid ID")
		}
	})

	t.Run("Scenario: Nodes subscribe to global topics", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		// Given a node in the global cluster
		cfg := libp2p.DefaultConfig()
		cfg.ListenAddrs = []string{"/ip4/127.0.0.1/tcp/0"}
		node, err := libp2p.NewNode(ctx, cfg)
		if err != nil {
			t.Fatalf("Failed to create node: %v", err)
		}
		defer node.Close()

		// When subscribing to global topics
		err = node.SubscribeGlobalTopics(ctx)

		// Then subscription should succeed
		if err != nil {
			t.Fatalf("Failed to subscribe to global topics: %v", err)
		}

		// And all core topics should be subscribed
		for _, topic := range libp2p.CoreTopics() {
			sub := node.GetSubscription(topic)
			if sub == nil {
				t.Errorf("Expected subscription for topic %s", topic)
			}
		}
	})

	t.Run("Scenario: Two nodes communicate via global topics", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()

		// Given two nodes in the global cluster
		cfg1 := libp2p.DefaultConfig()
		cfg1.ListenAddrs = []string{"/ip4/127.0.0.1/tcp/0"}
		node1, err := libp2p.NewNode(ctx, cfg1)
		if err != nil {
			t.Fatalf("Failed to create node1: %v", err)
		}
		defer node1.Close()

		cfg2 := libp2p.DefaultConfig()
		cfg2.ListenAddrs = []string{"/ip4/127.0.0.1/tcp/0"}
		node2, err := libp2p.NewNode(ctx, cfg2)
		if err != nil {
			t.Fatalf("Failed to create node2: %v", err)
		}
		defer node2.Close()

		// When they subscribe to global topics and connect
		node1.SubscribeGlobalTopics(ctx)
		node2.SubscribeGlobalTopics(ctx)

		node2Info := node2.Host().Peerstore().PeerInfo(node2.ID())
		if err := node1.Host().Connect(ctx, node2Info); err != nil {
			t.Fatalf("Failed to connect nodes: %v", err)
		}
		time.Sleep(500 * time.Millisecond)

		// Then they should be connected
		peers := node1.ConnectedPeers()
		found := false
		for _, p := range peers {
			if p == node2.ID() {
				found = true
				break
			}
		}
		if !found {
			t.Error("Node1 should be connected to node2")
		}
	})
}

// Feature: Global Topic Structure
// As the agent-collab system
// I want well-defined global topics
// So that different types of messages are properly separated

func TestFeature_GlobalTopicStructure(t *testing.T) {
	t.Run("Scenario: Core topics include essential channels", func(t *testing.T) {
		// When getting core topics
		topics := libp2p.CoreTopics()

		// Then they should include events and lock topics
		expected := map[string]bool{
			libp2p.TopicEvents:      false,
			libp2p.TopicLockIntent:  false,
			libp2p.TopicLockAcquire: false,
			libp2p.TopicLockRelease: false,
			libp2p.TopicContextSync: false,
		}

		for _, topic := range topics {
			if _, ok := expected[topic]; ok {
				expected[topic] = true
			}
		}

		for topic, found := range expected {
			if !found {
				t.Errorf("Core topics should include %s", topic)
			}
		}
	})

	t.Run("Scenario: Topics follow consistent naming pattern", func(t *testing.T) {
		// When checking all global topics
		topics := libp2p.AllGlobalTopics()

		// Then all should start with /agent-collab/
		prefix := "/agent-collab/"
		for _, topic := range topics {
			if len(topic) < len(prefix) || topic[:len(prefix)] != prefix {
				t.Errorf("Topic %s should start with %s", topic, prefix)
			}
		}
	})

	t.Run("Scenario: Cluster topics are separate from data topics", func(t *testing.T) {
		// When getting cluster topics
		clusterTopics := libp2p.ClusterTopics()

		// Then they should be distinct from core topics
		coreTopics := libp2p.CoreTopics()

		for _, ct := range clusterTopics {
			for _, core := range coreTopics {
				if ct == core {
					t.Errorf("Cluster topic %s should not be in core topics", ct)
				}
			}
		}
	})
}

// Feature: Global mDNS Discovery
// As the agent-collab system
// I want global mDNS discovery
// So that agents on the same network can find each other automatically

func TestFeature_GlobalDiscovery(t *testing.T) {
	t.Run("Scenario: Discovery service uses global name", func(t *testing.T) {
		// Then the global service name should be defined
		if libp2p.GlobalServiceName != "agent-collab-global" {
			t.Errorf("Expected 'agent-collab-global', got '%s'", libp2p.GlobalServiceName)
		}
	})

	t.Run("Scenario: Discovery service created without projectID", func(t *testing.T) {
		if testing.Short() {
			t.Skip("Skipping P2P test in short mode")
		}

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		// Given a node
		cfg := libp2p.DefaultConfig()
		cfg.ListenAddrs = []string{"/ip4/127.0.0.1/tcp/0"}
		node, err := libp2p.NewNode(ctx, cfg)
		if err != nil {
			t.Fatalf("Failed to create node: %v", err)
		}
		defer node.Close()

		// When creating discovery service (no projectID parameter)
		discovery := libp2p.NewDiscoveryService(node.Host())

		// Then it should be created successfully
		if discovery == nil {
			t.Fatal("Discovery service should not be nil")
		}
		defer discovery.Close()
	})
}
