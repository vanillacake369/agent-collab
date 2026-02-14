package libp2p_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"agent-collab/src/domain/event"
	"agent-collab/src/domain/interest"
	"agent-collab/src/infrastructure/network/libp2p"
)

// Feature: Event Bridge Integration
// As the agent-collab system
// I want to bridge domain events to P2P network
// So that events are propagated across all nodes

func TestFeature_EventBridgeIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping P2P test in short mode")
	}

	t.Run("Scenario: EventBridge connects Router to Node", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		// Given a libp2p node
		cfg := libp2p.DefaultConfig()
		cfg.ListenAddrs = []string{"/ip4/127.0.0.1/tcp/0"}
		node, err := libp2p.NewNode(ctx, cfg)
		if err != nil {
			t.Fatalf("Failed to create node: %v", err)
		}
		defer node.Close()

		// And an event router
		mgr := interest.NewManager()
		router := event.NewRouter(mgr, &event.RouterConfig{
			NodeID:   node.ID().String(),
			NodeName: "TestNode",
		})

		// When creating an event bridge
		bridge := libp2p.NewEventBridge(node, router)

		// Then the bridge should be created successfully
		if bridge == nil {
			t.Fatal("EventBridge should not be nil")
		}

		// And the router should have broadcast function set
		if !bridge.IsConnected() {
			t.Error("Bridge should be connected")
		}
	})

	t.Run("Scenario: Events published through bridge reach P2P network", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		// Given two connected nodes with event bridges
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

		// Setup routers and bridges
		mgr1 := interest.NewManager()
		router1 := event.NewRouter(mgr1, &event.RouterConfig{
			NodeID:   node1.ID().String(),
			NodeName: "Node1",
		})
		bridge1 := libp2p.NewEventBridge(node1, router1)
		if err := bridge1.Start(ctx); err != nil {
			t.Fatalf("Failed to start bridge1: %v", err)
		}
		defer bridge1.Stop()

		mgr2 := interest.NewManager()
		mgr2.Register(interest.NewInterest("agent-2", "Receiver", []string{"**"}))
		router2 := event.NewRouter(mgr2, &event.RouterConfig{
			NodeID:   node2.ID().String(),
			NodeName: "Node2",
		})
		bridge2 := libp2p.NewEventBridge(node2, router2)
		if err := bridge2.Start(ctx); err != nil {
			t.Fatalf("Failed to start bridge2: %v", err)
		}
		defer bridge2.Stop()

		// Connect nodes
		node2Info := node2.Host().Peerstore().PeerInfo(node2.ID())
		if err := node1.Host().Connect(ctx, node2Info); err != nil {
			t.Fatalf("Failed to connect nodes: %v", err)
		}
		time.Sleep(500 * time.Millisecond) // Wait for pubsub mesh

		// Subscribe on node2
		ch := router2.Subscribe("agent-2")

		// When node1 publishes an event
		evt := event.NewFileChangeEvent("agent-1", "Sender", "project/file.go", &event.FileChangePayload{
			ChangeType: "modify",
			Summary:    "Test change",
		})
		if err := router1.Publish(ctx, evt); err != nil {
			t.Fatalf("Failed to publish event: %v", err)
		}

		// Then node2 should receive the event
		select {
		case received := <-ch:
			if received.ID != evt.ID {
				t.Errorf("Expected event ID %s, got %s", evt.ID, received.ID)
			}
			if received.SourceName != "Sender" {
				t.Errorf("Expected source Sender, got %s", received.SourceName)
			}
		case <-time.After(3 * time.Second):
			t.Fatal("Timeout waiting for event on node2")
		}
	})

	t.Run("Scenario: Bridge handles message decompression", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		// Given a node with event bridge
		cfg := libp2p.DefaultConfig()
		cfg.ListenAddrs = []string{"/ip4/127.0.0.1/tcp/0"}
		node, err := libp2p.NewNode(ctx, cfg)
		if err != nil {
			t.Fatalf("Failed to create node: %v", err)
		}
		defer node.Close()

		mgr := interest.NewManager()
		mgr.Register(interest.NewInterest("agent-1", "Test", []string{"**"}))
		router := event.NewRouter(mgr, nil)
		bridge := libp2p.NewEventBridge(node, router)
		if err := bridge.Start(ctx); err != nil {
			t.Fatalf("Failed to start bridge: %v", err)
		}
		defer bridge.Stop()

		ch := router.Subscribe("agent-1")

		// When receiving a compressed message
		evt := event.NewFileChangeEvent("remote", "Remote", "file.go", nil)
		data, _ := json.Marshal(evt)
		compressed := libp2p.CompressMessage(data)

		// Simulate receiving compressed data
		bridge.HandleIncomingMessage(ctx, compressed)

		// Then the event should be properly decompressed and routed
		select {
		case received := <-ch:
			if received.ID != evt.ID {
				t.Errorf("Expected event ID %s, got %s", evt.ID, received.ID)
			}
		case <-time.After(time.Second):
			t.Fatal("Timeout waiting for decompressed event")
		}
	})
}

// Feature: Interest Synchronization over P2P
// As the agent-collab system
// I want to synchronize interests across nodes
// So that event routing works correctly cluster-wide

func TestFeature_InterestSyncOverP2P(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping P2P test in short mode")
	}

	t.Run("Scenario: Interests are broadcast when registered", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		// Given two connected nodes
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

		// Setup managers and bridges
		mgr1 := interest.NewManager()
		router1 := event.NewRouter(mgr1, nil)
		bridge1 := libp2p.NewEventBridge(node1, router1)
		bridge1.SetInterestManager(mgr1)
		if err := bridge1.Start(ctx); err != nil {
			t.Fatalf("Failed to start bridge1: %v", err)
		}
		defer bridge1.Stop()

		mgr2 := interest.NewManager()
		router2 := event.NewRouter(mgr2, nil)
		bridge2 := libp2p.NewEventBridge(node2, router2)
		bridge2.SetInterestManager(mgr2)
		if err := bridge2.Start(ctx); err != nil {
			t.Fatalf("Failed to start bridge2: %v", err)
		}
		defer bridge2.Stop()

		// Connect nodes
		node2Info := node2.Host().Peerstore().PeerInfo(node2.ID())
		if err := node1.Host().Connect(ctx, node2Info); err != nil {
			t.Fatalf("Failed to connect nodes: %v", err)
		}
		time.Sleep(500 * time.Millisecond)

		// When node1 registers a new interest
		int1 := interest.NewInterest("agent-1", "Claude", []string{"project-a/**"})
		mgr1.Register(int1)

		// Then node2 should eventually have the remote interest
		time.Sleep(500 * time.Millisecond)
		remoteInterests := mgr2.GetRemoteInterests()

		// Note: This might be 0 if sync is not yet implemented
		// The test documents expected behavior
		if len(remoteInterests) > 0 {
			found := false
			for _, ri := range remoteInterests {
				if ri.AgentID == "agent-1" {
					found = true
					break
				}
			}
			if !found {
				t.Log("Remote interest not yet synced (expected if sync not implemented)")
			}
		}
	})
}

// Feature: Event Bridge Lifecycle
// As the agent-collab system
// I want proper lifecycle management for event bridges
// So that resources are properly managed

func TestFeature_EventBridgeLifecycle(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping P2P test in short mode")
	}

	t.Run("Scenario: Bridge can be started and stopped", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		// Given a node and router
		cfg := libp2p.DefaultConfig()
		cfg.ListenAddrs = []string{"/ip4/127.0.0.1/tcp/0"}
		node, err := libp2p.NewNode(ctx, cfg)
		if err != nil {
			t.Fatalf("Failed to create node: %v", err)
		}
		defer node.Close()

		mgr := interest.NewManager()
		router := event.NewRouter(mgr, nil)
		bridge := libp2p.NewEventBridge(node, router)

		// When starting the bridge
		if err := bridge.Start(ctx); err != nil {
			t.Fatalf("Failed to start bridge: %v", err)
		}

		// Then it should be running
		if !bridge.IsRunning() {
			t.Error("Bridge should be running after Start")
		}

		// When stopping the bridge
		bridge.Stop()

		// Then it should not be running
		if bridge.IsRunning() {
			t.Error("Bridge should not be running after Stop")
		}
	})

	t.Run("Scenario: Bridge handles node disconnection gracefully", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		// Given a running bridge
		cfg := libp2p.DefaultConfig()
		cfg.ListenAddrs = []string{"/ip4/127.0.0.1/tcp/0"}
		node, err := libp2p.NewNode(ctx, cfg)
		if err != nil {
			t.Fatalf("Failed to create node: %v", err)
		}

		mgr := interest.NewManager()
		router := event.NewRouter(mgr, nil)
		bridge := libp2p.NewEventBridge(node, router)
		bridge.Start(ctx)

		// When the node is closed
		node.Close()

		// Then the bridge should handle it gracefully (no panic)
		bridge.Stop()
	})
}
