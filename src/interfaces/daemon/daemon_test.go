package daemon_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"agent-collab/src/application"
	"agent-collab/src/interfaces/daemon"
)

func TestDaemonStartStop(t *testing.T) {
	// Create temp directory for test
	tmpDir, err := os.MkdirTemp("", "daemon-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create application with custom config
	cfg := &application.Config{
		DataDir:    tmpDir,
		ListenPort: 0, // Auto-assign
	}
	app, err := application.New(cfg)
	if err != nil {
		t.Fatalf("Failed to create app: %v", err)
	}

	// Initialize the app first (required before daemon start)
	ctx := context.Background()
	_, err = app.Initialize(ctx, "daemon-test-project")
	if err != nil {
		t.Fatalf("Failed to initialize app: %v", err)
	}

	// Create server
	server := daemon.NewServer(app)

	// Start daemon
	if err := server.Start(ctx); err != nil {
		t.Fatalf("Failed to start daemon: %v", err)
	}

	// Verify socket exists
	socketPath := daemon.DefaultSocketPath()
	if _, err := os.Stat(socketPath); os.IsNotExist(err) {
		t.Errorf("Socket file not created: %s", socketPath)
	}

	// Verify PID file exists
	pidFile := daemon.DefaultPIDFile()
	if _, err := os.Stat(pidFile); os.IsNotExist(err) {
		t.Errorf("PID file not created: %s", pidFile)
	}

	// Test client connection
	client := daemon.NewClient()
	if !client.IsRunning() {
		t.Error("Client.IsRunning() returned false, expected true")
	}

	// Get status
	status, err := client.Status()
	if err != nil {
		t.Errorf("Failed to get status: %v", err)
	}
	if status == nil {
		t.Error("Status is nil")
	} else {
		if status.ProjectName != "daemon-test-project" {
			t.Errorf("ProjectName = %s, expected daemon-test-project", status.ProjectName)
		}
	}

	// Stop daemon
	if err := server.Stop(); err != nil {
		t.Errorf("Failed to stop daemon: %v", err)
	}

	// Wait for cleanup
	time.Sleep(100 * time.Millisecond)

	// Verify socket is removed
	if _, err := os.Stat(socketPath); !os.IsNotExist(err) {
		t.Error("Socket file should be removed after stop")
	}
}

func TestClientIsRunningWithoutDaemon(t *testing.T) {
	// Create client without running daemon
	client := daemon.NewClient()

	// Should return false when daemon is not running
	if client.IsRunning() {
		t.Error("Client.IsRunning() should return false when daemon is not running")
	}
}

func TestDaemonSocketDirectory(t *testing.T) {
	socketPath := daemon.DefaultSocketPath()
	dir := filepath.Dir(socketPath)

	// Directory should be in user's home
	home, _ := os.UserHomeDir()
	expectedDir := filepath.Join(home, ".agent-collab")

	if dir != expectedDir {
		t.Errorf("Socket directory = %s, expected %s", dir, expectedDir)
	}
}

func TestDaemonPIDFile(t *testing.T) {
	pidFile := daemon.DefaultPIDFile()

	// PID file should be in user's home
	home, _ := os.UserHomeDir()
	expectedFile := filepath.Join(home, ".agent-collab", "daemon.pid")

	if pidFile != expectedFile {
		t.Errorf("PID file = %s, expected %s", pidFile, expectedFile)
	}
}
