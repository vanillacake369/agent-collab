package wireguard

import (
	"context"
	"testing"

	"agent-collab/src/infrastructure/network/wireguard/platform"
)

func TestNewManager(t *testing.T) {
	p := platform.NewMockPlatform()
	mgr := NewManager(p)

	if mgr == nil {
		t.Fatal("NewManager() returned nil")
	}
}

func TestManagerInitialize(t *testing.T) {
	p := platform.NewMockPlatform()
	mgr := NewManager(p)

	ctx := context.Background()
	cfg := &ManagerConfig{
		InterfaceName:       "wg-test",
		ListenPort:          51820,
		Subnet:              "10.100.0.0/24",
		MTU:                 1420,
		PersistentKeepalive: 25,
		AutoDetectEndpoint:  true,
	}

	err := mgr.Initialize(ctx, cfg)
	if err != nil {
		t.Fatalf("Initialize() error = %v", err)
	}

	// Should have generated key pair
	kp := mgr.GetKeyPair()
	if kp.PrivateKey == "" {
		t.Error("GetKeyPair().PrivateKey is empty after Initialize()")
	}
	if kp.PublicKey == "" {
		t.Error("GetKeyPair().PublicKey is empty after Initialize()")
	}

	// Should have allocated local IP
	localIP := mgr.GetLocalIP()
	if localIP == "" {
		t.Error("GetLocalIP() is empty after Initialize()")
	}
	if localIP != "10.100.0.1/24" {
		t.Errorf("GetLocalIP() = %s, want 10.100.0.1/24", localIP)
	}
}

func TestManagerStartStop(t *testing.T) {
	p := platform.NewMockPlatform()
	mgr := NewManager(p)

	ctx := context.Background()
	cfg := &ManagerConfig{
		InterfaceName:       "wg-test",
		ListenPort:          51820,
		Subnet:              "10.100.0.0/24",
		MTU:                 1420,
		PersistentKeepalive: 25,
	}

	err := mgr.Initialize(ctx, cfg)
	if err != nil {
		t.Fatalf("Initialize() error = %v", err)
	}

	// Start
	err = mgr.Start(ctx)
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	// Check status
	status, err := mgr.GetStatus()
	if err != nil {
		t.Fatalf("GetStatus() error = %v", err)
	}
	if !status.Up {
		t.Error("GetStatus().Up = false after Start()")
	}

	// Stop
	err = mgr.Stop()
	if err != nil {
		t.Fatalf("Stop() error = %v", err)
	}
}

func TestManagerAddRemovePeer(t *testing.T) {
	p := platform.NewMockPlatform()
	mgr := NewManager(p)

	ctx := context.Background()
	cfg := &ManagerConfig{
		InterfaceName:       "wg-test",
		ListenPort:          51820,
		Subnet:              "10.100.0.0/24",
		MTU:                 1420,
		PersistentKeepalive: 25,
	}

	err := mgr.Initialize(ctx, cfg)
	if err != nil {
		t.Fatalf("Initialize() error = %v", err)
	}

	err = mgr.Start(ctx)
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	defer mgr.Stop()

	// Generate peer key
	peerKP, err := GenerateKeyPair()
	if err != nil {
		t.Fatalf("GenerateKeyPair() error = %v", err)
	}

	peer := &Peer{
		PublicKey:           peerKP.PublicKey,
		AllowedIPs:          []string{"10.100.0.2/32"},
		Endpoint:            "1.2.3.4:51820",
		PersistentKeepalive: 25,
	}

	// Add peer
	err = mgr.AddPeer(peer)
	if err != nil {
		t.Fatalf("AddPeer() error = %v", err)
	}

	// List peers
	peers, err := mgr.ListPeers()
	if err != nil {
		t.Fatalf("ListPeers() error = %v", err)
	}
	if len(peers) != 1 {
		t.Errorf("ListPeers() returned %d peers, want 1", len(peers))
	}

	// Remove peer
	err = mgr.RemovePeer(peerKP.PublicKey)
	if err != nil {
		t.Fatalf("RemovePeer() error = %v", err)
	}

	// Verify removed
	peers, err = mgr.ListPeers()
	if err != nil {
		t.Fatalf("ListPeers() error = %v", err)
	}
	if len(peers) != 0 {
		t.Errorf("ListPeers() returned %d peers after removal, want 0", len(peers))
	}
}

func TestManagerAllocateIP(t *testing.T) {
	p := platform.NewMockPlatform()
	mgr := NewManager(p)

	ctx := context.Background()
	cfg := &ManagerConfig{
		InterfaceName: "wg-test",
		ListenPort:    51820,
		Subnet:        "10.100.0.0/24",
		MTU:           1420,
	}

	err := mgr.Initialize(ctx, cfg)
	if err != nil {
		t.Fatalf("Initialize() error = %v", err)
	}

	// Manager gets first IP (.1)
	localIP := mgr.GetLocalIP()
	if localIP != "10.100.0.1/24" {
		t.Errorf("Manager local IP = %s, want 10.100.0.1/24", localIP)
	}

	// Allocate for peer
	peerIP, err := mgr.AllocateIP("peer1")
	if err != nil {
		t.Fatalf("AllocateIP() error = %v", err)
	}
	if peerIP != "10.100.0.2/24" {
		t.Errorf("AllocateIP(peer1) = %s, want 10.100.0.2/24", peerIP)
	}

	// Allocate for another peer
	peerIP2, err := mgr.AllocateIP("peer2")
	if err != nil {
		t.Fatalf("AllocateIP() error = %v", err)
	}
	if peerIP2 != "10.100.0.3/24" {
		t.Errorf("AllocateIP(peer2) = %s, want 10.100.0.3/24", peerIP2)
	}
}

func TestManagerGetConfig(t *testing.T) {
	p := platform.NewMockPlatform()
	mgr := NewManager(p)

	ctx := context.Background()
	cfg := &ManagerConfig{
		InterfaceName: "wg-test",
		ListenPort:    51820,
		Subnet:        "10.100.0.0/24",
		MTU:           1420,
	}

	err := mgr.Initialize(ctx, cfg)
	if err != nil {
		t.Fatalf("Initialize() error = %v", err)
	}

	config := mgr.GetConfig()
	if config == nil {
		t.Fatal("GetConfig() returned nil")
	}
	if config.ListenPort != 51820 {
		t.Errorf("GetConfig().ListenPort = %d, want 51820", config.ListenPort)
	}
	if config.Subnet != "10.100.0.0/24" {
		t.Errorf("GetConfig().Subnet = %s, want 10.100.0.0/24", config.Subnet)
	}
}

func TestManagerGetEndpoint(t *testing.T) {
	p := platform.NewMockPlatform()
	mgr := NewManager(p)

	ctx := context.Background()
	cfg := &ManagerConfig{
		InterfaceName:      "wg-test",
		ListenPort:         51820,
		Subnet:             "10.100.0.0/24",
		MTU:                1420,
		AutoDetectEndpoint: true,
	}

	err := mgr.Initialize(ctx, cfg)
	if err != nil {
		t.Fatalf("Initialize() error = %v", err)
	}

	endpoint := mgr.GetEndpoint()
	if endpoint == "" {
		t.Error("GetEndpoint() is empty")
	}
	// Should be in IP:port format
	if len(endpoint) < 7 { // minimum "x.x.x.x:p"
		t.Errorf("GetEndpoint() = %s, expected IP:port format", endpoint)
	}
}

func TestManagerWithoutInitialize(t *testing.T) {
	p := platform.NewMockPlatform()
	mgr := NewManager(p)

	ctx := context.Background()

	// Start without Initialize should fail
	err := mgr.Start(ctx)
	if err == nil {
		t.Error("Start() should fail without Initialize()")
	}

	// Other operations should also fail or return empty values
	if mgr.GetLocalIP() != "" {
		t.Error("GetLocalIP() should be empty without Initialize()")
	}

	if mgr.GetConfig() != nil {
		t.Error("GetConfig() should be nil without Initialize()")
	}
}
