package tui

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"agent-collab/src/interface/daemon"
)

// BDD-style tests for TUI execute functions
// Feature: TUI Daemon Integration
// As a user of the TUI
// I want to execute cluster operations (init, join, leave, release lock)
// So that I can manage the collaboration cluster through the TUI interface

// Mock daemon server for TUI testing
type mockTUIDaemonServer struct {
	listener   net.Listener
	socketPath string
	handlers   map[string]http.HandlerFunc
}

func newMockTUIDaemonServer(t *testing.T) *mockTUIDaemonServer {
	t.Helper()

	socketPath := filepath.Join(os.TempDir(), "tui-test.sock")
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

	return &mockTUIDaemonServer{
		listener:   listener,
		socketPath: socketPath,
		handlers:   handlers,
	}
}

func (m *mockTUIDaemonServer) SetHandler(path string, handler http.HandlerFunc) {
	m.handlers[path] = handler
}

func (m *mockTUIDaemonServer) Close() {
	m.listener.Close()
	os.Remove(m.socketPath)
}

func (m *mockTUIDaemonServer) Client() *daemon.Client {
	transport := &http.Transport{
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			return net.Dial("unix", m.socketPath)
		},
	}
	return daemon.NewClientWithTransport(transport, m.socketPath)
}

// Scenario: Execute cluster initialization
func TestFeature_TUIExecute_Scenario_Init(t *testing.T) {
	t.Run("Given a TUI model with daemon client", func(t *testing.T) {
		server := newMockTUIDaemonServer(t)
		defer server.Close()

		initCalled := false
		projectNameReceived := ""

		server.SetHandler("/init", func(w http.ResponseWriter, r *http.Request) {
			initCalled = true
			var req struct {
				ProjectName string `json:"project_name"`
			}
			json.NewDecoder(r.Body).Decode(&req)
			projectNameReceived = req.ProjectName

			json.NewEncoder(w).Encode(map[string]interface{}{
				"success":      true,
				"project_name": req.ProjectName,
				"node_id":      "node-abc123",
				"token":        "test-invite-token",
			})
		})

		client := server.Client()
		m := NewModelWithClient(client)

		t.Run("When I execute init with project name 'my-project'", func(t *testing.T) {
			err := m.executeInitWithClient("my-project")

			t.Run("Then the daemon init endpoint should be called", func(t *testing.T) {
				if !initCalled {
					t.Error("expected daemon /init endpoint to be called")
				}
			})

			t.Run("And the project name should be sent", func(t *testing.T) {
				if projectNameReceived != "my-project" {
					t.Errorf("expected project name 'my-project', got: %s", projectNameReceived)
				}
			})

			t.Run("And no error should be returned", func(t *testing.T) {
				if err != nil {
					t.Errorf("expected no error, got: %v", err)
				}
			})

			t.Run("And the model project name should be updated", func(t *testing.T) {
				if m.projectName != "my-project" {
					t.Errorf("expected model project name 'my-project', got: %s", m.projectName)
				}
			})
		})
	})
}

// Scenario: Execute cluster join
func TestFeature_TUIExecute_Scenario_Join(t *testing.T) {
	t.Run("Given a TUI model with daemon client", func(t *testing.T) {
		server := newMockTUIDaemonServer(t)
		defer server.Close()

		joinCalled := false
		tokenReceived := ""

		server.SetHandler("/join", func(w http.ResponseWriter, r *http.Request) {
			joinCalled = true
			var req struct {
				Token string `json:"token"`
			}
			json.NewDecoder(r.Body).Decode(&req)
			tokenReceived = req.Token

			json.NewEncoder(w).Encode(map[string]interface{}{
				"success":      true,
				"project_name": "remote-project",
				"peer_count":   3,
			})
		})

		client := server.Client()
		m := NewModelWithClient(client)

		t.Run("When I execute join with a valid token", func(t *testing.T) {
			err := m.executeJoinWithClient("invite-token-xyz123")

			t.Run("Then the daemon join endpoint should be called", func(t *testing.T) {
				if !joinCalled {
					t.Error("expected daemon /join endpoint to be called")
				}
			})

			t.Run("And the token should be sent", func(t *testing.T) {
				if tokenReceived != "invite-token-xyz123" {
					t.Errorf("expected token 'invite-token-xyz123', got: %s", tokenReceived)
				}
			})

			t.Run("And no error should be returned", func(t *testing.T) {
				if err != nil {
					t.Errorf("expected no error, got: %v", err)
				}
			})
		})
	})
}

// Scenario: Execute cluster leave
func TestFeature_TUIExecute_Scenario_Leave(t *testing.T) {
	t.Run("Given a TUI model connected to a cluster", func(t *testing.T) {
		server := newMockTUIDaemonServer(t)
		defer server.Close()

		leaveCalled := false

		server.SetHandler("/leave", func(w http.ResponseWriter, r *http.Request) {
			leaveCalled = true
			json.NewEncoder(w).Encode(daemon.LeaveResponse{
				Success: true,
				Message: "Leave initiated",
				Status: daemon.LeaveStatusResponse{
					State:       "initiated",
					CurrentStep: "Starting leave process",
				},
			})
		})

		client := server.Client()
		m := NewModelWithClient(client)

		t.Run("When I execute leave", func(t *testing.T) {
			err := m.executeLeaveWithClient()

			t.Run("Then the daemon leave endpoint should be called", func(t *testing.T) {
				if !leaveCalled {
					t.Error("expected daemon /leave endpoint to be called")
				}
			})

			t.Run("And no error should be returned", func(t *testing.T) {
				if err != nil {
					t.Errorf("expected no error, got: %v", err)
				}
			})
		})
	})
}

// Scenario: Execute lock release
func TestFeature_TUIExecute_Scenario_ReleaseLock(t *testing.T) {
	t.Run("Given a TUI model with an active lock", func(t *testing.T) {
		server := newMockTUIDaemonServer(t)
		defer server.Close()

		releaseCalled := false
		lockIDReceived := ""

		server.SetHandler("/lock/release", func(w http.ResponseWriter, r *http.Request) {
			releaseCalled = true
			var req struct {
				LockID string `json:"lock_id"`
			}
			json.NewDecoder(r.Body).Decode(&req)
			lockIDReceived = req.LockID

			json.NewEncoder(w).Encode(map[string]interface{}{
				"success": true,
			})
		})

		client := server.Client()
		m := NewModelWithClient(client)

		t.Run("When I execute release lock for 'lock-abc123'", func(t *testing.T) {
			err := m.executeReleaseLockWithClient("lock-abc123")

			t.Run("Then the daemon lock/release endpoint should be called", func(t *testing.T) {
				if !releaseCalled {
					t.Error("expected daemon /lock/release endpoint to be called")
				}
			})

			t.Run("And the lock ID should be sent", func(t *testing.T) {
				if lockIDReceived != "lock-abc123" {
					t.Errorf("expected lock ID 'lock-abc123', got: %s", lockIDReceived)
				}
			})

			t.Run("And no error should be returned", func(t *testing.T) {
				if err != nil {
					t.Errorf("expected no error, got: %v", err)
				}
			})
		})
	})
}

// Scenario: Handle daemon error gracefully
func TestFeature_TUIExecute_Scenario_HandleDaemonError(t *testing.T) {
	t.Run("Given a TUI model with daemon returning errors", func(t *testing.T) {
		server := newMockTUIDaemonServer(t)
		defer server.Close()

		server.SetHandler("/init", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"error": "project name already exists",
			})
		})

		client := server.Client()
		m := NewModelWithClient(client)

		t.Run("When the daemon returns an error", func(t *testing.T) {
			err := m.executeInitWithClient("existing-project")

			t.Run("Then the error should be returned", func(t *testing.T) {
				if err == nil {
					t.Error("expected error when daemon returns error")
				}
			})

			t.Run("And the error should be descriptive", func(t *testing.T) {
				if err != nil && err.Error() == "" {
					t.Error("error message should not be empty")
				}
			})
		})
	})
}

// Scenario: Fetch token usage from daemon
func TestFeature_TUIExecute_Scenario_FetchTokenUsage(t *testing.T) {
	t.Run("Given a TUI model with daemon running", func(t *testing.T) {
		server := newMockTUIDaemonServer(t)
		defer server.Close()

		server.SetHandler("/tokens/usage", func(w http.ResponseWriter, r *http.Request) {
			json.NewEncoder(w).Encode(daemon.TokenUsageResponse{
				TokensToday:   15000,
				TokensWeek:    75000,
				TokensMonth:   200000,
				TokensPerHour: 1250.5,
				CostToday:     0.015,
				DailyLimit:    200000,
				UsagePercent:  7.5,
			})
		})

		client := server.Client()
		m := NewModelWithClient(client)

		t.Run("When I fetch token usage", func(t *testing.T) {
			usage, err := m.fetchTokenUsageWithClient()

			t.Run("Then it should succeed", func(t *testing.T) {
				if err != nil {
					t.Fatalf("expected no error, got: %v", err)
				}
			})

			t.Run("And it should return today's token count", func(t *testing.T) {
				if usage.TodayUsed != 15000 {
					t.Errorf("expected 15000 tokens today, got: %d", usage.TodayUsed)
				}
			})

			t.Run("And it should return the daily limit", func(t *testing.T) {
				if usage.DailyLimit != 200000 {
					t.Errorf("expected 200000 daily limit, got: %d", usage.DailyLimit)
				}
			})
		})
	})
}

// Scenario: Fetch context stats from daemon
func TestFeature_TUIExecute_Scenario_FetchContextStats(t *testing.T) {
	t.Run("Given a TUI model with daemon running", func(t *testing.T) {
		server := newMockTUIDaemonServer(t)
		defer server.Close()

		server.SetHandler("/context/stats", func(w http.ResponseWriter, r *http.Request) {
			json.NewEncoder(w).Encode(daemon.ContextStatsResponse{
				TotalDocuments:  150,
				TotalEmbeddings: 150,
				SharedContexts:  45,
				WatchedFiles:    12,
				PendingDeltas:   3,
				Collections: []daemon.CollectionStats{
					{Name: "default", Count: 150, Dimension: 1536},
				},
				RecentActivity: []daemon.ContextActivity{
					{Timestamp: time.Now().Format(time.RFC3339), Type: "context_updated", FilePath: "/src/main.go"},
				},
			})
		})

		client := server.Client()
		m := NewModelWithClient(client)

		t.Run("When I fetch context stats", func(t *testing.T) {
			stats, err := m.fetchContextStatsWithClient()

			t.Run("Then it should succeed", func(t *testing.T) {
				if err != nil {
					t.Fatalf("expected no error, got: %v", err)
				}
			})

			t.Run("And it should return total embeddings", func(t *testing.T) {
				if stats.TotalEmbeddings != 150 {
					t.Errorf("expected 150 embeddings, got: %d", stats.TotalEmbeddings)
				}
			})

			t.Run("And it should return shared contexts count", func(t *testing.T) {
				if stats.SharedContexts != 45 {
					t.Errorf("expected 45 shared contexts, got: %d", stats.SharedContexts)
				}
			})
		})
	})
}

// Scenario: Daemon not running
func TestFeature_TUIExecute_Scenario_DaemonNotRunning(t *testing.T) {
	t.Run("Given a TUI model with no daemon running", func(t *testing.T) {
		// Use non-existent socket
		transport := &http.Transport{
			DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				return net.Dial("unix", "/nonexistent/socket.sock")
			},
		}
		client := daemon.NewClientWithTransport(transport, "/nonexistent/socket.sock")
		m := NewModelWithClient(client)

		t.Run("When I try to execute init", func(t *testing.T) {
			err := m.executeInitWithClient("test-project")

			t.Run("Then it should return an error", func(t *testing.T) {
				if err == nil {
					t.Error("expected error when daemon not running")
				}
			})
		})

		t.Run("When I try to execute leave", func(t *testing.T) {
			err := m.executeLeaveWithClient()

			t.Run("Then it should return an error", func(t *testing.T) {
				if err == nil {
					t.Error("expected error when daemon not running")
				}
			})
		})
	})
}
