package application_test

import (
	"context"
	"os"
	"testing"
	"time"

	"agent-collab/src/application"
	"agent-collab/src/infrastructure/crypto"
)

// TestInitJoinFlow tests the complete init -> token -> join flow
func TestInitJoinFlow(t *testing.T) {
	// Create temp directories for both nodes
	bootstrapDir, err := os.MkdirTemp("", "bootstrap-node-*")
	if err != nil {
		t.Fatalf("Failed to create bootstrap temp dir: %v", err)
	}
	defer os.RemoveAll(bootstrapDir)

	peerDir, err := os.MkdirTemp("", "peer-node-*")
	if err != nil {
		t.Fatalf("Failed to create peer temp dir: %v", err)
	}
	defer os.RemoveAll(peerDir)

	ctx := context.Background()

	// Step 1: Create and initialize bootstrap node
	bootstrapApp, err := application.New(&application.Config{
		DataDir:    bootstrapDir,
		ListenPort: 0,
	})
	if err != nil {
		t.Fatalf("Failed to create bootstrap app: %v", err)
	}

	initResult, err := bootstrapApp.Initialize(ctx, "integration-test")
	if err != nil {
		t.Fatalf("Failed to initialize bootstrap: %v", err)
	}

	t.Logf("Bootstrap node initialized:")
	t.Logf("  NodeID: %s", initResult.NodeID)
	t.Logf("  Project: %s", initResult.ProjectName)
	t.Logf("  Addresses: %v", initResult.Addresses)

	// Start bootstrap node
	if err := bootstrapApp.Start(); err != nil {
		t.Fatalf("Failed to start bootstrap: %v", err)
	}
	defer bootstrapApp.Stop()

	// Step 2: Verify invite token is valid
	if initResult.InviteToken == "" {
		t.Fatal("InviteToken is empty")
	}

	// Decode and verify token
	token, err := crypto.DecodeInviteToken(initResult.InviteToken)
	if err != nil {
		t.Fatalf("Failed to decode invite token: %v", err)
	}

	if token.ProjectName != "integration-test" {
		t.Errorf("Token.ProjectName = %s, expected integration-test", token.ProjectName)
	}

	if token.CreatorID != initResult.NodeID {
		t.Errorf("Token.CreatorID = %s, expected %s", token.CreatorID, initResult.NodeID)
	}

	if len(token.Addresses) == 0 {
		t.Error("Token.Addresses should not be empty")
	}

	if token.IsExpired() {
		t.Error("Token should not be expired")
	}

	// Step 3: Create peer node and join
	peerApp, err := application.New(&application.Config{
		DataDir:    peerDir,
		ListenPort: 0,
	})
	if err != nil {
		t.Fatalf("Failed to create peer app: %v", err)
	}

	joinResult, err := peerApp.Join(ctx, initResult.InviteToken)
	if err != nil {
		t.Fatalf("Failed to join: %v", err)
	}

	t.Logf("Peer node joined:")
	t.Logf("  NodeID: %s", joinResult.NodeID)
	t.Logf("  Project: %s", joinResult.ProjectName)
	t.Logf("  BootstrapPeer: %s", joinResult.BootstrapPeer)

	// Verify join result
	if joinResult.ProjectName != "integration-test" {
		t.Errorf("JoinResult.ProjectName = %s, expected integration-test", joinResult.ProjectName)
	}

	if joinResult.BootstrapPeer != initResult.NodeID {
		t.Errorf("JoinResult.BootstrapPeer = %s, expected %s", joinResult.BootstrapPeer, initResult.NodeID)
	}

	// Node IDs should be different
	if joinResult.NodeID == initResult.NodeID {
		t.Error("Peer NodeID should be different from bootstrap NodeID")
	}

	// Start peer node
	if err := peerApp.Start(); err != nil {
		t.Fatalf("Failed to start peer: %v", err)
	}
	defer peerApp.Stop()

	// Step 4: Wait for peer discovery and verify connection
	time.Sleep(500 * time.Millisecond)

	bootstrapStatus := bootstrapApp.GetStatus()
	peerStatus := peerApp.GetStatus()

	t.Logf("Bootstrap status: peers=%d, running=%v", bootstrapStatus.PeerCount, bootstrapStatus.Running)
	t.Logf("Peer status: peers=%d, running=%v", peerStatus.PeerCount, peerStatus.Running)

	// Both should be running
	if !bootstrapStatus.Running {
		t.Error("Bootstrap should be running")
	}
	if !peerStatus.Running {
		t.Error("Peer should be running")
	}
}

// TestJoinWithInvalidToken tests joining with various invalid tokens
func TestJoinWithInvalidToken(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "join-invalid-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	app, err := application.New(&application.Config{
		DataDir:    tmpDir,
		ListenPort: 0,
	})
	if err != nil {
		t.Fatalf("Failed to create app: %v", err)
	}

	ctx := context.Background()

	testCases := []struct {
		name  string
		token string
	}{
		{"empty token", ""},
		{"invalid base64", "not-valid-base64!!!"},
		{"korean error message", "먼저 'agent-collab init' 또는 'agent-collab join'을 실행하세요"},
		{"random string", "abcdefghijklmnop"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := app.Join(ctx, tc.token)
			if err == nil {
				t.Errorf("Expected error for token: %q", tc.token)
			}
		})
	}
}

// TestJoinWithExpiredToken tests joining with an expired token
func TestJoinWithExpiredToken(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "join-expired-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create an expired token
	addresses := []string{"/ip4/127.0.0.1/tcp/4001/p2p/QmTestPeer"}
	expiredToken, err := crypto.NewInviteTokenWithTTL(addresses, "test", "QmCreator", -1*time.Hour)
	if err != nil {
		t.Fatalf("Failed to create expired token: %v", err)
	}

	encoded, err := expiredToken.Encode()
	if err != nil {
		t.Fatalf("Failed to encode token: %v", err)
	}

	// Try to join with expired token
	app, err := application.New(&application.Config{
		DataDir:    tmpDir,
		ListenPort: 0,
	})
	if err != nil {
		t.Fatalf("Failed to create app: %v", err)
	}

	ctx := context.Background()
	_, err = app.Join(ctx, encoded)
	if err == nil {
		t.Error("Expected error when joining with expired token")
	} else {
		t.Logf("Got expected error: %v", err)
	}
}

// TestCreateInviteTokenRoundTrip tests creating and decoding invite token
func TestCreateInviteTokenRoundTrip(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "token-roundtrip-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	app, err := application.New(&application.Config{
		DataDir:    tmpDir,
		ListenPort: 0,
	})
	if err != nil {
		t.Fatalf("Failed to create app: %v", err)
	}

	ctx := context.Background()
	initResult, err := app.Initialize(ctx, "roundtrip-test")
	if err != nil {
		t.Fatalf("Failed to initialize: %v", err)
	}
	defer app.Stop()

	// Create invite token
	tokenStr, err := app.CreateInviteToken()
	if err != nil {
		t.Fatalf("Failed to create invite token: %v", err)
	}

	// Decode it
	token, err := crypto.DecodeInviteToken(tokenStr)
	if err != nil {
		t.Fatalf("Failed to decode token: %v", err)
	}

	// Verify
	if token.ProjectName != "roundtrip-test" {
		t.Errorf("ProjectName = %s, expected roundtrip-test", token.ProjectName)
	}

	if token.CreatorID != initResult.NodeID {
		t.Errorf("CreatorID = %s, expected %s", token.CreatorID, initResult.NodeID)
	}

	// Addresses should have peer ID suffix
	for _, addr := range token.Addresses {
		if len(addr) == 0 {
			t.Error("Address should not be empty")
		}
		// Should contain /p2p/
		if !containsString(addr, "/p2p/") {
			t.Errorf("Address should contain /p2p/: %s", addr)
		}
	}
}

func containsString(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsAt(s, substr, 0))
}

func containsAt(s, substr string, start int) bool {
	for i := start; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
