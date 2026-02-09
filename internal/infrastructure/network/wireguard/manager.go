package wireguard

import (
	"context"
	"fmt"
	"net"
	"sync"

	"agent-collab/internal/infrastructure/network/wireguard/platform"
)

// WireGuardManager implements the Manager interface.
type WireGuardManager struct {
	mu sync.RWMutex

	// Configuration
	managerConfig *ManagerConfig
	config        *Config
	keyPair       *KeyPair

	// Platform and device
	platform platform.Platform
	device   platform.Device

	// IP allocation
	ipAllocator *IPAllocator

	// State
	running    bool
	externalIP string
	localIP    string
	ctx        context.Context
	cancel     context.CancelFunc
}

// NewManager creates a new WireGuard manager.
func NewManager(p platform.Platform) *WireGuardManager {
	if p == nil {
		p = platform.GetPlatform()
	}
	return &WireGuardManager{
		platform: p,
	}
}

// Initialize sets up the WireGuard manager.
func (m *WireGuardManager) Initialize(ctx context.Context, cfg *ManagerConfig) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.running {
		return ErrAlreadyRunning
	}

	if cfg == nil {
		cfg = DefaultManagerConfig()
	}
	m.managerConfig = cfg

	// Check platform support
	if !m.platform.IsSupported() {
		return ErrNotSupported
	}

	// Generate key pair
	keyPair, err := GenerateKeyPair()
	if err != nil {
		return fmt.Errorf("failed to generate key pair: %w", err)
	}
	m.keyPair = keyPair

	// Create IP allocator
	allocator, err := NewIPAllocator(cfg.Subnet)
	if err != nil {
		return fmt.Errorf("failed to create IP allocator: %w", err)
	}
	m.ipAllocator = allocator

	// Allocate IP for self (first node gets .1)
	localIP, err := allocator.Allocate("self")
	if err != nil {
		return fmt.Errorf("failed to allocate local IP: %w", err)
	}
	m.localIP = localIP

	// Detect external IP
	if cfg.AutoDetectEndpoint {
		externalIP, err := m.platform.GetExternalIP()
		if err != nil {
			// Not fatal, just log warning
			fmt.Printf("Warning: could not detect external IP: %v\n", err)
		} else {
			m.externalIP = externalIP
		}
	}

	// Build config
	m.config = &Config{
		PrivateKey: keyPair.PrivateKey,
		PublicKey:  keyPair.PublicKey,
		ListenPort: cfg.ListenPort,
		LocalIP:    localIP,
		Subnet:     cfg.Subnet,
		MTU:        cfg.MTU,
		Peers:      make([]*Peer, 0),
	}

	return nil
}

// InitializeWithConfig sets up the manager with an existing config.
func (m *WireGuardManager) InitializeWithConfig(ctx context.Context, cfg *Config, mgrCfg *ManagerConfig) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.running {
		return ErrAlreadyRunning
	}

	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("invalid config: %w", err)
	}

	if mgrCfg == nil {
		mgrCfg = DefaultManagerConfig()
	}
	m.managerConfig = mgrCfg
	m.config = cfg.Clone()

	// Set key pair from config
	m.keyPair = &KeyPair{
		PrivateKey: cfg.PrivateKey,
		PublicKey:  cfg.PublicKey,
	}

	// Create IP allocator
	allocator, err := NewIPAllocator(cfg.Subnet)
	if err != nil {
		return fmt.Errorf("failed to create IP allocator: %w", err)
	}
	m.ipAllocator = allocator

	// Reserve local IP
	if err := allocator.AllocateSpecific("self", cfg.LocalIP); err != nil {
		return fmt.Errorf("failed to reserve local IP: %w", err)
	}
	m.localIP = cfg.LocalIP

	// Detect external IP
	if mgrCfg.AutoDetectEndpoint {
		externalIP, err := m.platform.GetExternalIP()
		if err == nil {
			m.externalIP = externalIP
		}
	}

	return nil
}

// Start starts the WireGuard interface.
func (m *WireGuardManager) Start(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.running {
		return ErrAlreadyRunning
	}

	if m.config == nil {
		return ErrNotInitialized
	}

	// Create interface
	device, err := m.platform.CreateInterface(m.managerConfig.InterfaceName)
	if err != nil {
		return fmt.Errorf("failed to create interface: %w", err)
	}
	m.device = device

	// Decode private key
	privateKey, err := DecodeKey(m.config.PrivateKey)
	if err != nil {
		m.device.Close()
		return fmt.Errorf("failed to decode private key: %w", err)
	}

	// Configure device
	deviceCfg := &platform.DeviceConfig{
		PrivateKey: privateKey,
		ListenPort: m.config.ListenPort,
		Peers:      make([]platform.PeerConfig, 0, len(m.config.Peers)),
	}

	for _, peer := range m.config.Peers {
		peerCfg, err := m.toPlatformPeerConfig(peer)
		if err != nil {
			m.device.Close()
			return fmt.Errorf("failed to convert peer config: %w", err)
		}
		deviceCfg.Peers = append(deviceCfg.Peers, *peerCfg)
	}

	if err := m.device.Configure(deviceCfg); err != nil {
		m.device.Close()
		return fmt.Errorf("failed to configure device: %w", err)
	}

	// Add IP address
	if err := m.device.AddIP(m.config.LocalIP); err != nil {
		m.device.Close()
		return fmt.Errorf("failed to add IP: %w", err)
	}

	// Bring interface up
	if err := m.device.Up(); err != nil {
		m.device.Close()
		return fmt.Errorf("failed to bring interface up: %w", err)
	}

	// Set up context for lifecycle management
	m.ctx, m.cancel = context.WithCancel(ctx)
	m.running = true

	return nil
}

// Stop stops the WireGuard interface.
func (m *WireGuardManager) Stop() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.running {
		return nil
	}

	if m.cancel != nil {
		m.cancel()
	}

	if m.device != nil {
		if err := m.device.Down(); err != nil {
			fmt.Printf("Warning: failed to bring interface down: %v\n", err)
		}
		if err := m.device.Close(); err != nil {
			fmt.Printf("Warning: failed to close device: %v\n", err)
		}
		m.device = nil
	}

	// Delete the interface
	if err := m.platform.DeleteInterface(m.managerConfig.InterfaceName); err != nil {
		fmt.Printf("Warning: failed to delete interface: %v\n", err)
	}

	m.running = false
	return nil
}

// AddPeer adds a new peer.
func (m *WireGuardManager) AddPeer(peer *Peer) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if peer == nil {
		return fmt.Errorf("peer cannot be nil")
	}

	// Check if peer already exists
	for _, p := range m.config.Peers {
		if p.PublicKey == peer.PublicKey {
			return ErrPeerExists
		}
	}

	// Add to config
	m.config.Peers = append(m.config.Peers, peer.Clone())

	// If running, update device
	if m.running && m.device != nil {
		peerCfg, err := m.toPlatformPeerConfig(peer)
		if err != nil {
			return fmt.Errorf("failed to convert peer config: %w", err)
		}

		deviceCfg := &platform.DeviceConfig{
			Peers: []platform.PeerConfig{*peerCfg},
		}

		// Get current private key for the update
		privateKey, err := DecodeKey(m.config.PrivateKey)
		if err != nil {
			return fmt.Errorf("failed to decode private key: %w", err)
		}
		deviceCfg.PrivateKey = privateKey
		deviceCfg.ListenPort = m.config.ListenPort

		if err := m.device.Configure(deviceCfg); err != nil {
			// Rollback config change
			m.config.Peers = m.config.Peers[:len(m.config.Peers)-1]
			return fmt.Errorf("failed to add peer to device: %w", err)
		}
	}

	return nil
}

// RemovePeer removes a peer by public key.
func (m *WireGuardManager) RemovePeer(publicKey string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Find and remove from config
	found := false
	for i, p := range m.config.Peers {
		if p.PublicKey == publicKey {
			m.config.Peers = append(m.config.Peers[:i], m.config.Peers[i+1:]...)
			found = true
			break
		}
	}

	if !found {
		return ErrPeerNotFound
	}

	// If running, update device
	if m.running && m.device != nil {
		pubKey, err := DecodeKey(publicKey)
		if err != nil {
			return fmt.Errorf("invalid public key: %w", err)
		}

		deviceCfg := &platform.DeviceConfig{
			Peers: []platform.PeerConfig{
				{
					PublicKey: pubKey,
					RemoveMe:  true,
				},
			},
		}

		privateKey, err := DecodeKey(m.config.PrivateKey)
		if err != nil {
			return fmt.Errorf("failed to decode private key: %w", err)
		}
		deviceCfg.PrivateKey = privateKey
		deviceCfg.ListenPort = m.config.ListenPort

		if err := m.device.Configure(deviceCfg); err != nil {
			return fmt.Errorf("failed to remove peer from device: %w", err)
		}
	}

	return nil
}

// AllocateIP allocates an IP for a peer.
func (m *WireGuardManager) AllocateIP(peerID string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.ipAllocator == nil {
		return "", ErrNotInitialized
	}

	return m.ipAllocator.Allocate(peerID)
}

// ReleaseIP releases an IP for a peer.
func (m *WireGuardManager) ReleaseIP(peerID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.ipAllocator == nil {
		return ErrNotInitialized
	}

	return m.ipAllocator.Release(peerID)
}

// GetConfig returns the current configuration.
func (m *WireGuardManager) GetConfig() *Config {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.config == nil {
		return nil
	}
	return m.config.Clone()
}

// GetStatus returns the current status.
func (m *WireGuardManager) GetStatus() (*InterfaceStatus, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if !m.running || m.device == nil {
		return &InterfaceStatus{
			Name: m.managerConfig.InterfaceName,
			Up:   false,
		}, nil
	}

	status := &InterfaceStatus{
		Name:       m.managerConfig.InterfaceName,
		PublicKey:  m.config.PublicKey,
		ListenPort: m.config.ListenPort,
		LocalIP:    m.localIP,
		Up:         m.device.IsUp(),
		Peers:      make([]PeerStatus, 0, len(m.config.Peers)),
	}

	// Get peer status from device
	deviceCfg, err := m.device.GetConfig()
	if err == nil {
		for _, peer := range deviceCfg.Peers {
			peerStatus := PeerStatus{
				PublicKey:           EncodeKey(peer.PublicKey),
				PersistentKeepalive: peer.PersistentKeepaliveInterval,
				AllowedIPs:          make([]string, 0, len(peer.AllowedIPs)),
			}
			if peer.Endpoint != nil {
				peerStatus.Endpoint = peer.Endpoint.String()
			}
			for _, ip := range peer.AllowedIPs {
				peerStatus.AllowedIPs = append(peerStatus.AllowedIPs, ip.String())
			}
			status.Peers = append(status.Peers, peerStatus)
		}
	}

	return status, nil
}

// GetKeyPair returns the WireGuard key pair.
func (m *WireGuardManager) GetKeyPair() *KeyPair {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.keyPair == nil {
		return nil
	}
	return &KeyPair{
		PrivateKey: m.keyPair.PrivateKey,
		PublicKey:  m.keyPair.PublicKey,
	}
}

// GetLocalIP returns the local VPN IP.
func (m *WireGuardManager) GetLocalIP() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.localIP
}

// GetEndpoint returns the external endpoint.
func (m *WireGuardManager) GetEndpoint() string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.externalIP == "" {
		return ""
	}
	return fmt.Sprintf("%s:%d", m.externalIP, m.config.ListenPort)
}

// IsRunning returns true if the manager is running.
func (m *WireGuardManager) IsRunning() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.running
}

// Close shuts down the manager.
func (m *WireGuardManager) Close() error {
	return m.Stop()
}

// toPlatformPeerConfig converts a Peer to platform.PeerConfig.
func (m *WireGuardManager) toPlatformPeerConfig(peer *Peer) (*platform.PeerConfig, error) {
	publicKey, err := DecodeKey(peer.PublicKey)
	if err != nil {
		return nil, fmt.Errorf("invalid public key: %w", err)
	}

	cfg := &platform.PeerConfig{
		PublicKey:                   publicKey,
		AllowedIPs:                  make([]net.IPNet, 0, len(peer.AllowedIPs)),
		PersistentKeepaliveInterval: peer.PersistentKeepalive,
		ReplaceAllowedIPs:           true,
	}

	// Parse endpoint
	if peer.Endpoint != "" {
		endpoint, err := net.ResolveUDPAddr("udp", peer.Endpoint)
		if err != nil {
			return nil, fmt.Errorf("invalid endpoint: %w", err)
		}
		cfg.Endpoint = endpoint
	}

	// Parse allowed IPs
	for _, allowedIP := range peer.AllowedIPs {
		_, ipNet, err := net.ParseCIDR(allowedIP)
		if err != nil {
			return nil, fmt.Errorf("invalid allowed IP %s: %w", allowedIP, err)
		}
		cfg.AllowedIPs = append(cfg.AllowedIPs, *ipNet)
	}

	return cfg, nil
}

// Ensure WireGuardManager implements Manager
var _ Manager = (*WireGuardManager)(nil)
