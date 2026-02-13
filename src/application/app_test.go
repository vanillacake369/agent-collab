package application_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"agent-collab/src/application"
)

func TestNew_DefaultConfig(t *testing.T) {
	// Test creating app with nil config (should use defaults)
	app, err := application.New(nil)
	if err != nil {
		t.Fatalf("Failed to create app with default config: %v", err)
	}

	if app == nil {
		t.Fatal("App should not be nil")
	}

	config := app.Config()
	if config == nil {
		t.Fatal("Config should not be nil")
	}

	// Default config should have empty project name
	if config.ProjectName != "" {
		t.Errorf("ProjectName should be empty, got %s", config.ProjectName)
	}
}

func TestNew_CustomConfig(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "app-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	config := &application.Config{
		DataDir:     tmpDir,
		ProjectName: "test-project",
		ListenPort:  0,
	}

	app, err := application.New(config)
	if err != nil {
		t.Fatalf("Failed to create app: %v", err)
	}

	if app.Config().DataDir != tmpDir {
		t.Errorf("DataDir = %s, expected %s", app.Config().DataDir, tmpDir)
	}
}

func TestNew_DataDirCreation(t *testing.T) {
	tmpDir := filepath.Join(os.TempDir(), "app-test-newdir-"+time.Now().Format("20060102150405"))
	defer os.RemoveAll(tmpDir)

	config := &application.Config{
		DataDir: tmpDir,
	}

	_, err := application.New(config)
	if err != nil {
		t.Fatalf("Failed to create app: %v", err)
	}

	// Data dir should be created
	if _, err := os.Stat(tmpDir); os.IsNotExist(err) {
		t.Error("DataDir should have been created")
	}
}

func TestApp_Initialize(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "app-init-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	config := &application.Config{
		DataDir:    tmpDir,
		ListenPort: 0, // Auto-assign
	}

	app, err := application.New(config)
	if err != nil {
		t.Fatalf("Failed to create app: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Initialize cluster
	result, err := app.Initialize(ctx, "test-cluster")
	if err != nil {
		t.Fatalf("Failed to initialize: %v", err)
	}

	// Verify result
	if result.ProjectName != "test-cluster" {
		t.Errorf("ProjectName = %s, expected test-cluster", result.ProjectName)
	}

	if result.NodeID == "" {
		t.Error("NodeID should not be empty")
	}

	if result.InviteToken == "" {
		t.Error("InviteToken should not be empty")
	}

	if len(result.Addresses) == 0 {
		t.Error("Addresses should not be empty")
	}

	// Key file should exist
	keyPath := filepath.Join(tmpDir, "key.json")
	if _, err := os.Stat(keyPath); os.IsNotExist(err) {
		t.Error("Key file should have been created")
	}

	// Config file should exist
	configPath := filepath.Join(tmpDir, "config.json")
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		t.Error("Config file should have been created")
	}

	// Stop the app
	app.Stop()
}

func TestApp_InitializeTwiceFails(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "app-init-twice-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	config := &application.Config{
		DataDir:    tmpDir,
		ListenPort: 0,
	}

	app, err := application.New(config)
	if err != nil {
		t.Fatalf("Failed to create app: %v", err)
	}

	ctx := context.Background()

	// First initialization
	_, err = app.Initialize(ctx, "test-cluster")
	if err != nil {
		t.Fatalf("First initialize failed: %v", err)
	}

	// Start the app (marks it as running)
	if err := app.Start(); err != nil {
		t.Fatalf("Failed to start app: %v", err)
	}

	// Second initialization should fail (app is running)
	_, err = app.Initialize(ctx, "another-cluster")
	if err == nil {
		t.Error("Expected error when initializing twice while running")
	}

	app.Stop()
}

func TestApp_LoadFromConfig_NoConfig(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "app-load-noconfig-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	config := &application.Config{
		DataDir: tmpDir,
	}

	app, err := application.New(config)
	if err != nil {
		t.Fatalf("Failed to create app: %v", err)
	}

	ctx := context.Background()

	// LoadFromConfig should fail when no config exists
	err = app.LoadFromConfig(ctx)
	if err == nil {
		t.Error("Expected error when loading from non-existent config")
	}
}

func TestApp_LoadFromConfig_AfterInit(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "app-load-afterinit-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	config := &application.Config{
		DataDir:    tmpDir,
		ListenPort: 0,
	}

	// First app instance: initialize
	app1, err := application.New(config)
	if err != nil {
		t.Fatalf("Failed to create app1: %v", err)
	}

	ctx := context.Background()
	result, err := app1.Initialize(ctx, "load-test-cluster")
	if err != nil {
		t.Fatalf("Failed to initialize: %v", err)
	}

	originalNodeID := result.NodeID
	app1.Stop()

	// Second app instance: load from config
	app2, err := application.New(config)
	if err != nil {
		t.Fatalf("Failed to create app2: %v", err)
	}

	err = app2.LoadFromConfig(ctx)
	if err != nil {
		t.Fatalf("Failed to load from config: %v", err)
	}

	// Node ID should be the same (loaded same key)
	if app2.Node() == nil {
		t.Fatal("Node should not be nil after LoadFromConfig")
	}

	loadedNodeID := app2.Node().ID().String()
	if loadedNodeID != originalNodeID {
		t.Errorf("NodeID mismatch: got %s, expected %s", loadedNodeID, originalNodeID)
	}

	// Project name should be loaded
	if app2.Config().ProjectName != "load-test-cluster" {
		t.Errorf("ProjectName = %s, expected load-test-cluster", app2.Config().ProjectName)
	}

	app2.Stop()
}

func TestApp_GetStatus_NotInitialized(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "app-status-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	config := &application.Config{
		DataDir: tmpDir,
	}

	app, err := application.New(config)
	if err != nil {
		t.Fatalf("Failed to create app: %v", err)
	}

	status := app.GetStatus()
	if status == nil {
		t.Fatal("Status should not be nil")
	}

	if status.Running {
		t.Error("Running should be false before initialization")
	}

	if status.NodeID != "" {
		t.Error("NodeID should be empty before initialization")
	}
}

func TestApp_GetStatus_Initialized(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "app-status-init-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	config := &application.Config{
		DataDir:    tmpDir,
		ListenPort: 0,
	}

	app, err := application.New(config)
	if err != nil {
		t.Fatalf("Failed to create app: %v", err)
	}

	ctx := context.Background()
	_, err = app.Initialize(ctx, "status-test")
	if err != nil {
		t.Fatalf("Failed to initialize: %v", err)
	}

	app.Start()
	defer app.Stop()

	status := app.GetStatus()

	if !status.Running {
		t.Error("Running should be true after Start")
	}

	if status.ProjectName != "status-test" {
		t.Errorf("ProjectName = %s, expected status-test", status.ProjectName)
	}

	if status.NodeID == "" {
		t.Error("NodeID should not be empty")
	}

	if len(status.Addresses) == 0 {
		t.Error("Addresses should not be empty")
	}
}

func TestApp_Services_AfterInit(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "app-services-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	config := &application.Config{
		DataDir:    tmpDir,
		ListenPort: 0,
	}

	app, err := application.New(config)
	if err != nil {
		t.Fatalf("Failed to create app: %v", err)
	}

	ctx := context.Background()
	_, err = app.Initialize(ctx, "services-test")
	if err != nil {
		t.Fatalf("Failed to initialize: %v", err)
	}
	defer app.Stop()

	// All services should be initialized
	if app.LockService() == nil {
		t.Error("LockService should not be nil")
	}

	if app.SyncManager() == nil {
		t.Error("SyncManager should not be nil")
	}

	if app.Node() == nil {
		t.Error("Node should not be nil")
	}

	if app.KeyPair() == nil {
		t.Error("KeyPair should not be nil")
	}

	if app.VectorStore() == nil {
		t.Error("VectorStore should not be nil")
	}

	if app.EmbeddingService() == nil {
		t.Error("EmbeddingService should not be nil")
	}

	if app.TokenTracker() == nil {
		t.Error("TokenTracker should not be nil")
	}

	if app.AgentRegistry() == nil {
		t.Error("AgentRegistry should not be nil")
	}
}

func TestApp_CreateInviteToken_BeforeInit(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "app-token-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	config := &application.Config{
		DataDir: tmpDir,
	}

	app, err := application.New(config)
	if err != nil {
		t.Fatalf("Failed to create app: %v", err)
	}

	// Should fail before initialization
	_, err = app.CreateInviteToken()
	if err == nil {
		t.Error("Expected error when creating token before init")
	}
}

func TestApp_CreateInviteToken_AfterInit(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "app-token-init-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	config := &application.Config{
		DataDir:    tmpDir,
		ListenPort: 0,
	}

	app, err := application.New(config)
	if err != nil {
		t.Fatalf("Failed to create app: %v", err)
	}

	ctx := context.Background()
	_, err = app.Initialize(ctx, "token-test")
	if err != nil {
		t.Fatalf("Failed to initialize: %v", err)
	}
	defer app.Stop()

	// Should succeed after initialization
	token, err := app.CreateInviteToken()
	if err != nil {
		t.Fatalf("Failed to create invite token: %v", err)
	}

	if token == "" {
		t.Error("Token should not be empty")
	}
}

func TestDefaultConfig(t *testing.T) {
	config := application.DefaultConfig()

	if config == nil {
		t.Fatal("DefaultConfig should not return nil")
	}

	if config.DataDir == "" {
		t.Error("DataDir should not be empty")
	}

	// Should be in user's home directory
	home, err := os.UserHomeDir()
	if err == nil {
		expected := filepath.Join(home, ".agent-collab")
		if config.DataDir != expected {
			t.Errorf("DataDir = %s, expected %s", config.DataDir, expected)
		}
	}

	// ListenPort should be 0 (auto-assign)
	if config.ListenPort != 0 {
		t.Errorf("ListenPort = %d, expected 0", config.ListenPort)
	}
}
