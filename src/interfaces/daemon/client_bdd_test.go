package daemon

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// BDD-style tests for daemon client
// Feature: Daemon Client API
// As a TUI or CLI application
// I want to communicate with the daemon via client API
// So that I can manage the cluster and retrieve information

// Mock server helper
type mockDaemonServer struct {
	listener   net.Listener
	socketPath string
	handlers   map[string]http.HandlerFunc
}

func newMockDaemonServer(t *testing.T) *mockDaemonServer {
	t.Helper()

	// Create temp socket path (keep it short to avoid UNIX socket path length limits)
	socketPath := filepath.Join(os.TempDir(), fmt.Sprintf("daemon-test-%d.sock", time.Now().UnixNano()%100000))

	// Create handlers map
	handlers := make(map[string]http.HandlerFunc)

	// Create mux with default handlers
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if handler, ok := handlers[r.URL.Path]; ok {
			handler(w, r)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})

	// Create unix socket listener
	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		t.Fatalf("failed to create socket: %v", err)
	}

	server := &http.Server{Handler: mux}
	go server.Serve(listener)

	return &mockDaemonServer{
		listener:   listener,
		socketPath: socketPath,
		handlers:   handlers,
	}
}

func (m *mockDaemonServer) SetHandler(path string, handler http.HandlerFunc) {
	m.handlers[path] = handler
}

func (m *mockDaemonServer) Close() {
	m.listener.Close()
	os.Remove(m.socketPath)
}

func (m *mockDaemonServer) Client() *Client {
	transport := &http.Transport{
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			return net.Dial("unix", m.socketPath)
		},
	}
	return &Client{
		socketPath:  m.socketPath,
		eventClient: NewEventClient(),
		httpClient: &http.Client{
			Transport: transport,
			Timeout:   5 * time.Second,
		},
	}
}

// Scenario: Leave cluster gracefully
func TestFeature_DaemonClient_Scenario_LeaveCluster(t *testing.T) {
	t.Run("Given a running daemon with active cluster", func(t *testing.T) {
		server := newMockDaemonServer(t)
		defer server.Close()

		// Setup leave handler
		server.SetHandler("/leave", func(w http.ResponseWriter, r *http.Request) {
			json.NewEncoder(w).Encode(LeaveResponse{
				Success: true,
				Message: "Leave process initiated",
				Status: LeaveStatusResponse{
					State:       "initiated",
					CurrentStep: "Initiating leave process",
				},
			})
		})

		client := server.Client()

		t.Run("When I call Leave()", func(t *testing.T) {
			result, err := client.Leave()

			t.Run("Then it should succeed", func(t *testing.T) {
				if err != nil {
					t.Fatalf("expected no error, got: %v", err)
				}
			})

			t.Run("And the response should indicate leave initiated", func(t *testing.T) {
				if !result.Success {
					t.Error("expected success to be true")
				}
			})

			t.Run("And the status should show the current state", func(t *testing.T) {
				if result.Status.State != "initiated" {
					t.Errorf("expected state 'initiated', got: %s", result.Status.State)
				}
			})
		})
	})

	t.Run("Given a daemon with leave already in progress", func(t *testing.T) {
		server := newMockDaemonServer(t)
		defer server.Close()

		server.SetHandler("/leave", func(w http.ResponseWriter, r *http.Request) {
			json.NewEncoder(w).Encode(LeaveResponse{
				Success: false,
				Error:   "leave already in progress",
				Status: LeaveStatusResponse{
					State:       "releasing_locks",
					CurrentStep: "Releasing all locks",
				},
			})
		})

		client := server.Client()

		t.Run("When I call Leave()", func(t *testing.T) {
			result, err := client.Leave()

			t.Run("Then it should return an error", func(t *testing.T) {
				if err == nil {
					t.Error("expected error for leave in progress")
				}
			})

			t.Run("And result should still contain status", func(t *testing.T) {
				if result != nil && result.Status.State == "" {
					t.Error("should have status even on error")
				}
			})
		})
	})
}

// Scenario: Check leave status
func TestFeature_DaemonClient_Scenario_LeaveStatus(t *testing.T) {
	t.Run("Given a leave process in progress", func(t *testing.T) {
		server := newMockDaemonServer(t)
		defer server.Close()

		server.SetHandler("/leave/status", func(w http.ResponseWriter, r *http.Request) {
			json.NewEncoder(w).Encode(LeaveStatusResponse{
				State:         "syncing",
				CurrentStep:   "Syncing pending context",
				LocksReleased: 5,
				ContextSynced: false,
				StartedAt:     time.Now().Add(-2 * time.Second).Format(time.RFC3339),
			})
		})

		client := server.Client()

		t.Run("When I call LeaveStatus()", func(t *testing.T) {
			status, err := client.LeaveStatus()

			t.Run("Then it should succeed", func(t *testing.T) {
				if err != nil {
					t.Fatalf("expected no error, got: %v", err)
				}
			})

			t.Run("And it should show the current state", func(t *testing.T) {
				if status.State != "syncing" {
					t.Errorf("expected state 'syncing', got: %s", status.State)
				}
			})

			t.Run("And it should show locks released count", func(t *testing.T) {
				if status.LocksReleased != 5 {
					t.Errorf("expected 5 locks released, got: %d", status.LocksReleased)
				}
			})
		})
	})
}

// Scenario: Get token usage statistics
func TestFeature_DaemonClient_Scenario_TokenUsage(t *testing.T) {
	t.Run("Given a daemon with token tracking enabled", func(t *testing.T) {
		server := newMockDaemonServer(t)
		defer server.Close()

		server.SetHandler("/tokens/usage", func(w http.ResponseWriter, r *http.Request) {
			json.NewEncoder(w).Encode(TokenUsageResponse{
				TokensToday:   15000,
				TokensWeek:    75000,
				TokensMonth:   200000,
				TokensPerHour: 1250.5,
				CostToday:     0.015,
				CostWeek:      0.075,
				CostMonth:     0.20,
				DailyLimit:    200000,
				UsagePercent:  7.5,
				Provider:      "openai",
				Model:         "text-embedding-3-small",
			})
		})

		client := server.Client()

		t.Run("When I call TokenUsage()", func(t *testing.T) {
			usage, err := client.TokenUsage()

			t.Run("Then it should succeed", func(t *testing.T) {
				if err != nil {
					t.Fatalf("expected no error, got: %v", err)
				}
			})

			t.Run("And it should return today's token count", func(t *testing.T) {
				if usage.TokensToday != 15000 {
					t.Errorf("expected 15000 tokens today, got: %d", usage.TokensToday)
				}
			})

			t.Run("And it should return usage percentage", func(t *testing.T) {
				if usage.UsagePercent != 7.5 {
					t.Errorf("expected 7.5%% usage, got: %.1f%%", usage.UsagePercent)
				}
			})

			t.Run("And it should return cost estimates", func(t *testing.T) {
				if usage.CostToday != 0.015 {
					t.Errorf("expected $0.015 cost today, got: $%.3f", usage.CostToday)
				}
			})

			t.Run("And it should include provider information", func(t *testing.T) {
				if usage.Provider != "openai" {
					t.Errorf("expected provider 'openai', got: %s", usage.Provider)
				}
			})
		})
	})
}

// Scenario: Get context statistics
func TestFeature_DaemonClient_Scenario_ContextStats(t *testing.T) {
	t.Run("Given a daemon with vector store initialized", func(t *testing.T) {
		server := newMockDaemonServer(t)
		defer server.Close()

		server.SetHandler("/context/stats", func(w http.ResponseWriter, r *http.Request) {
			json.NewEncoder(w).Encode(ContextStatsResponse{
				TotalDocuments:  150,
				TotalEmbeddings: 150,
				SharedContexts:  45,
				WatchedFiles:    12,
				PendingDeltas:   3,
				Collections: []CollectionStats{
					{Name: "default", Count: 150, Dimension: 1536},
				},
				RecentActivity: []ContextActivity{
					{Timestamp: time.Now().Format(time.RFC3339), Type: "context_updated", FilePath: "/src/main.go"},
				},
			})
		})

		client := server.Client()

		t.Run("When I call ContextStats()", func(t *testing.T) {
			stats, err := client.ContextStats()

			t.Run("Then it should succeed", func(t *testing.T) {
				if err != nil {
					t.Fatalf("expected no error, got: %v", err)
				}
			})

			t.Run("And it should return total documents", func(t *testing.T) {
				if stats.TotalDocuments != 150 {
					t.Errorf("expected 150 documents, got: %d", stats.TotalDocuments)
				}
			})

			t.Run("And it should return shared contexts count", func(t *testing.T) {
				if stats.SharedContexts != 45 {
					t.Errorf("expected 45 shared contexts, got: %d", stats.SharedContexts)
				}
			})

			t.Run("And it should return watched files count", func(t *testing.T) {
				if stats.WatchedFiles != 12 {
					t.Errorf("expected 12 watched files, got: %d", stats.WatchedFiles)
				}
			})

			t.Run("And it should include collection info", func(t *testing.T) {
				if len(stats.Collections) == 0 {
					t.Error("expected at least one collection")
				}
				if stats.Collections[0].Dimension != 1536 {
					t.Errorf("expected dimension 1536, got: %d", stats.Collections[0].Dimension)
				}
			})

			t.Run("And it should include recent activity", func(t *testing.T) {
				if len(stats.RecentActivity) == 0 {
					t.Error("expected recent activity")
				}
			})
		})
	})
}

// Scenario: Check if daemon is running
func TestFeature_DaemonClient_Scenario_IsRunning(t *testing.T) {
	t.Run("Given a running daemon", func(t *testing.T) {
		server := newMockDaemonServer(t)
		defer server.Close()

		server.SetHandler("/status", func(w http.ResponseWriter, r *http.Request) {
			json.NewEncoder(w).Encode(StatusResponse{
				Running:     true,
				ProjectName: "test-project",
				PeerCount:   2,
			})
		})

		// Override the socketPath check
		client := server.Client()

		t.Run("When I check IsRunning()", func(t *testing.T) {
			running := client.IsRunning()

			t.Run("Then it should return true", func(t *testing.T) {
				if !running {
					t.Error("expected IsRunning to return true")
				}
			})
		})
	})

	t.Run("Given no daemon running", func(t *testing.T) {
		// Use non-existent socket
		client := &Client{
			socketPath: "/nonexistent/socket.sock",
			httpClient: &http.Client{Timeout: 1 * time.Second},
		}

		t.Run("When I check IsRunning()", func(t *testing.T) {
			running := client.IsRunning()

			t.Run("Then it should return false", func(t *testing.T) {
				if running {
					t.Error("expected IsRunning to return false")
				}
			})
		})
	})
}

// Scenario: Get metrics
func TestFeature_DaemonClient_Scenario_Metrics(t *testing.T) {
	t.Run("Given a daemon with metrics collection", func(t *testing.T) {
		server := newMockDaemonServer(t)
		defer server.Close()

		server.SetHandler("/metrics", func(w http.ResponseWriter, r *http.Request) {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"messages_sent":     100,
				"messages_received": 150,
				"bytes_sent":        50000,
				"bytes_received":    75000,
				"connected_peers":   3,
			})
		})

		client := server.Client()

		t.Run("When I call Metrics()", func(t *testing.T) {
			metrics, err := client.Metrics()

			t.Run("Then it should succeed", func(t *testing.T) {
				if err != nil {
					t.Fatalf("expected no error, got: %v", err)
				}
			})

			t.Run("And it should return metrics data", func(t *testing.T) {
				if metrics == nil {
					t.Error("expected metrics data")
				}
			})
		})
	})
}

// Scenario: Handle connection errors gracefully
func TestFeature_DaemonClient_Scenario_ConnectionErrors(t *testing.T) {
	t.Run("Given a daemon that is not running", func(t *testing.T) {
		tmpDir := t.TempDir()
		socketPath := filepath.Join(tmpDir, "nonexistent.sock")

		transport := &http.Transport{
			DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				return net.Dial("unix", socketPath)
			},
		}
		client := &Client{
			socketPath: socketPath,
			httpClient: &http.Client{
				Transport: transport,
				Timeout:   1 * time.Second,
			},
		}

		t.Run("When I call Status()", func(t *testing.T) {
			_, err := client.Status()

			t.Run("Then it should return an error", func(t *testing.T) {
				if err == nil {
					t.Error("expected error when daemon not running")
				}
			})
		})

		t.Run("When I call Leave()", func(t *testing.T) {
			_, err := client.Leave()

			t.Run("Then it should return an error", func(t *testing.T) {
				if err == nil {
					t.Error("expected error when daemon not running")
				}
			})
		})
	})
}

// Scenario: Existing socket but server error
func TestFeature_DaemonClient_Scenario_ServerErrors(t *testing.T) {
	t.Run("Given a daemon returning server errors", func(t *testing.T) {
		server := newMockDaemonServer(t)
		defer server.Close()

		server.SetHandler("/leave", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "internal server error",
			})
		})

		client := server.Client()

		t.Run("When I call Leave() and server returns 500", func(t *testing.T) {
			result, _ := client.Leave()

			t.Run("Then the result should indicate failure", func(t *testing.T) {
				// Even with 500, we should handle gracefully
				if result != nil && result.Success {
					t.Error("should not indicate success on server error")
				}
			})
		})
	})
}

// Helper to ensure file exists for IsRunning check
func createSocketFile(t *testing.T, path string) {
	t.Helper()
	dir := filepath.Dir(path)
	os.MkdirAll(dir, 0755)
	f, _ := os.Create(path)
	f.Close()
}
