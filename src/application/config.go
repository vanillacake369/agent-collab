package application

import (
	"os"
	"path/filepath"
)

// Config는 애플리케이션 설정입니다.
type Config struct {
	ProjectName   string   `json:"project_name"`
	DataDir       string   `json:"data_dir"`
	ListenPort    int      `json:"listen_port"`
	ListenAddrs   []string `json:"listen_addrs,omitempty"`   // 실제 바인딩된 주소들
	Bootstrap     []string `json:"bootstrap"`                // Bootstrap peer 주소들
	BootstrapPeer string   `json:"bootstrap_peer,omitempty"` // Bootstrap peer ID

	// WireGuard VPN settings
	WireGuard *WireGuardConfig `json:"wireguard,omitempty"`
}

// WireGuardConfig holds WireGuard VPN configuration.
type WireGuardConfig struct {
	Enabled             bool   `json:"enabled"`
	ListenPort          int    `json:"listen_port"`
	Subnet              string `json:"subnet"`
	MTU                 int    `json:"mtu"`
	PersistentKeepalive int    `json:"persistent_keepalive"`
	InterfaceName       string `json:"interface_name"`
}

// DefaultWireGuardConfig returns default WireGuard configuration.
func DefaultWireGuardConfig() *WireGuardConfig {
	return &WireGuardConfig{
		Enabled:             false,
		ListenPort:          51820,
		Subnet:              "10.100.0.0/24",
		MTU:                 1420,
		PersistentKeepalive: 25,
		InterfaceName:       "wg-agent",
	}
}

// DefaultConfig는 기본 설정을 반환합니다.
func DefaultConfig() *Config {
	// Check environment variable first (for Docker/container use)
	dataDir := os.Getenv("AGENT_COLLAB_DATA_DIR")
	if dataDir == "" {
		home, _ := os.UserHomeDir()
		dataDir = filepath.Join(home, ".agent-collab")
	}

	return &Config{
		DataDir:    dataDir,
		ListenPort: 0, // 자동 할당
		Bootstrap:  []string{},
	}
}

// InitializeOptions holds options for cluster initialization.
type InitializeOptions struct {
	ProjectName     string
	EnableWireGuard bool
	WireGuardPort   int
	Subnet          string
}
