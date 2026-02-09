//go:build e2e

package e2e

import (
	"context"
	"os"
	"testing"
	"time"

	"agent-collab/internal/application"
	"agent-collab/internal/infrastructure/network/wireguard"
	"agent-collab/internal/infrastructure/network/wireguard/platform"
)

// isRoot returns true if running as root/admin.
func isRoot() bool {
	return os.Geteuid() == 0
}

// TestWireGuardClusterFormation tests forming a cluster with WireGuard enabled.
// This test requires root privileges to create actual WireGuard interfaces.
func TestWireGuardClusterFormation(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping WireGuard e2e test in short mode")
	}

	// Skip if not root (actual WireGuard requires root)
	if !isRoot() {
		t.Skip("skipping WireGuard cluster test: root privileges required")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// Create test cluster with WireGuard support
	cluster := NewWireGuardTestCluster(t, 2)

	// Initialize leader with WireGuard
	leader, initResult, err := cluster.InitializeLeaderWithWireGuard(ctx, "test-wg-project")
	if err != nil {
		t.Fatalf("failed to initialize leader: %v", err)
	}

	t.Logf("Leader initialized: NodeID=%s, WireGuardIP=%s", initResult.NodeID, initResult.WireGuardIP)

	// Verify WireGuard is enabled on leader
	if !initResult.WireGuardEnabled {
		t.Fatal("WireGuard should be enabled on leader")
	}

	if initResult.WireGuardIP == "" {
		t.Fatal("WireGuard IP should be assigned")
	}

	if initResult.InviteToken == "" {
		t.Fatal("invite token should be generated")
	}

	// Join follower with WireGuard
	follower, joinResult, err := cluster.JoinNodeWithWireGuard(ctx, "follower", initResult.InviteToken)
	if err != nil {
		t.Fatalf("failed to join follower: %v", err)
	}

	t.Logf("Follower joined: NodeID=%s, WireGuardIP=%s", joinResult.NodeID, joinResult.WireGuardIP)

	// Verify WireGuard is enabled on follower
	if !joinResult.WireGuardEnabled {
		t.Fatal("WireGuard should be enabled on follower")
	}

	if joinResult.WireGuardIP == "" {
		t.Fatal("WireGuard IP should be assigned to follower")
	}

	// Verify different IPs assigned
	if initResult.WireGuardIP == joinResult.WireGuardIP {
		t.Fatalf("Leader and follower should have different WireGuard IPs: %s vs %s",
			initResult.WireGuardIP, joinResult.WireGuardIP)
	}

	// Wait for peers to connect
	if err := cluster.WaitForPeers(leader, 1, 10*time.Second); err != nil {
		t.Fatalf("leader failed to see follower: %v", err)
	}

	if err := cluster.WaitForPeers(follower, 1, 10*time.Second); err != nil {
		t.Fatalf("follower failed to see leader: %v", err)
	}

	t.Log("WireGuard cluster formation successful!")
}

// TestWireGuardIPAllocation tests that IPs are allocated correctly.
// This test requires root privileges to create actual WireGuard interfaces.
func TestWireGuardIPAllocation(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping WireGuard e2e test in short mode")
	}

	// Skip if not root (actual WireGuard requires root)
	if !isRoot() {
		t.Skip("skipping WireGuard IP allocation test: root privileges required")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	// Create cluster with 4 nodes
	cluster := NewWireGuardTestCluster(t, 4)

	// Initialize leader
	_, initResult, err := cluster.InitializeLeaderWithWireGuard(ctx, "test-ip-alloc")
	if err != nil {
		t.Fatalf("failed to initialize leader: %v", err)
	}

	// Expected: leader gets .1
	expectedLeaderIP := "10.100.0.1/24"
	if initResult.WireGuardIP != expectedLeaderIP {
		t.Errorf("leader IP = %s, want %s", initResult.WireGuardIP, expectedLeaderIP)
	}

	// Join 3 followers
	expectedIPs := []string{"10.100.0.2/24", "10.100.0.3/24", "10.100.0.4/24"}
	for i, expectedIP := range expectedIPs {
		_, joinResult, err := cluster.JoinNodeWithWireGuard(ctx, "follower-"+string(rune('a'+i)), initResult.InviteToken)
		if err != nil {
			t.Fatalf("failed to join follower-%d: %v", i, err)
		}

		if joinResult.WireGuardIP != expectedIP {
			t.Errorf("follower-%d IP = %s, want %s", i, joinResult.WireGuardIP, expectedIP)
		}
	}

	t.Log("IP allocation test passed!")
}

// TestWireGuardFallbackToLibp2p tests that cluster works without WireGuard.
func TestWireGuardFallbackToLibp2p(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping WireGuard e2e test in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// Create regular cluster (no WireGuard)
	cluster := NewTestCluster(t, 2)

	// Initialize leader without WireGuard
	leader, initResult, err := cluster.InitializeLeader(ctx, "test-no-wg")
	if err != nil {
		t.Fatalf("failed to initialize leader: %v", err)
	}

	t.Logf("Leader initialized without WireGuard: NodeID=%s", initResult.NodeID)

	// Join follower
	follower, err := cluster.JoinNode(ctx, "follower", initResult.InviteToken)
	if err != nil {
		t.Fatalf("failed to join follower: %v", err)
	}

	// Wait for peers
	if err := cluster.WaitForPeers(leader, 1, 10*time.Second); err != nil {
		t.Fatalf("leader failed to see follower: %v", err)
	}

	if err := cluster.WaitForPeers(follower, 1, 10*time.Second); err != nil {
		t.Fatalf("follower failed to see leader: %v", err)
	}

	t.Log("Fallback to libp2p-only cluster successful!")
}

// TestWireGuardManagerLifecycle tests WireGuard manager start/stop.
func TestWireGuardManagerLifecycle(t *testing.T) {
	// Use mock platform for this test (no root required)
	mockPlatform := platform.NewMockPlatform()
	manager := wireguard.NewManager(mockPlatform)

	ctx := context.Background()

	// Initialize
	cfg := &wireguard.ManagerConfig{
		InterfaceName:       "wg-test",
		ListenPort:          51820,
		Subnet:              "10.100.0.0/24",
		MTU:                 1420,
		PersistentKeepalive: 25,
		AutoDetectEndpoint:  false,
	}

	if err := manager.Initialize(ctx, cfg); err != nil {
		t.Fatalf("failed to initialize manager: %v", err)
	}

	// Verify config
	config := manager.GetConfig()
	if config == nil {
		t.Fatal("config should not be nil after initialization")
	}

	if config.Subnet != "10.100.0.0/24" {
		t.Errorf("subnet = %s, want 10.100.0.0/24", config.Subnet)
	}

	// Get local IP
	localIP := manager.GetLocalIP()
	if localIP != "10.100.0.1/24" {
		t.Errorf("local IP = %s, want 10.100.0.1/24", localIP)
	}

	// Start
	if err := manager.Start(ctx); err != nil {
		t.Fatalf("failed to start manager: %v", err)
	}

	if !manager.IsRunning() {
		t.Error("manager should be running after Start()")
	}

	// Stop
	if err := manager.Stop(); err != nil {
		t.Fatalf("failed to stop manager: %v", err)
	}

	if manager.IsRunning() {
		t.Error("manager should not be running after Stop()")
	}

	t.Log("Manager lifecycle test passed!")
}

// TestWireGuardPeerManagement tests adding/removing peers.
func TestWireGuardPeerManagement(t *testing.T) {
	mockPlatform := platform.NewMockPlatform()
	manager := wireguard.NewManager(mockPlatform)

	ctx := context.Background()

	cfg := &wireguard.ManagerConfig{
		InterfaceName:      "wg-test",
		ListenPort:         51820,
		Subnet:             "10.100.0.0/24",
		MTU:                1420,
		AutoDetectEndpoint: false,
	}

	if err := manager.Initialize(ctx, cfg); err != nil {
		t.Fatalf("failed to initialize: %v", err)
	}

	if err := manager.Start(ctx); err != nil {
		t.Fatalf("failed to start: %v", err)
	}
	defer manager.Stop()

	// Generate a peer key
	peerKeyPair, err := wireguard.GenerateKeyPair()
	if err != nil {
		t.Fatalf("failed to generate peer key: %v", err)
	}

	// Allocate IP for peer
	peerIP, err := manager.AllocateIP("peer1")
	if err != nil {
		t.Fatalf("failed to allocate IP: %v", err)
	}

	if peerIP != "10.100.0.2/24" {
		t.Errorf("peer IP = %s, want 10.100.0.2/24", peerIP)
	}

	// Add peer
	peer := &wireguard.Peer{
		PublicKey:           peerKeyPair.PublicKey,
		AllowedIPs:          []string{peerIP},
		Endpoint:            "192.168.1.100:51820",
		PersistentKeepalive: 25,
	}

	if err := manager.AddPeer(peer); err != nil {
		t.Fatalf("failed to add peer: %v", err)
	}

	// List peers
	peers, err := manager.ListPeers()
	if err != nil {
		t.Fatalf("failed to list peers: %v", err)
	}

	if len(peers) != 1 {
		t.Errorf("peer count = %d, want 1", len(peers))
	}

	// Remove peer
	if err := manager.RemovePeer(peerKeyPair.PublicKey); err != nil {
		t.Fatalf("failed to remove peer: %v", err)
	}

	peers, err = manager.ListPeers()
	if err != nil {
		t.Fatalf("failed to list peers after removal: %v", err)
	}

	if len(peers) != 0 {
		t.Errorf("peer count after removal = %d, want 0", len(peers))
	}

	t.Log("Peer management test passed!")
}

// TestWireGuardMockClusterFormation tests cluster formation using mock platform.
// This test does not require root privileges.
func TestWireGuardMockClusterFormation(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping WireGuard e2e test in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Create two WireGuard managers with mock platform
	mockPlatform1 := platform.NewMockPlatform()
	mockPlatform2 := platform.NewMockPlatform()

	manager1 := wireguard.NewManager(mockPlatform1)
	manager2 := wireguard.NewManager(mockPlatform2)

	// Initialize manager1 (leader)
	cfg1 := &wireguard.ManagerConfig{
		InterfaceName:       "wg-leader",
		ListenPort:          51820,
		Subnet:              "10.100.0.0/24",
		MTU:                 1420,
		PersistentKeepalive: 25,
		AutoDetectEndpoint:  false,
	}

	if err := manager1.Initialize(ctx, cfg1); err != nil {
		t.Fatalf("failed to initialize manager1: %v", err)
	}

	if err := manager1.Start(ctx); err != nil {
		t.Fatalf("failed to start manager1: %v", err)
	}
	defer manager1.Stop()

	leaderIP := manager1.GetLocalIP()
	leaderKeyPair := manager1.GetKeyPair()
	t.Logf("Leader: IP=%s, PublicKey=%s", leaderIP, leaderKeyPair.PublicKey)

	// Allocate IP for follower from leader's allocator
	followerIP, err := manager1.AllocateIP("follower")
	if err != nil {
		t.Fatalf("failed to allocate IP for follower: %v", err)
	}
	t.Logf("Allocated follower IP from leader: %s", followerIP)

	// Initialize manager2 (follower) with pre-allocated IP
	// In real scenario, follower receives its IP via the invite token
	followerKeyPair, err := wireguard.GenerateKeyPair()
	if err != nil {
		t.Fatalf("failed to generate follower key pair: %v", err)
	}

	// Create follower config with the allocated IP
	followerConfig := &wireguard.Config{
		PrivateKey: followerKeyPair.PrivateKey,
		PublicKey:  followerKeyPair.PublicKey,
		ListenPort: 51821,
		LocalIP:    followerIP,
		Subnet:     "10.100.0.0/24",
		MTU:        1420,
		Peers:      []*wireguard.Peer{},
	}

	cfg2 := &wireguard.ManagerConfig{
		InterfaceName:       "wg-follower",
		ListenPort:          51821, // Different port
		Subnet:              "10.100.0.0/24",
		MTU:                 1420,
		PersistentKeepalive: 25,
		AutoDetectEndpoint:  false,
	}

	if err := manager2.InitializeWithConfig(ctx, followerConfig, cfg2); err != nil {
		t.Fatalf("failed to initialize manager2 with config: %v", err)
	}

	if err := manager2.Start(ctx); err != nil {
		t.Fatalf("failed to start manager2: %v", err)
	}
	defer manager2.Stop()

	actualFollowerIP := manager2.GetLocalIP()
	t.Logf("Follower: IP=%s, PublicKey=%s", actualFollowerIP, followerKeyPair.PublicKey)

	// Verify different IPs
	if leaderIP == actualFollowerIP {
		t.Errorf("leader and follower should have different IPs: %s vs %s", leaderIP, actualFollowerIP)
	}

	// Add peer to leader (follower info)
	followerPeer := &wireguard.Peer{
		PublicKey:           followerKeyPair.PublicKey,
		AllowedIPs:          []string{followerIP},
		Endpoint:            "192.168.1.101:51821",
		PersistentKeepalive: 25,
	}

	if err := manager1.AddPeer(followerPeer); err != nil {
		t.Fatalf("failed to add follower peer to leader: %v", err)
	}

	// Add peer to follower (leader info)
	leaderPeer := &wireguard.Peer{
		PublicKey:           leaderKeyPair.PublicKey,
		AllowedIPs:          []string{leaderIP},
		Endpoint:            "192.168.1.100:51820",
		PersistentKeepalive: 25,
	}

	if err := manager2.AddPeer(leaderPeer); err != nil {
		t.Fatalf("failed to add leader peer to follower: %v", err)
	}

	// Update followerPeer to use actual follower IP
	followerPeer.AllowedIPs = []string{actualFollowerIP}

	// Verify peers
	leaderPeers, err := manager1.ListPeers()
	if err != nil {
		t.Fatalf("failed to list leader peers: %v", err)
	}
	if len(leaderPeers) != 1 {
		t.Errorf("leader should have 1 peer, got %d", len(leaderPeers))
	}

	followerPeers, err := manager2.ListPeers()
	if err != nil {
		t.Fatalf("failed to list follower peers: %v", err)
	}
	if len(followerPeers) != 1 {
		t.Errorf("follower should have 1 peer, got %d", len(followerPeers))
	}

	t.Log("Mock cluster formation test passed!")
}

// TestWireGuardKeyGeneration tests key pair generation.
func TestWireGuardKeyGeneration(t *testing.T) {
	keyPair1, err := wireguard.GenerateKeyPair()
	if err != nil {
		t.Fatalf("failed to generate key pair: %v", err)
	}

	if keyPair1.PrivateKey == "" {
		t.Error("private key should not be empty")
	}

	if keyPair1.PublicKey == "" {
		t.Error("public key should not be empty")
	}

	// Generate another key pair - should be different
	keyPair2, err := wireguard.GenerateKeyPair()
	if err != nil {
		t.Fatalf("failed to generate second key pair: %v", err)
	}

	if keyPair1.PrivateKey == keyPair2.PrivateKey {
		t.Error("two generated private keys should be different")
	}

	if keyPair1.PublicKey == keyPair2.PublicKey {
		t.Error("two generated public keys should be different")
	}

	// Verify public key derivation
	derivedPubKey, err := wireguard.DerivePublicKey(keyPair1.PrivateKey)
	if err != nil {
		t.Fatalf("failed to derive public key: %v", err)
	}

	if derivedPubKey != keyPair1.PublicKey {
		t.Errorf("derived public key = %s, want %s", derivedPubKey, keyPair1.PublicKey)
	}

	t.Log("Key generation test passed!")
}

// WireGuardTestCluster extends TestCluster with WireGuard support.
type WireGuardTestCluster struct {
	*TestCluster
	mockPlatform *platform.MockPlatform
}

// NewWireGuardTestCluster creates a new test cluster with WireGuard support.
func NewWireGuardTestCluster(t *testing.T, nodeCount int) *WireGuardTestCluster {
	return &WireGuardTestCluster{
		TestCluster:  NewTestCluster(t, nodeCount),
		mockPlatform: platform.NewMockPlatform(),
	}
}

// InitializeLeaderWithWireGuard initializes the leader with WireGuard enabled.
func (tc *WireGuardTestCluster) InitializeLeaderWithWireGuard(ctx context.Context, projectName string) (*application.App, *application.InitResult, error) {
	if len(tc.nodes) == 0 {
		app, err := tc.CreateNode("leader")
		if err != nil {
			return nil, nil, err
		}
		tc.nodes = append(tc.nodes, app)
	}

	leader := tc.nodes[0]

	opts := &application.InitializeOptions{
		ProjectName:     projectName,
		EnableWireGuard: true,
		WireGuardPort:   51820,
		Subnet:          "10.100.0.0/24",
	}

	result, err := leader.InitializeWithOptions(ctx, opts)
	if err != nil {
		return nil, nil, err
	}

	if err := leader.Start(); err != nil {
		return nil, nil, err
	}

	return leader, result, nil
}

// JoinNodeWithWireGuard creates and joins a new node with WireGuard.
func (tc *WireGuardTestCluster) JoinNodeWithWireGuard(ctx context.Context, name, token string) (*application.App, *application.JoinResult, error) {
	app, err := tc.CreateNode(name)
	if err != nil {
		return nil, nil, err
	}

	result, err := app.Join(ctx, token)
	if err != nil {
		return nil, nil, err
	}

	if err := app.Start(); err != nil {
		return nil, nil, err
	}

	return app, result, nil
}
