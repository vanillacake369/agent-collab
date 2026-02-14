package cli

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"agent-collab/src/interfaces/daemon"
)

// BDD-style tests for status command
// Feature: Status Command
// As a user
// I want to check the cluster status
// So that I can understand the current state of my agent collaboration cluster

// Mock server helper (similar to daemon_bdd_test.go)
type mockStatusServer struct {
	listener   net.Listener
	socketPath string
	handlers   map[string]http.HandlerFunc
	server     *http.Server
}

func newMockStatusServer(t *testing.T) *mockStatusServer {
	t.Helper()

	// Create temp socket path
	socketPath := filepath.Join(os.TempDir(), "status-test.sock")
	os.Remove(socketPath) // Clean up any existing socket

	handlers := make(map[string]http.HandlerFunc)

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if handler, ok := handlers[r.URL.Path]; ok {
			handler(w, r)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})

	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		t.Fatalf("failed to create socket: %v", err)
	}

	server := &http.Server{Handler: mux}
	go server.Serve(listener)

	return &mockStatusServer{
		listener:   listener,
		socketPath: socketPath,
		handlers:   handlers,
		server:     server,
	}
}

func (m *mockStatusServer) SetHandler(path string, handler http.HandlerFunc) {
	m.handlers[path] = handler
}

func (m *mockStatusServer) Close() {
	m.server.Close()
	m.listener.Close()
	os.Remove(m.socketPath)
}

func (m *mockStatusServer) Client() *daemon.Client {
	transport := &http.Transport{
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			return net.Dial("unix", m.socketPath)
		},
	}
	return daemon.NewClientWithTransport(transport, m.socketPath)
}

// Scenario: Active cluster status
func TestFeature_Status_Scenario_ActiveCluster(t *testing.T) {
	t.Run("Given a daemon running with active cluster", func(t *testing.T) {
		server := newMockStatusServer(t)
		defer server.Close()

		// Setup status handler
		server.SetHandler("/status", func(w http.ResponseWriter, r *http.Request) {
			json.NewEncoder(w).Encode(daemon.StatusResponse{
				Running:           true,
				ProjectName:       "test-project",
				NodeID:            "12D3KooWTestNodeID123456789",
				PeerCount:         3,
				LockCount:         2,
				AgentCount:        1,
				EmbeddingProvider: "openai",
			})
		})

		// Setup token usage handler
		server.SetHandler("/tokens/usage", func(w http.ResponseWriter, r *http.Request) {
			json.NewEncoder(w).Encode(daemon.TokenUsageResponse{
				TokensToday:   25000,
				TokensPerHour: 1250.0,
				CostToday:     0.025,
				DailyLimit:    200000,
				UsagePercent:  12.5,
			})
		})

		// Setup peers handler
		server.SetHandler("/peers/list", func(w http.ResponseWriter, r *http.Request) {
			json.NewEncoder(w).Encode(daemon.ListPeersResponse{
				Peers: []daemon.PeerInfo{
					{ID: "12D3KooWPeer1", Addresses: []string{"/ip4/192.168.1.10/tcp/4001"}, Latency: 15, Connected: true},
					{ID: "12D3KooWPeer2", Addresses: []string{"/ip4/192.168.1.11/tcp/4001"}, Latency: 23, Connected: true},
					{ID: "12D3KooWPeer3", Addresses: []string{"/ip4/192.168.1.12/tcp/4001"}, Latency: 8, Connected: true},
				},
			})
		})

		client := server.Client()

		t.Run("When I call Status()", func(t *testing.T) {
			status, err := client.Status()

			t.Run("Then it should succeed", func(t *testing.T) {
				if err != nil {
					t.Fatalf("expected no error, got: %v", err)
				}
			})

			t.Run("And it should show the cluster is running", func(t *testing.T) {
				if !status.Running {
					t.Error("expected Running to be true")
				}
			})

			t.Run("And it should show the project name", func(t *testing.T) {
				if status.ProjectName != "test-project" {
					t.Errorf("expected project 'test-project', got: %s", status.ProjectName)
				}
			})

			t.Run("And it should show the peer count", func(t *testing.T) {
				if status.PeerCount != 3 {
					t.Errorf("expected 3 peers, got: %d", status.PeerCount)
				}
			})
		})

		t.Run("When I call TokenUsage()", func(t *testing.T) {
			tokenUsage, err := client.TokenUsage()

			t.Run("Then it should succeed", func(t *testing.T) {
				if err != nil {
					t.Fatalf("expected no error, got: %v", err)
				}
			})

			t.Run("And it should show today's token usage", func(t *testing.T) {
				if tokenUsage.TokensToday != 25000 {
					t.Errorf("expected 25000 tokens, got: %d", tokenUsage.TokensToday)
				}
			})

			t.Run("And it should show usage percentage", func(t *testing.T) {
				if tokenUsage.UsagePercent != 12.5 {
					t.Errorf("expected 12.5%% usage, got: %.1f%%", tokenUsage.UsagePercent)
				}
			})
		})

		t.Run("When I call ListPeers()", func(t *testing.T) {
			peersResp, err := client.ListPeers()

			t.Run("Then it should succeed", func(t *testing.T) {
				if err != nil {
					t.Fatalf("expected no error, got: %v", err)
				}
			})

			t.Run("And it should return the peer list", func(t *testing.T) {
				if len(peersResp.Peers) != 3 {
					t.Errorf("expected 3 peers, got: %d", len(peersResp.Peers))
				}
			})

			t.Run("And each peer should have latency info", func(t *testing.T) {
				for _, peer := range peersResp.Peers {
					if peer.Latency == 0 {
						t.Errorf("expected non-zero latency for peer %s", peer.ID)
					}
				}
			})
		})
	})
}

// Scenario: No daemon running
func TestFeature_Status_Scenario_NoDaemon(t *testing.T) {
	t.Run("Given no daemon is running", func(t *testing.T) {
		// Use a path that doesn't exist
		transport := &http.Transport{
			DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				return net.Dial("unix", "/tmp/nonexistent-daemon.sock")
			},
		}
		client := daemon.NewClientWithTransport(transport, "/tmp/nonexistent-daemon.sock")

		t.Run("When I check if daemon is running", func(t *testing.T) {
			running := client.IsRunning()

			t.Run("Then it should return false", func(t *testing.T) {
				if running {
					t.Error("expected daemon to not be running")
				}
			})
		})
	})
}

// Scenario: JSON output
func TestFeature_Status_Scenario_JSONOutput(t *testing.T) {
	t.Run("Given a daemon running with active cluster", func(t *testing.T) {
		server := newMockStatusServer(t)
		defer server.Close()

		server.SetHandler("/status", func(w http.ResponseWriter, r *http.Request) {
			json.NewEncoder(w).Encode(daemon.StatusResponse{
				Running:     true,
				ProjectName: "json-test-project",
				NodeID:      "12D3KooWJSONTestNode",
				PeerCount:   2,
				LockCount:   1,
			})
		})

		client := server.Client()

		t.Run("When I get status", func(t *testing.T) {
			status, err := client.Status()
			if err != nil {
				t.Fatalf("expected no error, got: %v", err)
			}

			t.Run("Then I can marshal it to JSON", func(t *testing.T) {
				data, err := json.Marshal(status)
				if err != nil {
					t.Fatalf("expected no error, got: %v", err)
				}

				t.Run("And the JSON should be valid", func(t *testing.T) {
					var decoded map[string]interface{}
					if err := json.Unmarshal(data, &decoded); err != nil {
						t.Fatalf("expected valid JSON, got error: %v", err)
					}
				})

				t.Run("And it should contain project_name", func(t *testing.T) {
					var decoded map[string]interface{}
					json.Unmarshal(data, &decoded)
					if decoded["project_name"] != "json-test-project" {
						t.Errorf("expected project_name 'json-test-project', got: %v", decoded["project_name"])
					}
				})
			})
		})
	})
}

// Scenario: Events listing
func TestFeature_Status_Scenario_EventsListing(t *testing.T) {
	t.Run("Given a daemon with recent events", func(t *testing.T) {
		server := newMockStatusServer(t)
		defer server.Close()

		now := time.Now()
		server.SetHandler("/events/list", func(w http.ResponseWriter, r *http.Request) {
			json.NewEncoder(w).Encode(daemon.ListEventsResponse{
				Events: []daemon.Event{
					{Type: "lock_acquired", Timestamp: now.Add(-1 * time.Minute)},
					{Type: "context_updated", Timestamp: now.Add(-2 * time.Minute)},
					{Type: "peer_connected", Timestamp: now.Add(-5 * time.Minute)},
				},
				Count: 3,
			})
		})

		client := server.Client()

		t.Run("When I call ListEvents()", func(t *testing.T) {
			eventsResp, err := client.ListEvents(10, "", false)

			t.Run("Then it should succeed", func(t *testing.T) {
				if err != nil {
					t.Fatalf("expected no error, got: %v", err)
				}
			})

			t.Run("And it should return recent events", func(t *testing.T) {
				if len(eventsResp.Events) != 3 {
					t.Errorf("expected 3 events, got: %d", len(eventsResp.Events))
				}
			})

			t.Run("And each event should have a type", func(t *testing.T) {
				for _, event := range eventsResp.Events {
					if event.Type == "" {
						t.Error("expected event type to be set")
					}
				}
			})
		})
	})
}

// Scenario: Uninitialized cluster
func TestFeature_Status_Scenario_Uninitialized(t *testing.T) {
	t.Run("Given a daemon with no cluster initialized", func(t *testing.T) {
		server := newMockStatusServer(t)
		defer server.Close()

		server.SetHandler("/status", func(w http.ResponseWriter, r *http.Request) {
			json.NewEncoder(w).Encode(daemon.StatusResponse{
				Running:     true,
				ProjectName: "", // Empty means not initialized
				NodeID:      "",
				PeerCount:   0,
				LockCount:   0,
			})
		})

		client := server.Client()

		t.Run("When I call Status()", func(t *testing.T) {
			status, err := client.Status()

			t.Run("Then it should succeed", func(t *testing.T) {
				if err != nil {
					t.Fatalf("expected no error, got: %v", err)
				}
			})

			t.Run("And the project name should be empty", func(t *testing.T) {
				if status.ProjectName != "" {
					t.Errorf("expected empty project name, got: %s", status.ProjectName)
				}
			})

			t.Run("And there should be no peers", func(t *testing.T) {
				if status.PeerCount != 0 {
					t.Errorf("expected 0 peers, got: %d", status.PeerCount)
				}
			})
		})
	})
}

// Scenario: Enhanced status with all information
func TestFeature_Status_Scenario_EnhancedStatus(t *testing.T) {
	t.Run("Given all daemon endpoints are available", func(t *testing.T) {
		server := newMockStatusServer(t)
		defer server.Close()

		// Setup all handlers
		server.SetHandler("/status", func(w http.ResponseWriter, r *http.Request) {
			json.NewEncoder(w).Encode(daemon.StatusResponse{
				Running:     true,
				ProjectName: "enhanced-test",
				NodeID:      "12D3KooWEnhancedNode",
				PeerCount:   2,
				LockCount:   1,
			})
		})

		server.SetHandler("/tokens/usage", func(w http.ResponseWriter, r *http.Request) {
			json.NewEncoder(w).Encode(daemon.TokenUsageResponse{
				TokensToday:  50000,
				CostToday:    0.05,
				DailyLimit:   200000,
				UsagePercent: 25.0,
			})
		})

		server.SetHandler("/peers/list", func(w http.ResponseWriter, r *http.Request) {
			json.NewEncoder(w).Encode(daemon.ListPeersResponse{
				Peers: []daemon.PeerInfo{
					{ID: "peer1", Latency: 10, Connected: true},
					{ID: "peer2", Latency: 20, Connected: true},
				},
			})
		})

		server.SetHandler("/events/list", func(w http.ResponseWriter, r *http.Request) {
			json.NewEncoder(w).Encode(daemon.ListEventsResponse{
				Events: []daemon.Event{
					{Type: "test_event", Timestamp: time.Now()},
				},
				Count: 1,
			})
		})

		client := server.Client()

		t.Run("When I build enhanced status", func(t *testing.T) {
			status, _ := client.Status()
			tokenUsage, _ := client.TokenUsage()
			peers, _ := client.ListPeers()
			events, _ := client.ListEvents(10, "", false)

			t.Run("Then status should be available", func(t *testing.T) {
				if status.ProjectName != "enhanced-test" {
					t.Errorf("expected project 'enhanced-test', got: %s", status.ProjectName)
				}
			})

			t.Run("And token usage should be available", func(t *testing.T) {
				if tokenUsage.TokensToday != 50000 {
					t.Errorf("expected 50000 tokens, got: %d", tokenUsage.TokensToday)
				}
			})

			t.Run("And peers should be available", func(t *testing.T) {
				if len(peers.Peers) != 2 {
					t.Errorf("expected 2 peers, got: %d", len(peers.Peers))
				}
			})

			t.Run("And events should be available", func(t *testing.T) {
				if len(events.Events) != 1 {
					t.Errorf("expected 1 event, got: %d", len(events.Events))
				}
			})
		})
	})
}

// Scenario: Format token count helper
func TestFeature_Status_Scenario_FormatTokenCount(t *testing.T) {
	tests := []struct {
		input    int64
		expected string
	}{
		{500, "500"},
		{1500, "1.5K"},
		{25000, "25.0K"},
		{1500000, "1.5M"},
	}

	for _, tc := range tests {
		t.Run("Given token count "+tc.expected, func(t *testing.T) {
			result := formatTokenCount(tc.input)
			if result != tc.expected {
				t.Errorf("formatTokenCount(%d) = %s, expected %s", tc.input, result, tc.expected)
			}
		})
	}
}
