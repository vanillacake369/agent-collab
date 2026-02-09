package wireguard

import (
	"os"
	"path/filepath"
	"testing"
)

func TestConfigValidate(t *testing.T) {
	// Generate a valid key pair for testing
	kp, err := GenerateKeyPair()
	if err != nil {
		t.Fatalf("GenerateKeyPair() error = %v", err)
	}

	tests := []struct {
		name    string
		config  *Config
		wantErr bool
	}{
		{
			name: "valid config",
			config: &Config{
				PrivateKey: kp.PrivateKey,
				PublicKey:  kp.PublicKey,
				ListenPort: 51820,
				LocalIP:    "10.100.0.1/24",
				Subnet:     "10.100.0.0/24",
				MTU:        1420,
			},
			wantErr: false,
		},
		{
			name: "missing private key",
			config: &Config{
				PublicKey:  kp.PublicKey,
				ListenPort: 51820,
				LocalIP:    "10.100.0.1/24",
				Subnet:     "10.100.0.0/24",
				MTU:        1420,
			},
			wantErr: true,
		},
		{
			name: "missing public key",
			config: &Config{
				PrivateKey: kp.PrivateKey,
				ListenPort: 51820,
				LocalIP:    "10.100.0.1/24",
				Subnet:     "10.100.0.0/24",
				MTU:        1420,
			},
			wantErr: true,
		},
		{
			name: "invalid port negative",
			config: &Config{
				PrivateKey: kp.PrivateKey,
				PublicKey:  kp.PublicKey,
				ListenPort: -1,
				LocalIP:    "10.100.0.1/24",
				Subnet:     "10.100.0.0/24",
				MTU:        1420,
			},
			wantErr: true,
		},
		{
			name: "invalid port too high",
			config: &Config{
				PrivateKey: kp.PrivateKey,
				PublicKey:  kp.PublicKey,
				ListenPort: 70000,
				LocalIP:    "10.100.0.1/24",
				Subnet:     "10.100.0.0/24",
				MTU:        1420,
			},
			wantErr: true,
		},
		{
			name: "missing local IP",
			config: &Config{
				PrivateKey: kp.PrivateKey,
				PublicKey:  kp.PublicKey,
				ListenPort: 51820,
				Subnet:     "10.100.0.0/24",
				MTU:        1420,
			},
			wantErr: true,
		},
		{
			name: "invalid local IP",
			config: &Config{
				PrivateKey: kp.PrivateKey,
				PublicKey:  kp.PublicKey,
				ListenPort: 51820,
				LocalIP:    "invalid-ip",
				Subnet:     "10.100.0.0/24",
				MTU:        1420,
			},
			wantErr: true,
		},
		{
			name: "invalid subnet",
			config: &Config{
				PrivateKey: kp.PrivateKey,
				PublicKey:  kp.PublicKey,
				ListenPort: 51820,
				LocalIP:    "10.100.0.1/24",
				Subnet:     "invalid-subnet",
				MTU:        1420,
			},
			wantErr: true,
		},
		{
			name: "MTU too low",
			config: &Config{
				PrivateKey: kp.PrivateKey,
				PublicKey:  kp.PublicKey,
				ListenPort: 51820,
				LocalIP:    "10.100.0.1/24",
				Subnet:     "10.100.0.0/24",
				MTU:        100,
			},
			wantErr: true,
		},
		{
			name: "MTU too high",
			config: &Config{
				PrivateKey: kp.PrivateKey,
				PublicKey:  kp.PublicKey,
				ListenPort: 51820,
				LocalIP:    "10.100.0.1/24",
				Subnet:     "10.100.0.0/24",
				MTU:        100000,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestConfigClone(t *testing.T) {
	kp, err := GenerateKeyPair()
	if err != nil {
		t.Fatalf("GenerateKeyPair() error = %v", err)
	}

	original := &Config{
		PrivateKey: kp.PrivateKey,
		PublicKey:  kp.PublicKey,
		ListenPort: 51820,
		LocalIP:    "10.100.0.1/24",
		Subnet:     "10.100.0.0/24",
		MTU:        1420,
		Peers: []*Peer{
			{
				PublicKey:           "peer-public-key",
				AllowedIPs:          []string{"10.100.0.2/32"},
				Endpoint:            "1.2.3.4:51820",
				PersistentKeepalive: 25,
			},
		},
	}

	clone := original.Clone()

	// Verify values are equal
	if clone.PrivateKey != original.PrivateKey {
		t.Error("PrivateKey not cloned correctly")
	}
	if clone.PublicKey != original.PublicKey {
		t.Error("PublicKey not cloned correctly")
	}
	if clone.ListenPort != original.ListenPort {
		t.Error("ListenPort not cloned correctly")
	}
	if clone.LocalIP != original.LocalIP {
		t.Error("LocalIP not cloned correctly")
	}
	if clone.MTU != original.MTU {
		t.Error("MTU not cloned correctly")
	}
	if len(clone.Peers) != len(original.Peers) {
		t.Error("Peers not cloned correctly")
	}

	// Verify deep copy (modifying clone doesn't affect original)
	clone.ListenPort = 12345
	if original.ListenPort == clone.ListenPort {
		t.Error("Clone is not a deep copy (ListenPort)")
	}

	clone.Peers[0].Endpoint = "5.6.7.8:51820"
	if original.Peers[0].Endpoint == clone.Peers[0].Endpoint {
		t.Error("Clone is not a deep copy (Peer.Endpoint)")
	}
}

func TestConfigFileSaveLoad(t *testing.T) {
	kp, err := GenerateKeyPair()
	if err != nil {
		t.Fatalf("GenerateKeyPair() error = %v", err)
	}

	configFile := &ConfigFile{
		Interface: InterfaceConfig{
			PrivateKey: kp.PrivateKey,
			ListenPort: 51820,
			Address:    "10.100.0.1/24",
			MTU:        1420,
		},
		Peers: []PeerConfig{
			{
				PublicKey:           "peer-public-key",
				AllowedIPs:          []string{"10.100.0.2/32"},
				Endpoint:            "1.2.3.4:51820",
				PersistentKeepalive: 25,
			},
		},
	}

	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "wireguard-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	configPath := filepath.Join(tmpDir, "wg.json")

	// Save config
	err = SaveConfigFile(configFile, configPath)
	if err != nil {
		t.Fatalf("SaveConfigFile() error = %v", err)
	}

	// Load config
	loaded, err := LoadConfigFile(configPath)
	if err != nil {
		t.Fatalf("LoadConfigFile() error = %v", err)
	}

	// Verify loaded config
	if loaded.Interface.PrivateKey != configFile.Interface.PrivateKey {
		t.Error("PrivateKey not loaded correctly")
	}
	if loaded.Interface.ListenPort != configFile.Interface.ListenPort {
		t.Error("ListenPort not loaded correctly")
	}
	if loaded.Interface.Address != configFile.Interface.Address {
		t.Error("Address not loaded correctly")
	}
	if len(loaded.Peers) != len(configFile.Peers) {
		t.Error("Peers not loaded correctly")
	}
	if loaded.Peers[0].PublicKey != configFile.Peers[0].PublicKey {
		t.Error("Peer PublicKey not loaded correctly")
	}
}

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.ListenPort != 51820 {
		t.Errorf("DefaultConfig().ListenPort = %d, want 51820", cfg.ListenPort)
	}
	if cfg.Subnet != "10.100.0.0/24" {
		t.Errorf("DefaultConfig().Subnet = %s, want 10.100.0.0/24", cfg.Subnet)
	}
	if cfg.MTU != 1420 {
		t.Errorf("DefaultConfig().MTU = %d, want 1420", cfg.MTU)
	}
	if cfg.Peers == nil {
		t.Error("DefaultConfig().Peers is nil, want empty slice")
	}
}

func TestDefaultManagerConfig(t *testing.T) {
	cfg := DefaultManagerConfig()

	if cfg.InterfaceName != "wg-agent" {
		t.Errorf("DefaultManagerConfig().InterfaceName = %s, want wg-agent", cfg.InterfaceName)
	}
	if cfg.ListenPort != 51820 {
		t.Errorf("DefaultManagerConfig().ListenPort = %d, want 51820", cfg.ListenPort)
	}
	if cfg.Subnet != "10.100.0.0/24" {
		t.Errorf("DefaultManagerConfig().Subnet = %s, want 10.100.0.0/24", cfg.Subnet)
	}
	if cfg.MTU != 1420 {
		t.Errorf("DefaultManagerConfig().MTU = %d, want 1420", cfg.MTU)
	}
	if cfg.PersistentKeepalive != 25 {
		t.Errorf("DefaultManagerConfig().PersistentKeepalive = %d, want 25", cfg.PersistentKeepalive)
	}
	if !cfg.AutoDetectEndpoint {
		t.Error("DefaultManagerConfig().AutoDetectEndpoint = false, want true")
	}
}
